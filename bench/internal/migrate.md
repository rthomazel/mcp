# SQL migrations

The package embeds the migration files for the stats database.

# Types

# Functions

## MigrateDB(db, migrations) error

1. Create the migration source from the embedded FS.
2. Build the driver, migrator, and run the up migrations.

## newMigrateDriver(db) (Driver, error)

1. Create the schema_migrations table.

## Driver.Open(_) (Driver, error)

1. Return an error stating that URL-open is not supported.

## Driver.Run(reader) error

1. Read the migration and execute it.

## Driver.Lock() error

1. No-op.

## Driver.Unlock() error

1. No-op.

## Driver.SetVersion(version, dirty) error

1. Insert or delete the version row inside a transaction.

## Driver.Version() (int, bool, error)

1. Scan the latest version, returning nil version when absent.

## Driver.Drop() error

1. Return an error stating that drop is not supported.

#### Rationale

- The driver records each version in schema_migrations so migrations run only once.
