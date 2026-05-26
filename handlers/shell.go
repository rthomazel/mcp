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
	"github.com/rthomazel/bench-mcp/internal"
	"github.com/rthomazel/bench-mcp/internal/stats"
	"github.com/rthomazel/bench-mcp/internal/xml"
)

type commandResult struct {
	Command    string
	Stdout     string
	Stderr     string
	ExitCode   int
	Duration   string
	DurationMS int64
	TimedOut   bool
	err        string
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

	results := make([]*commandResult, len(commands))
	for i, cmd := range commands {
		start := time.Now()
		r := runCommand(ctx, h.cfg, cmd, cwd)
		r.Command = cmd
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
			Command:   cmd,
			ExitCode:  &exitCode,
			TimedOut:  r.TimedOut,
			CWD:       cwd,
		})

		if r.err != "" {
			return mcp.NewToolResultError(r.err), nil
		}
	}

	multi := len(results) > 1
	return mcp.NewToolResultText(formatCommandResults(results, multi)), nil
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
