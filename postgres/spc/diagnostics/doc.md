---
id: 2026-11-13-postgres-diagnostics
type: documentation
summary: Documents ping, explain, explain_analyze, active_connections, and active_locks.
author: Thom
created: 2026-11-13
agents: claude
---

# diagnostics

## Description

Operational tools. `ping` is always registered. The others expose query text, usernames, client addresses, and
workload details, so each is off by default.

## Tools

| tool | registered when | behavior |
| --- | --- | --- |
| `ping` | always | pings the pool, then returns server version and ping latency |
| `explain` | allow-diagnostics | runs `EXPLAIN` on the given SQL without executing it |
| `explain_analyze` | allow-explain-analyze | runs `EXPLAIN (ANALYZE, FORMAT TEXT)`, executing the statement |
| `active_connections` | allow-diagnostics | other sessions from `pg_stat_activity`: pid, user, application, client address, state, wait type, wait event, query start |
| `active_locks` | allow-diagnostics | waiting locks joined to the granted lock that blocks them: blocked pid and user, blocking pid and user, blocked query |

## explain and explain_analyze

The caller passes only the inner SQL; the tool builds the `EXPLAIN` statement. A leading `EXPLAIN` in the input is
rejected. This keeps `EXPLAIN ANALYZE` of a write from being routed through the read tool.

- `explain` runs outside an explicit transaction under the query timeout. The inner SQL is not passed through the SQL
  validation steps, so only the query timeout and the database user's grants bound what `EXPLAIN` can be given.
- `explain_analyze` runs in a transaction with a local statement timeout and **always rolls back**. When the inner statement's
  first token is a DML keyword, the allow-mutate flag is also required. DDL and DCL inner statements are not gated by
  their own flags here.
- Output is the plan, one line per row.

## active_connections and active_locks

- Both are capped at max rows and flag when capped.
- `active_connections` excludes the server's own backend and orders by query start.
- Empty results return "no active connections" / "no active locks found".

## Refs

- doc: 2026-11-13-postgres-config
- doc: 2026-11-13-postgres-sql-validation
