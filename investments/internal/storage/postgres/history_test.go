package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// seedSeries stores one snapshot row per day with the given market value,
// starting on start. A zero value skips the day.
func seedSeries(t *testing.T, store *Store, instrumentID int, start domain.Date, values ...int) {
	t.Helper()

	for offset, value := range values {
		if value == 0 {
			continue
		}

		_, err := store.DB().Exec(
			`INSERT INTO snapshots (date, instrument_id, quantity, close_price, price_date) VALUES ($1, $2, 1, $3, $1)`,
			start.AddDate(0, 0, offset), instrumentID, value)
		require.NoError(t, err)
	}
}

func seedCashSnapshot(t *testing.T, store *Store, on domain.Date, balance string) {
	t.Helper()

	_, err := store.DB().Exec(`INSERT INTO cash_snapshots (date, balance_usd) VALUES ($1, $2::numeric)`, on, balance)
	require.NoError(t, err)
}

func TestValueHistoryEmpty(t *testing.T) {
	store := newTestStore(t)

	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{})
	require.NoError(t, err)
	assert.Empty(t, history.Points)
	assert.Nil(t, history.Securities)
	assert.Nil(t, history.Equity)
	assert.Nil(t, history.NetContributionsUSD)
	assert.Equal(t, 1, history.WindowDays)
}

func TestValueHistoryMeasures(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	// Jan 1..8: 100 120 150 (none) 100 200 140 170
	seedSeries(t, store, voo.ID, date(2026, 1, 1), 100, 120, 150, 0, 100, 200, 140, 170)

	// The range starts on Jan 4, a day without a snapshot, so Jan 3 is the baseline.
	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{From: dp(date(2026, 1, 4))})
	require.NoError(t, err)
	require.Len(t, history.Points, 4)

	summary := history.Securities
	require.NotNil(t, summary)
	assert.Nil(t, history.Equity)
	assert.Equal(t, date(2026, 1, 8), summary.Latest.Date)
	assert.Equal(t, "170", string(summary.Latest.ValueUSD))
	assert.Equal(t, date(2026, 1, 6), summary.AllTimeHigh.Date)
	assert.Equal(t, "200", string(summary.AllTimeHigh.ValueUSD))
	assert.Equal(t, "-30", string(summary.FromHighUSD))
	assert.Equal(t, "-15", string(summary.FromHighPercent))
	assert.Equal(t, date(2026, 1, 3), summary.Baseline.Date, "last snapshot on or before the range start")
	assert.Equal(t, "20", string(summary.ChangeUSD))
	assert.Contains(t, string(*summary.ChangePercent), "13.33")
	assert.Equal(t, "100", string(summary.RangeLow.ValueUSD))
	assert.Equal(t, "200", string(summary.RangeHigh.ValueUSD))

	// Within Jan 5..8 the deepest fall is from the Jan 6 peak to Jan 7, and Jan 8 has not recovered.
	drawdown := summary.MaxDrawdown
	assert.Equal(t, date(2026, 1, 6), drawdown.PeakDate)
	assert.Equal(t, date(2026, 1, 7), drawdown.TroughDate)
	assert.Equal(t, "-60", string(drawdown.DrawdownUSD))
	assert.Equal(t, "-30", string(drawdown.DrawdownPercent))
	assert.False(t, drawdown.Recovered)
}

func TestValueHistoryAllTimeHighIsBoundedByTheRangeEnd(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	seedSeries(t, store, voo.ID, date(2026, 1, 1), 100, 200, 120, 300)

	history, err := store.GetValueHistory(context.Background(),
		domain.HistoryFilter{From: dp(date(2026, 1, 3)), To: dp(date(2026, 1, 3))})
	require.NoError(t, err)

	summary := history.Securities
	assert.Equal(t, date(2026, 1, 2), summary.AllTimeHigh.Date, "earlier than the range, and the later 300 is not counted")
	assert.Equal(t, "-80", string(summary.FromHighUSD))
}

func TestValueHistoryFirstInRangeIsTheBaselineWithoutEarlierData(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	seedSeries(t, store, voo.ID, date(2026, 1, 10), 100, 110)

	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{From: dp(date(2026, 1, 1))})
	require.NoError(t, err)
	assert.Equal(t, date(2026, 1, 10), history.Securities.Baseline.Date)
	assert.Equal(t, "10", string(history.Securities.ChangeUSD))
}

func TestValueHistoryWindowsKeepTheNewestSnapshotAndSkipEmptyWindows(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	// Mon Jan 1: snapshots on Jan 1, 3, 5, 9 only. Three-day windows from Jan 1: [1-3] [4-6] [7-9] [10-12].
	seedSeries(t, store, voo.ID, date(2026, 1, 1), 10, 0, 30, 0, 50, 0, 0, 0, 90)

	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{WindowDays: 3})
	require.NoError(t, err)

	dates := []domain.Date{}
	for _, point := range history.Points {
		dates = append(dates, point.Date)
	}

	assert.Equal(t, []domain.Date{date(2026, 1, 3), date(2026, 1, 5), date(2026, 1, 9)}, dates)

	// Measures are over daily rows whatever the window.
	assert.Equal(t, "10", string(history.Securities.RangeLow.ValueUSD))
}

func TestValueHistoryRejectsTooManyPoints(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	_, err := store.DB().Exec(
		`INSERT INTO snapshots (date, instrument_id, quantity, close_price, price_date)
		 SELECT d::date, $1, 1, 1, d::date FROM generate_series('2020-01-01'::date, '2020-01-01'::date + 1000, '1 day') d`, voo.ID)
	require.NoError(t, err)

	_, err = store.GetValueHistory(context.Background(), domain.HistoryFilter{})
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{WindowDays: 7})
	require.NoError(t, err)
	assert.Less(t, len(history.Points), domain.MaxHistoryPoints)

	_, err = store.GetValueHistory(context.Background(), domain.HistoryFilter{WindowDays: -1})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestValueHistoryEquityAndNetContributions(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	// Securities exist Jan 1..4; cash snapshots from Jan 2, and Jan 5 is cash only.
	seedSeries(t, store, voo.ID, date(2026, 1, 1), 100, 100, 100, 100)
	seedCashSnapshot(t, store, date(2026, 1, 2), "50")
	seedCashSnapshot(t, store, date(2026, 1, 3), "50")
	seedCashSnapshot(t, store, date(2026, 1, 4), "250")
	seedCashSnapshot(t, store, date(2026, 1, 5), "250")

	mustRecordCash(t, store, domain.CashOpeningBalance, "50", date(2026, 1, 2))
	mustRecordCash(t, store, domain.CashDeposit, "300", date(2026, 1, 4))
	mustRecordCash(t, store, domain.CashWithdrawal, "-100", date(2026, 1, 5))
	mustRecordCash(t, store, domain.CashDeposit, "999", date(2026, 1, 6)) // after the last equity date

	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{})
	require.NoError(t, err)
	require.Len(t, history.Points, 5)

	assert.Nil(t, history.Points[0].AccountEquityUSD, "no cash snapshot on Jan 1")
	assert.Equal(t, "150", string(*history.Points[1].AccountEquityUSD))
	assert.Equal(t, "0", string(history.Points[4].SecuritiesValueUSD), "cash-only date")
	assert.Equal(t, "250", string(*history.Points[4].AccountEquityUSD))

	equity := history.Equity
	require.NotNil(t, equity)
	assert.Equal(t, date(2026, 1, 5), equity.Latest.Date)
	assert.Equal(t, "350", string(equity.AllTimeHigh.ValueUSD))
	assert.Equal(t, date(2026, 1, 4), equity.AllTimeHigh.Date)
	assert.Equal(t, date(2026, 1, 2), equity.Baseline.Date)

	// The opening balance and the deposit after the latest equity date are excluded.
	require.NotNil(t, history.NetContributionsUSD)
	assert.Equal(t, "200", string(*history.NetContributionsUSD), "300 deposited minus 100 withdrawn after the baseline")
}

func TestValueHistoryFlatSeriesHasZeroDrawdown(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	seedSeries(t, store, voo.ID, date(2026, 1, 1), 100, 110, 120)

	history, err := store.GetValueHistory(context.Background(), domain.HistoryFilter{})
	require.NoError(t, err)

	drawdown := history.Securities.MaxDrawdown
	assert.Equal(t, "0", string(drawdown.DrawdownUSD))
	assert.True(t, drawdown.Recovered)
	assert.Equal(t, drawdown.PeakDate, drawdown.TroughDate)
}
