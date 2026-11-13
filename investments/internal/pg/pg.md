# Constants

pingAttempts = 5.
pingBackoff = 10 seconds, the database may not be reachable immediately on startup.
postgresUniqueViolation = "23505".
postgresForeignKeyViolation = "23503".
postgresCheckViolation = "23514".

# Functions

## Connect(ctx Context, dsn string) (*pgxpool.Pool, error)

1. Parse the DSN into pool settings.
2. Create the pool.
3. Ping, retrying up to pingAttempts with pingBackoff between attempts.
4. Return the pool.

#### Errors

- **1.** if parsing fails, return the wrapped error.

---

- **2.** if creating fails, return the wrapped error.

---

- **3.** if the last attempt fails, close the pool and return the wrapped error naming the attempt count.

## Migrate(dsn string) error

1. Open a migrator over db.Migrations with the pgx v5 database driver.
2. Apply all pending up migrations.
3. Close the migrator.
4. Return nil.

#### Errors

- **1.** if opening fails, return the wrapped error.

---

- **2.** if the only result is no change, treat as success.
- **2.1.** otherwise return the wrapped error.

golang-migrate holds a Postgres advisory lock while migrating, so concurrent starts are safe.
Only up migrations exist; there is no down path.

## WithTx(ctx Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error

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

## IsUniqueViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresUniqueViolation.

## IsForeignKeyViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresForeignKeyViolation.

## IsCheckViolation(err error) bool

Report whether err is a Postgres error with SQLSTATE postgresCheckViolation.
