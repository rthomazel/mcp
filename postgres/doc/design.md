# postgres-mcp — design

A Model Context Protocol (MCP) server that exposes a PostgreSQL database to an AI agent. Written in Go, using `pgx` for database access and `mcp-go` for the MCP layer — consistent with the `bench` server in this repo.

## reference

[twn39/pgsql-mcp-server](https://github.com/twn39/pgsql-mcp-server) is a Python/FastMCP implementation used as a reference. This server covers the same ground and extends it.

---

## tools

### group 1 — schema introspection

Always enabled. All queries run against `information_schema` and `pg_catalog` — never against user data. Schema filtering (see [configuration](#configuration)) applies to all tools in this group.

| tool | description |
|---|---|
| `list_schemas` | All schemas in the database |
| `list_tables(schema?)` | Tables in a schema (default: `public`) |
| `describe_table(table, schema?)` | Columns, types, nullability, defaults, comments |
| `list_indexes(table, schema?)` | Index names, columns, uniqueness |
| `list_foreign_keys(table, schema?)` | Foreign key constraints |
| `list_views(schema?)` | Views and their definitions |
| `list_functions(schema?)` | Stored procedures and functions |
| `table_stats(table, schema?)` | Row count, live/dead tuples, last vacuum/analyze — from `pg_stat_user_tables` |
| `database_size` | Total DB size and per-table sizes |
| `search_schema(term)` | Text search across table, column, and view names |
| `er_diagram(schema?)` | Mermaid ERD built from FK relationships in `information_schema` |

### group 2 — query execution

Gated by configuration flags. The real enforcement boundary is the database user's own grants — the server's keyword check is a first-line guard and a clear signal to the agent about intent, not a security guarantee.

| tool | SQL class | accepts | flag |
|---|---|---|---|
| `query(sql)` | DQL — Data Query Language | `SELECT`, `SHOW`, `TABLE` | always on |
| `execute(sql)` | DML — Data Manipulation Language | `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE` | `ALLOW_DML` |
| `execute_schema(sql)` | DDL — Data Definition Language | `CREATE`, `ALTER`, `DROP` | `ALLOW_DDL` |
| `execute_permissions(sql)` | DCL — Data Control Language | `GRANT`, `REVOKE` | `ALLOW_DCL` |

#### keyword validation

Before execution each tool:
1. Strips SQL comments (`--` line comments and `/* */` block comments)
2. Trims whitespace, normalizes to uppercase
3. Rejects input containing multiple statements (`;` followed by non-whitespace)
4. Checks the first token against that tool's allowlist
5. Returns an error without touching the database if any check fails

Schema filtering does **not** apply to execution tools. Unqualified names, views, functions, and CTEs make pre-execution schema enforcement unreliable. Use a restricted database user for that boundary.

#### `query` transaction model

`query` always runs inside `BEGIN READ ONLY ... ROLLBACK`. This prevents volatile functions or advisory lock calls from producing lasting side effects, and ensures the read-only intent is enforced at the transaction level in addition to the keyword check.

### group 3 — transactions

Enabled by `ALLOW_TRANSACTIONS`.

| tool | description |
|---|---|
| `execute_batch(statements[])` | Runs multiple SQL statements in a single transaction. Commits on full success; rolls back and reports the failing statement on any error. Each statement is validated against the same keyword and class rules as the single-statement tools — `ALLOW_TRANSACTIONS` does not bypass DML/DDL/DCL flags. |
| `dry_run(sql)` | Wraps the statement in a transaction, executes it, returns the result or affected row count, then **unconditionally rolls back**. Undoes transactional writes; does not undo sequence advances, `pg_notify` calls, or volatile function side effects. Rejects transaction-control keywords (`BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`). Requires the same class flag as the equivalent single-statement tool. |

Stateful `BEGIN`/`COMMIT` across separate MCP tool calls is not supported — the request/response model carries no session state between calls.

### group 4 — diagnostics

Enabled by `ALLOW_DIAGNOSTICS` (default: **false** — these tools expose query text, usernames, client addresses, and workload details). `ping` is always on regardless of this flag.

| tool | description |
|---|---|
| `ping` | Health check — server version and connection round-trip latency. Always enabled. |
| `explain(sql)` | Accepts inner SQL, builds and runs `EXPLAIN <sql>` — no execution of the statement itself |
| `explain_analyze(sql)` | Accepts inner SQL, builds and runs `EXPLAIN (ANALYZE, FORMAT TEXT) <sql>`. Requires `ALLOW_EXPLAIN_ANALYZE`. If the inner statement is DML, also requires `ALLOW_DML`. |
| `active_connections` | Connection states and wait events from `pg_stat_activity` |
| `active_locks` | Blocking lock chains from `pg_locks` joined to `pg_stat_activity` |

#### `explain` / `explain_analyze` tool-built SQL

The caller passes only the **inner SQL** — not an `EXPLAIN` statement. The tool constructs the full statement:

```
// caller: explain(sql: "SELECT * FROM appointments WHERE id = 1")
// tool builds: EXPLAIN SELECT * FROM appointments WHERE id = 1

// caller: explain_analyze(sql: "SELECT * FROM appointments WHERE id = 1")
// tool builds: EXPLAIN (ANALYZE, FORMAT TEXT) SELECT * FROM appointments WHERE id = 1
```

This closes the bypass vector where a caller could pass `EXPLAIN ANALYZE DELETE ...` to the always-on `query` tool. `query`'s allowlist is `SELECT`, `SHOW`, `TABLE` only — `EXPLAIN` is not accepted there.

`explain_analyze` validates the inner statement's first token before wrapping it. If the inner statement is DML, `ALLOW_DML` is also required.

---

## configuration

All config is env-var only — no flags, no config files. Pattern matches `bench`.

### access control

| variable | default | description |
|---|---|---|
| `POSTGRES_MCP_ALLOW_DML` | `false` | Enable `execute` (INSERT, UPDATE, DELETE, TRUNCATE) |
| `POSTGRES_MCP_ALLOW_DDL` | `false` | Enable `execute_schema` (CREATE, ALTER, DROP) |
| `POSTGRES_MCP_ALLOW_DCL` | `false` | Enable `execute_permissions` (GRANT, REVOKE) |
| `POSTGRES_MCP_ALLOW_TRANSACTIONS` | `false` | Enable `execute_batch` and `dry_run` (class flags still apply per statement) |
| `POSTGRES_MCP_ALLOW_DIAGNOSTICS` | `false` | Enable explain, active_connections, active_locks |
| `POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE` | `false` | Enable `explain_analyze` (executes the inner query) |

### schema filtering (introspection only)

| variable | default | description |
|---|---|---|
| `POSTGRES_MCP_ALLOWED_SCHEMAS` | _(all)_ | Comma-separated allowlist — only these schemas appear in introspection tools |
| `POSTGRES_MCP_DENIED_SCHEMAS` | `pg_toast,pg_catalog` | Comma-separated blocklist — excluded from introspection results. Does not affect how introspection queries the catalog internally. |

### safety limits

| variable | default | description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | PostgreSQL DSN (`postgres://user:pass@host:5432/dbname`) |
| `POSTGRES_MCP_MAX_ROWS` | `500` | Row cap — rows are streamed and collection stops at this limit, not truncated after fetch |
| `POSTGRES_MCP_QUERY_TIMEOUT` | `30s` | Per-query timeout applied at the `context.WithTimeout` call site and via `SET LOCAL statement_timeout` inside the transaction |
| `POSTGRES_MCP_POOL_SIZE` | `5` | Max connection pool size |
| `POSTGRES_MCP_TRANSPORT` | _(unset)_ | `mcpo` or `mcp-proxy` for HTTP wrapping, same as bench |

---

## implementation

### language and libraries

- **Go** — matches bench, single binary, small container
- **`pgx/v5`** — idiomatic Go Postgres driver with native pooling (`pgxpool`)
- **`mcp-go`** — same MCP library as bench
- Introspection queries `information_schema` and `pg_catalog` directly — no ORM

### layout

```
postgres/
  main.go                   server wiring, tool registration, startup
  internal/
    config.go               Config struct, env var loading, defaults
    sqlcheck/
      sqlcheck.go           comment stripping, multi-statement rejection, first-token validation
      sqlcheck_test.go
  handlers/
    handler.go              Handler struct, shared pgxpool
    introspect.go           list_schemas, list_tables, describe_table, etc.
    query.go                query, execute, execute_schema, execute_permissions
    transaction.go          execute_batch, dry_run
    diagnostics.go          explain, explain_analyze, active_connections, active_locks, ping
  doc/
    design.md               this file
  Dockerfile
  docker-compose-sample.yml
  go.mod
  go.sum
  AGENTS.md
```

### request flow

All tools follow the same path:
1. **Config check** — is this tool enabled? Return a descriptive error if not.
2. **Keyword validation** — strip comments, reject multi-statement, check first token. (Execution tools only.)
3. **Execute** — via `pgxpool` with `context.WithTimeout`. Mutations and `query` run inside an explicit transaction with `SET LOCAL statement_timeout` inside it.
4. **Format** — plain text, tab-separated. NULLs rendered as `NULL`. Rows capped at `MAX_ROWS` by stopping collection, not post-fetch truncation.

### transaction model per tool

| tool | transaction |
|---|---|
| `query` | `BEGIN READ ONLY` → `SET LOCAL statement_timeout` → execute → `ROLLBACK` |
| `execute` / `execute_schema` / `execute_permissions` | `BEGIN` → `SET LOCAL statement_timeout` → execute → `COMMIT` (or `ROLLBACK` on error) |
| `execute_batch` | `BEGIN` → `SET LOCAL statement_timeout` → each statement in order → `COMMIT` or `ROLLBACK` |
| `dry_run` | `BEGIN` → `SET LOCAL statement_timeout` → execute → `ROLLBACK` (unconditional) |
| introspection / diagnostics | single query with `context.WithTimeout`, no explicit transaction |

### `dry_run` caveat (documented to callers)

The tool description explicitly states: rolls back transactional writes only. Sequence values advanced during the run are not restored. `pg_notify` events fired during the run are not recalled. Volatile user-defined functions may have external side effects.

---

## what this is not

- Not a query builder or ORM
- Not a migration tool
- Not a replication or CDC tool
- Not a multi-database router

Scoped to one thing: give an AI agent safe, structured access to one Postgres database instance.
