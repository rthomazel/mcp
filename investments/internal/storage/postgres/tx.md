# Functions

## Store.withTx(ctx Context, fn func(tx *sqlx.Tx) error) error

1. Begin a transaction at read committed isolation.
2. Call fn with the transaction.
3. Commit.
4. Return nil.

#### Errors

- **1.** if beginning fails, return the wrapped error.

---

- **2.** if fn fails, roll back.
- **2.1.** if the rollback also fails, return both errors combined.
- **2.2.** otherwise return fn's error unchanged so callers can match it.

---

- **3.** if committing fails, return the wrapped error.

Read committed is sufficient: writers that need a consistent ledger take the instrument row lock before reading it.
Every Store method that touches more than one statement uses this, so partial writes never become visible.
