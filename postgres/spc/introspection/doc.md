---
id: 2026-11-13-postgres-introspection
type: documentation
summary: Documents the always-on schema introspection tools of postgres-mcp.
author: Thom
created: 2026-11-13
agents: claude
---

# introspection

## Description

Introspection tools read `information_schema` and `pg_catalog`, never user table data. They are always registered.
Results can expose view definitions, defaults, and comments, so connect only databases whose metadata the agent may see.

## Schema filtering

Tools taking a `schema` parameter (default: the default schema) return an error `schema "x" is not allowed` when
the schema fails the filter. Tools without a schema parameter drop rows from filtered schemas after querying.
See [config](../config/doc.md).

## Tools

| tool | parameters | output |
| --- | --- | --- |
| `list_schemas` | none | allowed schema names, one per line |
| `describe_schema` | `schema?` | per table or view: name, type, row estimate, total size, comment. Intended as the first call of a session. |
| `list_tables` | `schema?` | table name and type |
| `describe_table` | `table`, `schema?` | column, type, nullable, default, comment |
| `list_indexes` | `table`, `schema?` | index name, definition, unique flag |
| `list_foreign_keys` | `table`, `schema?` | constraint, column, referenced schema, table, column |
| `list_views` | `schema?` | view name and definition |
| `list_functions` | `schema?` | routine name, kind, return type |
| `table_stats` | `table`, `schema?` | live and dead rows, last vacuum, autovacuum, analyze, autoanalyze (or `never`) |
| `database_size` | none | total database size, then per-table sizes largest first |
| `search_schema` | `term` | kind (table, column, view), schema, table, column, type |
| `er_diagram` | `schema?` | Mermaid `erDiagram` of foreign key relationships |

## Behavior notes

- `table` is required for the per-table tools; a missing table yields a "not found" or "no ... found" message, not an error.
- `search_schema` matches names case-insensitively by substring; for columns it also matches the data type name.
- `database_size`: the total is for the whole database regardless of filters; the table list is filtered and capped at max rows.
- `search_schema` is capped at max rows. `table_stats` is a single row. The other tools return all rows without a cap.
- `list_indexes` marks an index unique when its definition text contains "UNIQUE".
- `er_diagram` emits one line per foreign key column, `referencing_table ||--o{ referenced_table : "column -> referenced_column"`, using the Mermaid one-to-many notation with the referencing table on the left.

## Refs

- doc: 2026-11-13-postgres-config
- doc: 2026-11-13-postgres-architecture
