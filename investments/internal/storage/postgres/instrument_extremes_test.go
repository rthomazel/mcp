package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// seedAdjustedSeries stores one price per day from start whose adjusted closes are the values; the real
// close stays 100, so a result that follows the values is read from the adjusted close.
func seedAdjustedSeries(t *testing.T, store *Stores, instrumentID int, start domain.Date, values ...string) {
	t.Helper()

	for offset, value := range values {
		insertAdjusted(t, store, instrumentID, start.AddDate(0, 0, offset), value)
	}
}

func TestInstrumentDrawdownStateFollowsAdjustedCloses(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	// High 120 on 2 January; -5.8% on the 3rd and -10.8% on the 4th.
	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 1), "100", "120", "113", "107")

	state, err := store.Monitors.GetInstrumentDrawdownState(ctx, "QQQ", "5")
	require.NoError(t, err)
	assert.Equal(t, date(2026, 1, 4), state.Date)
	assert.Equal(t, domain.Decimal("107"), state.ValueUSD)
	assert.Equal(t, date(2026, 1, 2), state.HighDate)
	assert.Equal(t, domain.Decimal("120"), state.HighValueUSD)
	assert.Contains(t, string(state.DrawdownPercent), "-10.83")
	assert.Equal(t, []domain.DrawdownLevel{
		{Level: "-5", CrossedOn: date(2026, 1, 3)}, {Level: "-10", CrossedOn: date(2026, 1, 4)},
	}, state.Levels)

	// A recovery above -5 clears the levels, and a new fall starts a new stay.
	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 5), "116")

	state, err = store.Monitors.GetInstrumentDrawdownState(ctx, "QQQ", "5")
	require.NoError(t, err)
	assert.Empty(t, state.Levels)

	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 6), "113")

	state, err = store.Monitors.GetInstrumentDrawdownState(ctx, "QQQ", "5")
	require.NoError(t, err)
	assert.Equal(t, []domain.DrawdownLevel{{Level: "-5", CrossedOn: date(2026, 1, 6)}}, state.Levels)

	// A new high clears every level.
	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 7), "130")

	state, err = store.Monitors.GetInstrumentDrawdownState(ctx, "QQQ", "5")
	require.NoError(t, err)
	assert.Empty(t, state.Levels)
	assert.Equal(t, domain.Decimal("0"), state.DrawdownPercent)
}

func TestInstrumentExtremesNeedAPriceOnTheLatestDate(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 1), "100", "90", "80")
	seedAdjustedSeries(t, store, voo.ID, date(2026, 1, 1), "100", "90")

	state, err := store.Monitors.GetInstrumentDrawdownState(ctx, "VOO", "5")
	require.NoError(t, err)
	assert.Empty(t, state.Levels, "VOO has no price on the latest date")

	high, err := store.Monitors.GetInstrumentHighState(ctx, "VOO")
	require.NoError(t, err)
	assert.Nil(t, high)

	state, err = store.Monitors.GetInstrumentDrawdownState(ctx, "QQQ", "5")
	require.NoError(t, err)
	assert.Len(t, state.Levels, 4)
}

func TestInstrumentHighState(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 1), "100")

	high, err := store.Monitors.GetInstrumentHighState(ctx, "QQQ")
	require.NoError(t, err)
	require.NotNil(t, high)
	assert.False(t, high.NewHigh, "the first price is not a new high")
	assert.Nil(t, high.Previous)

	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 2), "110", "105", "120")

	high, err = store.Monitors.GetInstrumentHighState(ctx, "QQQ")
	require.NoError(t, err)
	require.NotNil(t, high)
	assert.True(t, high.NewHigh)
	assert.Equal(t, date(2026, 1, 4), high.Date)
	assert.Equal(t, domain.Decimal("120"), high.ValueUSD)
	assert.Equal(t, &domain.Extreme{Date: date(2026, 1, 2), ValueUSD: "110"}, high.Previous)
	assert.Equal(t, domain.Decimal("10"), high.IncreaseUSD)
	assert.Contains(t, string(high.IncreasePercent), "9.09")

	// Equalling the high is not a new one.
	seedAdjustedSeries(t, store, qqq.ID, date(2026, 1, 5), "120")

	high, err = store.Monitors.GetInstrumentHighState(ctx, "QQQ")
	require.NoError(t, err)
	assert.False(t, high.NewHigh)
}

func TestInstrumentExtremesReject(t *testing.T) {
	store := newTestStore(t)
	registerETF(t, store, "QQQ")
	ctx := context.Background()

	_, err := store.Monitors.GetInstrumentDrawdownState(ctx, "NOPE", "5")
	require.ErrorIs(t, err, domain.ErrInstrumentNotFound)

	_, err = store.Monitors.GetInstrumentHighState(ctx, "NOPE")
	require.ErrorIs(t, err, domain.ErrInstrumentNotFound)

	_, err = store.Monitors.GetInstrumentDrawdownState(ctx, "QQQ", "0")
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.Monitors.GetPriceTarget(ctx, "NOPE", "5")
	require.ErrorIs(t, err, domain.ErrInstrumentNotFound)

	_, err = store.Monitors.GetPriceTarget(ctx, "QQQ", "0")
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.Monitors.GetPriceTarget(ctx, "QQQ", "-1")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func insertRange(t *testing.T, store *Stores, instrumentID int, on domain.Date, low, high, closePrice string) {
	t.Helper()

	_, err := store.DB().Exec(
		`INSERT INTO prices (instrument_id, date, open, high, low, close, adjusted_close, volume)
		 VALUES ($1, $2, $3::numeric, $4::numeric, $3::numeric, $5::numeric, 1, 1000)`, instrumentID, on, low, high, closePrice)
	require.NoError(t, err)
}

func TestPriceTargetIsReachedWithinTheDaysRange(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	insertRange(t, store, voo.ID, date(2026, 1, 1), "230", "236", "234")
	insertRange(t, store, qqq.ID, date(2026, 1, 2), "230", "236", "234.5")

	for price, want := range map[domain.Decimal]bool{
		"232": true, "230": true, "236": true, "232.50": true, "229.99": false, "236.01": false, "300": false, "1": false,
	} {
		got, err := store.Monitors.GetPriceTarget(ctx, "QQQ", price)
		require.NoError(t, err, price)
		require.NotNil(t, got, price)
		assert.Equal(t, want, got.Reached, price)
		assert.Equal(t, date(2026, 1, 2), got.Date)
		assert.Equal(t, domain.Decimal("230"), got.Low)
		assert.Equal(t, domain.Decimal("236"), got.High)
		assert.Equal(t, domain.Decimal("234.5"), got.Close)
	}

	got, err := store.Monitors.GetPriceTarget(ctx, "VOO", "232")
	require.NoError(t, err)
	assert.Nil(t, got, "VOO has no price on the latest date")
}

func TestListHeldTickersReadsTheLatestSnapshotDate(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	tickers, err := store.Monitors.ListHeldTickers(ctx)
	require.NoError(t, err)
	assert.Empty(t, tickers)

	seedPosition(t, store, qqq.ID, date(2026, 1, 1), "1", "100")
	seedPosition(t, store, voo.ID, date(2026, 1, 1), "1", "100")
	seedPosition(t, store, voo.ID, date(2026, 1, 2), "1", "100")

	tickers, err = store.Monitors.ListHeldTickers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"VOO"}, tickers, "QQQ was sold")

	seedPosition(t, store, qqq.ID, date(2026, 1, 3), "1", "100")
	seedPosition(t, store, voo.ID, date(2026, 1, 3), "1", "100")

	tickers, err = store.Monitors.ListHeldTickers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"QQQ", "VOO"}, tickers)
}
