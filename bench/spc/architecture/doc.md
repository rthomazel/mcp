---
id: 2026-09-29-architecture
type: documentation
summary: Documents the overall system layout, request flow, and design decisions of bench-mcp.
created: 2026-09-29
updated: 2026-11-13
---
# architecture

## layout

```
main.go                          server wiring, tool registration, MCP server init, PATH setup
internal/config.go               Config struct, env var loading, defaults, hints
internal/params.go               command-array and string-slice parameter parsing
internal/migrate.go              stats database migrations
internal/xml/                    tag builder for plain-text responses
internal/file/                   file edit primitives: locking, checksums, cursors, atomic write
internal/pathsnapshot/           startup PATH snapshot and diff
internal/stats/                  tool-call history: writer, redaction, encryption, queries
db/                              embedded SQL migrations
handlers/handler.go              Handler struct, job store, startJob, background job GC
handlers/context.go              HandleContext, parseMounts, formatPlainTextContext
handlers/shell.go                HandleShell, command expansion, runCommand (shared by context)
handlers/shell_background.go     HandleShellBackground
handlers/status.go               HandleStatus
handlers/setup.go                HandleSetup, orderedRules, setupScriptCandidates
handlers/stats.go                HandleStats
handlers/file_create.go          HandleFileCreate
handlers/file_replace.go         HandleFileReplace
handlers/file_replace_all.go     HandleFileReplaceAll
handlers/file_insert.go          HandleFileInsert
handlers/file_delete.go          HandleFileDelete
handlers/file_cursor.go          shared cursor parsing and edit lifecycle for insert and delete
handlers/file_edit.go            shared validation and diagnostics for the replace tools
```

Each handler has a model file (`handlers/*.md`) describing its functions; tool behavior is documented under `spc/`.

## request flow

All tools go through `mcp-go` → handler method → plain text response.

Responses are formatted as human-readable plain text. Metadata fields are wrapped in `<metadata>` tags, one field per line. Command output is wrapped in `<stdout>` and `<stderr>` tags, raw and unindented.

`shell` and `context` run commands synchronously via `runCommand`, which wraps `bash -c` with a context timeout. `shell` and `shell_background` accept exactly one non-empty command array through the canonical `commands` parameter or an alias configured with `BENCH_MCP_SHELL_COMMANDS_ALIASES`; no aliases exist by default. See [shell](../shell/doc.md).

`shell_background` calls `startJob`, which spawns a goroutine, assigns a random 4-digit ID, and returns immediately. Jobs run under `nice`. The caller polls with `status`. See [shell-background](../shell-background/doc.md).

`setup` detects the project's package manager by checking for known manifest files in order, builds a compound shell command (`&&`-joined), and launches it as a background job per path. If a `setup.sh` (or equivalent) is found it runs first.

`context` reads `/proc/mounts`, drops pseudo filesystems and system paths, collects tool versions via `runCommand`, and diffs the current `PATH` against the startup snapshot to report newly installed executables. See [context](../context/doc.md).

The file tools edit in place under a per-file lock with a checksum recheck and an atomic rename. See the per-tool docs and specs for contracts.

Every tool call is recorded asynchronously to the stats history when stats are enabled. See [stats](../stats/doc.md).

## concurrency

`Handler.jobs` is a `map[string]*job` guarded by `Handler.mu` (RWMutex). Each job has its own `sync.Mutex` protecting its output buffers and done flag. A background goroutine sweeps completed jobs older than 1 hour every 5 minutes.

## configuration

Config is env-var only. See [config](../config/doc.md).

## design decisions

- No command filtering — the container is the security boundary, not the server
- `bash -c` gives agents pipes, redirects, `&&`, subshells
- Plain text responses over JSON — more readable for humans inspecting output; `<metadata>`, `<stdout>`, `<stderr>` XML tags prevent content/metadata collision
- `/mise/shims` and `$HOME/bin` are prepended to `PATH` at startup in `main.go` so all subprocesses inherit mise-managed and ad-hoc tools without requiring shell init files
- `slog.SetDefault` at startup — no logger threaded through the codebase
- `internal/` for config; `handlers/` is a top-level package (not internal) so its types remain accessible if needed

## persistence

Containers are ephemeral — `docker compose run --rm` creates a new container each session and removes it on exit. Anything written to the container writable layer is lost.

Only named volumes persist across sessions:

| volume           | mountpoint | contents                                     |
| ---------------- | ---------- | -------------------------------------------- |
| `bench-mcp-mise` | `/mise`    | mise installs, shims                         |
| `bench-mcp-root` | `/root`    | Go module cache, path snapshot, ad-hoc tools |

Volumes are deleted only by `docker volume rm` or `docker compose down -v`.

This means `setup` only needs to run once per project — language installs and downloaded modules persist.
The path snapshot at `/root/.bench-mcp-path-snapshot` also persists, so `auto-detected in path:` correctly reflects tools installed in prior sessions rather than treating them as newly detected.

To install ad-hoc tools that survive across sessions, install to `$HOME/bin` (`/root/bin`) — the server creates this directory on startup and prepends it to `PATH`. Do not install to `/usr/local/bin` or other paths outside volumes; they will not survive the next session.
