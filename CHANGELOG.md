# CHANGELOG

<!--
  FORMAT GUIDE (for agents and humans)

  Entry heading:
    ## [version](PR URL) type: brief title
    - PR URL: run `git log --oneline` and look for "Merge pull request #N" or "(#N)" in the
      merge commit message, then use https://github.com/rthomazel/jail-mcp/pull/N
    - type follows conventional commits:
      build | ci | docs | feat | fix | misc | perf | refactor | revert | style | test

  Section headings (only include sections that have entries):
    ### build | ci | docs | feat | fix | misc | perf | refactor | revert | style | test

  Bullets:
    - [`shortHash`](https://github.com/rthomazel/jail-mcp/commit/shortHash) **(scope)** short label — longer description.
    - scope is the file, package, or area changed e.g. (config), (bin/setup), (workflows).
    - Em dash (—) separates the short label from the explanation.
-->

## [0.4.0](https://github.com/rthomazel/jail-mcp/pull/20) feat: file_replace and file_replace_all tools

### feat

- [`1c31132`](https://github.com/rthomazel/jail-mcp/commit/1c31132) **(handlers)** `file_replace` — find and replace unique substrings in a file; accepts a batch of find/replace pairs; fail-fast pre-pass validates all items against original content before any write; rich diagnostics for zero and multi-match cases including partial first-line hints, char positions, and line context; `line_number` narrows ambiguous matches; overlap detection across items; returns unified diff.
- [`1c31132`](https://github.com/rthomazel/jail-mcp/commit/1c31132) **(handlers)** `file_replace_all` — replace every occurrence of a substring; optional `start_line`/`end_line` scope with full-containment semantics; same diagnostic quality as `file_replace`; returns unified diff.
- [`1c31132`](https://github.com/rthomazel/jail-mcp/commit/1c31132) **(config)** `JAIL_MCP_EDIT_MAX_LINES` and `JAIL_MCP_MAX_CANDIDATES` — configurable replace line-limit (default 50) and diagnostic candidate cap (default 5).

### refactor

- [`c0afc90`](https://github.com/rthomazel/jail-mcp/commit/c0afc90) **(internal/file)** shared file-editing helpers — `Match`, `FindMatches`, `CountLines`, `CountNewlines`, `FirstNonEmptyLine`, `Excerpt`, `ExcerptRange`, `ComputeDiff` (Myers diff via hexops/gotextdiff), `AtomicWrite`, `AcquireLock`/`ReleaseLock` (ref-counted per-file mutex).
- [`c0afc90`](https://github.com/rthomazel/jail-mcp/commit/c0afc90) **(internal/xml)** `Builder`, `OpenTag`/`CloseTag`/`Tag` moved from `handlers/xml.go` to `internal/xml`; `ParseStringSlice` moved to `internal/params.go`. Handlers package now contains only handler files.
- [`fb5df0d`](https://github.com/rthomazel/jail-mcp/commit/fb5df0d) **(handlers)** `file_edit.go` — `openFileForEdit`, `(*editedFile).commit`, `validateFindReplace`, `partialMatchDiagnostic`, `zeroMatchError`, `multiMatchError` extracted as shared helpers used by both handlers; eliminates ~130 lines of duplication.

## [0.3.5] feat: signed commits

### feat

- [`ab4e457`](https://github.com/rthomazel/jail-mcp/commit/ab4e457) **(bin/setup)** GPG signing — `setup_gpg_signing` imports the key from `$GPG_PRIVATE_KEY` (base64-encoded), configures `commit.gpgsign`, `user.signingkey`, and optionally wraps `gpg` with a passphrase for non-interactive signing.
- [`a584364`](https://github.com/rthomazel/jail-mcp/commit/a584364) **(bin/jailmcphttp)** `--pass-environment` — mcp-proxy now forwards its full environment to the jail-mcp subprocess, allowing secrets promoted by the entrypoint to reach agent-run scripts.

### misc

- [`4828461`](https://github.com/rthomazel/jail-mcp/commit/4828461) **(bin/setup)** removed GITHUB_TOKEN / toolchain / prettier logic — jail-mcp has no private dependencies, so the agent/human branch split is unnecessary. setup now only configures GPG signing.

## [0.3.4](https://github.com/rthomazel/jail-mcp/pull/10) fix: array items schema and arm64 build

### fix

- [`5e1a8f5`](https://github.com/rthomazel/jail-mcp/commit/5e1a8f5) **(main)** array `items` schema — all array-typed tool parameters (`commands`, `job_ids`, `paths`) were missing an `items` field in their JSON Schema definition. OpenAI and Google Gemini reject tool schemas without it, returning a 400. Added `mcp.Items` with `{"type": "string"}` to each.
- [`5e1a8f5`](https://github.com/rthomazel/jail-mcp/commit/5e1a8f5) **(workflows)** arm64 docker build — release workflow was missing `docker/setup-qemu-action` before buildx. Without QEMU binfmt handlers, arm64 images built on amd64 CI hosts may fail at the first `RUN` instruction with `exec /bin/sh: no such file or directory`.

### refactor

- [`5e1a8f5`](https://github.com/rthomazel/jail-mcp/commit/5e1a8f5) **(run)** build commands — `build` renamed to `build:go`; `build:docker` removed and replaced with `build:amd64` and `build:arm64` for explicit single-arch local builds. Shared logic extracted to a `build_arch()` helper.

## [0.3.3](https://github.com/rthomazel/jail-mcp/pull/9) feat: configurable home and mise directories

### feat

- [`3dfdb19`](https://github.com/rthomazel/jail-mcp/commit/3dfdb19) **(config)** `JAIL_MCP_HOME` — new env variable controlling the base directory for the path snapshot file and the persistent-install note in context output. Defaults to `$HOME` via `os.UserHomeDir()`, so root users are unaffected. Non-root users or custom setups can override without touching the image.
- [`546d8f3`](https://github.com/rthomazel/jail-mcp/commit/546d8f3) **(config)** `JAIL_MCP_MISE_DIR` — new env variable for the mise mount directory. Defaults to `/mise`. Used to derive the shims path prepended to `$PATH` on startup and to identify the mise volume as persistent in context output. `snapshotFile` restored as a package-level const in `pathsnapshot`.

## [0.3.2](https://github.com/rthomazel/jail-mcp/pull/8) feat: SSE mode with mcp-proxy wrapper

### feat

- [`07feac8`](https://github.com/rthomazel/jail-mcp/commit/07feac8) **(docker)** mcp-proxy transport — `mcp-proxy` added to the image alongside `mcpo`. `bin/jailmcphttp` now dispatches across three modes via `JAIL_MCP_TRANSPORT`: `mcpo` (OpenAI-compatible REST), `mcp-proxy` (native MCP/SSE), and stdio (default). `JAIL_MCP_HTTP=true` remains supported as a legacy alias for `mcpo`. sample compose file updated to default to `mcp-proxy`.

### docs

- [`07feac8`](https://github.com/rthomazel/jail-mcp/commit/07feac8) `JAIL_MCP_TRANSPORT` added to config reference — documents the new variable and its accepted values. README updated with setup overview and refined known client bug instructions.

### misc

- [`07feac8`](https://github.com/rthomazel/jail-mcp/commit/07feac8) **(run)** `format:others` command — runs prettier on non-Go files (JSON, YAML, Markdown).

## [0.3.1](https://github.com/rthomazel/jail-mcp/pull/7) fix: arm64 release support

### fix

- [`79b82fa`](https://github.com/rthomazel/jail-mcp/commit/79b82fa) **(workflows)** arm64 release — release workflow gains `docker/setup-buildx-action` and `platforms: linux/amd64,linux/arm64`. local `build:docker` command updated to use `docker buildx build` with the same platform flags.

## [0.3](https://github.com/rthomazel/jail-mcp/pull/6) feat: volumes, multi-command, and context improvements

### feat

- [`e7c0c03`](https://github.com/rthomazel/jail-mcp/commit/e7c0c03) **(internal/pathsnapshot)** auto-detect binaries in path — context now diffs PATH entries and reports tools discovered at runtime, so agents see what's available without manual inspection.
- [`f0c3ede`](https://github.com/rthomazel/jail-mcp/commit/f0c3ede) **(handlers)** multiple commands in exec and background — `exec_sync` and `exec_background` now accept an array of commands, running each in sequence and returning per-command results.
- [`e77006a`](https://github.com/rthomazel/jail-mcp/commit/e77006a) **(handlers)** multiple jobs in status — `status` accepts an array of job IDs and returns results for all of them in one call.
- [`eab80dc`](https://github.com/rthomazel/jail-mcp/commit/eab80dc) **(docker)** volumes — compose configuration gains named volume support for persisting data across container restarts.

### refactor

- [`00226b6`](https://github.com/rthomazel/jail-mcp/commit/00226b6) **(handlers/context)** remove mise shims — dedicated mise shims block removed from context output; shims are already covered by the auto-detected path entries.
- [`a2a9ca1`](https://github.com/rthomazel/jail-mcp/commit/a2a9ca1) **(handlers/context)** show persistent volumes — volumes backed by persistent mounts are now labeled `persistent` in context output instead of `rw`.

### docs

- [`64c9e5e`](https://github.com/rthomazel/jail-mcp/commit/64c9e5e) update volume persistence instructions — clarified how to persist tools and files across ephemeral container sessions.

### misc

- [`984c37b`](https://github.com/rthomazel/jail-mcp/commit/984c37b) short curl version — curl version string trimmed to `curl x.x.x` to reduce noise in context output.
- [`cb0e14b`](https://github.com/rthomazel/jail-mcp/commit/cb0e14b) **(docker)** update volume docs.
- [`8346e8b`](https://github.com/rthomazel/jail-mcp/commit/8346e8b) **(run)** remove publish command.

## [0.2.2](https://github.com/rthomazel/jail-mcp/pull/5) feat: hidden paths & context improvements

### feat

- [`8861fe9`](https://github.com/rthomazel/jail-mcp/commit/8861fe9) **(docker)** hidden paths — volume mounts can shadow files or directories inside a project to hide them from the agent. mount `/dev/null` over a file or an empty host directory over a subdirectory; mount order in the compose file determines precedence.
- [`2acf59e`](https://github.com/rthomazel/jail-mcp/commit/2acf59e) **(run)** add publish script — `run` gains a `publish` command for tagging and pushing new builds.
- [`e651fc1`](https://github.com/rthomazel/jail-mcp/commit/e651fc1) **(handlers/context)** mise shims block, path — `context` now reports the mise shims directory and includes it in the returned `path`, so agents can verify tool availability without manual inspection.

### fix

- [`89528dd`](https://github.com/rthomazel/jail-mcp/commit/89528dd) **(mise)** add shims to path at server start — mise shims are injected into `PATH` when the server starts, making agent-installed tools immediately available to all commands.

### refactor

- [`b58eeec`](https://github.com/rthomazel/jail-mcp/commit/b58eeec) **(handlers)** builder WriteString — handler output construction switched to `WriteString` for consistency.
- [`c32f73c`](https://github.com/rthomazel/jail-mcp/commit/c32f73c) **(handlers)** plain text output — handlers emit plain text instead of structured markup, simplifying agent parsing.

### build

- [`e8d40bc`](https://github.com/rthomazel/jail-mcp/commit/e8d40bc) improve version string — version now embeds commit hash and build timestamp for clearer traceability.

### docs

- [`7e709ad`](https://github.com/rthomazel/jail-mcp/commit/7e709ad) update docs — README updated with hidden mounts documentation and known bugs section covering tool discovery failure after server update.
- [`571cae0`](https://github.com/rthomazel/jail-mcp/commit/571cae0) update ideas.

### misc

- [`0c1faf9`](https://github.com/rthomazel/jail-mcp/commit/0c1faf9) change module name.
- [`cb7a321`](https://github.com/rthomazel/jail-mcp/commit/cb7a321) **(context)** remove go and node from response — trimmed unused fields to reduce noise.

## [0.2.1](https://github.com/rthomazel/jail-mcp/pull/4) feat: http mode & runtime improvements

### features

- **http mode** — new optional HTTP/SSE transport alongside stdio. `bin/jailmcphttp` helper script and `docker-compose-http-sample.yml` added for running the server over HTTP.
- **jujutsu in container** — `jj` binary installed in the runtime image, enabling agents to use jj commands inside the container.
- **setup script support** — `setup` tool now discovers and sources a `setup.sh` (or equivalent at `setup`, `bin/setup`, `script/setup`, `scripts/setup`, `scripts/setup.sh`) before running manifest install commands.
- **version in context** — `context` tool now returns the server build version.

### fixes

- **dockerfile** — fixed jj archive extraction (member path is `./jj`, requires `--strip-components=1`).
- **setup race** — fixed a race condition between concurrent setup jobs sharing output buffers.
- **setup tag & command** — corrected `go install tool` invocation and run script image tagging.

### improvements

- **tool descriptions** — updated `exec_sync` and `exec_background` descriptions to better guide agents on when to use each.
- **CI** — added `release.yml` workflow to push the image to `ghcr.io` on tag push.
- **compose sample** — renamed `docker-compose.sample.yml` → `docker-compose-sample.yml` for consistency; updated to reference the locally built image.
- **docs** — added `doc/architecture.md`, `doc/config.md`; updated `CLAUDE.md` to be agent-directives-only.

## [0.2.0](https://github.com/rthomazel/jail-mcp/pull/3) feat(handlers): setup

### features

- **`setup`** — new tool that installs dependencies for given project paths in parallel. detects supported manifests (`.tool-versions`, `go.mod`, `yarn.lock`, `package.json`, `requirements.txt`, `pyproject.toml`, `Gemfile`, `Cargo.toml`, `mix.exs`) and runs the appropriate install commands.

### improvements

- **Go tool directive** — migrated from `tools.go` pattern to Go 1.24 `tool` directive in `go.mod`. tools are now declared with `go get -tool` and run via `go tool`. `tools.go` removed.
- **`go install tool`** — setup appends `go install tool` to the `go mod download` step. safe when no tools are declared (exits 0 with a warning).

## [0.1.0](https://github.com/rthomazel/jail-mcp/pull/2) feat(handlers): add exec background and status

### features

- **`exec_background`** — run long-running commands without blocking. returns a `job_id` immediately. background jobs have a separate timeout (`JAIL_MCP_BACKGROUND_TIMEOUT`, default 5m).
- **`exec_status`** — poll a background job for state, stdout, stderr, exit code, and duration.
- **`exec_sync`** — renamed from `shell_exec`.
- **context** — returns `shell_exec_timeout` so agents know the sync timeout upfront.

### improvements

- **dynamic mounts** — `context` no longer hardcodes `/projects`. reads `/proc/mounts` and reports all user-mounted volumes regardless of mount location.
- **Go tools** — `godotenv` and `gofumpt` pinned in `tools.go` and installed from the module graph at docker build time. tool versions stay in sync with the project.
- **Go runtime** — container installs Go 1.25 from upstream instead of the stale apt package.
- **CI** — `pr.yml` workflow runs build, test, lint, and go mod tidy check on every PR to main.

### fixes

- logs go to stderr only; removed log file path from context output.
- fixed dockerfile GOPATH for alpine builder stage (`/go/bin` not `/root/go/bin`).

### docs

- `doc/ideas.md` — planned features: setup tool, language version management, concurrent context, per-command timeout, command stats.
- `doc/tools.md` — documents the `tools.go` pattern; projects bring their own tool versions, container picks them up at build time.
- `changelog.md` — this file.
