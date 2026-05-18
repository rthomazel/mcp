# configuration

Config is loaded from environment variables only — no flags, no config files.

| variable                      | default   | description                                                                                                                                             |
| ----------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `JAIL_MCP_TIMEOUT`            | `15s`     | Timeout for `exec_sync` commands                                                                                                                        |
| `JAIL_MCP_BACKGROUND_TIMEOUT` | `5m`      | Timeout for `exec_background` / `setup` jobs                                                                                                            |
| `JAIL_MCP_TRANSPORT`          | _(unset)_ | HTTP wrapper: `mcpo` (OpenAI-compatible REST) or `mcp-proxy` (native MCP/SSE). `JAIL_MCP_HTTP=true` is equivalent to `mcpo` and remains supported.      |
| `JAIL_MCP_HOME`               | `$HOME`   | Base directory for the path snapshot file and the persistent-install note. Override when running as a non-root user without access to the default home. |
| `JAIL_MCP_MISE_DIR`           | `/mise`   | Directory where mise is mounted. Used to prepend the shims path to `$PATH` on startup and to mark the volume as persistent in context output.           |
| `JAIL_MCP_EDIT_MAX_LINES`     | `50`      | Maximum lines allowed in `replace` per item for `file_replace` and `file_replace_all`. Increase to allow larger replacements.                           |

Values must be valid Go duration strings (e.g. `30s`, `2m`, `1h`).

Set these in the `environment:` section of `docker-compose.yml`.

## path snapshot

On startup the server scans all directories in `$PATH` and writes a snapshot of
the discovered executables to `$JAIL_MCP_HOME/.jail-mcp-path-snapshot` (TSV: `name\tpath`,
one entry per line, sorted). This file is created once and never updated.

On each `context` call the server rescans `$PATH` and diffs against the snapshot.
Any executables not present at startup appear under `auto-detected in path:` in
the response — useful for binaries installed by setup scripts that are not
managed by mise.
