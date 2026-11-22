package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	pingAttempts = 5
	pingBackoff  = 10 * time.Second

	postgresUniqueViolation     = "23505"
	postgresForeignKeyViolation = "23503"
	postgresCheckViolation      = "23514"
)

var _ domain.Store = (*Store)(nil)

// Store implements domain.Store; its methods are spread across files by concern.
type Store struct {
	db               *sqlx.DB
	stalePriceDays   int
	marketLocation   *time.Location
	maxHistoryPoints int
	events           domain.EventPolicy
	clock            func() time.Time // replaceable in tests
}

// Settings are the store's tunables, taken from the server configuration.
type Settings struct {
	StalePriceDays   int
	MarketTimezone   string
	MaxHistoryPoints int
	Events           domain.EventPolicy
}

// Open connects to the database and returns the store and a cleanup function.
func Open(dsn string, settings Settings) (*Store, func() error, error) {
	connConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing dsn: %w", err)
	}

	handle := sqlx.NewDb(stdlib.OpenDB(*connConfig), "pgx")

	for attempt := range pingAttempts {
		if err = handle.PingContext(context.Background()); err == nil {
			break
		}

		if attempt == pingAttempts-1 {
			_ = handle.Close()

			return nil, nil, fmt.Errorf("pinging database after %d attempts: %w", pingAttempts, err)
		}

		slog.Warn("ping failed, retrying", "attempt", attempt+1, "backoff", pingBackoff, "err", err)
		time.Sleep(pingBackoff)
	}

	location, err := time.LoadLocation(settings.MarketTimezone)
	if err != nil {
		_ = handle.Close()

		return nil, nil, fmt.Errorf("loading timezone %s: %w", settings.MarketTimezone, err)
	}

	store := &Store{
		db: handle, stalePriceDays: settings.StalePriceDays, marketLocation: location,
		maxHistoryPoints: settings.MaxHistoryPoints, events: settings.Events, clock: time.Now,
	}

	return store, handle.Close, nil
}

// DB returns the underlying handle, used by the migration runner.
func (s *Store) DB() *sql.DB {
	return s.db.DB
}

func isUniqueViolation(err error) bool {
	return hasSQLState(err, postgresUniqueViolation)
}

func isForeignKeyViolation(err error) bool {
	return hasSQLState(err, postgresForeignKeyViolation)
}

func isCheckViolation(err error) bool {
	return hasSQLState(err, postgresCheckViolation)
}

func hasSQLState(err error, code string) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == code
}
