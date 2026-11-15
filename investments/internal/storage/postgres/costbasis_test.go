package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func recordFee(t *testing.T, store *Store, instrumentID int, event domain.EventType, quantity, cash, fee string, effective domain.Date) error {
	t.Helper()

	_, err := store.RecordTransaction(context.Background(), domain.DraftTransaction{
		InstrumentID: instrumentID, EventType: event, QuantityChange: domain.Decimal(quantity),
		CashAmount: domain.Decimal(cash), Fee: domain.Decimal(fee),
		ExecutedAt: effective.Add(12 * time.Hour), EffectiveDate: effective,
	})

	return err
}

func mustRecord(t *testing.T, store *Store, instrumentID int, event domain.EventType, quantity, cash string, effective domain.Date) {
	t.Helper()

	_, err := record(t, store, instrumentID, event, quantity, cash, effective)
	require.NoError(t, err)
}

func TestPnLAverageCostAcrossBuysAndPartialSell(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ref := date(2026, 6, 10)

	require.NoError(t, recordFee(t, store, voo.ID, domain.EventBuy, "10", "-1000", "10", date(2026, 6, 1)))
	mustRecord(t, store, voo.ID, domain.EventBuy, "10", "-1200", date(2026, 6, 2))
	// Selling 5 of 20 releases 5/20 of the 2200 cost: 550; net proceeds 700 give 150 realized.
	require.NoError(t, recordFee(t, store, voo.ID, domain.EventSell, "-5", "700", "2", date(2026, 6, 3)))
	mustRecord(t, store, voo.ID, domain.EventDividend, "0", "20", date(2026, 6, 4))
	mustRecord(t, store, voo.ID, domain.EventTax, "0", "-3", date(2026, 6, 4))
	insertPrice(t, store, voo.ID, ref, "120")

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	require.True(t, portfolio.Complete)
	require.Len(t, portfolio.Holdings, 1)

	holding := portfolio.Holdings[0]
	assert.Equal(t, "15", string(holding.Quantity))
	assert.Equal(t, "1650", string(holding.CostUSD))
	assert.Equal(t, "110", string(holding.AverageCostUSD)[:3])
	assert.Equal(t, "1800", string(*holding.MarketValueUSD))
	assert.Equal(t, "150", string(*holding.UnrealizedPnLUSD))
	assert.Contains(t, string(*holding.UnrealizedPnLPercent), "9.09")
	assert.Equal(t, "150", string(holding.RealizedPnLUSD))
	assert.Equal(t, "20", string(holding.DividendsUSD))
	assert.Equal(t, "3", string(holding.TaxUSD))
	assert.Equal(t, "12", string(holding.FeesUSD))
	assert.Equal(t, "317", string(*holding.TotalReturnUSD), "unrealized + realized + dividends - tax; fees are not subtracted again")

	assert.Equal(t, "1650", string(portfolio.PnL.CostUSD))
	assert.Equal(t, "317", string(*portfolio.PnL.TotalReturnUSD))
	assert.Empty(t, portfolio.ClosedPositions)
}

func TestPnLSellingEverythingReleasesTheWholeCostAndClosesThePosition(t *testing.T) {
	store := newTestStore(t)
	third := registerETF(t, store, "TRD")
	ref := date(2026, 6, 10)

	// Three equal buys of a third each leave rounding residue unless the last sale takes all remaining cost.
	mustRecord(t, store, third.ID, domain.EventBuy, "3", "-100", date(2026, 6, 1))
	mustRecord(t, store, third.ID, domain.EventSell, "-1", "40", date(2026, 6, 2))
	mustRecord(t, store, third.ID, domain.EventSell, "-2", "90", date(2026, 6, 3))
	mustRecord(t, store, third.ID, domain.EventDividend, "0", "5", date(2026, 6, 4))

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	require.True(t, portfolio.Complete, "a closed position needs no price")
	assert.Empty(t, portfolio.Holdings)
	require.Len(t, portfolio.ClosedPositions, 1)

	closed := portfolio.ClosedPositions[0]
	assert.Equal(t, "30", string(closed.RealizedPnLUSD), "40 + 90 - 100")
	assert.Equal(t, "35", string(closed.TotalReturnUSD))
	assert.Equal(t, "0", string(portfolio.PnL.CostUSD), "no residue")
}

func TestPnLSplitAndAdjustment(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ref := date(2026, 6, 10)

	mustRecord(t, store, voo.ID, domain.EventBuy, "10", "-1000", date(2026, 6, 1))
	mustRecord(t, store, voo.ID, domain.EventSplit, "10", "", date(2026, 6, 2))         // 20 shares, cost 1000
	mustRecord(t, store, voo.ID, domain.EventAdjustment, "2", "-100", date(2026, 6, 3)) // reinvested dividend adds cost
	mustRecord(t, store, voo.ID, domain.EventAdjustment, "1", "", date(2026, 6, 4))     // free shares, zero cost
	mustRecord(t, store, voo.ID, domain.EventAdjustment, "1", "50", date(2026, 6, 5))   // positive cash on an increase counts as none
	mustRecord(t, store, voo.ID, domain.EventAdjustment, "-4", "", date(2026, 6, 6))    // decrease realizes like a sale with zero proceeds
	insertPrice(t, store, voo.ID, ref, "60")

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	require.Len(t, portfolio.Holdings, 1)

	holding := portfolio.Holdings[0]
	// 20 + 2 + 1 + 1 = 24 shares at cost 1100, then 4 leave: 4/24 of 1100 is released.
	assert.Equal(t, "20", string(holding.Quantity))
	assert.Contains(t, string(holding.CostUSD), "916.66")
	assert.Contains(t, string(holding.RealizedPnLUSD), "-183.33")
}

func TestPnLWithholdsUnrealizedWhenAPriceIsMissing(t *testing.T) {
	store := newTestStore(t)
	priced := registerETF(t, store, "AAA")
	unpriced := registerETF(t, store, "BBB")
	ref := date(2026, 6, 10)

	mustRecord(t, store, priced.ID, domain.EventBuy, "1", "-10", date(2026, 6, 1))
	mustRecord(t, store, unpriced.ID, domain.EventBuy, "1", "-20", date(2026, 6, 1))
	insertPrice(t, store, priced.ID, ref, "15")

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	assert.False(t, portfolio.Complete)
	assert.Nil(t, portfolio.AccountEquityUSD)
	assert.Nil(t, portfolio.PnL.UnrealizedPnLUSD)
	assert.Nil(t, portfolio.PnL.TotalReturnUSD)
	assert.Equal(t, "30", string(portfolio.PnL.CostUSD), "cost is never withheld")

	require.Len(t, portfolio.Holdings, 2)
	assert.Equal(t, "5", string(*portfolio.Holdings[0].UnrealizedPnLUSD), "usable holdings keep their P&L")
	assert.Nil(t, portfolio.Holdings[1].UnrealizedPnLUSD)
	assert.Nil(t, portfolio.Holdings[1].TotalReturnUSD)
	assert.Equal(t, "20", string(portfolio.Holdings[1].CostUSD))
}

func TestPortfolioAccountEquity(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ref := date(2026, 6, 10)

	mustRecordCash(t, store, domain.CashOpeningBalance, "1000", date(2026, 6, 1))
	mustRecord(t, store, voo.ID, domain.EventBuy, "10", "-400", date(2026, 6, 2))
	insertPrice(t, store, voo.ID, ref, "50")

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	assert.Equal(t, "500", string(*portfolio.SecuritiesValueUSD))
	assert.Equal(t, "600", string(*portfolio.Cash.BalanceUSD))
	assert.Equal(t, "1100", string(*portfolio.AccountEquityUSD))
	assert.Contains(t, string(*portfolio.Holdings[0].Allocation), "1", "allocation stays over securities")
}

func TestPortfolioWithoutCashHasNoEquity(t *testing.T) {
	store := newTestStore(t)
	ref := date(2026, 6, 10)

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	assert.True(t, portfolio.Complete)
	assert.Equal(t, "0", string(*portfolio.SecuritiesValueUSD))
	assert.False(t, portfolio.Cash.Initialized)
	assert.Nil(t, portfolio.AccountEquityUSD)
	assert.Equal(t, "0", string(portfolio.PnL.CostUSD))
}

func TestFeeCannotExceedTheCashAmount(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	err := recordFee(t, store, voo.ID, domain.EventBuy, "1", "-10", "11", date(2026, 6, 1))
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	err = recordFee(t, store, voo.ID, domain.EventBuy, "1", "-10", "10", date(2026, 6, 1))
	require.NoError(t, err, "a fee equal to the cash amount is allowed")
}

func TestPortfolioSkipsAnInstrumentWhoseLedgerWentNegative(t *testing.T) {
	store := newTestStore(t)
	good := registerETF(t, store, "GOOD")
	broken := registerETF(t, store, "BAD")
	ref := date(2026, 6, 10)

	mustRecord(t, store, good.ID, domain.EventBuy, "1", "-10", date(2026, 6, 1))
	insertPrice(t, store, good.ID, ref, "12")

	// The store rejects this sale; a manual edit could still leave it in the ledger.
	_, err := store.DB().Exec(
		`INSERT INTO transactions (instrument_id, event_type, quantity_change, cash_amount, executed_at, effective_date)
		 VALUES ($1, 'SELL', -1, 5, $2::timestamptz, $2::date)`, broken.ID, date(2026, 6, 2))
	require.NoError(t, err)

	portfolio, err := store.GetPortfolio(context.Background(), &ref)
	require.NoError(t, err)
	require.Len(t, portfolio.IntegrityErrors, 1)
	assert.Equal(t, "BAD", portfolio.IntegrityErrors[0].Symbol)
	require.Len(t, portfolio.Holdings, 1)
	assert.Equal(t, "GOOD", portfolio.Holdings[0].Symbol)
	assert.Empty(t, portfolio.ClosedPositions)
	assert.Equal(t, "10", string(portfolio.PnL.CostUSD), "the broken instrument does not poison the totals")
}
