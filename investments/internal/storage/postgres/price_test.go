package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func bar(day int, closePrice string) domain.PriceBar {
	return domain.PriceBar{
		Date: date(2026, 11, day), Open: "100.1234567890", High: "110", Low: "90", Close: domain.Decimal(closePrice),
		AdjustedClose: domain.Decimal(closePrice), Volume: 1000,
	}
}

type priceRow struct {
	Open, Close string
	Volume      int64
}

func storedPrices(t *testing.T, store *Store, instrumentID int) map[string]priceRow {
	t.Helper()

	rows, err := store.DB().Query(
		`SELECT to_char(date, 'YYYY-MM-DD'), open::text, close::text, volume FROM prices WHERE instrument_id = $1`, instrumentID)
	require.NoError(t, err)

	defer func() { _ = rows.Close() }()

	got := map[string]priceRow{}

	for rows.Next() {
		var (
			day string
			row priceRow
		)

		require.NoError(t, rows.Scan(&day, &row.Open, &row.Close, &row.Volume))

		got[day] = row
	}

	require.NoError(t, rows.Err())

	return got
}

func TestUpsertPrices(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	require.NoError(t, store.UpsertPrices(ctx, voo.ID, nil), "empty is a no-op")

	require.NoError(t, store.UpsertPrices(ctx, voo.ID, []domain.PriceBar{bar(12, "696.00"), bar(13, "693.38")}))

	stored := storedPrices(t, store, voo.ID)
	require.Len(t, stored, 2)
	assert.Equal(t, "100.1234567890", stored["2026-11-12"].Open, "exact decimal text survives")
	assert.Equal(t, "693.38", stored["2026-11-13"].Close)

	// A revised day replaces the stored record; other days are untouched.
	revised := bar(13, "694.00")
	revised.Volume = 2000

	require.NoError(t, store.UpsertPrices(ctx, voo.ID, []domain.PriceBar{revised, bar(14, "700")}))

	stored = storedPrices(t, store, voo.ID)
	require.Len(t, stored, 3)
	assert.Equal(t, "694.00", stored["2026-11-13"].Close)
	assert.EqualValues(t, 2000, stored["2026-11-13"].Volume)
	assert.Equal(t, "696.00", stored["2026-11-12"].Close)
}

func TestUpsertPricesRejections(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	assert.ErrorIs(t, store.UpsertPrices(ctx, 9999, []domain.PriceBar{bar(12, "1")}), domain.ErrInstrumentNotFound)

	// One bad bar means none are stored.
	assert.ErrorIs(t, store.UpsertPrices(ctx, voo.ID, []domain.PriceBar{bar(12, "696"), bar(13, "0")}), domain.ErrInvalidInput)
	assert.ErrorIs(t, store.UpsertPrices(ctx, voo.ID, []domain.PriceBar{bar(12, "696"), bar(12, "697")}), domain.ErrInvalidInput)
	assert.Empty(t, storedPrices(t, store, voo.ID))
}

func TestLatestPriceDate(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	latest, err := store.LatestPriceDate(ctx, voo.ID)
	require.NoError(t, err)
	assert.Nil(t, latest)

	require.NoError(t, store.UpsertPrices(ctx, voo.ID, []domain.PriceBar{bar(12, "1"), bar(13, "2")}))

	latest, err = store.LatestPriceDate(ctx, voo.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, date(2026, 11, 13), *latest)

	other, err := store.LatestPriceDate(ctx, qqq.ID)
	require.NoError(t, err)
	assert.Nil(t, other, "per instrument")

	unknown, err := store.LatestPriceDate(ctx, 9999)
	require.NoError(t, err)
	assert.Nil(t, unknown)
}
