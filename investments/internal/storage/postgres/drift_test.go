package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

func decimalOf(text string) *domain.Decimal {
	value := domain.Decimal(text)

	return &value
}

// seedDay stores one snapshot day for VOO, VTI and GLD at quantity 1, so prices are values.
func seedDay(t *testing.T, store *Stores, ids map[string]int, on domain.Date, values map[string]string) {
	t.Helper()

	for symbol, value := range values {
		seedPosition(t, store, ids[symbol], on, "1", value)
	}
}

func driftFixture(t *testing.T) (*Stores, map[string]int) {
	t.Helper()

	store := newTestStore(t)
	ids := map[string]int{}

	for _, symbol := range []string{"VOO", "VTI", "GLD"} {
		ids[symbol] = registerETF(t, store, symbol).ID
	}

	return store, ids
}

func TestAllocationDriftStayAndStayStart(t *testing.T) {
	store, ids := driftFixture(t)
	ctx := context.Background()
	groups := []domain.AllocationGroup{
		{Name: "us-core", Tickers: []string{"VOO", "VTI"}, MaxPercent: decimalOf("60")},
		{Name: "gold", Tickers: []string{"GLD"}, MinPercent: decimalOf("20"), MaxPercent: decimalOf("40")},
	}

	// Day 1: us-core 50 of 100, gold 50 (above its max 40). Day 2 and 3: us-core 70 above, gold 30 inside.
	// Day 4: us-core back to 60, exactly at its bound. Day 5: us-core 80 above again, gold 20 at its minimum.
	seedDay(t, store, ids, date(2026, 1, 1), map[string]string{"VOO": "30", "VTI": "20", "GLD": "50"})
	seedDay(t, store, ids, date(2026, 1, 2), map[string]string{"VOO": "40", "VTI": "30", "GLD": "30"})
	seedDay(t, store, ids, date(2026, 1, 3), map[string]string{"VOO": "40", "VTI": "30", "GLD": "30"})

	drift, err := store.Monitors.GetAllocationDrift(ctx, groups)
	require.NoError(t, err)
	require.NotNil(t, drift)
	assert.Equal(t, date(2026, 1, 3), drift.Date)
	assert.Equal(t, "70", string(drift.Groups[0].AllocationPercent))
	assert.Equal(t, domain.SideAbove, drift.Groups[0].Side)
	assert.Equal(t, date(2026, 1, 2), *drift.Groups[0].StayStart, "inside the band on Jan 1")
	assert.Equal(t, []domain.MemberAllocation{{Symbol: "VOO", AllocationPercent: "40"}, {Symbol: "VTI", AllocationPercent: "30"}},
		drift.Groups[0].Members)
	assert.Empty(t, drift.Groups[1].Side, "30 is inside 20 to 40")
	assert.Nil(t, drift.Groups[1].StayStart)

	seedDay(t, store, ids, date(2026, 1, 4), map[string]string{"VOO": "35", "VTI": "25", "GLD": "40"})

	drift, err = store.Monitors.GetAllocationDrift(ctx, groups)
	require.NoError(t, err)
	assert.Empty(t, drift.Groups[0].Side, "exactly at the bound is inside")
	assert.Empty(t, drift.Groups[1].Side, "exactly at 40 is inside")

	seedDay(t, store, ids, date(2026, 1, 5), map[string]string{"VOO": "50", "VTI": "30", "GLD": "20"})

	drift, err = store.Monitors.GetAllocationDrift(ctx, groups)
	require.NoError(t, err)
	assert.Equal(t, date(2026, 1, 5), *drift.Groups[0].StayStart, "a new stay after returning inside")
	assert.Equal(t, domain.SideAbove, drift.Groups[0].Side)
	assert.Empty(t, drift.Groups[1].Side, "exactly at the minimum is inside")
}

func TestAllocationDriftNeverInsideStartsAtTheFirstDate(t *testing.T) {
	store, ids := driftFixture(t)

	seedDay(t, store, ids, date(2026, 1, 1), map[string]string{"VOO": "90", "GLD": "10"})
	seedDay(t, store, ids, date(2026, 1, 2), map[string]string{"VOO": "90", "GLD": "10"})

	drift, err := store.Monitors.GetAllocationDrift(context.Background(), []domain.AllocationGroup{
		{Name: "us", Tickers: []string{"VOO"}, MaxPercent: decimalOf("50")},
	})
	require.NoError(t, err)
	assert.Equal(t, date(2026, 1, 1), *drift.Groups[0].StayStart)
}

func TestAllocationDriftSideSwitchIsANewStay(t *testing.T) {
	store, ids := driftFixture(t)

	seedDay(t, store, ids, date(2026, 1, 1), map[string]string{"VOO": "50", "GLD": "50"})
	seedDay(t, store, ids, date(2026, 1, 2), map[string]string{"VOO": "10", "GLD": "90"})

	drift, err := store.Monitors.GetAllocationDrift(context.Background(), []domain.AllocationGroup{
		{Name: "us", Tickers: []string{"VOO"}, MinPercent: decimalOf("30"), MaxPercent: decimalOf("40")},
	})
	require.NoError(t, err)
	assert.Equal(t, domain.SideBelow, drift.Groups[0].Side)
	assert.Equal(t, date(2026, 1, 2), *drift.Groups[0].StayStart, "the other side the day before")
}

func TestAllocationDriftMissingHoldingsAndTickers(t *testing.T) {
	store, ids := driftFixture(t)

	seedDay(t, store, ids, date(2026, 1, 1), map[string]string{"VOO": "100"})

	drift, err := store.Monitors.GetAllocationDrift(context.Background(), []domain.AllocationGroup{
		{Name: "gold", Tickers: []string{"GLD", "NOPE"}, MinPercent: decimalOf("5")},
	})
	require.NoError(t, err)
	assert.Equal(t, "0", string(drift.Groups[0].AllocationPercent))
	assert.Equal(t, domain.SideBelow, drift.Groups[0].Side, "a minimum and no holdings is below the band")
	assert.Equal(t, date(2026, 1, 1), *drift.Groups[0].StayStart)
	assert.Equal(t, []domain.MemberAllocation{{Symbol: "GLD", AllocationPercent: "0"}}, drift.Groups[0].Members)
	assert.Equal(t, []string{"NOPE"}, drift.Unregistered)
}

func TestAllocationDriftWithoutSecuritiesRecordsNothing(t *testing.T) {
	store, ids := driftFixture(t)
	groups := []domain.AllocationGroup{{Name: "us", Tickers: []string{"VOO"}, MaxPercent: decimalOf("50")}}

	drift, err := store.Monitors.GetAllocationDrift(context.Background(), groups)
	require.NoError(t, err)
	assert.Nil(t, drift, "no snapshots")

	seedDay(t, store, ids, date(2026, 1, 1), map[string]string{"VOO": "100"})
	seedCashSnapshot(t, store, date(2026, 1, 2), "500")

	drift, err = store.Monitors.GetAllocationDrift(context.Background(), groups)
	require.NoError(t, err)
	assert.Nil(t, drift, "the latest date holds only cash")
}

func TestAllocationDriftAmbiguousTicker(t *testing.T) {
	store, ids := driftFixture(t)

	seedDay(t, store, ids, date(2026, 1, 1), map[string]string{"VOO": "100"})

	_, err := store.DB().Exec(
		`INSERT INTO instruments (symbol, exchange, name, currency, instrument_type) VALUES ('VOO', 'NASDAQ', 'VOO again', 'USD', 'ETF')`)
	require.NoError(t, err)

	_, err = store.Monitors.GetAllocationDrift(context.Background(), []domain.AllocationGroup{
		{Name: "us", Tickers: []string{"VOO"}, MaxPercent: decimalOf("50")},
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// insertAdjusted stores a price whose adjusted close differs from the close, as after a dividend.
func insertAdjusted(t *testing.T, store *Stores, instrumentID int, on domain.Date, adjusted string) {
	t.Helper()

	_, err := store.DB().Exec(
		`INSERT INTO prices (instrument_id, date, open, high, low, close, adjusted_close, volume)
		 VALUES ($1, $2, 100, 100, 100, 100, $3::numeric, 1000)`, instrumentID, on, adjusted)
	require.NoError(t, err)
}

func lookbackOf(t *testing.T, text string) duration.Duration {
	t.Helper()

	return window(t, text)
}

// seedAlternating stores days of adjusted closes that rise one percent and fall one percent in turn, then
// a last day that moves by finalPercent.
func seedAlternating(t *testing.T, store *Stores, instrumentID int, start domain.Date, days int, finalFactor string) {
	t.Helper()

	_, err := store.DB().Exec(`
		INSERT INTO prices (instrument_id, date, open, high, low, close, adjusted_close, volume)
		SELECT $1, $2::date + n, 100, 100, 100, 100, 100 * CASE WHEN n % 2 = 0 THEN 1.0 ELSE 1.01 END, 1000
		FROM generate_series(0, $3::int - 1) AS n`, instrumentID, start, days)
	require.NoError(t, err)

	_, err = store.DB().Exec(`
		INSERT INTO prices (instrument_id, date, open, high, low, close, adjusted_close, volume)
		SELECT $1, $2::date + $3::int, 100, 100, 100, 100,
		       (SELECT adjusted_close FROM prices WHERE instrument_id = $1 AND date = $2::date + $3::int - 1) * $4::numeric, 1000`,
		instrumentID, start, days, finalFactor)
	require.NoError(t, err)
}

func TestPriceVolatilityJudgesTheLatestMoveAgainstTheBaseline(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	// 40 days alternating about +1 and -1 percent, then a day up 3 percent.
	seedAlternating(t, store, qqq.ID, date(2026, 1, 1), 40, "1.03")

	got, err := store.Monitors.GetPriceVolatility(ctx, "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, date(2026, 2, 10), got.Date)
	assert.Equal(t, date(2026, 2, 9), got.StartDate)
	assert.Contains(t, string(got.MovePercent), "3")
	assert.Equal(t, 39, got.Moves, "every earlier day with a day before it, the judged move excluded")
	require.NotNil(t, got.VolatilityPercent)
	assert.True(t, got.AtLeastUp, "about three standard deviations against a threshold of 2")
	assert.False(t, got.AtLeastDown)

	got, err = store.Monitors.GetPriceVolatility(ctx, "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "4")
	require.NoError(t, err)
	assert.False(t, got.AtLeastUp, "below a threshold of 4")
}

func TestPriceVolatilityDownMoveAndShortLookback(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")

	seedAlternating(t, store, qqq.ID, date(2026, 1, 1), 40, "0.96")

	got, err := store.Monitors.GetPriceVolatility(context.Background(), "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	assert.True(t, got.AtLeastDown)
	assert.False(t, got.AtLeastUp)

	got, err = store.Monitors.GetPriceVolatility(context.Background(), "QQQ", window(t, "1d"), lookbackOf(t, "10d"), "2")
	require.NoError(t, err)
	assert.Equal(t, 9, got.Moves, "only the dates after the look-back start")
}

func TestPriceVolatilityOverAWindowUsesWindowMoves(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")

	seedAlternating(t, store, qqq.ID, date(2026, 1, 1), 60, "1.05")

	got, err := store.Monitors.GetPriceVolatility(context.Background(), "QQQ", window(t, "5d"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	assert.Equal(t, date(2026, 2, 25), got.StartDate, "five calendar days before March 2")
	assert.Equal(t, 55, got.Moves, "dates with a price five days before them")
}

func TestPriceVolatilityFlatPricesHaveNoVolatility(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")

	for day := range 30 {
		insertAdjusted(t, store, qqq.ID, date(2026, 1, 1).AddDate(0, 0, day), "100")
	}

	got, err := store.Monitors.GetPriceVolatility(context.Background(), "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	assert.Nil(t, got.VolatilityPercent)
	assert.Nil(t, got.Variation)
	assert.False(t, got.AtLeastUp || got.AtLeastDown)
}

func TestPriceVolatilityIgnoresSplitsInTheClose(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")

	seedAlternating(t, store, qqq.ID, date(2026, 1, 1), 40, "1.0")
	// A split halves the close; the adjusted close, which the move uses, does not change.
	_, err := store.DB().Exec(`UPDATE prices SET close = 50 WHERE instrument_id = $1 AND date = $2`, qqq.ID, date(2026, 2, 10))
	require.NoError(t, err)

	got, err := store.Monitors.GetPriceVolatility(context.Background(), "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	assert.False(t, got.AtLeastUp || got.AtLeastDown)
}

func TestPriceVolatilityEdges(t *testing.T) {
	store := newTestStore(t)
	qqq := registerETF(t, store, "QQQ")
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	seedAlternating(t, store, qqq.ID, date(2026, 1, 1), 40, "1.03")
	insertAdjusted(t, store, voo.ID, date(2026, 1, 1), "100")

	got, err := store.Monitors.GetPriceVolatility(ctx, "VOO", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	assert.Nil(t, got, "no price on the latest date of the store")

	_, err = store.Monitors.GetPriceVolatility(ctx, "NOPE", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.ErrorIs(t, err, domain.ErrInstrumentNotFound)

	_, err = store.Monitors.GetPriceVolatility(ctx, "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "0")
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	got, err = store.Monitors.GetPriceVolatility(ctx, "QQQ", window(t, "2y"), lookbackOf(t, "3mo"), "2")
	require.NoError(t, err)
	assert.Nil(t, got, "no price before the window start")

	_, err = store.DB().Exec(
		`INSERT INTO instruments (symbol, exchange, name, currency, instrument_type) VALUES ('QQQ', 'NASDAQ', 'QQQ again', 'USD', 'ETF')`)
	require.NoError(t, err)

	_, err = store.Monitors.GetPriceVolatility(ctx, "QQQ", window(t, "1d"), lookbackOf(t, "3mo"), "2")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSQLAndDurationCountBackTheSameWay(t *testing.T) {
	store := newTestStore(t)

	for _, text := range []string{"1d", "1w", "1mo", "3mo", "1y", "1mo2w", "1y2mo3d", "12mo"} {
		span := window(t, text)

		for _, from := range []domain.Date{date(2026, 3, 31), date(2028, 2, 29), date(2026, 1, 15), date(2026, 3, 20), date(2026, 12, 31)} {
			var got time.Time

			err := store.DB().QueryRow(`SELECT ($1::date - make_interval(months => $2::int, days => $3::int))::date`,
				from, span.Months, span.Days).Scan(&got)
			require.NoError(t, err)
			assert.Equal(t, span.Before(from).Format(time.DateOnly), got.Format(time.DateOnly), "%s before %s", text, from.Format(time.DateOnly))
		}
	}
}
