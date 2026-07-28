package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal"
	"github.com/rthomazel/mcp/bench/internal/stats"
	"github.com/rthomazel/mcp/bench/internal/xml"
)

type commandResult struct {
	Command    string
	Stdout     string
	Stderr     string
	ExitCode   int
	Duration   string
	DurationMS int64
	TimedOut   bool
	Hint       string
	err        string
}

type expandedCmd struct {
	cmd  string
	cwd  string
	hint string
}

func (h *Handler) HandleShell(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	commands, ok := internal.ParseStringSlice(args["commands"])
	if !ok || len(commands) == 0 {
		return mcp.NewToolResultError("missing required parameter: commands"), nil
	}

	cwd, _ := args["cwd"].(string)
	if cwd == "" {
		cwd = "/"
	}

	expanded := expandCommands(commands, cwd)
	results := make([]*commandResult, len(expanded))

	for i, ec := range expanded {
		start := time.Now()
		r := runCommand(ctx, h.cfg, ec.cmd, ec.cwd)
		r.Command = ec.cmd
		r.Hint = ec.hint
		results[i] = r

		errorKind := ""
		switch {
		case r.TimedOut:
			errorKind = "timeout"
		case r.err != "":
			errorKind = "start_failed"
		}
		exitCode := r.ExitCode
		h.record(stats.ToolCall{
			Tool:      "shell",
			StartedAt: start,
			Duration:  time.Duration(r.DurationMS) * time.Millisecond,
			ErrorKind: errorKind,
			Command:   ec.cmd,
			ExitCode:  &exitCode,
			TimedOut:  r.TimedOut,
			CWD:       ec.cwd,
		})

		if r.err != "" {
			return mcp.NewToolResultError(r.err), nil
		}
	}

	multi := len(results) > 1
	return mcp.NewToolResultText(formatCommandResults(results, multi)), nil
}

// expandCommands pre-processes commands before execution. This is a behavioral change:
// && chains are executed as independent commands rather than short-circuiting on failure.
// 1. Parses a leading "cd PATH &&" prefix and applies it as the effective cwd when no explicit cwd was given.
// 2. Splits unquoted " && " chains into independent commands, each with its own result entry.
func expandCommands(commands []string, cwd string) []expandedCmd {
	defaultCWD := cwd == "/"
	var out []expandedCmd

	for _, cmd := range commands {
		effectiveCWD := cwd
		var hints []string

		if defaultCWD {
			if parsed, remainder := parseCWD(cmd); parsed != "" {
				effectiveCWD = parsed
				cmd = remainder
				hints = append(hints, internal.HintCWDParsed)
			}
		}

		parts := splitOnAndAnd(cmd)
		if len(parts) > 1 {
			hints = append(hints, internal.HintAndAndSplit)
		}

		hint := strings.Join(hints, "; ")
		for _, part := range parts {
			out = append(out, expandedCmd{cmd: part, cwd: effectiveCWD, hint: hint})
		}
	}

	return out
}

// parseCWD extracts a leading "cd PATH &&" prefix, returning the path and the
// remaining command. Returns "", cmd unchanged when the pattern is absent or the path is
// complex (contains quotes or whitespace).
func parseCWD(cmd string) (path, remainder string) {
	rest, ok := strings.CutPrefix(cmd, "cd ")
	if !ok {
		return "", cmd
	}

	idx := strings.Index(rest, " && ")
	if idx < 0 {
		return "", cmd
	}

	path = rest[:idx]
	if path == "" || strings.ContainsAny(path, "\"' \t") {
		return "", cmd
	}

	return path, rest[idx+4:]
}

// reHeredocIntro matches a heredoc introducer "<<" or "<<-" followed by an optional
// quoted or backslash-escaped delimiter.
var reHeredocIntro = regexp.MustCompile(`^<<(-?)[ \t]*(?:'([^']*)'|"([^"]*)"|\\?([A-Za-z_][A-Za-z0-9_]*))`)

// extractHeredoc attempts to parse a heredoc introducer starting at cmd[start:].
// On success it returns the entire heredoc — introducer line remainder, body, and
// terminator line — as a single opaque string, plus the index immediately following it.
// The body is copied verbatim without further quote or && interpretation: heredoc
// content is literal text as far as && splitting is concerned, and body lines may
// legitimately contain unquoted " && " (e.g. embedded shell scripts). An unterminated
// heredoc consumes the remainder of the command rather than risk a bad split.
func extractHeredoc(cmd string, start int) (consumed string, end int, ok bool) {
	// m[0] m[1] -> whole match
	// m[2] m[3] -> group 1 dash flag
	// m[4] m[5] -> group 2 single-quoted delimiter
	// m[6] m[7] -> group 3 double-quoted delimiter
	// m[8] m[9] -> group 4 bare (optionally backslash-escaped) delimiter
	m := reHeredocIntro.FindStringSubmatchIndex(cmd[start:])
	if m == nil {
		return "", 0, false
	}

	// <<- allows the terminating delimiter to be indented with tabs.
	dash := cmd[start+m[2]:start+m[3]] == "-"

	var delim string
	switch {
	case m[4] != -1:
		delim = cmd[start+m[4] : start+m[5]]

	case m[6] != -1:
		delim = cmd[start+m[6] : start+m[7]]

	default:
		delim = cmd[start+m[8] : start+m[9]]
	}

	lineEnd := strings.IndexByte(cmd[start:], '\n')
	if lineEnd < 0 {
		// malformed heredoc introducer: consume the remainder as opaque text.
		return cmd[start:], len(cmd), true
	}

	for pos := start + lineEnd + 1; pos <= len(cmd); {
		nl := strings.IndexByte(cmd[pos:], '\n')

		line := cmd[pos:]
		lineEndAbs := len(cmd)
		if nl >= 0 {
			line = cmd[pos : pos+nl]
			lineEndAbs = pos + nl
		}

		if dash {
			line = strings.TrimLeft(line, "\t")
		}

		if line == delim {
			if nl >= 0 {
				// Return the complete heredoc, including the terminating line.
				return cmd[start : lineEndAbs+1], lineEndAbs + 1, true
			}

			// Return the complete heredoc ending at EOF.
			return cmd[start:], len(cmd), true
		}

		if nl < 0 {
			break
		}

		pos = lineEndAbs + 1
	}

	// malformed heredoc ending: consume the remainder as opaque text.
	return cmd[start:], len(cmd), true
}

// splitOnAndAnd splits a shell command on unquoted " && " sequences.
// Single-quoted and double-quoted regions are respected; backslash escapes inside
// double-quoted regions are honoured. $(...) subshells and backtick subshells are
// treated as opaque — && inside them is never a split point. Heredoc bodies
// ("<<EOF" ... "EOF") are also treated as opaque, since they commonly embed literal
// " && " (e.g. shell scripts, test fixtures) that must not be treated as a split point.
// Returns the original string as a single-element slice when no unquoted " && " is found.
func splitOnAndAnd(cmd string) []string {
	var parts []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	subshellDepth := 0
	inBacktick := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case inSingle:
			cur.WriteByte(c)
			if c == '\'' {
				inSingle = false
			}
		case inDouble && c == '\\' && i+1 < len(cmd):
			cur.WriteByte(c)
			i++
			cur.WriteByte(cmd[i])
		case inDouble:
			cur.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
			cur.WriteByte(c)
		case c == '"':
			inDouble = true
			cur.WriteByte(c)
		case c == '`':
			inBacktick = !inBacktick
			cur.WriteByte(c)
		case c == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
			subshellDepth++
			cur.WriteByte(c)
			i++
			cur.WriteByte(cmd[i])
		case c == ')' && subshellDepth > 0:
			subshellDepth--
			cur.WriteByte(c)
		case subshellDepth == 0 && !inBacktick && c == '<' && i+1 < len(cmd) && cmd[i+1] == '<':
			if heredoc, end, ok := extractHeredoc(cmd, i); ok {
				cur.WriteString(heredoc)
				i = end - 1 // loop i++ lands on first char after the heredoc
			} else {
				cur.WriteByte(c)
			}
		case subshellDepth == 0 && !inBacktick && strings.HasPrefix(cmd[i:], " && "):
			parts = append(parts, strings.TrimSpace(cur.String()))
			cur.Reset()
			i += 3 // skip " && "; loop i++ lands on char after trailing space
		default:
			cur.WriteByte(c)
		}
	}

	if s := strings.TrimSpace(cur.String()); s != "" {
		parts = append(parts, s)
	}

	if len(parts) == 0 {
		return []string{cmd}
	}

	return parts
}

func formatCommandResults(results []*commandResult, multi bool) string {
	var b xml.Builder

	for i, r := range results {
		if multi {
			b.OpenTag("command", "index", strconv.Itoa(i))
		}

		b.OpenTag("metadata")
		if multi {
			b.WriteString("command: " + r.Command + "\n")
		}

		b.WriteString("exit: " + strconv.Itoa(r.ExitCode) + "\n")
		b.WriteString("duration: " + r.Duration + "\n")
		if r.Hint != "" {
			b.WriteString("hint: " + r.Hint + "\n")
		}
		b.CloseTag("metadata", true)
		b.Tag("stdout", r.Stdout, true)
		b.Tag("stderr", r.Stderr, false)

		if multi {
			b.CloseTag("command", true)
		}
	}

	return b.String()
}

func runCommand(ctx context.Context, cfg *internal.Config, command, cwd string) *commandResult {
	start := time.Now()
	slog.Info("exec start", "cmd", command, "cwd", cwd)

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = cwd

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)
	exitCode := 0
	timedOut := ctx.Err() != nil

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			slog.Error("exec failed to start", "cmd", command, "err", err)
			return &commandResult{
				Duration:   duration.Round(1_000_000).String(),
				DurationMS: duration.Milliseconds(),
				TimedOut:   timedOut,
				ExitCode:   -1,
				err:        fmt.Sprintf("could not start process: %v", err),
			}
		}
	}

	slog.Info("exec done", "cmd", command, "exit_code", exitCode, "duration", duration.Round(time.Millisecond))

	return &commandResult{
		Stdout:     strings.TrimRight(stdout.String(), "\n"),
		Stderr:     strings.TrimRight(stderr.String(), "\n"),
		ExitCode:   exitCode,
		Duration:   duration.Round(1_000_000).String(),
		DurationMS: duration.Milliseconds(),
		TimedOut:   timedOut,
	}
}
