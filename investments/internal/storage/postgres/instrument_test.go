package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func TestInstruments(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	empty, err := store.ListInstruments(ctx)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)

	voo, err := store.RegisterInstrument(ctx, domain.Instrument{
		Symbol: " voo ", Exchange: "nyse", Name: "Vanguard S&P 500", Currency: "usd", InstrumentType: "etf",
	})
	require.NoError(t, err)
	assert.NotZero(t, voo.ID)
	assert.Equal(t, "VOO", voo.Symbol)
	assert.Equal(t, "NYSE", voo.Exchange)
	assert.Equal(t, "USD", voo.Currency)
	assert.Equal(t, "ETF", voo.InstrumentType)

	_, err = store.RegisterInstrument(ctx, domain.Instrument{
		Symbol: "VOO", Exchange: "NYSE", Name: "dup", Currency: "USD", InstrumentType: "ETF",
	})
	assert.ErrorIs(t, err, domain.ErrInstrumentExists)

	// Same ticker on another exchange is a different instrument.
	_, err = store.RegisterInstrument(ctx, domain.Instrument{
		Symbol: "VOO", Exchange: "BATS", Name: "other listing", Currency: "USD", InstrumentType: "ETF",
	})
	require.NoError(t, err)

	_, err = store.RegisterInstrument(ctx, domain.Instrument{
		Symbol: "PETR4", Exchange: "B3", Name: "Petrobras", Currency: "BRL", InstrumentType: "STOCK",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.RegisterInstrument(ctx, domain.Instrument{Symbol: "X", Exchange: "NYSE", Currency: "USD", InstrumentType: "ETF"})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = store.RegisterInstrument(ctx, domain.Instrument{Symbol: "AAPL", Exchange: "NASDAQ", Name: "Apple", Currency: "USD", InstrumentType: "STOCK"})
	require.NoError(t, err)

	list, err := store.ListInstruments(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, []string{"AAPL", "VOO", "VOO"}, []string{list[0].Symbol, list[1].Symbol, list[2].Symbol})
	assert.Equal(t, []string{"NASDAQ", "BATS", "NYSE"}, []string{list[0].Exchange, list[1].Exchange, list[2].Exchange})
}
