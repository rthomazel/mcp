---
id: 2026-11-13-postgres-architecture
type: documentation
summary: Documents the layout, startup, tool registration, and output conventions of postgres-mcp.
author: Thom
created: 2026-11-13
agents: claude
---

# architecture

## Description

`postgres-mcp` gives an agent structured access to one PostgreSQL database. It is a Go server using `pgx/v5` and
`mcp-go`, serving over stdio. Which tools exist depends on configuration flags.

## Layout

```
main.go                          startup, pool creation, ping retry, tool registration
internal/config.go               Config struct, env loading, defaults
internal/sqlcheck/sqlcheck.go    comment stripping, statement validation
handlers/handler.go              Handler, schema filter, allowlists, result formatting
handlers/introspect.go           schema introspection tools
handlers/query.go                query, mutate, mutate_schema, mutate_permissions
handlers/transaction.go          mutate_batch, dry_run
handlers/diagnostics.go          ping, explain, explain_analyze, active_connections, active_locks
bin/postgresmcphttp              container entrypoint selecting the transport
```

## Startup

1. Load config from environment; any error is fatal.
2. Create a `pgxpool` with a maximum of **one** connection.
3. Ping the database, up to 5 attempts 10 seconds apart; failure after the last attempt is fatal.
4. Register tools and serve on stdio.

## Tool registration

| tools | registered when |
| --- | --- |
| introspection tools, `query`, `ping` | always |
| `mutate` | allow-mutate flag |
| `mutate_schema` | allow-mutate-schema flag |
| `mutate_permissions` | allow-mutate-permissions flag |
| `mutate_batch`, `dry_run` | allow-transactions flag |
| `explain`, `active_connections`, `active_locks` | allow-diagnostics flag |
| `explain_analyze` | allow-explain-analyze flag |

Handlers for gated tools also re-check their flag and return a "disabled" error naming the variable.
See [config](../config/doc.md).

## Output conventions

- Responses are plain text. Tabular results are tab-separated with a header row.
- SQL NULL values render as an empty string; UUIDs render in canonical dashed form.
- Row-returning tools stop reading at `MAX_ROWS` rows and peek one row ahead; when more exist the output is prefixed with
  a note that results were capped and to use LIMIT/OFFSET.
- Empty results return a short tool-specific message instead of an empty table.
- Failures are MCP tool error results with a short prefix such as "validation error" or "execution error".

## Transports

Container entrypoint: unset → stdio; `POSTGRES_MCP_TRANSPORT=mcpo` → `mcpo` on port 8001; `mcp-proxy` → `mcp-proxy` on port 8001.

## Refs

- doc: 2026-11-13-postgres-config
- doc: 2026-11-13-postgres-sql-validation
- doc: 2026-11-13-postgres-introspection
- doc: 2026-11-13-postgres-query-and-mutation
- doc: 2026-11-13-postgres-transactions
- doc: 2026-11-13-postgres-diagnostics
