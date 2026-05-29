package internal

import (
	"database/sql"
	"fmt"
	"io"
	"io/fs"

	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// MigrateDB runs all pending up migrations from the embedded migrations FS.
func MigrateDB(db *sql.DB, migrations fs.FS) error {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}
	driver, err := newMigrateDriver(db)
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

type sqliteMigrateDriver struct {
	db *sql.DB
}

func newMigrateDriver(db *sql.DB) (database.Driver, error) {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER NOT NULL PRIMARY KEY,
			dirty   INTEGER NOT NULL
		)`)
	if err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	return &sqliteMigrateDriver{db: db}, nil
}

func (d *sqliteMigrateDriver) Open(_ string) (database.Driver, error) {
	return nil, fmt.Errorf("open by URL not supported")
}

func (d *sqliteMigrateDriver) Close() error { return nil }

func (d *sqliteMigrateDriver) Lock() error { return nil }

func (d *sqliteMigrateDriver) Unlock() error { return nil }

func (d *sqliteMigrateDriver) Run(r io.Reader) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	_, err = d.db.Exec(string(b))
	return err
}

func (d *sqliteMigrateDriver) SetVersion(version int, dirty bool) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var dirtyInt int
	if dirty {
		dirtyInt = 1
	}
	if version < 0 {
		_, err = tx.Exec("DELETE FROM schema_migrations")
	} else {
		_, err = tx.Exec(
			"INSERT OR REPLACE INTO schema_migrations (version, dirty) VALUES (?, ?)",
			version, dirtyInt,
		)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (d *sqliteMigrateDriver) Version() (version int, dirty bool, err error) {
	row := d.db.QueryRow(
		"SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1",
	)
	var dirtyInt int
	err = row.Scan(&version, &dirtyInt)
	if err == sql.ErrNoRows {
		return database.NilVersion, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return version, dirtyInt == 1, nil
}

func (d *sqliteMigrateDriver) Drop() error {
	return fmt.Errorf("drop not supported")
}
