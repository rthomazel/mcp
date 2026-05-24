# stats design

BenchMCP records statistics on every tool call into a local SQLite database.
The data enables planning (when to use `shell` vs `shell_background`), project-level
edit analysis, and review of command history — similar in spirit to a shell history file.

See also: the sqlite db entry in [ideas.md](ideas.md), where this feature originated.

## persistence

The database lives at `$BENCH_MCP_HOME/bench-mcp-stats.db` (default: `/root/bench-mcp-stats.db`).
`/root` is already a named persistent volume, so the file survives container restarts with no
additional configuration.

A `*sql.DB` is opened once in `handlers.New()` and closed on process exit.
The schema is bootstrapped on startup via `CREATE TABLE IF NOT EXISTS`.
If the schema needs to change in a future version, a `schema_version` table and a minimal
migration runner will be added at that time.

## dependency

`modernc.org/sqlite` — pure Go, no CGo, no gcc required in the container image.

## write strategy

All DB writes are fire-and-forget: the recording call is dispatched on a goroutine so that a
slow or locked DB never blocks a tool response. Errors are logged via `slog` and never returned
to the caller.

| tool | when recorded |
| ---- | ------------- |
| `shell` | after each `runCommand` returns — duration, exit code, and command all available |
| `shell_background` / `setup` | inside the job goroutine, after `cmd.Run()` returns — single INSERT, no two-phase write |
| `file_replace` / `file_replace_all` | at handler return |
| `context` / `status` | at handler return — no command fields |

## schema

```sql
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tool_calls (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    tool                  TEXT    NOT NULL,           -- 'shell', 'shell_background', 'file_replace', etc.
    called_at             TEXT    NOT NULL,            -- ISO-8601 UTC
    duration_ms           INTEGER,                    -- NULL for tools where duration is not meaningful

    -- shell / shell_background / setup
    base_cmd              TEXT,                       -- first token of the redacted command
    cmd_hash              TEXT,                       -- SHA-256(redacted command), hex; always set when a command is present
    cmd_encrypted         TEXT,                       -- AES-256-GCM(redacted command), base64; set only when BENCH_MCP_STATS_KEY is provided
    exit_code             INTEGER,
    timed_out             INTEGER NOT NULL DEFAULT 0,  -- 1 if the context deadline was exceeded
    cwd                   TEXT,                       -- working directory passed by the caller, full path

    -- file_replace / file_replace_all
    file_path             TEXT,                       -- full path of the file being edited
    replacement_count     INTEGER,                    -- number of replacement items passed
    replacement_bytes     TEXT,                       -- JSON array of byte counts, one per item: [find_bytes, replace_bytes]
    dry_run               INTEGER                     -- 1 if dry_run=true was passed
);
```

### notes

- `base_cmd` and `cmd_hash` are always recorded when a command is present, regardless of
  whether `BENCH_MCP_STATS_KEY` is set. The hash is one-way and reveals nothing about the
  command; it enables frequency counting and deduplication with zero configuration.
- `replacement_bytes` stores raw counts (e.g. `[[120, 85], [34, 0]]`) rather than a
  pre-computed average, so queries can compute any aggregate (avg, sum, max) directly.
- `file_path` and `cwd` are stored as provided — no project extraction or path stripping.
  Strip path components in queries as needed.
- No `cmd_plaintext` column. Storing a full command requires `BENCH_MCP_STATS_KEY`.
  If the key is absent, only `base_cmd` and `cmd_hash` are stored.

## configuration

Two new env vars, both optional:

| variable | default | description |
| -------- | ------- | ----------- |
| `BENCH_MCP_STATS_KEY` | _(unset)_ | Enables full command storage. Must be a base64-encoded 32-byte AES-256 key. When set, the redacted command is encrypted with AES-256-GCM and stored in `cmd_encrypted`. When unset, only `base_cmd` and `cmd_hash` are stored. |
| `BENCH_MCP_STATS_REDACT_PATTERNS` | _(unset)_ | Pipe-separated Go regex strings added as an extra redaction tier. Each match is replaced with `REDACTED`. Example: `acme-corp\|mrn-[0-9]+` |

## command processing pipeline

Applied in order before any DB write. Each pass is destructive — the raw command is never persisted.

```
raw command string
        │
        ▼
[1. TOOL NORMALIZATION]      per-tool, strips structurally useless fields
        │
        ▼
[2. REDACTION]               pattern-based, security and privacy focused
        │
        ▼
[3. LONG TOKEN NORMALIZATION] catch-all for remaining noise
        │
        ├──► base_cmd         first token of the result
        ├──► cmd_hash         SHA-256(result), hex
        └──► cmd_encrypted    AES-256-GCM(result) if BENCH_MCP_STATS_KEY is set
```

### pass 1 — tool normalization

Strips fields that carry no stats value before pattern matching runs.

| tool | what is kept | what is replaced |
| ---- | ------------ | ---------------- |
| `file_replace` | `path`, `dry_run`, item count | `find` and `replace` text → counted in `replacement_bytes`, not passed to further passes |
| `file_replace_all` | `path`, `start_line`, `end_line`, `dry_run` | `find` and `replace` text → same |
| `shell` / `shell_background` | full command string → passed to pass 2 | — |
| `setup` | path list only | — |
| `context` / `status` | no command fields | — |

### pass 2 — redaction

Applied only to shell command strings. All substitutions are destructive.
Byte counts in replacement tokens use raw byte length of the original matched text.

**tier 1 — known secret patterns**

| pattern | example | stored as |
| ------- | ------- | --------- |
| env var assignment with sensitive name | `TOKEN=abc123 cmd` | `TOKEN=REDACTED cmd` |
| `--flag=value` with sensitive flag name | `--password=hunter2` | `--password=REDACTED` |
| quoted value after sensitive `=` | `KEY='abc'` | `KEY=REDACTED` |
| URL credentials | `https://user:pass@host` | `https://REDACTED@host` |
| Bearer / token auth header value | `Bearer eyJhbGci...` | `Bearer REDACTED` |
| JWT (three base64url dot-separated segments) | `eyJ.eyJ.sig` | `[JWT]` |
| PEM block | `-----BEGIN PRIVATE KEY-----...` | `[PEM BLOCK]` |

Sensitive name keyword list (case-insensitive): `TOKEN`, `KEY`, `SECRET`, `PASSWORD`, `PASSWD`,
`PWD`, `PASS`, `AUTH`, `CRED`, `CREDENTIAL`, `API_KEY`, `PRIVATE`, `CERT`, `SIGNING`.

**tier 2 — high-entropy patterns**

| pattern | threshold | stored as |
| ------- | --------- | --------- |
| hex string | ≥ 32 contiguous hex chars | `[HEX 64B]` |
| base64 string | ≥ 24 chars, valid base64 alphabet | `[BASE64 44B]` |
| UUID | standard 8-4-4-4-12 format, always | `[UUID]` |

Byte counts in hex and base64 tokens reflect the byte length of the matched string.
The 32-char hex threshold preserves short git SHAs (≤ 12 chars) while catching full
SHA-256 outputs, HMAC values, and raw keys.

**tier 3 — PII**

| pattern | stored as |
| ------- | --------- |
| email address | `[EMAIL]` |
| public IP address | `[IP]` |

**tier 4 — user patterns (BENCH_MCP_STATS_REDACT_PATTERNS)**

Applied after tiers 1–3. Each pipe-separated regex is compiled at startup;
if any pattern fails to compile the server logs an error and skips that pattern.
Matches are replaced with `REDACTED`.

### pass 3 — long token normalization

Applied after redaction. Catches noise that pattern matching did not handle.
All byte counts use the byte length of the original matched text.

**catch-all: long tokens**

Any whitespace-delimited token longer than 80 bytes becomes `[LONG STRING 323B]`.
This handles deep paths, escaped strings, inline data, and anything else that
survived earlier passes but adds no stats value.

**shell structural constructs**

| construct | stored as |
| --------- | --------- |
| `$(...)` command substitution | `[SUBSHELL]` |
| `` `...` `` backtick | `[SUBSHELL]` |
| `<< 'EOF' ... EOF` heredoc (any delimiter) | `[HEREDOC 1247B]` |
| `<<< "..."` here-string | `[HERESTRING 42B]` |
| `<(...)` process substitution | `[PROCESS_SUB]` |

**inline scripts (interpreter `-c` / `-e` flag)**

Any interpreter invocation where `-c` or `-e` is followed by a quoted string:

| example | stored as |
| ------- | --------- |
| `python3 -c '...'` | `python3 -c [INLINE_SCRIPT 89B]` |
| `python -c "..."` | `python -c [INLINE_SCRIPT 89B]` |
| `perl -e '...'` | `perl -e [INLINE_SCRIPT 45B]` |
| `ruby -e '...'` | `ruby -e [INLINE_SCRIPT 45B]` |
| `node -e '...'` | `node -e [INLINE_SCRIPT 45B]` |
| `awk '{ print $1 }'` | `awk [INLINE_SCRIPT 14B]` |

**Python multiline string (inside `-c` arg)**

| pattern | stored as |
| ------- | --------- |
| `"""..."""` triple-quoted block | `[PYTHON_BLOCK 342B]` |

### example

Input:
```bash
TOKEN=secret git -C /projects/bench-mcp log --pretty=format:'%H %s' --author=user@example.com -n 20
```

After pipeline:
```
TOKEN=REDACTED git -C /projects/bench-mcp log --pretty=format:[INLINE_SCRIPT 6B] --author=[EMAIL] -n 20
```

`base_cmd = git`
`cmd_hash = SHA-256("TOKEN=REDACTED git -C /projects/bench-mcp log ...")`

Two runs with different token values produce the same hash. ✓

## `stats` tool

A new MCP tool. One optional parameter:

| parameter | type | default | description |
| --------- | ---- | ------- | ----------- |
| `days` | int | `30` | Rolling window in days. `0` means all time. |

Output is plain text, consistent with other tools. Example:

```
tool usage (last 30 days):
  shell              142 calls   avg 1.2s   p95 8.4s
  shell_background    38 calls   avg 45.2s
  file_replace        27 calls   avg 0.3s
  setup               12 calls
  context              8 calls

top commands by frequency:
  git   [a3f9c2]   89 calls   avg 2.1s   p95 12.3s   ← consider shell_background
  go    [b17d44]   31 calls   avg 5.4s   p95 47.1s   ← consider shell_background
  npm   [c90e11]   12 calls   avg 22.0s  p95 91.0s   ← consider shell_background
  cat   [f4a812]   18 calls   avg 0.1s
  grep  [221bc9]   15 calls   avg 0.2s

note: hash prefix shown — set BENCH_MCP_STATS_KEY to store and display full commands
```

The `note:` line is omitted when `BENCH_MCP_STATS_KEY` is set; full redacted commands
are shown instead of hash prefixes.

### shell_background hint

The `← consider shell_background` hint is appended when a command's p95 duration
exceeds `BENCH_MCP_TIMEOUT × 0.5`. This ties the hint directly to the configured
timeout — no extra env var needed.

## example queries

```sql
-- which project paths had the most file edits
SELECT file_path, COUNT(*) AS edits
FROM tool_calls
WHERE tool IN ('file_replace', 'file_replace_all')
GROUP BY file_path
ORDER BY edits DESC;

-- average and total replacement size per file
SELECT
    file_path,
    AVG(replacement_count) AS avg_items,
    SUM(replacement_count) AS total_items
FROM tool_calls
WHERE tool = 'file_replace'
GROUP BY file_path;

-- commands slow enough to warrant shell_background
SELECT
    base_cmd,
    COUNT(*) AS calls,
    CAST(AVG(duration_ms) AS INTEGER) AS avg_ms,
    MAX(duration_ms) AS max_ms
FROM tool_calls
WHERE tool = 'shell'
  AND called_at > datetime('now', '-30 days')
GROUP BY cmd_hash
ORDER BY max_ms DESC;

-- timed-out commands
SELECT base_cmd, cwd, called_at
FROM tool_calls
WHERE timed_out = 1
ORDER BY called_at DESC
LIMIT 20;
```
