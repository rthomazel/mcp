# postgres-mcp — design

A Model Context Protocol (MCP) server that exposes a PostgreSQL database to an AI agent. Written in Go, using `pgx` for database access and `mcp-go` for the MCP layer — consistent with the `bench` server in this repo.

## reference

[twn39/pgsql-mcp-server](https://github.com/twn39/pgsql-mcp-server) is a Python/FastMCP implementation used as a reference. This server covers the same ground and extends it.

---

## tools

### group 1 — schema introspection

Always enabled. All queries run against `information_schema` and `pg_catalog` — never against user data. Schema filtering (see [configuration](#configuration)) applies to all tools in this group.

Introspection results can include sensitive metadata: view definitions, function bodies, column defaults, and comments may contain business logic or accidentally embedded secrets. This is a trust assumption — introspection is not gated separately, so only connect this server to databases whose metadata you are comfortable exposing to the agent.

| tool                                | description                                                                                                                                   |
| ----------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `list_schemas`                      | All schemas in the database                                                                                                                   |
| `list_tables(schema?)`              | Tables in a schema (default: `public`)                                                                                                        |
| `describe_table(table, schema?)`    | Columns, types, nullability, defaults, comments                                                                                               |
| `list_indexes(table, schema?)`      | Index names, columns, uniqueness                                                                                                              |
| `list_foreign_keys(table, schema?)` | Foreign key constraints                                                                                                                       |
| `list_views(schema?)`               | Views and their definitions                                                                                                                   |
| `list_functions(schema?)`           | Stored procedures and functions                                                                                                               |
| `table_stats(table, schema?)`       | Row count, live/dead tuples, last vacuum/analyze — from `pg_stat_user_tables`                                                                 |
| `database_size`                     | Total DB size (unfiltered — reflects the full database regardless of schema filters) and per-table sizes (filtered by allowed/denied schemas) |
| `search_schema(term)`               | Text search across table, column, and view names                                                                                              |
| `er_diagram(schema?)`               | Mermaid ERD built from FK relationships in `information_schema`                                                                               |

### group 2 — query & mutation

Gated by configuration flags. The real enforcement boundary is the database user's own grants — the server's keyword check is a first-line guard and a clear signal to the agent about intent, not a security guarantee.

| tool                      | SQL class                        | accepts                                  | flag        |
| ------------------------- | -------------------------------- | ---------------------------------------- | ----------- |
| `query(sql)`              | DQL — Data Query Language        | `SELECT`, `SHOW`, `TABLE`, `WITH`        | always on   |
| `mutate(sql)`             | DML — Data Manipulation Language | `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE` | `ALLOW_DML` |
| `mutate_schema(sql)`      | DDL — Data Definition Language   | `CREATE`, `ALTER`, `DROP`                | `ALLOW_DDL` |
| `mutate_permissions(sql)` | DCL — Data Control Language      | `GRANT`, `REVOKE`                        | `ALLOW_DCL` |

#### keyword validation

Before execution each tool:

1. Strips SQL comments (`--` line comments and `/* */` block comments)
2. Trims whitespace, normalizes to uppercase
3. Rejects input containing multiple statements (`;` followed by non-whitespace)
4. Rejects transaction-control keywords: `BEGIN`, `COMMIT`, `ROLLBACK`, `ROLLBACK TO`, `SAVEPOINT`, `RELEASE`, `START TRANSACTION`. These would break the server's own transaction wrapping. This check runs before the per-tool allowlist and applies to all execution tools including `mutate_batch` and `dry_run`.
5. Checks the first token against that tool's allowlist
6. Returns an error without touching the database if any check fails

`WITH` (CTEs) is in `query`'s allowlist — it is safe because `query` runs inside `BEGIN READ ONLY`, so PostgreSQL itself rejects any data-modifying CTE before it executes. CTEs are not supported in mutation tools in v1.

**v1 limitation**: the scanner does not fully handle dollar-quoted strings or complex quoted identifiers — edge cases with `;` or comment-like tokens inside string literals may be incorrectly rejected. Known constraint, not a security gap.

Schema filtering does **not** apply to mutation tools. Unqualified names, views, functions, and CTEs make pre-execution schema enforcement unreliable. Use a restricted database user for that boundary.

#### `query` transaction model

`query` always runs inside `BEGIN READ ONLY ... ROLLBACK`. This enforces the read-only intent at the transaction level and prevents row-level writes. It does not prevent all side effects — session-level advisory locks taken inside the transaction survive the rollback and remain held by the pooled connection. Volatile user-defined functions may also have external effects. The real read-only guarantee comes from the database user's grants, not the transaction mode.

### group 3 — transactions

Enabled by `ALLOW_TRANSACTIONS`.

| tool                         | description                                                                                                                                                                                                                                                                                                                 |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `mutate_batch(statements[])` | Runs multiple SQL statements in a single transaction. Commits on full success; rolls back and reports the failing statement on any error. Each statement is validated against the same keyword and class rules as the single-statement tools — `ALLOW_TRANSACTIONS` does not bypass DML/DDL/DCL flags.                      |
| `dry_run(sql)`               | Wraps the statement in a transaction, executes it, returns the result or affected row count, then **unconditionally rolls back**. Undoes transactional writes; does not undo sequence advances, `pg_notify` calls, or volatile function side effects. Requires the same class flag as the equivalent single-statement tool. |

Stateful `BEGIN`/`COMMIT` across separate MCP tool calls is not supported — the request/response model carries no session state between calls.

### group 4 — diagnostics

Enabled by `ALLOW_DIAGNOSTICS` (default: **false** — these tools expose query text, usernames, client addresses, and workload details). `ping` is always on regardless of this flag.

| tool                   | description                                                                                                                                                            |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ping`                 | Health check — server version and connection round-trip latency. Always enabled.                                                                                       |
| `explain(sql)`         | Accepts inner SQL, builds and runs `EXPLAIN <sql>` — no execution of the statement itself                                                                              |
| `explain_analyze(sql)` | Accepts inner SQL, builds and runs `EXPLAIN (ANALYZE, FORMAT TEXT) <sql>`. Requires `ALLOW_EXPLAIN_ANALYZE`. If the inner statement is DML, also requires `ALLOW_DML`. |
| `active_connections`   | Connection states and wait events from `pg_stat_activity`                                                                                                              |
| `active_locks`         | Blocking lock chains from `pg_locks` joined to `pg_stat_activity`                                                                                                      |

#### `explain` / `explain_analyze` tool-built SQL

The caller passes only the **inner SQL** — not an `EXPLAIN` statement. The tool constructs the full statement:

```
// caller: explain(sql: "SELECT * FROM appointments WHERE id = 1")
// tool builds: EXPLAIN SELECT * FROM appointments WHERE id = 1

// caller: explain_analyze(sql: "SELECT * FROM appointments WHERE id = 1")
// tool builds: EXPLAIN (ANALYZE, FORMAT TEXT) SELECT * FROM appointments WHERE id = 1
```

This closes the bypass vector where a caller could pass `EXPLAIN ANALYZE DELETE ...` to the always-on `query` tool. `query`'s allowlist is `SELECT`, `SHOW`, `TABLE`, `WITH` — `EXPLAIN` is not accepted there.

`explain_analyze` validates the inner statement's first token before wrapping it. If the inner statement is DML, `ALLOW_DML` is also required. `explain_analyze` always runs inside a transaction that is unconditionally rolled back — this prevents DML inner statements from committing while still producing the real execution plan.

---

## configuration

All config is env-var only — no flags, no config files. Pattern matches `bench`.

### access control

| variable                             | default | description                                                                 |
| ------------------------------------ | ------- | --------------------------------------------------------------------------- |
| `POSTGRES_MCP_ALLOW_DML`             | `false` | Enable `mutate` (INSERT, UPDATE, DELETE, TRUNCATE)                          |
| `POSTGRES_MCP_ALLOW_DDL`             | `false` | Enable `mutate_schema` (CREATE, ALTER, DROP)                                |
| `POSTGRES_MCP_ALLOW_DCL`             | `false` | Enable `mutate_permissions` (GRANT, REVOKE)                                 |
| `POSTGRES_MCP_ALLOW_TRANSACTIONS`    | `false` | Enable `mutate_batch` and `dry_run` (class flags still apply per statement) |
| `POSTGRES_MCP_ALLOW_DIAGNOSTICS`     | `false` | Enable explain, active_connections, active_locks                            |
| `POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE` | `false` | Enable `explain_analyze` (executes the inner query)                         |

### schema filtering (introspection only)

| variable                       | default                                  | description                                                                                                                        |
| ------------------------------ | ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| `POSTGRES_MCP_ALLOWED_SCHEMAS` | _(all)_                                  | Comma-separated allowlist — only these schemas appear in introspection tools                                                       |
| `POSTGRES_MCP_DENIED_SCHEMAS`  | `pg_toast,pg_catalog,information_schema` | Comma-separated blocklist — excluded from introspection results. Does not affect how introspection queries the catalog internally. |

### safety limits

| variable                     | default      | description                                                                                                                   |
| ---------------------------- | ------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| `DATABASE_URL`               | _(required)_ | PostgreSQL DSN (`postgres://user:pass@host:5432/dbname`)                                                                      |
| `POSTGRES_MCP_MAX_ROWS`      | `500`        | Row cap — rows are streamed and collection stops at this limit, not truncated after fetch                                     |
| `POSTGRES_MCP_QUERY_TIMEOUT` | `30s`        | Per-query timeout applied at the `context.WithTimeout` call site and via `SET LOCAL statement_timeout` inside the transaction |
| `POSTGRES_MCP_POOL_SIZE`     | `5`          | Max connection pool size                                                                                                      |
| `POSTGRES_MCP_TRANSPORT`     | _(unset)_    | `mcpo` or `mcp-proxy` for HTTP wrapping, same as bench                                                                        |

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
    query.go                query, mutate, mutate_schema, mutate_permissions
    transaction.go          mutate_batch, dry_run
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
2. **Keyword validation** — strip comments, reject transaction-control keywords, reject multi-statement, check first token. (Mutation tools only; `query` runs steps 1–4 but skips per-tool allowlist since it runs in `BEGIN READ ONLY`.)
3. **Execute** — via `pgxpool` with `context.WithTimeout`. Mutations and `query` run inside an explicit transaction with `SET LOCAL statement_timeout` inside it.
4. **Format** — plain text, tab-separated. NULLs rendered as `NULL`. Rows capped at `MAX_ROWS` by stopping collection, not post-fetch truncation.

### transaction model per tool

| tool                                              | transaction                                                                                                                    |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| `query`                                           | `BEGIN READ ONLY` → `SET LOCAL statement_timeout` → execute → `ROLLBACK`                                                       |
| `mutate` / `mutate_schema` / `mutate_permissions` | `BEGIN` → `SET LOCAL statement_timeout` → execute → `COMMIT` (or `ROLLBACK` on error)                                          |
| `mutate_batch`                                    | `BEGIN` → `SET LOCAL statement_timeout` → each statement in order → `COMMIT` or `ROLLBACK`                                     |
| `dry_run`                                         | `BEGIN` → `SET LOCAL statement_timeout` → execute → `ROLLBACK` (unconditional)                                                 |
| `explain`                                         | single query with `context.WithTimeout`, no explicit transaction                                                               |
| `explain_analyze`                                 | `BEGIN` → `SET LOCAL statement_timeout` → execute → `ROLLBACK` (unconditional — prevents DML inner statements from committing) |
| introspection / other diagnostics                 | single query with `context.WithTimeout`, no explicit transaction                                                               |

### `dry_run` caveat (documented to callers)

The tool description explicitly states: rolls back transactional writes only. Sequence values advanced during the run are not restored. `pg_notify` events fired during the run are not recalled. Volatile user-defined functions may have external side effects.

---

## test plan

See [test-plan.md](test-plan.md).

---

## what this is not

- Not a query builder or ORM
- Not a migration tool
- Not a replication or CDC tool
- Not a multi-database router

Scoped to one thing: give an AI agent safe, structured access to one Postgres database instance.
