# postgres-mcp — design

A Model Context Protocol (MCP) server that exposes a PostgreSQL database to an AI agent. Written in Go, using `pgx` for database access and `mcp-go` for the MCP layer — consistent with the `bench` server in this repo.

## reference

[twn39/pgsql-mcp-server](https://github.com/twn39/pgsql-mcp-server) is a Python/FastMCP implementation used as a reference. This server covers the same ground and extends it significantly.

---

## tools

### group 1 — schema introspection

Always enabled. All read-only queries against `information_schema` and `pg_catalog`.

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
| `search_schema(term)` | Text search across table, column, and view names — useful in large schemas |
| `er_diagram(schema?)` | Returns a Mermaid ERD built from FK relationships |

### group 2 — query execution

Gated by configuration flags (see [config](#configuration)). Each tool validates the leading SQL keyword before sending to the database — the tool boundary is a clear signal to the agent and a first-line check, but the DB user's own permissions are the real enforcement boundary.

| tool | SQL class | what it accepts | flag |
|---|---|---|---|
| `query(sql)` | DQL — Data Query Language | `SELECT`, `EXPLAIN`, `SHOW` | always on |
| `execute(sql)` | DML — Data Manipulation Language | `INSERT`, `UPDATE`, `DELETE` | `ALLOW_DML` |
| `execute_schema(sql)` | DDL — Data Definition Language | `CREATE`, `ALTER`, `DROP`, `TRUNCATE` | `ALLOW_DDL` |
| `execute_permissions(sql)` | DCL — Data Control Language | `GRANT`, `REVOKE` | `ALLOW_DCL` |

#### keyword validation

Before execution each tool:
1. Strips SQL comments (`--` line comments and `/* */` block comments)
2. Trims whitespace and normalizes to uppercase
3. Checks the first token against an allowlist for that tool
4. Returns an error without touching the database if the check fails

### group 3 — transactions

Enabled by `ALLOW_TRANSACTIONS`. Adds two tools that provide safe, atomic execution:

| tool | description |
|---|---|
| `execute_batch(statements[])` | Runs multiple SQL statements in a single transaction. Commits on full success; rolls back on any failure. Returns affected rows per statement. |
| `dry_run(sql)` | Wraps the statement in a transaction, executes it, returns the result or affected row count, then **always rolls back**. Safe preview of any destructive query. |

Stateful `BEGIN`/`COMMIT` across separate tool calls is not supported — MCP's request/response model does not carry session state between calls. `execute_batch` and `dry_run` cover the real use cases without that complexity.

### group 4 — diagnostics

Enabled by `ALLOW_DIAGNOSTICS` (default: true). Reads from `pg_stat_*` views and `pg_locks`.

| tool | description |
|---|---|
| `explain(sql)` | Returns `EXPLAIN` plan — no execution |
| `explain_analyze(sql)` | Returns `EXPLAIN ANALYZE` — executes the query. Separate flag: `ALLOW_EXPLAIN_ANALYZE` |
| `slow_queries(limit?)` | Top N queries by mean execution time from `pg_stat_statements` |
| `active_connections` | Connection states and wait events from `pg_stat_activity` |
| `active_locks` | Blocking lock chains from `pg_locks` joined to `pg_stat_activity` |
| `ping` | Health check — returns server version and connection round-trip latency |

---

## configuration

All config is env-var only — no flags, no config files. Pattern matches `bench`.

### access control

| variable | default | description |
|---|---|---|
| `POSTGRES_MCP_ALLOW_DML` | `false` | Enable `execute` (INSERT, UPDATE, DELETE) |
| `POSTGRES_MCP_ALLOW_DDL` | `false` | Enable `execute_schema` (CREATE, ALTER, DROP) |
| `POSTGRES_MCP_ALLOW_DCL` | `false` | Enable `execute_permissions` (GRANT, REVOKE) |
| `POSTGRES_MCP_ALLOW_TRANSACTIONS` | `false` | Enable `execute_batch` and `dry_run` |
| `POSTGRES_MCP_ALLOW_DIAGNOSTICS` | `true` | Enable slow_queries, locks, connections tools |
| `POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE` | `false` | Enable `explain_analyze` (actually executes the query) |

### schema filtering

| variable | default | description |
|---|---|---|
| `POSTGRES_MCP_ALLOWED_SCHEMAS` | _(all)_ | Comma-separated allowlist — only these schemas are visible to any tool |
| `POSTGRES_MCP_DENIED_SCHEMAS` | `pg_toast,pg_catalog` | Comma-separated blocklist — always excluded |

Schema filtering applies to introspection tools and to query execution — a query targeting a denied schema is rejected before execution.

### safety limits

| variable | default | description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | PostgreSQL DSN (`postgres://user:pass@host:5432/dbname`) |
| `POSTGRES_MCP_MAX_ROWS` | `500` | Row cap applied to all query results — protects context window |
| `POSTGRES_MCP_QUERY_TIMEOUT` | `30s` | Per-query timeout via `SET LOCAL statement_timeout` |
| `POSTGRES_MCP_POOL_SIZE` | `5` | Max connection pool size |
| `POSTGRES_MCP_TRANSPORT` | _(unset)_ | `mcpo` or `mcp-proxy` for HTTP wrapping, same as bench |

---

## implementation

### language and libraries

- **Go** — matches bench, single binary, small container footprint
- **`pgx/v5`** — idiomatic Go Postgres driver with native connection pooling (`pgxpool`)
- **`mcp-go`** — same MCP library as bench
- Introspection tools query `information_schema` and `pg_catalog` directly — no ORM

### layout (proposed)

```
postgres/
  main.go                 server wiring, tool registration, startup
  internal/
    config.go             Config struct, env var loading, defaults
    query/
      validate.go         keyword stripping and first-token checks
      validate_test.go
  handlers/
    handler.go            Handler struct, shared db pool
    introspect.go         list_schemas, list_tables, describe_table, etc.
    query.go              query, execute, execute_schema, execute_permissions
    transaction.go        execute_batch, dry_run
    diagnostics.go        explain, slow_queries, active_connections, active_locks, ping
  doc/
    design.md             this file
    config.md
    tools.md
  Dockerfile
  docker-compose-sample.yml
  go.mod
  go.sum
  AGENTS.md
```

### request flow

All tools follow the same path:
1. Config check — is this tool enabled? If not, return a clear error message.
2. Schema filter — does the query/table target an allowed schema?
3. Keyword validation — does the leading SQL token match what this tool accepts? (query/execute/execute_schema/execute_permissions only)
4. Execute via `pgxpool` with a statement timeout set via `SET LOCAL`
5. Format result as plain text table (tab-separated) and return

### dry_run detail

```
BEGIN
  SET LOCAL statement_timeout = '<QUERY_TIMEOUT>';
  <user sql>
  -- capture rows / rowcount
ROLLBACK  -- always, unconditionally
```

The rollback is unconditional — even if execution succeeded. This makes `dry_run` safe to call on any mutation.

### er_diagram detail

Builds a Mermaid `erDiagram` block by querying FK relationships from `information_schema`. Returns plain text the LLM (and ChatUI) renders natively. No external tooling required.

---

## what this is not

- Not a query builder or ORM
- Not a migration tool
- Not a replication or CDC tool
- Not a multi-database router

The server is deliberately scoped: give an AI agent safe, structured access to one Postgres database instance.
