package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func insertRun(t *testing.T, store *Stores, dataType, status string, started time.Time, from, to *domain.Date) {
	t.Helper()

	_, err := store.DB().Exec(
		`INSERT INTO updates (data_type, started_at, completed_at, status, records_updated, date_from, date_to, reason)
		 VALUES ($1, $2, $2, $3, 0, $4, $5, CASE WHEN $1 = 'snapshot_rebuild' THEN 'test' END)`, dataType, started, status, from, to)
	require.NoError(t, err)
}

func TestStatusIsHealthyWhenEverythingIsCurrent(t *testing.T) {
	store := newTestStore(t)
	today := store.referenceDate(nil)
	voo := registerETF(t, store, "VOO")

	mustRecord(t, store, voo.ID, domain.EventBuy, "1", "-10", today.AddDate(0, 0, -3))
	insertPrice(t, store, voo.ID, today, "10")
	insertRun(t, store, "snapshots", "completed", time.Now(), &today, &today)

	status, err := store.GetStatus(context.Background())
	require.NoError(t, err)
	assert.True(t, status.Healthy, status.Reasons)
	assert.Empty(t, status.Reasons)
	assert.False(t, status.SnapshotMissing)
	assert.Equal(t, today, *status.LatestSnapshotDate)
	require.Len(t, status.Instruments, 1)
	assert.Empty(t, status.Instruments[0].PriceIssue)
}

func TestStatusReportsPriceAndSnapshotProblemsForHeldInstrumentsOnly(t *testing.T) {
	store := newTestStore(t)
	today := store.referenceDate(nil)
	stale, missing, sold := registerETF(t, store, "OLD"), registerETF(t, store, "NEW"), registerETF(t, store, "SLD")

	for _, id := range []int{stale.ID, missing.ID, sold.ID} {
		mustRecord(t, store, id, domain.EventBuy, "1", "-10", today.AddDate(0, 0, -30))
	}

	mustRecord(t, store, sold.ID, domain.EventSell, "-1", "10", today.AddDate(0, 0, -20))
	insertPrice(t, store, stale.ID, today.AddDate(0, 0, -6), "10")
	insertPrice(t, store, sold.ID, today.AddDate(0, 0, -25), "10")

	status, err := store.GetStatus(context.Background())
	require.NoError(t, err)

	assert.False(t, status.Healthy)
	require.Len(t, status.Instruments, 2, "the sold instrument is not checked")
	assert.Equal(t, "NEW", status.Instruments[0].Symbol)
	assert.Equal(t, domain.PriceIssueMissing, status.Instruments[0].PriceIssue)
	assert.Equal(t, domain.PriceIssueStale, status.Instruments[1].PriceIssue)

	priceDate := today.AddDate(0, 0, -6)
	assert.Equal(t, priceDate, *status.LatestPriceDate)
	assert.True(t, status.SnapshotMissing)
	assert.Nil(t, status.LatestSnapshotDate)
	assert.Len(t, status.Reasons, 3, "two instruments and the missing snapshot")
}

func TestStatusAttentionDropsSupersededRuns(t *testing.T) {
	store := newTestStore(t)
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	d1, d2, d3 := date(2026, 5, 4), date(2026, 5, 5), date(2026, 5, 6)

	// Snapshot runs: d1 is later completed, d2 is covered by the later rebuild.
	insertRun(t, store, "snapshots", "incomplete", base, &d1, &d1)
	insertRun(t, store, "snapshots", "completed", base.Add(time.Hour), &d1, &d1)
	insertRun(t, store, "snapshots", "incomplete", base.Add(2*time.Hour), &d2, &d2)

	// A failed rebuild is superseded only by a later completed rebuild that covers its whole range.
	insertRun(t, store, "snapshot_rebuild", "failed", base.Add(3*time.Hour), &d1, &d3)
	insertRun(t, store, "snapshot_rebuild", "completed", base.Add(4*time.Hour), &d1, &d2)

	// A failed price refresh is superseded by any later completed one.
	insertRun(t, store, "prices", "failed", base.Add(5*time.Hour), nil, nil)

	status, err := store.GetStatus(context.Background())
	require.NoError(t, err)
	require.Len(t, status.NeedsAttention, 2, "the narrower rebuild leaves the failed one open")
	assert.Equal(t, "prices", status.NeedsAttention[0].DataType, "newest first")
	assert.Equal(t, "snapshot_rebuild", status.NeedsAttention[1].DataType)

	insertRun(t, store, "snapshot_rebuild", "completed", base.Add(6*time.Hour), &d1, &d3)

	status, err = store.GetStatus(context.Background())
	require.NoError(t, err)
	require.Len(t, status.NeedsAttention, 1)
	assert.Equal(t, "prices", status.NeedsAttention[0].DataType)

	insertRun(t, store, "prices", "completed", base.Add(8*time.Hour), nil, nil)

	status, err = store.GetStatus(context.Background())
	require.NoError(t, err)
	assert.Empty(t, status.NeedsAttention)
}
