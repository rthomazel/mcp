package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func TestGetPortfolioComplete(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	gone := registerETF(t, store, "OLD")
	ctx := context.Background()
	ref := date(2026, 5, 15)

	_, err := record(t, store, voo.ID, domain.EventBuy, "10", "-4000", date(2026, 5, 1))
	require.NoError(t, err)
	_, err = record(t, store, qqq.ID, domain.EventBuy, "10", "-1000", date(2026, 5, 1))
	require.NoError(t, err)
	_, err = record(t, store, gone.ID, domain.EventBuy, "1", "-1", date(2026, 5, 1))
	require.NoError(t, err)
	_, err = record(t, store, gone.ID, domain.EventSell, "-1", "1", date(2026, 5, 2))
	require.NoError(t, err)
	// Cash events are excluded from valuation.
	_, err = record(t, store, voo.ID, domain.EventDividend, "0", "9999", date(2026, 5, 3))
	require.NoError(t, err)

	insertPrice(t, store, voo.ID, date(2026, 5, 13), "390") // older price, superseded
	insertPrice(t, store, voo.ID, date(2026, 5, 14), "400")
	insertPrice(t, store, voo.ID, date(2026, 5, 20), "999") // after the reference date
	insertPrice(t, store, qqq.ID, date(2026, 5, 15), "100")

	portfolio, err := store.GetPortfolio(ctx, &ref)
	require.NoError(t, err)

	assert.True(t, portfolio.Complete)
	assert.Equal(t, ref, portfolio.Date)
	require.NotNil(t, portfolio.SecuritiesValueUSD)
	assert.Equal(t, "5000", string(*portfolio.SecuritiesValueUSD))
	require.Len(t, portfolio.Holdings, 2)

	q, v := portfolio.Holdings[0], portfolio.Holdings[1]
	assert.Equal(t, "QQQ", q.Symbol)
	assert.Equal(t, "100", string(*q.ClosePrice))
	assert.Equal(t, "1000", string(*q.MarketValueUSD))
	assert.Equal(t, date(2026, 5, 15), *q.PriceDate)
	assert.Empty(t, q.PriceIssue)

	assert.Equal(t, "VOO", v.Symbol)
	assert.Equal(t, "400", string(*v.ClosePrice))
	assert.Equal(t, "4000", string(*v.MarketValueUSD))
	assert.Equal(t, date(2026, 5, 14), *v.PriceDate)

	require.NotNil(t, q.Allocation)
	require.NotNil(t, v.Allocation)
	assert.Contains(t, string(*q.Allocation), "0.2")
	assert.Contains(t, string(*v.Allocation), "0.8")
	assert.Empty(t, portfolio.IntegrityErrors)
}

func TestGetPortfolioIncomplete(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	ark := registerETF(t, store, "ARKK")
	ref := date(2026, 5, 15)

	for _, id := range []int{voo.ID, qqq.ID, ark.ID} {
		_, err := record(t, store, id, domain.EventBuy, "10", "-100", date(2026, 5, 1))
		require.NoError(t, err)
	}

	insertPrice(t, store, voo.ID, date(2026, 5, 15), "400")
	insertPrice(t, store, qqq.ID, date(2026, 5, 10), "100") // exactly 5 days old: stale
	// ARKK has no price: missing.

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)

	assert.False(t, portfolio.Complete)
	assert.Nil(t, portfolio.SecuritiesValueUSD)
	require.Len(t, portfolio.Holdings, 3)

	byIssue := map[string]domain.Holding{}
	for _, h := range portfolio.Holdings {
		assert.Nil(t, h.Allocation, h.Symbol)
		byIssue[h.Symbol] = h
	}

	usable := byIssue["VOO"]
	assert.Empty(t, usable.PriceIssue)
	assert.Equal(t, "4000", string(*usable.MarketValueUSD))

	stale := byIssue["QQQ"]
	assert.Equal(t, domain.PriceIssueStale, stale.PriceIssue)
	assert.Nil(t, stale.ClosePrice, "a stale price is never used")
	assert.Nil(t, stale.MarketValueUSD)
	assert.Equal(t, date(2026, 5, 10), *stale.PriceDate)

	missing := byIssue["ARKK"]
	assert.Equal(t, domain.PriceIssueMissing, missing.PriceIssue)
	assert.Nil(t, missing.ClosePrice)
	assert.Nil(t, missing.PriceDate)
}

func TestGetPortfolioStaleBoundary(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ref := date(2026, 5, 15)

	_, err := record(t, store, voo.ID, domain.EventBuy, "1", "-1", date(2026, 5, 1))
	require.NoError(t, err)
	insertPrice(t, store, voo.ID, date(2026, 5, 11), "400") // four days old: usable

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	assert.True(t, portfolio.Complete)
	assert.Equal(t, "400", string(*portfolio.SecuritiesValueUSD))
}

func TestGetPortfolioEmpty(t *testing.T) {
	store := newTestStore(t)
	ref := date(2026, 5, 15)

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	assert.True(t, portfolio.Complete)
	require.NotNil(t, portfolio.SecuritiesValueUSD)
	assert.Equal(t, "0", string(*portfolio.SecuritiesValueUSD))
	assert.NotNil(t, portfolio.Holdings)
	assert.Empty(t, portfolio.Holdings)
}

func TestGetPortfolioDefaultsToMarketToday(t *testing.T) {
	store := newTestStore(t)

	portfolio, err := store.GetPortfolio(context.Background(), nil)
	require.NoError(t, err)

	year, month, day := time.Now().In(store.marketLocation).Date()
	assert.Equal(t, date(year, month, day), portfolio.Date)
}

func TestGetPortfolioReportsIntegrityErrors(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ref := date(2026, 5, 15)

	_, err := record(t, store, voo.ID, domain.EventBuy, "1", "-1", date(2026, 5, 1))
	require.NoError(t, err)
	_, err = store.DB().Exec(`
		INSERT INTO transactions (instrument_id, event_type, quantity_change, cash_amount, executed_at, effective_date)
		VALUES ($1, 'SELL', -3, 3, '2026-05-02 12:00+00', '2026-05-02')`, voo.ID)
	require.NoError(t, err)

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	require.Len(t, portfolio.IntegrityErrors, 1)
	assert.Equal(t, "-2", string(portfolio.IntegrityErrors[0].Quantity))
}
