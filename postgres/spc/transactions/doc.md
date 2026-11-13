---
id: 2026-11-13-postgres-transactions
type: documentation
summary: Documents the mutate_batch and dry_run tools and statement classification.
author: Thom
created: 2026-11-13
agents: claude
---

# transactions

## Description

Two tools enabled by the allow-transactions flag. Both classify each statement by its first token, require the
class flag for that statement (DML, DDL, or DCL), then validate it against that class allowlist.
The transactions flag does not bypass the class flags.

Statements classified as DQL (`SELECT`, `SHOW`, `TABLE`, `WITH`) need no flag. Any other first token is rejected as
"not recognized"; an empty statement is rejected.

## mutate_batch

Parameter: `statements`, an array of strings.

- An empty array, or a non-string element, is an error.
- **All** statements are classified and validated before a transaction opens; the first failure aborts with the statement
  index and nothing runs.
- Statements then run in order in one transaction with a local statement timeout. Any failure rolls back everything
  and reports the failing statement index.
- Success commits and reports how many statements ran.
- Result rows of any `SELECT` in the batch are not returned.

## dry_run

Parameter: `sql`, one statement.

- Runs the statement in a transaction and **always rolls back**.
- The output is prefixed with a "dry run — changes rolled back" line.
- DQL statements return their rows, capped at max rows.
- DML statements report "would affect N rows".
- DDL and DCL statements report that the statement executed.

## Caveats

Rollback undoes transactional writes only. Sequence advances, notifications, and external effects of volatile functions
remain. Explicit transaction control (`BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`) is rejected.
There is no multi-call transaction; the server keeps no state between calls.

## Refs

- doc: 2026-11-13-postgres-sql-validation
- doc: 2026-11-13-postgres-query-and-mutation
