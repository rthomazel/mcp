# postgres-mcp

**Give your AI agent safe, structured access to a PostgreSQL database.**

[![License: BSD3](https://img.shields.io/badge/license-BSD3-green)](../LICENSE)

postgres-mcp is an MCP server that exposes a Postgres database to an AI agent through a set of well-defined tools covering schema introspection, query execution, batch mutations, transactions, and diagnostics. Written in Go. Single binary, minimal container.

---

## Tools

### Schema Introspection (always on)

| Tool | Description |
|---|---|
| `list_schemas` | All schemas in the database |
| `list_tables` | Tables in a schema |
| `describe_table` | Columns, types, nullability, defaults, comments |
| `list_indexes` | Index names, columns, uniqueness |
| `list_foreign_keys` | Foreign key constraints |
| `list_views` | Views and their definitions |
| `list_functions` | Stored procedures and functions |
| `table_stats` | Row count, live/dead tuples, last vacuum/analyze |
| `database_size` | Total DB size and per-table sizes |
| `search_schema` | Text search across table, column, and view names |
| `er_diagram` | Mermaid ERD from FK relationships |

### Query & Mutation

| Tool | SQL class | Flag required |
|---|---|---|
| `query` | SELECT, SHOW, TABLE, WITH | always on |
| `mutate` | INSERT, UPDATE, DELETE, TRUNCATE | `ALLOW_DML` |
| `mutate_schema` | CREATE, ALTER, DROP | `ALLOW_DDL` |
| `mutate_permissions` | GRANT, REVOKE | `ALLOW_DCL` |

### Transactions

| Tool | Description | Flag required |
|---|---|---|
| `mutate_batch` | Multiple statements in one transaction, atomic commit or rollback | `ALLOW_TRANSACTIONS` |
| `dry_run` | Execute and always roll back — safe preview of any mutation | `ALLOW_TRANSACTIONS` |

### Diagnostics

| Tool | Description | Flag required |
|---|---|---|
| `ping` | Version and connection latency | always on |
| `explain` | Query plan, no execution | `ALLOW_DIAGNOSTICS` |
| `explain_analyze` | Query plan with real stats, unconditionally rolled back | `ALLOW_EXPLAIN_ANALYZE` |
| `active_connections` | Connection states from `pg_stat_activity` | `ALLOW_DIAGNOSTICS` |
| `active_locks` | Blocking lock chains | `ALLOW_DIAGNOSTICS` |

---

## Configuration

All config is environment variables. Copy `.env-default` to `.env` and edit.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | _(required)_ | `postgres://user:pass@host:5432/dbname` |
| `POSTGRES_MCP_ALLOW_DML` | `false` | Enable mutate |
| `POSTGRES_MCP_ALLOW_DDL` | `false` | Enable mutate_schema |
| `POSTGRES_MCP_ALLOW_DCL` | `false` | Enable mutate_permissions |
| `POSTGRES_MCP_ALLOW_TRANSACTIONS` | `false` | Enable mutate_batch and dry_run |
| `POSTGRES_MCP_ALLOW_DIAGNOSTICS` | `false` | Enable explain, connections, locks |
| `POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE` | `false` | Enable explain_analyze |
| `POSTGRES_MCP_ALLOWED_SCHEMAS` | _(all)_ | Comma-separated introspection allowlist |
| `POSTGRES_MCP_DENIED_SCHEMAS` | `pg_toast,pg_catalog,information_schema` | Comma-separated introspection blocklist |
| `POSTGRES_MCP_MAX_ROWS` | `500` | Row cap per query |
| `POSTGRES_MCP_QUERY_TIMEOUT` | `30s` | Per-query timeout |
| `POSTGRES_MCP_POOL_SIZE` | `5` | Max connection pool size |
| `POSTGRES_MCP_TRANSPORT` | _(unset)_ | `mcpo` or `mcp-proxy` for HTTP mode |

**Security note:** tool-level flags are a first-line guard. The real enforcement boundary is the database user’s own grants. Connect with a restricted role.

---

## Quickstart

### stdio

```bash
cp postgres-mcp-sample.yaml postgres-mcp.yaml
# edit dsn and any allow_* flags
cp docker-compose-sample.yml docker-compose.yml
docker compose run --rm postgres-mcp
```

### HTTP

Set `POSTGRES_MCP_TRANSPORT=mcpo` (OpenAI-compatible) or `POSTGRES_MCP_TRANSPORT=mcp-proxy` (SSE) and expose port 8001.

---

## Development

```bash
# run locally (requires .env with DATABASE_URL)
./run dev  # requires postgres-mcp.yaml

# tests
./run test

# lint
./run lint

# format
./run format
```

See `doc/design.md` for architecture, tool design rationale, and known v1 limitations.

---

## License

BSD 3-Clause License
