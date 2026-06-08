package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rthomazel/mcp/bench/handlers"
	"github.com/rthomazel/mcp/bench/internal"
	"github.com/rthomazel/mcp/bench/internal/pathsnapshot"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "local"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	miseShims := cfg.MiseDir + "/shims"
	homeBin := cfg.Home + "/bin"
	_ = os.MkdirAll(homeBin, 0o755)

	current := os.Getenv("PATH")
	if !strings.Contains(current, miseShims) {
		current = miseShims + ":" + current
	}
	if !strings.Contains(current, homeBin) {
		current = homeBin + ":" + current
	}
	_ = os.Setenv("PATH", current)

	slog.SetDefault(slog.New(slog.NewTextHandler(
		os.Stderr,
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	defer func() {
		if msg := recover(); msg != nil {
			slog.Error("panic", "msg", msg, "stack", string(debug.Stack()))
			os.Exit(1)
		}
	}()

	pathsnapshot.Diff(cfg.Home)

	slog.Info("bench-mcp starting", "version", version, "timeout", cfg.Timeout, "background_timeout", cfg.BackgroundTimeout)

	h := handlers.New(cfg, version)
	defer h.Close()

	s := server.NewMCPServer(
		"bench-mcp",
		version,
		server.WithToolCapabilities(false),
	)

	s.AddTool(
		mcp.NewTool("context",
			mcp.WithDescription("Returns environment context. Call this at the start of a session to orient yourself."),
		),
		h.HandleContext,
	)

	s.AddTool(
		mcp.NewTool("shell",
			mcp.WithDescription("Execute one or more shell commands. Returns stdout, stderr, exit code, and duration per command. Times out after "+cfg.Timeout.String()+". Pass commands as separate array items rather than && chains — each item gets its own exit code and output block, making failures unambiguous. Use the cwd parameter instead of leading 'cd /path &&' prefixes. Most agents should call this now and defer shell_background."),
			mcp.WithArray("commands", mcp.Required(), mcp.Description("Shell commands to execute. Prefer separate array items over && chains — each runs independently with its own exit code, stdout, and stderr. && chains are auto-split when detected."), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithString("cwd", mcp.Description("Working directory for all commands. Does not persist across tool calls — pass it on every call. Preferred over embedding 'cd /path &&' in each command string.")),
		),
		h.HandleShell,
	)

	s.AddTool(
		mcp.NewTool("shell_background",
			mcp.WithDescription("Execute one or more shell commands in the background. Returns a job_id per command immediately. Use status to poll for results. Times out after "+cfg.BackgroundTimeout.String()+"."),
			mcp.WithArray("commands", mcp.Required(), mcp.Description("Shell commands to execute."), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithString("cwd", mcp.Description("Working directory. Defaults to /")),
		),
		h.HandleShellBackground,
	)

	s.AddTool(
		mcp.NewTool("status",
			mcp.WithDescription("Poll the status of one or more background jobs. Returns done, stdout, stderr, exit_code (if done), and duration per job."),
			mcp.WithArray("job_ids", mcp.Required(), mcp.Description("Job IDs returned by shell_background."), mcp.Items(map[string]any{"type": "string"})),
		),
		h.HandleStatus,
	)

	s.AddTool(
		mcp.NewTool("setup",
			mcp.WithDescription("Discover and install dependencies for the given project paths in parallel. Returns a map of project path to job_id or error. Use the status tool to poll results."),
			mcp.WithArray("paths", mcp.Required(), mcp.Description("Project paths to set up."), mcp.Items(map[string]any{"type": "string"})),
		),
		h.HandleSetup,
	)

	s.AddTool(
		mcp.NewTool("file_replace_all",
			mcp.WithDescription("Replace all occurrences of a substring in a file. Returns a unified diff."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the file.")),
			mcp.WithString("find", mcp.Required(), mcp.Description("Exact substring to find. All occurrences are replaced.")),
			mcp.WithString("replace", mcp.Required(), mcp.Description("Replacement text. Empty string deletes each match.")),
			mcp.WithNumber("start_line", mcp.Description("Optional. Restrict replacements to this line range, inclusive (original-file line numbers).")),
			mcp.WithNumber("end_line", mcp.Description("Optional. Restrict replacements to this line range, inclusive (original-file line numbers).")),
			mcp.WithBoolean("dry_run", mcp.Description("Optional. If true, validate and compute the diff without writing to disk.")),
		),
		h.HandleFileReplaceAll,
	)

	s.AddTool(
		mcp.NewTool("file_replace",
			mcp.WithDescription("Find and replace unique substrings in a file. Returns a unified diff."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the file.")),
			mcp.WithArray("replacements",
				mcp.Required(),
				mcp.Description("One or more find/replace pairs. Order does not matter."),
				mcp.Items(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"find":        map[string]any{"type": "string", "description": "Unique substring to find, matched by character including whitespace."},
						"replace":     map[string]any{"type": "string", "description": "Replacement text. Empty string deletes the match."},
						"line_number": map[string]any{"type": "integer", "description": "Optional. Narrows the match to occurrences spanning this line (original-file line number)."},
					},
					"required": []any{"find", "replace"},
				}),
			),
			mcp.WithBoolean("dry_run", mcp.Description("Optional. If true, validate and compute the diff without writing to disk.")),
		),
		h.HandleFileReplace,
	)

	s.AddTool(
		mcp.NewTool("stats",
			mcp.WithDescription("Returns tool-call statistics from the local history. Default is last 30 days, pass 0 to query all time."),
			mcp.WithNumber("days", mcp.Description("Rolling window in days. 0 returns all time. Defaults to 30.")),
		),
		h.HandleStats,
	)

	slog.Info("serving on stdio")
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}
