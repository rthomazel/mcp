package postgres

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/db"
	"github.com/rthomazel/mcp/investments/internal/domain"
)

const testDSNEnv = "INVESTMENTS_MCP_TEST_DSN"

// newTestStore creates a throwaway database on the server named by
// INVESTMENTS_MCP_TEST_DSN (a role allowed to create databases), applies the
// migrations and returns a store on it. Skips when the variable is unset.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	adminDSN := os.Getenv(testDSNEnv)
	if adminDSN == "" {
		t.Skipf("%s not set", testDSNEnv)
	}

	ctx := context.Background()
	name := fmt.Sprintf("investments_test_%d_%d", os.Getpid(), rand.Uint32())

	admin, err := pgx.Connect(ctx, adminDSN)
	require.NoError(t, err)

	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)

	u, err := url.Parse(adminDSN)
	require.NoError(t, err)

	u.Path = "/" + name

	store, cleanup, err := Open(u.String(), 5)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = cleanup()
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})

	driver, err := migratepgx.WithInstance(store.DB(), &migratepgx.Config{})
	require.NoError(t, err)

	source, err := iofs.New(db.Migrations, "migrations")
	require.NoError(t, err)

	migrator, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())

	return store
}

func date(year int, month time.Month, day int) domain.Date {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dp(d domain.Date) *domain.Date { return &d }

func registerETF(t *testing.T, store *Store, symbol string) domain.Instrument {
	t.Helper()

	instrument, err := store.RegisterInstrument(context.Background(), domain.Instrument{
		Symbol: symbol, Exchange: "NYSE", Name: symbol + " fund", Currency: "USD", InstrumentType: "ETF",
	})
	require.NoError(t, err)

	return instrument
}

// record inserts a transaction executed at noon UTC on the effective date.
func record(t *testing.T, store *Store, instrumentID int, event domain.EventType, quantity, cash string, effective domain.Date) (domain.RecordResult, error) {
	t.Helper()

	return store.RecordTransaction(context.Background(), domain.DraftTransaction{
		InstrumentID: instrumentID, EventType: event,
		QuantityChange: domain.Decimal(quantity), CashAmount: domain.Decimal(cash),
		ExecutedAt: effective.Add(12 * time.Hour), EffectiveDate: effective,
	})
}

func insertPrice(t *testing.T, store *Store, instrumentID int, on domain.Date, closePrice string) {
	t.Helper()

	_, err := store.DB().Exec(
		`INSERT INTO prices (instrument_id, date, open, high, low, close, adjusted_close, volume)
		 VALUES ($1, $2, $3::numeric, $3::numeric, $3::numeric, $3::numeric, $3::numeric, 1000)`,
		instrumentID, on, closePrice)
	require.NoError(t, err)
}

func insertSnapshot(t *testing.T, store *Store, instrumentID int, on domain.Date) {
	t.Helper()

	_, err := store.DB().Exec(
		`INSERT INTO snapshots (date, instrument_id, quantity, close_price, price_date) VALUES ($1, $2, 1, 1, $1)`,
		on, instrumentID)
	require.NoError(t, err)
}
