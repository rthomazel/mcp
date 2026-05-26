# stats design

BenchMCP records statistics on every tool call into a local SQLite database.
The data enables planning (when to use `shell` vs `shell_background`), file-edit
analysis, and review of command history — similar in spirit to a shell history file.

## persistence

The database lives at `$BENCH_MCP_HOME/bench-mcp-stats.db` (default: `/root/bench-mcp-stats.db`).
`/root` is already a named persistent volume, so the file survives container restarts with no
additional configuration. `BENCH_MCP_HOME` is documented in [config.md](config.md).

The directory is created on startup if it does not exist. If the DB cannot be opened or the
schema cannot be bootstrapped, stats are disabled for the session — the server continues
normally and logs the error via `slog`.

## dependency

`modernc.org/sqlite` — pure Go, no CGo, no gcc required in the container image.

## lifecycle

A `stats.Writer` is created in `handlers.New()` and stored on `Handler`. It wraps a
`*sql.DB` and a single goroutine draining a write channel.

`Handler` gains a `Close()` method that:

1. Closes the write channel, signalling the goroutine to stop accepting new records
2. Waits up to 1 minute for the goroutine to drain all buffered records and exit; any records remaining after the deadline are dropped and counted in the log
3. Closes the `*sql.DB`

The MCP server calls `Handler.Close()` on shutdown.

### SQLite connection settings

Applied at open time:

```sql
PRAGMA journal_mode=WAL;     -- allows concurrent readers alongside the single writer
PRAGMA busy_timeout=5000;    -- wait up to 5s before returning SQLITE_BUSY
PRAGMA foreign_keys=ON;
```

Go `*sql.DB` settings:

```go
db.SetMaxOpenConns(1)    // single writer — eliminates connection-level lock contention
db.SetMaxIdleConns(1)
db.SetConnMaxLifetime(0)
```

### async writer queue

All inserts go through a buffered channel (capacity 256). A single goroutine drains the
channel and executes inserts. If the channel is full, the record is dropped and the drop
is logged via `slog` — the tool response is never blocked.

On `Handler.Close()`, the channel is closed (new sends will panic — callers must not record
after `Close()`). The goroutine finishes processing all buffered records then exits. `Close()`
blocks until the goroutine returns, guaranteeing no writes are in flight when `*sql.DB` closes.

## write strategy

Every tool call produces one row. Recording happens on both success and failure paths.

| tool                                | when recorded                                                                 |
| ----------------------------------- | ----------------------------------------------------------------------------- |
| `shell`                             | after each `runCommand` returns — one row per command in the `commands` array |
| `shell_background` / `setup`        | inside the job goroutine after `cmd.Run()` returns                            |
| `file_replace` / `file_replace_all` | at handler return, regardless of success                                      |
| `context` / `status` / `stats`      | at handler return                                                             |

## schema

The schema is managed with `golang-migrate/migrate/v4`. Migration files live in `db/migrations/`
and are embedded in the binary.
A `stats.Writer.Migrate()` method runs pending migrations at startup before any inserts.
`schema_migrations` tracking is handled by golang-migrate — no custom version table is needed.

Initial migration (`db/migrations/1_initial_schema.up.sql`):

```sql
BEGIN;

CREATE TABLE tool_calls (
    id                    TEXT     PRIMARY KEY,          -- UUID v4
    tool                  TEXT     NOT NULL,
    called_at             DATETIME NOT NULL,             -- 'YYYY-MM-DD HH:MM:SS' UTC
    duration_ms           INTEGER  NOT NULL DEFAULT 0,
    server_version        TEXT,                         -- build version from Handler.version
    error_kind            TEXT,                         -- NULL on success; 'timeout',
                                                        -- 'start_failed', 'arg_error', 'write_error'

    -- shell / shell_background / setup
    base_cmd              TEXT,                         -- first meaningful token; see base_cmd extraction
    cmd_hash              TEXT,                         -- SHA-256(post-pipeline command), hex
    cmd_encrypted         TEXT,                         -- 'v1:<b64 nonce>:<b64 ciphertext+tag>'
    normalizer_version    INTEGER,                      -- bump when normalization rules change
    exit_code             INTEGER,
    timed_out             INTEGER  NOT NULL DEFAULT 0   CHECK (timed_out IN (0,1)),
    cwd                   TEXT,                         -- full path as provided by caller
    job_id                TEXT,                         -- links shell_background/setup to job ID
    redacted_byte_counts  TEXT,                         -- JSON: [N, ...] original byte size per redacted token

    -- file_replace / file_replace_all
    file_path             TEXT,                         -- full path as provided
    replacement_count     INTEGER,                      -- number of items in the replacements array
    replacement_bytes     TEXT,                         -- JSON: [[find_bytes, replace_bytes], ...]
    dry_run               INTEGER                       CHECK (dry_run IS NULL OR dry_run IN (0,1)),

    -- setup
    setup_paths           TEXT                          -- JSON array of paths passed to setup
);

CREATE INDEX idx_tc_called_at      ON tool_calls (called_at);
CREATE INDEX idx_tc_tool_date      ON tool_calls (tool, called_at);
CREATE INDEX idx_tc_tool_hash_date ON tool_calls (tool, cmd_hash, called_at);
CREATE INDEX idx_tc_file_path      ON tool_calls (tool, file_path);

COMMIT;
```

### column notes

- `id` is a UUID v4 generated in Go (`google/uuid` or `crypto/rand` + RFC 4122 formatting).
- `called_at` uses SQLite `DATETIME` text (`YYYY-MM-DD HH:MM:SS` UTC) so that
  `called_at > datetime('now', '-30 days')` works without conversion.
  In Go: `time.Now().UTC().Format("2006-01-02 15:04:05")`.
- `duration_ms` is always set — `0` for tools where wall time is not meaningful.
- `cmd_hash` is unsalted SHA-256 of the post-normalization string. Rows with different
  `normalizer_version` values should not be grouped by `cmd_hash`. See privacy notes.
- `normalizer_version` is an integer constant in the codebase, bumped when normalization
  rules change, so that stale hashes can be identified in queries.
- `error_kind` is NULL when a tool call completes normally — including shell commands that
  exit nonzero. A nonzero exit is a successful execution from the handler's perspective;
  it is captured in `exit_code`. `error_kind` is set only for handler-level failures:
  `timeout` (context deadline exceeded), `start_failed` (process could not launch),
  `arg_error` (missing or invalid handler arguments), `write_error` (file operation failure).
- `redacted_byte_counts` is a JSON array of the original byte sizes of every string replaced
  during passes 2 and 3, in order of substitution. Example: a command where `TOKEN=secret` (6B),
  an email (17B), and a heredoc (1247B) were replaced yields `[6, 17, 1247]`. Collected during
  the pipeline; stored as `NULL` for tools with no command string.
- `replacement_bytes` stores raw per-item byte counts so queries can compute any aggregate.
  Format: `[[find_bytes, replace_bytes], ...]` — one pair per item in order. For
  `file_replace_all` the single pair represents the find pattern and replacement string.
- `job_id` links `shell_background` and `setup` rows to the ID returned to the caller,
  enabling correlation with `status` output.
- `server_version` is populated from `Handler.version` — helps interpret stats across
  server upgrades.
- `setup_paths` stores the full path list passed to `setup` as a JSON array.

## configuration

One new env var and one Docker Secret, both optional:

| name                                | kind          | description                                                                                                                                                                                                                                                                        |
| ----------------------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bench_mcp_stats_encryption_key_v1` | Docker Secret | Enables full command storage. Base64-encoded 32-byte AES-256 key, read from `/run/secrets/bench_mcp_stats_encryption_key_v1`. When present, the post-pipeline redacted command is encrypted and stored in `cmd_encrypted`. When absent, only `base_cmd` and `cmd_hash` are stored. |
| `BENCH_MCP_STATS_REDACT_PATTERNS`   | env var       | Newline-separated Go regex strings added as a user-defined redaction tier. Each match is replaced with `[USER REDACTED]`. Patterns that fail to compile are skipped and logged at startup.                                                                                                |

`BENCH_MCP_HOME` controls the DB directory and is documented in [config.md](config.md).

### key setup with Docker Secrets

Generate a key and write it to a secrets file:

```bash
openssl rand -base64 32 > secrets/bench_mcp_stats_encryption_key_v1
chmod 600 secrets/bench_mcp_stats_encryption_key_v1
```

Declare the secret and mount it in `docker-compose.yml`:

```yaml
secrets:
  bench_mcp_stats_encryption_key_v1:
    file: ./secrets/bench_mcp_stats_encryption_key_v1

services:
  bench-mcp:
    secrets:
      - bench_mcp_stats_encryption_key_v1
```

The server reads the key from `/run/secrets/bench_mcp_stats_encryption_key_v1` at startup.
If the file is absent or empty, full command storage is disabled and only `base_cmd` and
`cmd_hash` are recorded.

### encrypted storage envelope

When the Docker Secret is present, each command is encrypted with AES-256-GCM and stored
in `cmd_encrypted` as:

```
v1:<base64(12-byte random nonce)>:<base64(ciphertext + 16-byte GCM tag)>
```

A fresh 12-byte nonce is generated per row via `crypto/rand`. The `v1:` prefix enables
future key rotation or algorithm changes without a schema migration.

The encrypted payload is always the post-pipeline, post-redaction string — never the raw
command. The `stats` tool decrypts on read and displays the redacted command in output.

## command processing pipeline

Applied in order before any DB write. Each pass is destructive — the raw command string is
never persisted.

```
call site (handler)
        │
        │  pass 1: tool normalization — handler extracts the command string
        │  and records structured fields (file_path, replacement_bytes, etc.)
        │  directly. Non-shell tools skip passes 2-4 entirely.
        │
        ▼  shell command string
[2. REDACTION]                   pattern-based, security and privacy
        │
        ▼
[3. STRUCTURAL NORMALIZATION]    shell and interpreter constructs
        │
        ▼
[4. LONG TOKEN NORMALIZATION]    catch-all for remaining noise
        │
        ├──► base_cmd             shell-aware extraction (see below)
        ├──► cmd_hash             SHA-256(result), hex
        └──► cmd_encrypted        AES-256-GCM(result) if key is set
```

Passes 2–4 are implemented in `stats.ProcessCommand` (`stats/destroy.go`). Pass 1
is intentionally at the call site in each handler — there is no single
normalization function for it, which keeps handler recording code explicit.

All byte counts in normalization tokens use the byte length of the original matched text.
Format: `[LABEL NB]` where N is an integer and B is the literal character `B`.

### pass 1 — tool normalization (at call sites)

Each handler records structured fields directly and decides whether to pass a
command string to `ProcessCommand`. There is no single function for this pass —
it is explicit in each handler.

| tool                           | what is recorded directly                                        | command passed to ProcessCommand |
| ------------------------------ | ---------------------------------------------------------------- | -------------------------------- |
| `file_replace`                 | `file_path`, `replacement_count`, `replacement_bytes`, `dry_run` | none — passes 2–4 skipped        |
| `file_replace_all`             | same + `start_line`, `end_line`                                  | none                             |
| `shell` / `shell_background`   | `cwd`, `exit_code`, `timed_out`                                  | full command string              |
| `setup`                        | `setup_paths`, `cwd`                                             | full command string              |
| `context` / `status` / `stats` | duration only                                                    | none                             |

### pass 2 — redaction

Applied to shell command strings only. All substitutions are destructive. Tiers are
applied in order; earlier matches take precedence.

**tier 1 — known secret patterns**

| pattern                                      | example                          | result                  |
| -------------------------------------------- | -------------------------------- | ----------------------- |
| env var assignment with sensitive name       | `TOKEN=abc123 cmd`               | `TOKEN=REDACTED cmd`    |
| `--flag=value` with sensitive flag name      | `--password=hunter2`             | `--password=REDACTED`   |
| quoted value after sensitive `=`             | `KEY='abc'` or `KEY="abc"`       | `KEY=REDACTED`          |
| URL credentials                              | `https://user:pass@host`         | `https://REDACTED@host` |
| Bearer / token auth header                   | `Bearer eyJhbGci...`             | `Bearer REDACTED`       |
| JWT (three base64url dot-separated segments) | `eyJ.eyJ.sig`                    | `[JWT]`                 |
| PEM block                                    | `-----BEGIN PRIVATE KEY-----...` | `[PEM BLOCK]`           |

Sensitive name list, matched at whole env-var or flag-name boundaries (case-insensitive
word boundary match to avoid false positives like `--passthrough` or `COMPASS_URL`):
`TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `PASSWD`, `PWD`, `PASS`, `AUTH`, `CRED`,
`CREDENTIAL`, `API_KEY`, `PRIVATE`, `CERT`, `SIGNING`.

**tier 2 — high-entropy patterns**

| pattern       | threshold                         | result         |
| ------------- | --------------------------------- | -------------- |
| hex string    | ≥ 32 contiguous hex chars         | `[HEX 64B]`    |
| base64 string | ≥ 24 chars, valid base64 alphabet | `[BASE64 44B]` |
| UUID          | standard 8-4-4-4-12 format        | `[UUID]`       |

The 32-char hex threshold preserves short git SHAs (≤ 12 chars) while catching
SHA-256 outputs, HMAC values, and raw keys.

**tier 3 — PII**

| pattern                                        | result        |
| ---------------------------------------------- | ------------- |
| email address                                  | `[EMAIL]`     |
| public IP address (non-RFC-1918, non-loopback) | `[PUBLIC IP]` |

RFC-1918 ranges (`10.x.x.x`, `172.16–31.x.x`, `192.168.x.x`) and loopback are retained
as they are useful operational context (e.g. Docker service addresses).

**tier 4 — user patterns**

Each newline-separated pattern from `BENCH_MCP_STATS_REDACT_PATTERNS` is compiled at
startup and applied in order. Each match becomes `[USER REDACTED]`, distinguishing
user-defined redactions from built-in pipeline redactions. Failed compilations are
logged and skipped.

Example `docker-compose.yml` entry:

```yaml
BENCH_MCP_STATS_REDACT_PATTERNS: |
  acme-corp
  mrn-[0-9]+
  patient-[a-f0-9]{8}
```

### pass 3 — structural normalization

Shell and interpreter constructs are replaced before the long-token rule so their
content does not trigger spurious long-token matches.

**shell constructs**

| construct                                      | result             |
| ---------------------------------------------- | ------------------ |
| `$(...)` command substitution                  | `[SUBSHELL]`       |
| `` `...` `` backtick substitution              | `[SUBSHELL]`       |
| `<< 'DELIM' ... DELIM` heredoc (any delimiter) | `[HEREDOC 1247B]`  |
| `<<- DELIM ... DELIM` heredoc                  | `[HEREDOC 1247B]`  |
| `<<< "..."` here-string                        | `[HERESTRING 42B]` |
| `<(...)` process substitution                  | `[PROCESS_SUB]`    |

**inline scripts**

Interpreter invocations where `-c` or `-e` is followed by a quoted argument:

| example            | result                           |
| ------------------ | -------------------------------- |
| `python3 -c '...'` | `python3 -c [INLINE_SCRIPT 89B]` |
| `python -c "..."`  | `python -c [INLINE_SCRIPT 89B]`  |
| `perl -e '...'`    | `perl -e [INLINE_SCRIPT 45B]`    |
| `ruby -e '...'`    | `ruby -e [INLINE_SCRIPT 45B]`    |
| `node -e '...'`    | `node -e [INLINE_SCRIPT 45B]`    |
| `awk '{...}'`      | `awk [INLINE_SCRIPT 14B]`        |

**Python multiline strings** (inside a `-c` argument):

| pattern                         | result                |
| ------------------------------- | --------------------- |
| `"""..."""` triple-quoted block | `[PYTHON_BLOCK 342B]` |

These patterns are best-effort. Complex quoting, escaping, and nesting may not be
handled correctly. Anything that survives is handled by pass 4.

### pass 4 — long token normalization

Applied last. Any whitespace-delimited token longer than 80 bytes becomes
`[LONG STRING NB]` where N is the byte count of the original token. This handles deep
paths, escaped strings, inline data, and anything else that passed earlier stages.

### base_cmd extraction

Applied to the post-pipeline string. The algorithm is iterative — steps 2–4 repeat
until no further progress is made:

1. **Split on pipeline boundary.** Locate the first unquoted `&&`, `||`, `|`, or `;`.
   Use only the segment after the last leading `cd ...` navigation and before the
   first operator that begins the "real" command. Specifically: if the first segment
   begins with `cd`, discard it and take the next segment instead.
2. **Strip leading env-var assignments.** Remove leading `WORD=value` and
   `WORD=REDACTED` tokens. Repeat until the first token is not an assignment.
3. **Strip leading wrapper tokens.** If the first token is a known wrapper
   (`sudo`, `env`, `time`, `command`, `exec`, `nice`, `ionice`), consume it
   along with any option flags it takes (`sudo -u USER`, `sudo -n`, `nice -n N`,
   etc.). Repeat step 2 after each wrapper consumed.
4. **Converge.** If neither step 2 nor step 3 made progress, stop.
5. **First remaining token is `base_cmd`.** If none remains, `base_cmd` is NULL.

Examples:

| post-pipeline command                      | base_cmd |
| ------------------------------------------ | -------- |
| `TOKEN=REDACTED git status`                | `git`    |
| `sudo -u root bash -c [INLINE_SCRIPT 42B]` | `bash`   |
| `cd /projects/foo && go test ./...`        | `go`     |
| `env GOPATH=/root go build`                | `go`     |
| `TOKEN=REDACTED`                           | NULL     |

Note on `sudo -u root bash`: step 3 consumes `sudo -u root` (the `-u USER` option pair),
leaving `bash` as the first token. Note on `env GOPATH=/root go build`: step 3 consumes
`env`, then step 2 strips `GOPATH=/root`, leaving `go`. Note on `cd ... && go test`: step 1
discards the `cd` segment, taking `go test ./...` from the next segment.

### pipeline example

Input:

```bash
TOKEN=secret git -C /projects/bench-mcp log --pretty=format:'%H %s' --author=user@example.com -n 20
```

After pass 2 (tier 1: `TOKEN=secret` → `TOKEN=REDACTED`; tier 3: email → `[EMAIL]`):

```
TOKEN=REDACTED git -C /projects/bench-mcp log --pretty=format:'%H %s' --author=[EMAIL] -n 20
```

After passes 3–4 (no structural constructs match; no tokens exceed 80 bytes): unchanged.

`base_cmd = git`
`cmd_hash = SHA-256("TOKEN=REDACTED git -C /projects/bench-mcp log ...")`

Two runs with different `TOKEN` values produce the same hash. ✓

## `stats` tool

One optional parameter:

| parameter | type | default | description                                                                                                                                          |
| --------- | ---- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `days`    | int  | `30`    | Rolling window in days. `0` returns all time — document this in the tool description so agents know querying without a time constraint is an option. |

### p95 calculation

SQLite has no built-in percentile function. The `stats` handler loads all matching
`duration_ms` values into a Go slice, sorts it, and uses the nearest-rank formula:
`index = ceil(0.95 × n) - 1` (zero-based). For 100 samples this yields index 94,
the 95th value. Groups with fewer than 20 samples omit the p95 field.

### output format

Plain text, consistent with other tools. Example:

```
tool usage (last 30 days):
  shell              142 calls   avg 1.2s   p95 8.4s
  shell_background    38 calls   avg 45.2s
  file_replace        27 calls   avg 0.3s
  setup               12 calls
  context              8 calls
  stats                3 calls

top commands by frequency (grouped by cmd_hash, labeled by base_cmd):
  git   [a3f9c2]   89 calls   avg 2.1s   p95 12.3s   ← consider shell_background
  go    [b17d44]   31 calls   avg 5.4s   p95 47.1s   ← consider shell_background
  npm   [c90e11]   12 calls   avg 22.0s  p95 91.0s   ← consider shell_background
  cat   [f4a812]   18 calls   avg 0.1s
  grep  [221bc9]   15 calls   avg 0.2s

note: commands stored as hash only — configure the bench_mcp_stats_encryption_key_v1 Docker Secret to store and display full commands
```

The `note:` line is omitted when the Docker Secret is configured; the decrypted redacted
command is shown instead of the hash prefix. The displayed command is always post-redaction —
never the raw input.

Hash prefixes shown are the first 6 hex characters of `cmd_hash`. Rows are grouped by
`(normalizer_version, cmd_hash)` — only rows sharing both values are aggregated, preventing
stale hashes from inflating counts. `base_cmd` labels the group in output. The stats tool
only groups rows matching the current `normalizer_version` by default; older rows are
counted in totals but excluded from the top-commands breakdown.

### shell_background hint

`← consider shell_background` is shown when a group's p95 duration exceeds
`BENCH_MCP_TIMEOUT × 0.5`. Suppressed when the sample size is below 20.

## privacy notes

- `cmd_hash` is unsalted SHA-256. Given a suspected command, anyone with DB access can
  confirm whether it was run by hashing and querying. Acceptable for a single-operator
  local tool, but worth noting when sharing the DB file.
- All content shown or decrypted by `stats` is post-redaction. Raw commands are never
  reconstructed or displayed.
- The encryption key is stored as a Docker Secret (file on disk), not in the environment.
  The DB file and the secrets file should not be co-located in the same backup or commit.
- `cwd` and `file_path` are stored as provided by the caller and may reveal project
  structure. No normalization is applied — strip path components in queries as needed.

## retention

The database is unbounded in v1. SQLite files in `/root` will grow with usage.
The `days` parameter on `stats` limits query scope, not storage. A pruning command
or row-count cap may be added in a future version.

## example queries

```sql
-- files edited most often
SELECT file_path, COUNT(*) AS edits
FROM tool_calls
WHERE tool IN ('file_replace', 'file_replace_all')
GROUP BY file_path
ORDER BY edits DESC;

-- replacement size distribution per file
SELECT
    file_path,
    SUM(replacement_count)  AS total_items,
    AVG(replacement_count)  AS avg_items
FROM tool_calls
WHERE tool = 'file_replace'
GROUP BY file_path;

-- commands by runtime — candidates for shell_background
-- restrict to current normalizer version to avoid stale-hash cross-contamination
SELECT
    base_cmd,
    COUNT(*)                        AS calls,
    CAST(AVG(duration_ms) AS INT)   AS avg_ms,
    MAX(duration_ms)                AS max_ms
FROM tool_calls
WHERE tool = 'shell'
  AND called_at > datetime('now', '-30 days')
  AND normalizer_version = (SELECT MAX(normalizer_version) FROM tool_calls)
GROUP BY normalizer_version, cmd_hash
ORDER BY max_ms DESC;

-- timed-out commands
SELECT base_cmd, cwd, called_at
FROM tool_calls
WHERE timed_out = 1
ORDER BY called_at DESC
LIMIT 20;

-- error breakdown
SELECT error_kind, COUNT(*) AS n
FROM tool_calls
WHERE error_kind IS NOT NULL
GROUP BY error_kind
ORDER BY n DESC;

-- background jobs correlated to their job IDs
SELECT job_id, base_cmd, duration_ms, exit_code, called_at
FROM tool_calls
WHERE tool = 'shell_background'
ORDER BY called_at DESC
LIMIT 20;

-- rows with stale normalizer version (exclude from cmd_hash grouping)
SELECT COUNT(*) AS stale_rows, normalizer_version
FROM tool_calls
WHERE normalizer_version < (SELECT MAX(normalizer_version) FROM tool_calls)
GROUP BY normalizer_version;
```
