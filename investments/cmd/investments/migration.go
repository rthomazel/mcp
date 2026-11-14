package main

import (
	"errors"
	"fmt"

	gomigrate "github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/rthomazel/mcp/investments/db"
	"github.com/rthomazel/mcp/investments/internal/storage/postgres"
)

// migrate applies pending up migrations over the store's own connection.
func migrate(store *postgres.Store) error {
	driver, err := migratepgx.WithInstance(store.DB(), &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	source, err := iofs.New(db.Migrations, "migrations")
	if err != nil {
		return fmt.Errorf("opening embedded migrations: %w", err)
	}

	migrator, err := gomigrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err := migrator.Up(); err != nil && !errors.Is(err, gomigrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}

	return nil
}
