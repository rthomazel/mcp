package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func newATH(store *fakeStore) *PortfolioATH { return NewPortfolioATH(store, store) }

func newHigh(on domain.Date) *domain.HighState {
	return &domain.HighState{
		Date: on, ValueUSD: "1100", NewHigh: true, Previous: &domain.Extreme{Date: day(11, 1), ValueUSD: "1000"},
		IncreaseUSD: "100", IncreasePercent: "10",
	}
}

func eventOn(on domain.Date) *domain.Event {
	return &domain.Event{Data: json.RawMessage(fmt.Sprintf(`{"date":%q}`, on.Format("2006-01-02T00:00:00Z")))}
}

func TestPortfolioATHIsNamedAfterItsEvent(t *testing.T) {
	monitor := newATH(&fakeStore{})

	assert.Equal(t, "portfolio.ath", monitor.Name())
}

func TestPortfolioATHRecordsANewHigh(t *testing.T) {
	store := &fakeStore{created: true, high: newHigh(day(11, 20))}

	require.NoError(t, newATH(store).Run(context.Background(), nil))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.PortfolioATH, store.recorded[0].Type)
	assert.Equal(t, "2026-11-20", *store.recorded[0].Subject)

	var data domain.PortfolioATHData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, domain.PortfolioATHData{
		Date: day(11, 20), ValueUSD: "1100", PreviousHighDate: day(11, 1), PreviousHighValueUSD: "1000",
		IncreaseUSD: "100", IncreasePercent: "10",
	}, data)
}

func TestPortfolioATHRecordsNothingWithoutANewHigh(t *testing.T) {
	notHigh := newHigh(day(11, 20))
	notHigh.NewHigh = false

	for name, high := range map[string]*domain.HighState{"no snapshots": nil, "not a high": notHigh} {
		store := &fakeStore{high: high}

		require.NoError(t, newATH(store).Run(context.Background(), nil), name)
		assert.Empty(t, store.recorded, name)
	}
}

func TestPortfolioATHIntervalSpacesAnnouncements(t *testing.T) {
	for name, tc := range map[string]struct {
		params   Params
		lastDay  int
		recorded int
	}{
		"default, 14 days after":  {nil, 6, 0},
		"default, 15 days after":  {nil, 5, 1},
		"one day announces every": {Params{"interval": "1d"}, 19, 1},
		"month interval":          {Params{"interval": "1mo"}, 6, 0},
	} {
		store := &fakeStore{created: true, high: newHigh(day(11, 20)), latest: eventOn(day(11, tc.lastDay))}

		require.NoError(t, newATH(store).Run(context.Background(), tc.params), name)
		assert.Len(t, store.recorded, tc.recorded, name)

		if tc.recorded == 1 {
			var data domain.PortfolioATHData
			require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
			assert.Equal(t, day(11, tc.lastDay), *data.LastAnnouncedDate, name)
		}
	}
}

func TestPortfolioATHValidatesTheInterval(t *testing.T) {
	monitor := newATH(&fakeStore{})

	assert.NoError(t, monitor.ValidateParams(nil))
	assert.NoError(t, monitor.ValidateParams(Params{"interval": "2w"}))

	for name, params := range map[string]Params{
		"bare number": {"interval": 15}, "zero": {"interval": "0d"}, "unknown": {"window": "1d"}, "intraday": {"interval": "12h"},
	} {
		assert.Error(t, monitor.ValidateParams(params), name)
	}
}
