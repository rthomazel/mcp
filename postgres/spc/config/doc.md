---
id: 2026-11-13-postgres-config
type: documentation
summary: Documents the environment variables, defaults, and validation of postgres-mcp.
author: Thom
created: 2026-11-13
agents: claude
---

# config

## Description

Configuration is environment variables only. Boolean variables are true only when the value equals `true`
(case-insensitive); anything else, including unset, is false.

## Variables

| variable | default | meaning |
| --- | --- | --- |
| `POSTGRES_MCP_DSN` | required | PostgreSQL connection string; startup fails if empty |
| `POSTGRES_MCP_DEFAULT_SCHEMA` | `public` | schema used when a tool call omits one |
| `POSTGRES_MCP_ALLOW_MUTATE` | false | enables `mutate` (DML) and DML in batch, dry-run, explain-analyze |
| `POSTGRES_MCP_ALLOW_MUTATE_SCHEMA` | false | enables `mutate_schema` (DDL) |
| `POSTGRES_MCP_ALLOW_MUTATE_PERMISSIONS` | false | enables `mutate_permissions` (DCL) |
| `POSTGRES_MCP_ALLOW_TRANSACTIONS` | false | enables `mutate_batch` and `dry_run` |
| `POSTGRES_MCP_ALLOW_DIAGNOSTICS` | false | enables `explain`, `active_connections`, `active_locks` |
| `POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE` | false | enables `explain_analyze` |
| `POSTGRES_MCP_ALLOWED_SCHEMAS` | all | comma-separated allowlist for introspection |
| `POSTGRES_MCP_DENIED_SCHEMAS` | `pg_toast,pg_catalog,information_schema` | comma-separated denylist for introspection |
| `POSTGRES_MCP_MAX_ROWS` | 100 | row cap for row-returning tools; non-positive values become the default |
| `POSTGRES_MCP_QUERY_TIMEOUT` | 30s | Go duration; per-call timeout and statement timeout |
| `POSTGRES_MCP_TRANSPORT` | unset | read by the container entrypoint only |

Invalid duration or integer values are fatal at startup. Setting a denylist replaces the default denylist entirely.
List entries are not trimmed of whitespace.

## Schema filtering

A schema passes if the allowlist is empty or contains it, and the denylist does not contain it.
Filtering applies to introspection tools only; see [introspection](../introspection/doc.md).
It does not apply to SQL sent through `query` or the mutation tools.

## Connection pool

The pool is fixed at one connection; there is no pool size setting. Calls are therefore serialized per process.

## Refs

- doc: 2026-11-13-postgres-architecture
