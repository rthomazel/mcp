package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func TestGetPositions(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	empty, err := store.GetPositions(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, empty.Positions)
	assert.Empty(t, empty.Positions)
	assert.Empty(t, empty.IntegrityErrors)

	for _, step := range []struct {
		id             int
		event          domain.EventType
		quantity, cash string
		day            int
	}{
		{voo.ID, domain.EventBuy, "10", "-100", 1},
		{voo.ID, domain.EventSell, "-4", "40", 10},
		{qqq.ID, domain.EventBuy, "5", "-50", 5},
		{qqq.ID, domain.EventSell, "-5", "55", 20},
	} {
		_, err := record(t, store, step.id, step.event, step.quantity, step.cash, date(2026, 4, step.day))
		require.NoError(t, err)
	}

	current, err := store.GetPositions(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, current.Date)
	require.Len(t, current.Positions, 1, "fully sold QQQ is not a position")
	assert.Equal(t, "VOO", current.Positions[0].Symbol)
	assert.Equal(t, "6", string(current.Positions[0].Quantity))

	// Positions as of a past date match the ledger up to that effective date.
	asOf := date(2026, 4, 5)
	past, err := store.GetPositions(ctx, &asOf)
	require.NoError(t, err)
	require.Len(t, past.Positions, 2)
	assert.Equal(t, "QQQ", past.Positions[0].Symbol)
	assert.Equal(t, "5", string(past.Positions[0].Quantity))
	assert.Equal(t, "VOO", past.Positions[1].Symbol)
	assert.Equal(t, "10", string(past.Positions[1].Quantity))
	assert.Equal(t, &asOf, past.Date)

	before := date(2026, 3, 31)
	none, err := store.GetPositions(ctx, &before)
	require.NoError(t, err)
	assert.Empty(t, none.Positions)
}

func TestGetPositionsReportsIntegrityErrors(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	_, err := record(t, store, voo.ID, domain.EventBuy, "10", "-100", date(2026, 1, 1))
	require.NoError(t, err)
	_, err = record(t, store, qqq.ID, domain.EventBuy, "5", "-50", date(2026, 1, 1))
	require.NoError(t, err)

	// Administrative edits bypass the write validation: VOO goes negative in
	// the middle but ends positive, QQQ ends negative.
	_, err = store.DB().Exec(`
		INSERT INTO transactions (instrument_id, event_type, quantity_change, cash_amount, executed_at, effective_date)
		VALUES ($1, 'SELL', -12, 120, '2026-01-02 12:00+00', '2026-01-02'),
		       ($1, 'BUY', 20, -200, '2026-01-03 12:00+00', '2026-01-03'),
		       ($2, 'SELL', -9, 90, '2026-01-02 12:00+00', '2026-01-02')`, voo.ID, qqq.ID)
	require.NoError(t, err)

	result, err := store.GetPositions(ctx, nil)
	require.NoError(t, err)

	require.Len(t, result.Positions, 1, "QQQ with a negative balance is not a position")
	assert.Equal(t, "VOO", result.Positions[0].Symbol)
	assert.Equal(t, "18", string(result.Positions[0].Quantity))

	require.Len(t, result.IntegrityErrors, 2, "never silently omitted")
	assert.Equal(t, "QQQ", result.IntegrityErrors[0].Symbol)
	assert.Equal(t, date(2026, 1, 2), result.IntegrityErrors[0].FirstNegativeDate)
	assert.Equal(t, "-4", string(result.IntegrityErrors[0].Quantity))
	assert.Equal(t, "VOO", result.IntegrityErrors[1].Symbol)
	assert.Equal(t, "-2", string(result.IntegrityErrors[1].Quantity))

	// Restricted to a date before the bad entries, the ledger is clean.
	asOf := date(2026, 1, 1)
	clean, err := store.GetPositions(ctx, &asOf)
	require.NoError(t, err)
	assert.Empty(t, clean.IntegrityErrors)
}
