---
id: 2026-11-13-postgres-sql-validation
type: documentation
summary: Documents the pre-execution SQL checks and statement class allowlists in postgres-mcp.
author: Thom
created: 2026-11-13
agents: claude
---

# SQL validation

## Description

Before execution, SQL passed to `query`, the mutation tools, `mutate_batch`, and `dry_run` goes through a lexical
check. It is a first-line guard and a signal of intent, not a security boundary; the database user's grants are the boundary.

## Steps

1. Strip `--` line comments and `/* */` block comments, then trim whitespace.
2. Reject if a semicolon appears anywhere except as the last non-whitespace character ("multiple statements").
3. Take the first whitespace-delimited token, uppercased. Reject transaction-control tokens:
   `BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `RELEASE`, `START`.
4. Reject if the token is not in the calling tool's allowlist.
5. Return the statement with exactly one trailing semicolon.

## Allowlists by class

| class | first tokens |
| --- | --- |
| DQL | `SELECT`, `SHOW`, `TABLE`, `WITH` |
| DML | `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE` |
| DDL | `CREATE`, `ALTER`, `DROP` |
| DCL | `GRANT`, `REVOKE` |

`EXPLAIN` is in no allowlist. The mutation tools accept only their own class.

## Known limitations

- Dollar-quoted strings and complex quoted identifiers are not understood. A semicolon or comment marker inside a
  string literal can cause a valid statement to be rejected.
- Only the first token is checked. `WITH` is accepted by `query` because it runs read-only; the mutation tools do not
  accept `WITH`.
- Validation does not look at schemas or object names.

## Refs

- doc: 2026-11-13-postgres-query-and-mutation
- doc: 2026-11-13-postgres-transactions
