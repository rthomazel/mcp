# stats

The tool summarizes tool-call statistics over a rolling window.

# Types

# Functions

## Handler.HandleStats(_ context.Context, req) (*mcp.CallToolResult, error)

1. Report that stats are disabled when the writer is nil.
2. Read the window, defaulting to 30 days and capping positive values to avoid full-history scans.
3. Query the stats database, returning an error on failure.
4. Format and return the report.

## formatStatsReport(report) string

1. List each tool with its call count, average, and p95 when available.
2. List the top commands by frequency, hashing commands that lack a key.
3. Note when commands are stored as hash only and how to enable full command display.

## msToString(ms) string

1. Milliseconds format as milliseconds.
2. Otherwise format as seconds.

#### Rationale

- A p95 hint is attached to a top command only when it exceeds half the shell timeout, signaling the agent to use shell_background instead.
- Commands are shown hashed unless a Docker Secret supplies the encryption key, keeping secrets out of the report.
