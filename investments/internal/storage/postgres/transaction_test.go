package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func TestRecordTransactionEvents(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	jan := date(2026, 1, 5)

	notes := "first buy"
	result, err := store.RecordTransaction(context.Background(), domain.DraftTransaction{
		InstrumentID: voo.ID, EventType: domain.EventBuy, QuantityChange: "10", CashAmount: "-5000",
		ExecutedAt: jan.Add(15 * time.Hour), EffectiveDate: jan, Notes: &notes,
	})
	require.NoError(t, err)
	assert.NotZero(t, result.Transaction.ID)
	assert.Equal(t, domain.EventBuy, result.Transaction.EventType)
	assert.Equal(t, "10", string(result.Transaction.QuantityChange))
	assert.Equal(t, "-5000", string(result.Transaction.CashAmount))
	assert.Equal(t, jan, result.Transaction.EffectiveDate)
	assert.Equal(t, &notes, result.Transaction.Notes)
	assert.Nil(t, result.RebuildFrom)

	for _, step := range []struct {
		event          domain.EventType
		quantity, cash string
		day            int
	}{
		{domain.EventSell, "-3", "1560", 6},
		{domain.EventDividend, "0", "50", 7},
		{domain.EventTax, "0", "-7.50", 7},
		{domain.EventSplit, "7", "", 8},
		{domain.EventAdjustment, "0.1", "-50", 9},
		{domain.EventAdjustment, "-0.1", "", 9},
	} {
		_, err := record(t, store, voo.ID, step.event, step.quantity, step.cash, date(2026, 1, step.day))
		require.NoError(t, err, "%s", step.event)
	}

	positions, err := store.GetPositions(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, positions.Positions, 1)
	assert.Equal(t, "14.0", string(positions.Positions[0].Quantity))

	// Empty cash is stored as zero.
	list, err := store.ListTransactions(context.Background(), domain.TransactionFilter{})
	require.NoError(t, err)
	require.Len(t, list, 7)
	assert.Equal(t, "0", string(list[0].CashAmount))
}

func TestRecordTransactionRejections(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	_, err := record(t, store, 9999, domain.EventBuy, "1", "-1", date(2026, 1, 5))
	assert.ErrorIs(t, err, domain.ErrInstrumentNotFound)

	_, err = record(t, store, voo.ID, domain.EventBuy, "-1", "-1", date(2026, 1, 5))
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = record(t, store, voo.ID, domain.EventBuy, "10", "-100", date(2026, 1, 5))
	require.NoError(t, err)

	// Selling more than the holding is rejected and nothing is written.
	_, err = record(t, store, voo.ID, domain.EventSell, "-11", "110", date(2026, 1, 10))
	var negative *domain.NegativeBalanceError
	require.ErrorAs(t, err, &negative)
	assert.ErrorIs(t, err, domain.ErrNegativeBalance)
	assert.Equal(t, voo.ID, negative.InstrumentID)
	assert.Equal(t, date(2026, 1, 10), negative.OnDate)
	assert.Equal(t, "-1", string(negative.Quantity))

	list, err := store.ListTransactions(ctx, domain.TransactionFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestRecordTransactionBackdatedSale(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	_, err := record(t, store, voo.ID, domain.EventBuy, "10", "-100", date(2026, 1, 1))
	require.NoError(t, err)
	_, err = record(t, store, voo.ID, domain.EventSell, "-8", "80", date(2026, 1, 20))
	require.NoError(t, err)

	// Balance after all entries stays positive (10 - 5 - 8 would be -3, so use 2):
	// selling 5 on Jan 10 is fine on its own but makes the Jan 20 point negative.
	_, err = record(t, store, voo.ID, domain.EventSell, "-5", "50", date(2026, 1, 10))
	var negative *domain.NegativeBalanceError
	require.ErrorAs(t, err, &negative)
	assert.Equal(t, date(2026, 1, 20), negative.OnDate)
	assert.Equal(t, "-3", string(negative.Quantity))

	// A backdated sale that keeps every point non-negative is accepted.
	_, err = record(t, store, voo.ID, domain.EventSell, "-2", "20", date(2026, 1, 10))
	require.NoError(t, err)
}

func TestRecordTransactionSameDayOrdering(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	day := date(2026, 3, 2)

	// Sale executed before the buy on the same effective date is rejected.
	_, err := store.RecordTransaction(context.Background(), domain.DraftTransaction{
		InstrumentID: voo.ID, EventType: domain.EventBuy, QuantityChange: "5", CashAmount: "-5",
		ExecutedAt: day.Add(14 * time.Hour), EffectiveDate: day,
	})
	require.NoError(t, err)

	_, err = store.RecordTransaction(context.Background(), domain.DraftTransaction{
		InstrumentID: voo.ID, EventType: domain.EventSell, QuantityChange: "-5", CashAmount: "5",
		ExecutedAt: day.Add(10 * time.Hour), EffectiveDate: day,
	})
	assert.ErrorIs(t, err, domain.ErrNegativeBalance)

	_, err = store.RecordTransaction(context.Background(), domain.DraftTransaction{
		InstrumentID: voo.ID, EventType: domain.EventSell, QuantityChange: "-5", CashAmount: "5",
		ExecutedAt: day.Add(16 * time.Hour), EffectiveDate: day,
	})
	assert.NoError(t, err)
}

func TestRecordTransactionConcurrentSales(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	_, err := record(t, store, voo.ID, domain.EventBuy, "10", "-100", date(2026, 1, 1))
	require.NoError(t, err)

	const writers = 6

	errs := make([]error, writers)

	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			_, errs[i] = record(t, store, voo.ID, domain.EventSell, "-4", "40", date(2026, 1, 2))
		}()
	}

	wg.Wait()

	accepted := 0

	for _, err := range errs {
		if err == nil {
			accepted++
		} else {
			assert.ErrorIs(t, err, domain.ErrNegativeBalance)
		}
	}

	assert.Equal(t, 2, accepted, "only two sales of 4 fit in a holding of 10")

	positions, err := store.GetPositions(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, positions.Positions, 1)
	assert.Equal(t, "2", string(positions.Positions[0].Quantity))
	assert.Empty(t, positions.IntegrityErrors)
}

func TestRecordTransactionRebuildFrom(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")

	result, err := record(t, store, voo.ID, domain.EventBuy, "10", "-100", date(2026, 1, 1))
	require.NoError(t, err)
	assert.Nil(t, result.RebuildFrom, "no snapshots yet")

	insertSnapshot(t, store, voo.ID, date(2026, 1, 15))

	result, err = record(t, store, voo.ID, domain.EventBuy, "1", "-10", date(2026, 1, 10))
	require.NoError(t, err)
	require.NotNil(t, result.RebuildFrom)
	assert.Equal(t, date(2026, 1, 10), *result.RebuildFrom)

	result, err = record(t, store, voo.ID, domain.EventBuy, "1", "-10", date(2026, 1, 15))
	require.NoError(t, err)
	require.NotNil(t, result.RebuildFrom, "on the latest snapshot date counts")

	result, err = record(t, store, voo.ID, domain.EventBuy, "1", "-10", date(2026, 1, 16))
	require.NoError(t, err)
	assert.Nil(t, result.RebuildFrom, "after the latest snapshot")

	result, err = record(t, store, voo.ID, domain.EventDividend, "0", "5", date(2026, 1, 10))
	require.NoError(t, err)
	assert.Nil(t, result.RebuildFrom, "cash-only events do not change snapshots")
}

func TestListTransactions(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	empty, err := store.ListTransactions(ctx, domain.TransactionFilter{})
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	for day := 1; day <= 5; day++ {
		_, err := record(t, store, voo.ID, domain.EventBuy, "1", "-1", date(2026, 2, day))
		require.NoError(t, err)
	}

	_, err = record(t, store, qqq.ID, domain.EventBuy, "1", "-1", date(2026, 2, 3))
	require.NoError(t, err)

	all, err := store.ListTransactions(ctx, domain.TransactionFilter{})
	require.NoError(t, err)
	require.Len(t, all, 6)
	assert.Equal(t, date(2026, 2, 5), all[0].EffectiveDate, "newest first")
	assert.Equal(t, date(2026, 2, 1), all[5].EffectiveDate)

	byInstrument, err := store.ListTransactions(ctx, domain.TransactionFilter{InstrumentID: &qqq.ID})
	require.NoError(t, err)
	assert.Len(t, byInstrument, 1)

	ranged, err := store.ListTransactions(ctx, domain.TransactionFilter{
		InstrumentID: &voo.ID, From: dp(date(2026, 2, 2)), To: dp(date(2026, 2, 4)),
	})
	require.NoError(t, err)
	require.Len(t, ranged, 3)
	assert.Equal(t, date(2026, 2, 4), ranged[0].EffectiveDate)

	limited, err := store.ListTransactions(ctx, domain.TransactionFilter{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, limited, 2)

	_, err = record(t, store, voo.ID, domain.EventDividend, "0", "5", date(2026, 2, 6))
	require.NoError(t, err)

	dividends, err := store.ListTransactions(ctx, domain.TransactionFilter{EventType: domain.EventDividend})
	require.NoError(t, err)
	require.Len(t, dividends, 1)
	assert.Equal(t, domain.EventDividend, dividends[0].EventType)

	buysOfQQQ, err := store.ListTransactions(ctx, domain.TransactionFilter{InstrumentID: &qqq.ID, EventType: domain.EventBuy})
	require.NoError(t, err)
	assert.Len(t, buysOfQQQ, 1)

	_, err = store.ListTransactions(ctx, domain.TransactionFilter{EventType: "GIFT"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.ListTransactions(ctx, domain.TransactionFilter{Limit: -1})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.ListTransactions(ctx, domain.TransactionFilter{From: dp(date(2026, 3, 1)), To: dp(date(2026, 2, 1))})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}
