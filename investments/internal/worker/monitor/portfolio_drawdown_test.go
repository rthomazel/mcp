package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func day(month time.Month, d int) domain.Date { return time.Date(2026, month, d, 0, 0, 0, 0, time.UTC) }

func newDrawdown(store *fakeStore) *PortfolioDrawdown { return NewPortfolioDrawdown(store, store) }

func TestPortfolioDrawdownIsNamedAfterItsEvent(t *testing.T) {
	monitor := newDrawdown(&fakeStore{})

	assert.Equal(t, "portfolio.drawdown", monitor.Name())
}

func TestPortfolioDrawdownRecordsOneEventPerLevel(t *testing.T) {
	store := &fakeStore{created: true, drawdown: domain.DrawdownState{
		Date: day(11, 20), ValueUSD: "928", DrawdownPercent: "-7.2", HighDate: day(11, 1), HighValueUSD: "1000",
		Levels: []domain.DrawdownLevel{{Level: "-3", CrossedOn: day(11, 10)}, {Level: "-6", CrossedOn: day(11, 18)}},
	}}

	require.NoError(t, newDrawdown(store).Run(context.Background(), Params{"increment_pct": "3"}))

	assert.Equal(t, domain.Decimal("3"), store.increment)
	require.Len(t, store.recorded, 2)

	for index, subject := range []string{"-3@2026-11-10", "-6@2026-11-18"} {
		assert.Equal(t, domain.Events.PortfolioDrawdown, store.recorded[index].Type)
		assert.Equal(t, subject, *store.recorded[index].Subject)
	}

	var data domain.PortfolioDrawdownData
	require.NoError(t, json.Unmarshal(store.recorded[1].Data, &data))
	assert.Equal(t, domain.PortfolioDrawdownData{
		Level: "-6", CrossedOn: day(11, 18), Date: day(11, 20), DrawdownPercent: "-7.2",
		ValueUSD: "928", HighDate: day(11, 1), HighValueUSD: "1000",
	}, data)
}

func TestPortfolioDrawdownRecordsNothingWithoutALevel(t *testing.T) {
	store := &fakeStore{drawdown: domain.DrawdownState{Date: day(11, 20), DrawdownPercent: "-1"}}

	require.NoError(t, newDrawdown(store).Run(context.Background(), Params{"increment_pct": "3"}))
	assert.Empty(t, store.recorded)
}

func TestPortfolioDrawdownReturnsErrors(t *testing.T) {
	params := Params{"increment_pct": "3"}
	levels := domain.DrawdownState{Levels: []domain.DrawdownLevel{{Level: "-3", CrossedOn: day(11, 10)}}}

	assert.Error(t, newDrawdown(&fakeStore{drawdownErr: errors.New("db down")}).Run(context.Background(), params))
	assert.Error(t, newDrawdown(&fakeStore{drawdown: levels, recordErr: errors.New("db down")}).Run(context.Background(), params))
	assert.Error(t, newDrawdown(&fakeStore{}).Run(context.Background(), Params{}))
}

func TestDrawdownSubject(t *testing.T) {
	assert.Equal(t, "-4.5@2026-01-05", drawdownSubject("-4.5", day(1, 5)))
}
