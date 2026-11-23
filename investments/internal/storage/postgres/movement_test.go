package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

func seedPosition(t *testing.T, store *Stores, instrumentID int, on domain.Date, quantity, price string) {
	t.Helper()

	_, err := store.DB().Exec(
		`INSERT INTO snapshots (date, instrument_id, quantity, close_price, price_date) VALUES ($1, $2, $3::numeric, $4::numeric, $1)`,
		on, instrumentID, quantity, price)
	require.NoError(t, err)
}

func window(t *testing.T, text string) duration.Duration {
	t.Helper()

	parsed, err := duration.Parse("window", text)
	require.NoError(t, err)

	return parsed
}

func TestHighStateNewHighAndPrevious(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	state, err := store.GetHighState(ctx)
	require.NoError(t, err)
	assert.Nil(t, state, "no snapshots")

	seedSeries(t, store, voo.ID, date(2026, 1, 1), 100)

	state, err = store.GetHighState(ctx)
	require.NoError(t, err)
	assert.False(t, state.NewHigh, "the first snapshot has nothing before it")
	assert.Nil(t, state.Previous)

	// 100 150 120 150 160: the repeated 150 is not a new high, 160 is.
	seedSeries(t, store, voo.ID, date(2026, 1, 1), 0, 150, 120, 150)

	state, err = store.GetHighState(ctx)
	require.NoError(t, err)
	assert.False(t, state.NewHigh, "equal to the previous high")
	assert.Equal(t, date(2026, 1, 2), state.Previous.Date, "earliest date wins a tie")

	seedSeries(t, store, voo.ID, date(2026, 1, 1), 0, 0, 0, 0, 160)

	state, err = store.GetHighState(ctx)
	require.NoError(t, err)
	assert.True(t, state.NewHigh)
	assert.Equal(t, date(2026, 1, 5), state.Date)
	assert.Equal(t, "160", string(state.ValueUSD))
	assert.Equal(t, "150", string(state.Previous.ValueUSD))
	assert.Equal(t, "10", string(state.IncreaseUSD))
	assert.Contains(t, string(state.IncreasePercent), "6.66")
}

func TestMarketMovementIgnoresTrades(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	gld := registerETF(t, store, "GLD")
	ctx := context.Background()

	// Day 1: 10 VOO at 100 and 5 QQQ at 200 = 2000.
	seedPosition(t, store, voo.ID, date(2026, 1, 1), "10", "100")
	seedPosition(t, store, qqq.ID, date(2026, 1, 1), "5", "200")
	// Day 2: VOO +3 per share on 10 shares, QQQ -4 on 5, and 2 GLD bought at 50 (a trade).
	seedPosition(t, store, voo.ID, date(2026, 1, 2), "10", "103")
	seedPosition(t, store, qqq.ID, date(2026, 1, 2), "5", "196")
	seedPosition(t, store, gld.ID, date(2026, 1, 2), "2", "50")

	movement, err := store.GetMarketMovement(ctx, window(t, "1d"), "1")
	require.NoError(t, err)
	require.NotNil(t, movement)

	assert.Equal(t, date(2026, 1, 1), movement.StartDate)
	assert.Equal(t, date(2026, 1, 2), movement.EndDate)
	assert.Equal(t, "2000", string(movement.StartValueUSD))
	assert.Equal(t, "2110", string(movement.EndValueUSD))
	assert.Equal(t, "10", string(movement.MarketUSD), "30 from VOO minus 20 from QQQ")
	assert.Equal(t, "100", string(movement.TradesUSD), "the GLD purchase")
	assert.Equal(t, "0.5", string(*movement.MarketPercent))
	assert.False(t, movement.AtLeastUp)
	assert.False(t, movement.AtLeastDown)
	assert.Equal(t, []domain.Contributor{{Symbol: "VOO", MovementUSD: "30"}}, movement.Gainers)
	assert.Equal(t, []domain.Contributor{{Symbol: "QQQ", MovementUSD: "-20"}}, movement.Losers)

	movement, err = store.GetMarketMovement(ctx, window(t, "1d"), "0.5")
	require.NoError(t, err)
	assert.True(t, movement.AtLeastUp, "the threshold is inclusive")
}

func TestMarketMovementOverAWindow(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	// Jan 1..5 at 100, 101, 102, 103, 104 on 10 shares; Jan 3 and 4 are the weekend.
	for _, point := range []struct {
		day   int
		price string
	}{{1, "100"}, {2, "101"}, {5, "104"}} {
		seedPosition(t, store, voo.ID, date(2026, 1, point.day), "10", point.price)
	}

	// Three days back from Jan 5 is Jan 2: the base, and the dates after it, are summed.
	movement, err := store.GetMarketMovement(ctx, window(t, "3d"), "3")
	require.NoError(t, err)
	require.NotNil(t, movement)
	assert.Equal(t, date(2026, 1, 2), movement.StartDate)
	assert.Equal(t, "30", string(movement.MarketUSD))
	assert.False(t, movement.AtLeastUp, "30 over 1010 is 2.97 percent")
}

func TestMarketMovementNeedsASnapshotBeforeTheWindow(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	ctx := context.Background()

	movement, err := store.GetMarketMovement(ctx, window(t, "1d"), "1")
	require.NoError(t, err)
	assert.Nil(t, movement)

	seedPosition(t, store, voo.ID, date(2026, 1, 5), "10", "100")

	movement, err = store.GetMarketMovement(ctx, window(t, "3d"), "1")
	require.NoError(t, err)
	assert.Nil(t, movement, "the window starts before the first snapshot")

	_, err = store.GetMarketMovement(ctx, window(t, "1d"), "0")
	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestMarketMovementDownAndClosedPositions(t *testing.T) {
	store := newTestStore(t)
	voo := registerETF(t, store, "VOO")
	qqq := registerETF(t, store, "QQQ")
	ctx := context.Background()

	seedPosition(t, store, voo.ID, date(2026, 1, 1), "10", "100")
	seedPosition(t, store, qqq.ID, date(2026, 1, 1), "1", "500")
	// QQQ is sold on day 2 and contributes nothing; VOO falls 5 percent.
	seedPosition(t, store, voo.ID, date(2026, 1, 2), "10", "95")

	movement, err := store.GetMarketMovement(ctx, window(t, "1d"), "3")
	require.NoError(t, err)
	assert.Equal(t, "-50", string(movement.MarketUSD))
	assert.True(t, movement.AtLeastDown)
	assert.False(t, movement.AtLeastUp)
	assert.Equal(t, "-500", string(movement.TradesUSD), "the QQQ sale")
}

func TestLatestEvent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	latest, err := store.Events.Latest(ctx, "a.b", "")
	require.NoError(t, err)
	assert.Nil(t, latest)

	first, second := "one", "two"
	recordDraft(t, store, "a.b", &first)
	recordDraft(t, store, "a.b", &second)
	recordDraft(t, store, "a.c", &first)

	latest, err = store.Events.Latest(ctx, "a.b", "")
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "two", *latest.Subject)

	latest, err = store.Events.Latest(ctx, "a.b", "on")
	require.NoError(t, err)
	assert.Equal(t, "one", *latest.Subject, "subject prefix")

	latest, err = store.Events.Latest(ctx, "a.b", "x")
	require.NoError(t, err)
	assert.Nil(t, latest)
}
