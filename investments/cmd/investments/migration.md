# Functions

## migrate(store *postgres.Store) error

1. Wrap store.DB() in the golang-migrate Postgres driver.
2. Create a migrator over the embedded db.Migrations.
3. Apply all pending up migrations.
4. Return nil.

#### Errors

- **1.** if creating the driver fails, return the wrapped error.

---

- **2.** if creating the migrator fails, return the wrapped error.

---

- **3.** if the only result is no change, treat as success.
- **3.1.** otherwise return the wrapped error.

It reuses the store's connection so migrations run exactly as the server connects.
golang-migrate holds a Postgres advisory lock while migrating, so concurrent starts are safe.
Only up migrations exist; there is no down path.
