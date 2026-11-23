package monitor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func newMovement(store *fakeStore) *PortfolioMovement { return NewPortfolioMovement(store, store) }

func movementOn(on domain.Date, up, down bool) *domain.MarketMovement {
	percent := domain.Decimal("3.4")

	return &domain.MarketMovement{
		StartDate: day(11, 19), EndDate: on, StartValueUSD: "1000", EndValueUSD: "1034", MarketUSD: "34", MarketPercent: &percent,
		TradesUSD: "0", AtLeastUp: up, AtLeastDown: down,
		Gainers: []domain.Contributor{{Symbol: "VOO", MovementUSD: "34"}}, Losers: []domain.Contributor{},
	}
}

func TestPortfolioMovementIsNamedAfterItsEvent(t *testing.T) {
	monitor := newMovement(&fakeStore{})

	assert.Equal(t, "portfolio.movement", monitor.Name())
	assert.Equal(t, domain.ClockTime{Hour: 7}, monitor.At())
}

func TestPortfolioMovementRecordsTheReachedSide(t *testing.T) {
	store := &fakeStore{created: true, movement: movementOn(day(11, 20), true, false)}

	require.NoError(t, newMovement(store).Run(context.Background(), Params{"variation_pct": "3", "window": "3d"}))

	assert.Equal(t, domain.Decimal("3"), store.threshold)
	assert.Equal(t, 3, store.window.Days)
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.PortfolioMovement, store.recorded[0].Type)
	assert.Equal(t, "2026-11-20@up", *store.recorded[0].Subject)

	var data domain.PortfolioMovementData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, "up", data.Direction)
	assert.Equal(t, "3d", data.Window)
	assert.Equal(t, domain.Decimal("3.4"), data.MarketPercent)
	assert.Equal(t, day(11, 19), data.StartDate)
	assert.Equal(t, "VOO", data.Gainers[0].Symbol)
}

func TestPortfolioMovementWatchesTheConfiguredDirection(t *testing.T) {
	for name, tc := range map[string]struct {
		direction string
		up, down  bool
		subject   string
	}{
		"up, reached up":     {"up", true, false, "2026-11-20@up"},
		"up, reached down":   {"up", false, true, ""},
		"down, reached down": {"down", false, true, "2026-11-20@down"},
		"down, reached up":   {"down", true, false, ""},
		"both, down":         {"both", false, true, "2026-11-20@down"},
		"both, neither":      {"both", false, false, ""},
	} {
		store := &fakeStore{created: true, movement: movementOn(day(11, 20), tc.up, tc.down)}

		require.NoError(t, newMovement(store).Run(context.Background(), Params{"variation_pct": "3", "direction": tc.direction}), name)

		if tc.subject == "" {
			assert.Empty(t, store.recorded, name)

			continue
		}

		require.Len(t, store.recorded, 1, name)
		assert.Equal(t, tc.subject, *store.recorded[0].Subject, name)
	}
}

func TestPortfolioMovementIntervalSpacesAnnouncements(t *testing.T) {
	for name, tc := range map[string]struct {
		interval string
		lastDay  int
		recorded int
	}{
		"default, previous day": {"", 19, 1},
		"default, same date":    {"", 20, 0},
		"3d, two days after":    {"3d", 18, 0},
		"3d, three days after":  {"3d", 17, 1},
	} {
		params := Params{"variation_pct": "3"}
		if tc.interval != "" {
			params["interval"] = tc.interval
		}

		store := &fakeStore{created: true, movement: movementOn(day(11, 20), true, false), latest: eventOn(day(11, tc.lastDay))}

		require.NoError(t, newMovement(store).Run(context.Background(), params), name)
		assert.Len(t, store.recorded, tc.recorded, name)
	}
}

func TestPortfolioMovementRecordsNothingWithoutEnoughSnapshots(t *testing.T) {
	store := &fakeStore{}

	require.NoError(t, newMovement(store).Run(context.Background(), Params{"variation_pct": "3"}))
	assert.Empty(t, store.recorded)
}

func TestPortfolioMovementValidatesItsSettings(t *testing.T) {
	monitor := newMovement(&fakeStore{})

	assert.NoError(t, monitor.ValidateParams(Params{"variation_pct": "3"}))
	assert.NoError(t, monitor.ValidateParams(Params{
		"variation_pct": "2.5", "direction": "Down", "window": "1w", "interval": "3 days",
	}))

	for name, params := range map[string]Params{
		"missing variation":  {},
		"zero variation":     {"variation_pct": "0"},
		"negative variation": {"variation_pct": "-1"},
		"number variation":   {"variation_pct": 3},
		"unknown direction":  {"variation_pct": "3", "direction": "sideways"},
		"bare window":        {"variation_pct": "3", "window": 5},
		"zero interval":      {"variation_pct": "3", "interval": "0d"},
		"unknown setting":    {"variation_pct": "3", "lookback": "3mo"},
	} {
		assert.Error(t, monitor.ValidateParams(params), name)
	}
}
