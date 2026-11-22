package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func recordCash(t *testing.T, store *Stores, kind domain.CashKind, amount string, effective domain.Date) (domain.CashRecordResult, error) {
	t.Helper()

	return store.Cash.Record(context.Background(), domain.CashTransaction{
		Kind: kind, Amount: domain.Decimal(amount), ExecutedAt: effective.Add(12 * time.Hour), EffectiveDate: effective,
	})
}

func mustRecordCash(t *testing.T, store *Stores, kind domain.CashKind, amount string, effective domain.Date) {
	t.Helper()

	_, err := recordCash(t, store, kind, amount, effective)
	require.NoError(t, err)
}

func TestCashIsNotInitializedWithoutAnOpeningBalance(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	balance, err := store.Cash.Get(ctx, dp(date(2026, 3, 1)))
	require.NoError(t, err)
	assert.False(t, balance.Initialized)
	assert.Nil(t, balance.BalanceUSD)
	assert.Nil(t, balance.BuyingPowerUSD)
	assert.Nil(t, balance.OpeningDate)

	mustRecordCash(t, store, domain.CashOpeningBalance, "1000", date(2026, 3, 10))

	balance, err = store.Cash.Get(ctx, dp(date(2026, 3, 1)))
	require.NoError(t, err)
	assert.False(t, balance.Initialized, "before the opening date")
	assert.Equal(t, date(2026, 3, 10), *balance.OpeningDate)
	assert.Nil(t, balance.BalanceUSD)
}

func TestCashBalanceCountsEachEntryOnceFromTheOpeningDate(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	// Cash before the opening balance is not part of cash.
	_, err := record(t, store, voo.ID, domain.EventBuy, "1", "-500", date(2026, 3, 1))
	require.NoError(t, err)

	mustRecordCash(t, store, domain.CashOpeningBalance, "1000", date(2026, 3, 10))
	mustRecordCash(t, store, domain.CashDeposit, "200.50", date(2026, 3, 11))
	mustRecordCash(t, store, domain.CashFee, "-0.50", date(2026, 3, 12))
	_, err = record(t, store, voo.ID, domain.EventBuy, "2", "-300", date(2026, 3, 13))
	require.NoError(t, err)
	_, err = record(t, store, voo.ID, domain.EventDividend, "0", "7.25", date(2026, 3, 14))
	require.NoError(t, err)
	_, err = record(t, store, voo.ID, domain.EventSplit, "3", "", date(2026, 3, 14))
	require.NoError(t, err)

	balance, err := store.Cash.Get(ctx, dp(date(2026, 3, 14)))
	require.NoError(t, err)
	require.True(t, balance.Initialized)
	assert.Equal(t, "907.25", string(*balance.BalanceUSD))
	assert.Equal(t, "907.25", string(*balance.BuyingPowerUSD))
	assert.Empty(t, balance.IntegrityErrors)

	earlier, err := store.Cash.Get(ctx, dp(date(2026, 3, 11)))
	require.NoError(t, err)
	assert.Equal(t, "1200.50", string(*earlier.BalanceUSD))
}

func TestNegativeCashIsReportedNotRejected(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	mustRecordCash(t, store, domain.CashOpeningBalance, "100", date(2026, 3, 1))
	_, err := record(t, store, voo.ID, domain.EventBuy, "1", "-250", date(2026, 3, 2))
	require.NoError(t, err, "a balance going negative is not rejected")
	mustRecordCash(t, store, domain.CashDeposit, "500", date(2026, 3, 3))

	negative, err := store.Cash.Get(context.Background(), dp(date(2026, 3, 2)))
	require.NoError(t, err)
	assert.Equal(t, "-150", string(*negative.BalanceUSD))
	assert.Equal(t, "0", string(*negative.BuyingPowerUSD), "buying power is floored at zero")
	require.Len(t, negative.IntegrityErrors, 1)
	assert.Equal(t, date(2026, 3, 2), negative.IntegrityErrors[0].FirstNegativeDate)

	later, err := store.Cash.Get(context.Background(), dp(date(2026, 3, 3)))
	require.NoError(t, err)
	assert.Equal(t, "350", string(*later.BalanceUSD))
	assert.Len(t, later.IntegrityErrors, 1, "the earlier dip is still reported")
}

func TestOnlyOneOpeningBalance(t *testing.T) {
	store := newTestStore(t)

	mustRecordCash(t, store, domain.CashOpeningBalance, "100", date(2026, 3, 1))

	_, err := recordCash(t, store, domain.CashOpeningBalance, "5", date(2026, 3, 2))
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = recordCash(t, store, domain.CashDeposit, "-5", date(2026, 3, 2))
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestCashRebuildFromOnlyForEntriesThatMoveAStoredBalance(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	insertSnapshot(t, store, voo.ID, date(2026, 4, 10))

	result, err := recordCash(t, store, domain.CashOpeningBalance, "100", date(2026, 4, 5))
	require.NoError(t, err)
	require.NotNil(t, result.RebuildFrom, "opening balance on or before the latest snapshot")
	assert.Equal(t, date(2026, 4, 5), *result.RebuildFrom)

	result, err = recordCash(t, store, domain.CashDeposit, "10", date(2026, 4, 1))
	require.NoError(t, err)
	assert.Nil(t, result.RebuildFrom, "before the opening balance")

	result, err = recordCash(t, store, domain.CashDeposit, "10", date(2026, 4, 11))
	require.NoError(t, err)
	assert.Nil(t, result.RebuildFrom, "after the latest snapshot")

	tradeResult, err := record(t, store, voo.ID, domain.EventDividend, "0", "5", date(2026, 4, 8))
	require.NoError(t, err)
	require.NotNil(t, tradeResult.RebuildFrom, "a dividend moves a stored cash balance")
	assert.Equal(t, date(2026, 4, 8), *tradeResult.RebuildFrom)
}

func TestListCashTransactionsFilters(t *testing.T) {
	store := newTestStore(t)

	mustRecordCash(t, store, domain.CashOpeningBalance, "100", date(2026, 5, 1))
	mustRecordCash(t, store, domain.CashDeposit, "10", date(2026, 5, 2))
	mustRecordCash(t, store, domain.CashWithdrawal, "-5", date(2026, 5, 3))

	all, err := store.Cash.List(context.Background(), domain.CashTransactionFilter{})
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, domain.CashWithdrawal, all[0].Kind, "newest first")

	deposits, err := store.Cash.List(context.Background(), domain.CashTransactionFilter{Kind: domain.CashDeposit})
	require.NoError(t, err)
	assert.Len(t, deposits, 1)

	ranged, err := store.Cash.List(context.Background(),
		domain.CashTransactionFilter{From: dp(date(2026, 5, 2)), To: dp(date(2026, 5, 2))})
	require.NoError(t, err)
	assert.Len(t, ranged, 1)
}

func TestSnapshotsCarryTheCashBalance(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// A date with no positions still gets a cash snapshot.
	mustRecordCash(t, store, domain.CashOpeningBalance, "250", date(2026, 11, 2))

	result, err := store.Snapshots.Write(ctx, date(2026, 11, 2))
	require.NoError(t, err)
	assert.True(t, result.Complete)
	assert.True(t, result.CashWritten)
	assert.Equal(t, 0, result.Rows)
	assert.Equal(t, "250", cashSnapshot(t, store, date(2026, 11, 2)))

	// Before the opening balance, no cash snapshot.
	result, err = store.Snapshots.Write(ctx, date(2026, 11, 1))
	require.NoError(t, err)
	assert.False(t, result.CashWritten)

	// A rebuild replaces the cash snapshots with the securities ones.
	mustRecordCash(t, store, domain.CashDeposit, "50", date(2026, 11, 2))
	seedHolding(t, store, "VTI", "1", date(2026, 11, 3), date(2026, 11, 2), date(2026, 11, 3)) // buys for 100 on the 3rd

	rebuilt, err := store.Snapshots.Rebuild(ctx, date(2026, 11, 2), date(2026, 11, 3), "deposit recorded late")
	require.NoError(t, err)
	assert.Equal(t, 2, rebuilt.CashDates)
	assert.Equal(t, "300", cashSnapshot(t, store, date(2026, 11, 2)))
	assert.Equal(t, "200", cashSnapshot(t, store, date(2026, 11, 3)))
}

func TestWriteSnapshotRefusesAnExistingCashSnapshot(t *testing.T) {
	store := newTestStore(t)

	mustRecordCash(t, store, domain.CashOpeningBalance, "250", date(2026, 11, 2))
	_, err := store.DB().Exec(`INSERT INTO cash_snapshots (date, balance_usd) VALUES ($1, 1)`, date(2026, 11, 2))
	require.NoError(t, err)

	_, err = store.Snapshots.Write(context.Background(), date(2026, 11, 2))
	require.ErrorIs(t, err, domain.ErrSnapshotExists)
	assert.Empty(t, updateStatuses(t, store, "snapshots"), "nothing is marked complete")
}

func cashSnapshot(t *testing.T, store *Stores, on domain.Date) string {
	t.Helper()

	var balance string

	require.NoError(t, store.DB().QueryRow(`SELECT balance_usd::text FROM cash_snapshots WHERE date = $1`, on).Scan(&balance))

	return balance
}
