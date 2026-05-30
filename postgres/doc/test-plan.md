# test plan

Unit tests cover logic that is complex, stateful, or has edge cases that matter for correctness and security. Tests that require a live database are integration tests and are out of scope for the unit layer.

## already tested

`internal/sqlcheck` — `StripComments`, `FirstToken`, `Validate` are fully covered in `sqlcheck_test.go`.

## worth adding

### `internal/config` — `LoadConfig`

Tests the env-var parsing pipeline. Covers:
- All defaults are applied when no env vars are set (except `DATABASE_URL`)
- Missing `DATABASE_URL` returns an error
- Each duration var (`QUERY_TIMEOUT`) rejects invalid values and accepts valid Go duration strings
- Each integer var (`POOL_SIZE`, `MAX_ROWS`) rejects non-integers and values < 1
- Each bool var (`ALLOW_DML`, etc.) rejects invalid values and accepts `true`/`false`/`1`/`0`
- Comma-separated schema lists are split and trimmed correctly (including whitespace around commas)
- Setting `DENIED_SCHEMAS` overrides the default list entirely (not appended)

### `handlers` — `schemaAllowed`

The allowlist/denylist interaction has four cases worth pinning:
- No allowlist, no denylist: all schemas pass
- Allowlist set: only listed schemas pass, others are rejected
- Denylist set: listed schemas are rejected, others pass
- Both set: schema must be in allowlist AND not in denylist

These are pure logic tests with no DB dependency — just construct a `Handler` with a mock `Config` and call `schemaAllowed`.

### `handlers` — `sqlClassify`

The token → allowlist/flag mapping in `transaction.go`. Covers:
- Each known DML token maps to `dmlAllowlist` and `AllowDML`
- Each known DDL token maps to `ddlAllowlist` and `AllowDDL`
- Each known DCL token maps to `dclAllowlist` and `AllowDCL`
- DQL tokens map to `dqlAllowlist` with no flag requirement
- An empty statement returns an error
- An unrecognized token returns an error
- Tokens are case-insensitive (input `insert` maps same as `INSERT`)

### `handlers` — `formatTable`

The output formatting function. Covers:
- Empty rows returns just the header line
- Single row produces two lines, tab-separated
- Multi-row output has correct line count
- Empty string cells render as empty (not `<nil>` or `NULL`)
- Tab characters in values are passed through as-is (no escaping — known v1 behavior)

## not worth unit testing

- Any handler method that issues SQL (`HandleQuery`, `HandleListTables`, etc.) — these require a live DB and belong in an integration test suite
- Transaction begin/commit/rollback patterns — correctness is a property of pgx + Postgres, not our code
- `main.go` wiring — startup integration test if at all
