package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
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
	commands, ok := internal.ParseStringSlice(req.Params.Arguments["commands"])
	if !ok || len(commands) == 0 {
		return mcp.NewToolResultError("missing required parameter: commands"), nil
	}

	cwd, _ := req.Params.Arguments["cwd"].(string)
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
				hints = append(hints, "cwd parsed from 'cd' prefix; pass cwd= directly instead")
			}
		}

		parts := splitOnAndAnd(cmd)
		if len(parts) > 1 {
			hints = append(hints, "auto-split from && chain; commands run independently (no short-circuit on failure)")
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

// splitOnAndAnd splits a shell command on unquoted " && " sequences.
// Single-quoted and double-quoted regions are respected; backslash escapes inside
// double-quoted regions are honoured. $(...) subshells and backtick subshells are
// treated as opaque — && inside them is never a split point.
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
