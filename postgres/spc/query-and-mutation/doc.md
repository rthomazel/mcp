---
id: 2026-11-13-postgres-query-and-mutation
type: documentation
summary: Documents the query, mutate, mutate_schema, and mutate_permissions tools.
author: Thom
created: 2026-11-13
agents: claude
---

# query and mutation

## Description

Four single-statement tools, each taking one `sql` string. Each validates the statement against its class allowlist
(see [SQL validation](../sql-validation/doc.md)) and runs it with the configured statement timeout.
Empty `sql` is an error.

## Tools

| tool | class | registered when | transaction | success output |
| --- | --- | --- | --- | --- |
| `query` | DQL | always | read-only, always rolled back | tab-separated rows, or "query returned no results" |
| `mutate` | DML | allow-mutate | commit on success | "rows affected: N" |
| `mutate_schema` | DDL | allow-mutate-schema | commit on success | fixed success message |
| `mutate_permissions` | DCL | allow-mutate-permissions | commit on success | fixed success message |

Mutation tools return a "disabled" error naming the environment variable when their flag is off.

## Transaction handling

Each call opens its own transaction and sets a local statement timeout equal to the query timeout before the statement.
Any error rolls back. `query` never commits; it relies on the read-only transaction, so a data-modifying CTE under
`WITH` is rejected by PostgreSQL itself.

## Limits and caveats

- `query` returns at most max rows and flags when capped.
- A read-only transaction does not undo non-transactional effects such as advisory locks held by the pooled connection or
  side effects of volatile functions.
- Schema filtering does not apply to these tools; restrict access through the database user's grants.
- One statement per call. Multiple statements go through [transactions](../transactions/doc.md).

## Refs

- doc: 2026-11-13-postgres-sql-validation
- doc: 2026-11-13-postgres-config
