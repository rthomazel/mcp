# ideas

- [ ] Go dependency update cron job workflow copy from other projects.

## concurrent context

`context` tool runs subprocesses serially.
Could run them with goroutines and be meaningfully faster.

## per-command timeout

Timeout is global via `BENCH_MCP_TIMEOUT`.
Letting `shell` accept an optional `timeout` param would be useful for known slow commands.

## sqlite db with command stats

See [stats-design.md](stats-design.md) for the full design.

# what not to add

- filesystem MCP tool — redundant, shell already does cat/ls/cp/find
- command allowlists — defeats the purpose, Docker is the boundary

## indented xml output

`xmlBuilder` has no depth tracking. A `depth int` field incremented by `openTag` /
decremented by `closeTag` would let all write methods prepend indentation.
Metadata fields written directly via `WriteString` would need a `b.line(s)` helper
to respect depth — a wider refactor touching all handlers.

## path snapshot registration file

Setup scripts could write a `.bench-mcp-extras` file in the project root —
one `name: /path/to/binary` pair per line. `context` reads all such files under
known project roots and surfaces them alongside the `auto-detected in path:` block.
Explicit opt-in, works for non-PATH installs, but requires setup scripts to be
authored with the convention.

## command variables

We could have a variable, a plaintext constant that the model can provide into the shell maybe or the github tool and it will get replaced by the mcp with the actual token.
That way we keep the token out of the context, and don't overbuild this.
Actually doing this in bench MCP would be so easy. There could be a new tool called load variable and then the model passes in a string. It can even pick whatever string it wants. It also passes the environment variable to read in the server to retrieve the value. The server could reply with messages variable is not set, variable is empty and those would be errors but if the variable is set and not empty it would reply with a success value and the checksum of the value.
Having the checksum gives the model the ability to use the value without knowing the value.
Then the model can use the other tools in any way that it wants. When the server sees a string compatible with a variable, it replaces in the token before executing.
We should probably have some type of exclusive string so that we can catch unset variables in commands and return an error and refuse to process the command.
We could also have a tool to list the variables that are currently loaded in the server.
A design question is, should this live in the bench or be a separate MCP tool? This design kind of couples the variable idea to another tool that executes commands. So I think this has to be done in the bench.
The obvious upside is that it's so easy and simple.

## integration test suite

The project has no integration tests. Unit tests cover pure helpers; the full
request path (MCP tool call → handler → disk/process → response) is untested.

### scope

Each MCP tool should have at least one integration test that exercises its handler
end-to-end: parse params, execute the operation, assert on the returned text and
any side effects (file contents, job state, etc.).

### approach

- Tests live in `handlers/` as `_test.go` files in `package handlers` (white-box,
  so inner functions are accessible).
- A small `testutil_test.go` provides shared helpers: temp file creation, temp dir
  scaffolding, a fake `Config`, and a `Handler` factory.
- All tests use `t.TempDir()` for filesystem isolation and clean up automatically.
- Use `t.Parallel()` cautiously — file-lock tests must not share paths.

### per-tool cases

**shell / shell_background / status**

- Basic command execution, stdout/stderr captured correctly
- Non-zero exit code returned without error
- CWD respected
- Background job transitions from pending to done; status reflects it
- Timeout fires and is surfaced

**context**

- Returns expected metadata fields (os, arch, disk, path)

**setup**

- Detects manifest file and launches the right command (can mock by injecting a no-op rule)

**file_replace**

- Basic single-item replace, returns unified diff, file updated
- Batch: multiple non-overlapping items applied in correct order
- `line_number` narrows an ambiguous match
- Overlap detection across items returns error, file unchanged
- Zero matches returns error with diagnostic
- Dry-run: diff returned, file not modified
- External-modification detection: file changed between read and write, returns abort error
- Symlink: path is a symlink to a regular file, write targets real file
- Binary file: rejected before any write
- Permissions preserved after atomic write

**file_replace_all**

- Replaces all occurrences, file updated, diff returned
- `start_line`/`end_line` scope: matches outside range not replaced
- Multi-line match crossing scope boundary not replaced
- Zero matches within scope returns error with range excerpt
- Dry-run, external-modification, symlink, binary — same as file_replace

### tricky cases

- **External-modification test**: requires modifying the file after `openFileForEdit`
  acquires the lock but before `commit` re-reads. Simplest approach: expose a
  `testHookAfterRead func()` on `editedFile` set only in tests, called just before
  the SHA-256 recheck.
- **Permissions**: create file at a non-default mode (e.g. 0o644 vs 0o600), assert
  mode is preserved after the atomic rename.

### maintenance notes

- Integration tests touch the real filesystem and spawn real subprocesses — they
  will be slower than unit tests. Keep them in a separate build tag (`//go:build integration`)
  or accept the cost and run them in CI alongside unit tests.
- Avoid sharing temp directories across parallel subtests; each subtest gets its own
  `t.TempDir()`.
- Flaky timeout tests can be avoided by using very short commands (e.g. `true`) and
  asserting on zero exit rather than timing behaviour.
