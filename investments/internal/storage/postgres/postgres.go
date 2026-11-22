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

var (
	_ domain.Store         = (*Store)(nil)
	_ domain.CashStore     = (*Cash)(nil)
	_ domain.PriceStore    = (*Prices)(nil)
	_ domain.SnapshotStore = (*Snapshots)(nil)
	_ domain.UpdateStore   = (*Updates)(nil)
	_ domain.EventStore    = (*Events)(nil)
)

// Store implements domain.Store and holds what every store shares: the
// connection and the settings. Its methods are spread across files by concern.
type Store struct {
	db               *sqlx.DB
	stalePriceDays   int
	marketLocation   *time.Location
	maxHistoryPoints int
	events           domain.EventPolicy
	clock            func() time.Time // replaceable in tests
}

// Cash, Prices, Snapshots, Updates and Events implement the narrower stores.
// Each shares the Store's connection and settings.
type (
	Cash      struct{ *Store }
	Prices    struct{ *Store }
	Snapshots struct{ *Store }
	Updates   struct{ *Store }
	Events    struct{ *Store }
)

// Stores is every store over one connection. The embedded Store is the generic
// one, so Stores itself also implements domain.Store.
type Stores struct {
	*Store

	Cash      *Cash
	Prices    *Prices
	Snapshots *Snapshots
	Updates   *Updates
	Events    *Events
}

// Domain returns the stores as the domain bundle.
func (s *Stores) Domain() domain.Stores {
	return domain.Stores{Store: s.Store, Cash: s.Cash, Prices: s.Prices, Snapshots: s.Snapshots, Updates: s.Updates, Events: s.Events}
}

// Settings are the store's tunables, taken from the server configuration.
type Settings struct {
	StalePriceDays   int
	MarketTimezone   string
	MaxHistoryPoints int
	Events           domain.EventPolicy
}

// Open connects to the database and returns the stores and a cleanup function.
func Open(dsn string, settings Settings) (*Stores, func() error, error) {
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

	return &Stores{
		Store: store, Cash: &Cash{store}, Prices: &Prices{store}, Snapshots: &Snapshots{store},
		Updates: &Updates{store}, Events: &Events{store},
	}, handle.Close, nil
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
