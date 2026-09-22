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

	shellOptions := shellCommandToolOptions(cfg, "Execute one or more shell commands. Returns stdout, stderr, exit code, and duration per command. Times out after "+cfg.Timeout.String()+". Pass commands as separate array items for independent commands, and use cwd instead of embedding a leading cd prefix. && expansion is controlled by BENCH_MCP_SHELL_EXPAND_COMMANDS.")
	shellOptions = append(shellOptions, mcp.WithString("cwd", mcp.Description("Working directory for all commands. Does not persist across tool calls — pass it on every call. Preferred over embedding 'cd /path &&' in each command string.")))
	s.AddTool(mcp.NewTool("shell", shellOptions...), h.HandleShell)

	backgroundOptions := shellCommandToolOptions(cfg, "Execute one or more shell commands in the background. Returns a job_id per command immediately. Use status to poll for results. Times out after "+cfg.BackgroundTimeout.String()+". Jobs are deprioritized (nice) so they do not starve foreground shell commands, but memory is still shared — a very large job can exhaust container memory.")
	backgroundOptions = append(backgroundOptions, mcp.WithString("cwd", mcp.Description("Working directory. Defaults to /")))
	s.AddTool(mcp.NewTool("shell_background", backgroundOptions...), h.HandleShellBackground)

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
			mcp.WithDescription("Find and replace unique substrings in a file. Returns a unified diff. The file must already exist."),
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
		mcp.NewTool("file_create",
			mcp.WithDescription("Create a new file, or overwrite an empty one. Creates any missing parent directories. Returns a short message on success; on dry_run it returns a unified diff. Replaces the old 'file_replace creates a missing file' behavior."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Absolute path to the file.")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Full contents of the new file.")),
			mcp.WithBoolean("dry_run", mcp.Description("Optional. If true, validate and compute the diff without writing to disk.")),
		),
		h.HandleFileCreate,
	)

	s.AddTool(
		mcp.NewTool("stats",
			mcp.WithDescription("Returns tool-call statistics from the local history. Default is last 30 days, pass 0 to query all time."),
			mcp.WithNumber("days", mcp.Description("Rolling window in days. 0 returns all time. Defaults to 30.")),
		),
		h.HandleStats,
	)

	slog.Info("serving on stdio", "tool_call_workers", cfg.ToolCallWorkers)
	// WithWorkerPoolSize(1) (the default, see internal/config.go) makes mcp-go's stdio
	// transport drain queued tool calls strictly FIFO, so responses are written back in
	// the same order requests arrived. With a larger pool, workers process and write
	// responses independently, so completion order is not guaranteed to match submission
	// order -- a real hazard when a client issues ordered-dependent calls (e.g. a commit
	// followed by a push) in the same batch. Only raise BENCH_MCP_TOOL_CALL_WORKERS above 1
	// for workloads where every batched call is truly independent.
	if err := server.ServeStdio(s, server.WithWorkerPoolSize(cfg.ToolCallWorkers)); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}

func shellCommandToolOptions(cfg *internal.Config, description string) []mcp.ToolOption {
	options := []mcp.ToolOption{
		mcp.WithDescription(description + " **Shell command escaping:** command values are JSON strings. Escape the command for JSON before sending it—especially backslashes. A shell backslash such as `\\|`, `\\*`, `\\$`, or `\\(` must be written as `\\\\` in JSON. Do not send invalid JSON escapes such as `\\|` or `\\*`."),
		mcp.WithArray("commands", mcp.Description("Canonical parameter. Shell commands to execute. Provide exactly one: commands or one alias. Separate array items run independently."), mcp.Items(map[string]any{"type": "string"})),
	}
	for _, alias := range cfg.ShellCommandsAliases {
		options = append(options, mcp.WithArray(alias, mcp.Description("Alias to commands. Shell commands to execute. Provide exactly one: commands or one alias."), mcp.Items(map[string]any{"type": "string"})))
	}
	return options
}
