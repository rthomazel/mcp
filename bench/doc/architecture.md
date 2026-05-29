# architecture

## layout

```
main.go                       server wiring, tool registration, MCP server init, PATH setup
internal/config.go            Config struct, env var loading, defaults
handlers/handler.go           Handler struct, job store, startJob, background job GC
handlers/context.go           HandleContext, parseMounts, discoverMiseShims, formatPlainTextContext
handlers/shell.go         HandleExec, runCommand (shared by context), formatPlainText
handlers/shell_background.go   HandleExecBackground
handlers/status.go            HandleStatus
handlers/setup.go             HandleSetup, orderedRules, setupScriptCandidates
```

## request flow

All tools go through `mcp-go` → handler method → plain text response.

Responses are formatted as human-readable plain text. Metadata fields are wrapped in `<metadata>` tags, one field per line. Command output is wrapped in `<stdout>` and `<stderr>` tags, raw and unindented.

`shell` and `context` run commands synchronously via `runCommand`, which wraps `bash -c` with a context timeout.

`shell_background` calls `startJob`, which spawns a goroutine, assigns a random 4-digit ID, and returns immediately. The caller polls with `status`.

`setup` detects the project's package manager by checking for known manifest files in order, builds a compound shell command (`&&`-joined), and launches it as a background job per path. If a `setup.sh` (or equivalent) is found it runs first.

`context` reads `/proc/mounts`, filters noise (proc/sysfs/tmpfs/overlay/etc.), deduplicates child mounts, collects tool versions via `runCommand`, and discovers mise-managed executables by reading `/mise/shims` directly.

## concurrency

`Handler.jobs` is a `map[string]*job` guarded by `Handler.mu` (RWMutex). Each job has its own `sync.Mutex` protecting its output buffers and done flag. A background goroutine sweeps completed jobs older than 1 hour every 5 minutes.

## configuration

Config is env-var only. See [config.md](config.md).

## design decisions

- No command filtering — the container is the security boundary, not the server
- `bash -c` gives agents pipes, redirects, `&&`, subshells
- Plain text responses over JSON — more readable for humans inspecting output; `<metadata>`, `<stdout>`, `<stderr>` XML tags prevent content/metadata collision
- `/mise/shims` is prepended to `PATH` at startup in `main.go` so all subprocesses inherit mise-managed tools without requiring shell init files
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
