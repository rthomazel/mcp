package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// seedHolding registers a symbol bought on the given day and priced on the days.
func seedHolding(t *testing.T, store *Store, symbol, quantity string, bought domain.Date, priced ...domain.Date) domain.Instrument {
	t.Helper()

	instrument := registerETF(t, store, symbol)
	_, err := record(t, store, instrument.ID, domain.EventBuy, quantity, "-100", bought)
	require.NoError(t, err)

	for _, on := range priced {
		insertPrice(t, store, instrument.ID, on, "10")
	}

	return instrument
}

func snapshotCount(t *testing.T, store *Store, on domain.Date) int {
	t.Helper()

	var count int

	require.NoError(t, store.DB().QueryRow(`SELECT count(*) FROM snapshots WHERE date = $1`, on).Scan(&count))

	return count
}

func updateStatuses(t *testing.T, store *Store, dataType string) []string {
	t.Helper()

	statuses := []string{}
	rows, err := store.DB().Query(`SELECT status FROM updates WHERE data_type = $1 ORDER BY id`, dataType)
	require.NoError(t, err)

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var status string
		require.NoError(t, rows.Scan(&status))

		statuses = append(statuses, status)
	}

	return statuses
}

func TestWriteSnapshotComplete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2))
	seedHolding(t, store, "BND", "2", date(2026, 11, 2), date(2026, 11, 2))

	result, err := store.WriteSnapshot(ctx, date(2026, 11, 2))

	require.NoError(t, err)
	assert.True(t, result.Complete)
	assert.Equal(t, 2, result.Rows)
	assert.Equal(t, 2, snapshotCount(t, store, date(2026, 11, 2)))
	assert.Equal(t, []string{"completed"}, updateStatuses(t, store, "snapshots"))

	var value string

	require.NoError(t, store.DB().QueryRow(
		`SELECT sum(market_value_usd)::text FROM snapshots WHERE date = $1`, date(2026, 11, 2)).Scan(&value))
	assert.Equal(t, "50", value)
}

func TestWriteSnapshotIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2))

	_, err := store.WriteSnapshot(ctx, date(2026, 11, 2))
	require.NoError(t, err)

	result, err := store.WriteSnapshot(ctx, date(2026, 11, 2))

	require.NoError(t, err)
	assert.True(t, result.Skipped)
	assert.Equal(t, 1, snapshotCount(t, store, date(2026, 11, 2)))
	assert.Len(t, updateStatuses(t, store, "snapshots"), 1, "a skipped run logs nothing")
}

func TestWriteSnapshotNoPositionsIsCompleteAndMarked(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	other := registerETF(t, store, "VTI")
	insertPrice(t, store, other.ID, date(2026, 11, 2), "10")

	result, err := store.WriteSnapshot(ctx, date(2026, 11, 2))

	require.NoError(t, err)
	assert.True(t, result.Complete)
	assert.Zero(t, result.Rows)

	again, err := store.WriteSnapshot(ctx, date(2026, 11, 2))
	require.NoError(t, err)
	assert.True(t, again.Skipped, "the updates row marks a date with no positions")
}

func TestWriteSnapshotMissingPriceWritesNothing(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2))
	seedHolding(t, store, "BND", "2", date(2026, 11, 2))

	result, err := store.WriteSnapshot(ctx, date(2026, 11, 2))

	require.NoError(t, err)
	assert.False(t, result.Complete)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, "BND", result.Issues[0].Symbol)
	assert.Equal(t, domain.PriceIssueMissing, result.Issues[0].Issue)
	assert.Zero(t, snapshotCount(t, store, date(2026, 11, 2)))
	assert.Equal(t, []string{"incomplete"}, updateStatuses(t, store, "snapshots"))

	// Completing the price and retrying succeeds: incomplete is not a marker.
	insertPrice(t, store, result.Issues[0].InstrumentID, date(2026, 11, 2), "10")

	retry, err := store.WriteSnapshot(ctx, date(2026, 11, 2))
	require.NoError(t, err)
	assert.True(t, retry.Complete)
	assert.Equal(t, 2, retry.Rows)
}

func TestWriteSnapshotOlderPriceIsStaleNotUsed(t *testing.T) {
	store := newTestStore(t)
	seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2))
	seedHolding(t, store, "BND", "2", date(2026, 11, 2), date(2026, 11, 2))
	other := registerETF(t, store, "QQQ")
	_, err := record(t, store, other.ID, domain.EventBuy, "1", "-1", date(2026, 11, 2))
	require.NoError(t, err)
	insertPrice(t, store, other.ID, date(2026, 10, 30), "10")

	result, err := store.WriteSnapshot(context.Background(), date(2026, 11, 2))

	require.NoError(t, err)
	assert.False(t, result.Complete)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, domain.PriceIssueStale, result.Issues[0].Issue)
	assert.Zero(t, snapshotCount(t, store, date(2026, 11, 2)))
}

func TestWriteSnapshotExistingRowsAreNeverOverwritten(t *testing.T) {
	store := newTestStore(t)
	instrument := seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2))
	insertSnapshot(t, store, instrument.ID, date(2026, 11, 2))

	_, err := store.WriteSnapshot(context.Background(), date(2026, 11, 2))

	assert.ErrorIs(t, err, domain.ErrSnapshotExists)
	assert.Empty(t, updateStatuses(t, store, "snapshots"))
}

func TestWriteSnapshotRejectsFutureDate(t *testing.T) {
	store := newTestStore(t)

	_, err := store.WriteSnapshot(context.Background(), time.Now().AddDate(0, 0, 3).UTC().Truncate(24*time.Hour))

	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestWriteSnapshotUsesHistoricalPositions(t *testing.T) {
	store := newTestStore(t)
	instrument := seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2), date(2026, 11, 4))
	_, err := record(t, store, instrument.ID, domain.EventSell, "-1", "10", date(2026, 11, 3))
	require.NoError(t, err)

	_, err = store.WriteSnapshot(context.Background(), date(2026, 11, 2))
	require.NoError(t, err)

	var quantity string

	require.NoError(t, store.DB().QueryRow(`SELECT quantity::text FROM snapshots WHERE date = $1`, date(2026, 11, 2)).Scan(&quantity))
	assert.Equal(t, "3", quantity, "a later sale does not change an earlier date")
}

func TestRebuildReplacesRangeAndIsLogged(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	instrument := seedHolding(t, store, "VTI", "3", date(2026, 11, 2),
		date(2026, 11, 2), date(2026, 11, 3), date(2026, 11, 4))
	insertSnapshot(t, store, instrument.ID, date(2026, 11, 3))
	_, err := record(t, store, instrument.ID, domain.EventBuy, "1", "-10", date(2026, 11, 3))
	require.NoError(t, err)

	result, err := store.RebuildSnapshots(ctx, date(2026, 11, 3), date(2026, 11, 4), " backdated buy ")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Dates)
	assert.Equal(t, 2, result.Rows)

	var quantity, reason string

	require.NoError(t, store.DB().QueryRow(`SELECT quantity::text FROM snapshots WHERE date = $1`, date(2026, 11, 3)).Scan(&quantity))
	assert.Equal(t, "4", quantity)
	assert.Zero(t, snapshotCount(t, store, date(2026, 11, 2)), "dates outside the range are untouched")
	require.NoError(t, store.DB().QueryRow(`SELECT reason FROM updates WHERE data_type = 'snapshot_rebuild'`).Scan(&reason))
	assert.Equal(t, "backdated buy", reason)

	// A rebuilt date counts as completed for the daily job.
	again, err := store.WriteSnapshot(ctx, date(2026, 11, 4))
	require.NoError(t, err)
	assert.True(t, again.Skipped)
}

func TestRebuildIsAllOrNothing(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	vti := seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2), date(2026, 11, 3))
	seedHolding(t, store, "BND", "2", date(2026, 11, 2), date(2026, 11, 2))
	insertSnapshot(t, store, vti.ID, date(2026, 11, 2))

	_, err := store.RebuildSnapshots(ctx, date(2026, 11, 2), date(2026, 11, 3), "fix")

	var incomplete *domain.IncompleteSnapshotError

	require.ErrorAs(t, err, &incomplete)
	assert.ErrorIs(t, err, domain.ErrSnapshotIncomplete)
	assert.Equal(t, date(2026, 11, 3), incomplete.Date)
	assert.Equal(t, 1, snapshotCount(t, store, date(2026, 11, 2)), "the original rows are back")
	assert.Equal(t, []string{"failed"}, updateStatuses(t, store, "snapshot_rebuild"))
}

func TestRebuildValidation(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.RebuildSnapshots(ctx, date(2026, 11, 2), date(2026, 11, 3), "  ")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.RebuildSnapshots(ctx, date(2026, 11, 3), date(2026, 11, 2), "fix")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.RebuildSnapshots(ctx, date(2026, 11, 2), time.Now().AddDate(0, 0, 3).UTC().Truncate(24*time.Hour), "fix")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestLatestSessionDate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	latest, err := store.LatestSessionDate(ctx)
	require.NoError(t, err)
	assert.Nil(t, latest)

	a, b := registerETF(t, store, "VTI"), registerETF(t, store, "BND")
	insertPrice(t, store, a.ID, date(2026, 11, 2), "1")
	insertPrice(t, store, b.ID, date(2026, 11, 4), "1")

	latest, err = store.LatestSessionDate(ctx)
	require.NoError(t, err)
	assert.Equal(t, date(2026, 11, 4), *latest)
}

func TestRecordUpdate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	provider := "tiingo"
	now := time.Now()

	require.NoError(t, store.RecordUpdate(ctx, domain.Update{
		Provider: &provider, DataType: domain.UpdateDataPrices, StartedAt: now, CompletedAt: now,
		Status: domain.UpdateStatusIncomplete, RecordsUpdated: 4, ErrorMessage: "not refreshed: VTI",
	}))
	assert.Equal(t, []string{"incomplete"}, updateStatuses(t, store, "prices"))

	err := store.RecordUpdate(ctx, domain.Update{DataType: "bogus", Status: domain.UpdateStatusCompleted})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

// The database session zone, such as a host in Brasilia time, must not move dates.
func TestSnapshotIgnoresSessionTimezone(t *testing.T) {
	store := newTestStore(t)
	_, err := store.DB().Exec(`ALTER DATABASE ` + currentDatabase(t, store) + ` SET timezone = 'America/Sao_Paulo'`)
	require.NoError(t, err)

	store.DB().SetMaxIdleConns(0) // new sessions pick up the setting

	seedHolding(t, store, "VTI", "3", date(2026, 11, 2), date(2026, 11, 2))

	result, err := store.WriteSnapshot(context.Background(), date(2026, 11, 2))

	require.NoError(t, err)
	assert.True(t, result.Complete)
	assert.Equal(t, 1, snapshotCount(t, store, date(2026, 11, 2)))
}

func currentDatabase(t *testing.T, store *Store) string {
	t.Helper()

	var name string

	require.NoError(t, store.DB().QueryRow(`SELECT current_database()`).Scan(&name))

	return name
}
