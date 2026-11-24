package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func (s *fakeStore) GetInstrumentDrawdownState(_ context.Context, ticker string, increment domain.Decimal) (domain.DrawdownState, error) {
	s.increment = increment

	return s.instrumentDrawdown[ticker], s.instrumentErrs[ticker]
}

func (s *fakeStore) GetInstrumentHighState(_ context.Context, ticker string) (*domain.HighState, error) {
	return s.instrumentHigh[ticker], s.instrumentErrs[ticker]
}

func (s *fakeStore) GetPriceTarget(_ context.Context, ticker string, price domain.Decimal) (*domain.PriceTarget, error) {
	s.targetQueries = append(s.targetQueries, ticker+"@"+string(price))

	return s.priceTargets[ticker+"@"+string(price)], s.instrumentErrs[ticker]
}

func (s *fakeStore) ListHeldTickers(context.Context) ([]string, error) { return s.held, s.heldErr }

func qqqDrawdown() domain.DrawdownState {
	return domain.DrawdownState{
		Date: day(11, 20), ValueUSD: "400", DrawdownPercent: "-11", HighDate: day(10, 1), HighValueUSD: "450",
		Levels: []domain.DrawdownLevel{{Level: "-5", CrossedOn: day(11, 5)}, {Level: "-10", CrossedOn: day(11, 18)}},
	}
}

func TestInstrumentDrawdownRecordsEachLevelPerInstrument(t *testing.T) {
	store := &fakeStore{created: true, instrumentDrawdown: map[string]domain.DrawdownState{
		"QQQ": qqqDrawdown(), "VOO": {Date: day(11, 20), DrawdownPercent: "-1"},
	}}
	monitor := NewInstrumentDrawdown(store, store)

	assert.Equal(t, "instrument.drawdown", monitor.Name())

	params := yamlParams(t, "increment_pct: \"5\"\ninstruments: [qqq, VOO]\n")
	require.NoError(t, monitor.Run(context.Background(), params))
	assert.Equal(t, domain.Decimal("5"), store.increment)

	require.Len(t, store.recorded, 2)
	assert.Equal(t, domain.Events.InstrumentDrawdown, store.recorded[0].Type)
	assert.Equal(t, "QQQ@-5@2026-11-05", *store.recorded[0].Subject)
	assert.Equal(t, "QQQ@-10@2026-11-18", *store.recorded[1].Subject)

	var data domain.InstrumentDrawdownData
	require.NoError(t, json.Unmarshal(store.recorded[1].Data, &data))
	assert.Equal(t, domain.InstrumentDrawdownData{
		Instrument: "QQQ", Level: "-10", CrossedOn: day(11, 18), Date: day(11, 20), DrawdownPercent: "-11",
		Close: "400", HighDate: day(10, 1), HighClose: "450",
	}, data)
}

func TestInstrumentDrawdownHeldSelectionFollowsTheHoldings(t *testing.T) {
	store := &fakeStore{
		created: true, held: []string{"QQQ"},
		instrumentDrawdown: map[string]domain.DrawdownState{"QQQ": qqqDrawdown(), "VOO": qqqDrawdown()},
	}

	params := yamlParams(t, "increment_pct: \"5\"\ninstruments: held\n")
	require.NoError(t, NewInstrumentDrawdown(store, store).Run(context.Background(), params))
	assert.Len(t, store.recorded, 2, "VOO is not held")

	store = &fakeStore{heldErr: errors.New("down")}
	assert.Error(t, NewInstrumentDrawdown(store, store).Run(context.Background(), params))
}

func TestInstrumentDrawdownSkipsUnregisteredAndReportsFailures(t *testing.T) {
	store := &fakeStore{created: true, instrumentDrawdown: map[string]domain.DrawdownState{"QQQ": qqqDrawdown()},
		instrumentErrs: map[string]error{"NOPE": domain.ErrInstrumentNotFound, "VOO": errors.New("boom")}}
	monitor := NewInstrumentDrawdown(store, store)

	err := monitor.Run(context.Background(), yamlParams(t, "increment_pct: \"5\"\ninstruments: [NOPE, VOO, QQQ]\n"))
	require.ErrorContains(t, err, "VOO")
	assert.NotContains(t, err.Error(), "NOPE")
	assert.Len(t, store.recorded, 2, "one failing instrument does not stop the others")

	store.recordErr = errors.New("full")
	assert.Error(t, monitor.Run(context.Background(), yamlParams(t, "increment_pct: \"5\"\ninstruments: [QQQ]\n")))
}

func TestInstrumentDrawdownValidatesSettings(t *testing.T) {
	monitor := NewInstrumentDrawdown(&fakeStore{}, &fakeStore{})

	for name, content := range map[string]string{
		"valid":            "increment_pct: \"5\"\ninstruments: held\n",
		"missing":          "instruments: held\n",
		"zero":             "increment_pct: \"0\"\ninstruments: held\n",
		"no instruments":   "increment_pct: \"5\"\n",
		"empty list":       "increment_pct: \"5\"\ninstruments: []\n",
		"repeated":         "increment_pct: \"5\"\ninstruments: [QQQ, qqq]\n",
		"not held or list": "increment_pct: \"5\"\ninstruments: all\n",
		"unknown":          "increment_pct: \"5\"\ninstruments: held\nwindow: 1d\n",
	} {
		err := monitor.ValidateParams(yamlParams(t, content))
		if name == "valid" {
			assert.NoError(t, err, name)
		} else {
			assert.Error(t, err, name)
		}
	}

	assert.Error(t, monitor.Run(context.Background(), nil))
}

func instrumentHigh(on domain.Date) *domain.HighState {
	return &domain.HighState{
		Date: on, ValueUSD: "450", NewHigh: true, Previous: &domain.Extreme{Date: day(11, 1), ValueUSD: "440"},
		IncreaseUSD: "10", IncreasePercent: "2.27",
	}
}

func TestInstrumentATHRecordsNewHighsSpacedByInterval(t *testing.T) {
	params := yamlParams(t, "instruments: [QQQ]\n")

	store := &fakeStore{created: true, instrumentHigh: map[string]*domain.HighState{"QQQ": instrumentHigh(day(11, 20))}}
	monitor := NewInstrumentATH(store, store)
	assert.Equal(t, "instrument.ath", monitor.Name())

	require.NoError(t, monitor.Run(context.Background(), params))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.InstrumentATH, store.recorded[0].Type)
	assert.Equal(t, "QQQ@2026-11-20", *store.recorded[0].Subject)

	var data domain.InstrumentATHData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, domain.InstrumentATHData{
		Instrument: "QQQ", Date: day(11, 20), Close: "450", PreviousHighDate: day(11, 1), PreviousHigh: "440", IncreasePercent: "2.27",
	}, data)

	for name, tc := range map[string]struct {
		lastDay  int
		recorded int
	}{"14 days after": {6, 0}, "15 days after": {5, 1}} {
		store.recorded, store.latest = nil, eventOn(day(11, tc.lastDay))

		require.NoError(t, monitor.Run(context.Background(), params), name)
		assert.Len(t, store.recorded, tc.recorded, name)
	}
}

func TestInstrumentATHRecordsNothingWithoutANewHigh(t *testing.T) {
	notHigh := instrumentHigh(day(11, 20))
	notHigh.NewHigh = false

	store := &fakeStore{instrumentHigh: map[string]*domain.HighState{"QQQ": notHigh}}

	require.NoError(t, NewInstrumentATH(store, store).Run(context.Background(), yamlParams(t, "instruments: [QQQ, VOO]\n")))
	assert.Empty(t, store.recorded)
}

func TestInstrumentATHSelectionAndSettings(t *testing.T) {
	store := &fakeStore{
		created: true, held: []string{"QQQ"}, instrumentErrs: map[string]error{"NOPE": domain.ErrInstrumentNotFound},
		instrumentHigh: map[string]*domain.HighState{"QQQ": instrumentHigh(day(11, 20)), "VOO": instrumentHigh(day(11, 20))},
	}
	monitor := NewInstrumentATH(store, store)

	require.NoError(t, monitor.Run(context.Background(), yamlParams(t, "instruments: held\ninterval: 1d\n")))
	assert.Len(t, store.recorded, 1, "VOO is not held")

	require.NoError(t, monitor.Run(context.Background(), yamlParams(t, "instruments: [NOPE]\n")))

	for name, content := range map[string]string{
		"no instruments": "interval: 15d\n", "bare interval": "instruments: held\ninterval: 15\n",
		"bad interval": "instruments: held\ninterval: soon\n", "unknown": "instruments: held\nincrement_pct: \"5\"\n",
	} {
		assert.Error(t, monitor.ValidateParams(yamlParams(t, content)), name)
	}

	store.instrumentErrs["QQQ"] = errors.New("boom")
	assert.Error(t, monitor.Run(context.Background(), yamlParams(t, "instruments: [QQQ]\n")))

	store.instrumentErrs, store.recordErr = nil, errors.New("full")
	assert.Error(t, monitor.Run(context.Background(), yamlParams(t, "instruments: [QQQ]\ninterval: 1d\n")))
}

const targetSettings = `
targets:
  - instrument: qqq
    price: "232"
  - instrument: QQQ
    price: "250.50"
`

func TestPriceTargetRecordsTargetsWithinTheRange(t *testing.T) {
	reached := &domain.PriceTarget{Date: day(11, 20), Low: "230", High: "236", Close: "234", Reached: true}
	store := &fakeStore{created: true, priceTargets: map[string]*domain.PriceTarget{
		"QQQ@232":   reached,
		"QQQ@250.5": {Date: day(11, 20), Low: "230", High: "236", Close: "234"},
	}}
	monitor := NewPriceTarget(store, store)

	assert.Equal(t, "price.target", monitor.Name())
	require.NoError(t, monitor.Run(context.Background(), yamlParams(t, targetSettings)))
	assert.Equal(t, []string{"QQQ@232", "QQQ@250.5"}, store.targetQueries, "prices are normalized")

	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.PriceTarget, store.recorded[0].Type)
	assert.Equal(t, "QQQ@232", *store.recorded[0].Subject)

	var data domain.PriceTargetData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, domain.PriceTargetData{Instrument: "QQQ", Price: "232", Date: day(11, 20), Low: "230", High: "236", Close: "234"}, data)
}

func TestPriceTargetSkipsUnpricedAndUnregisteredAndReportsFailures(t *testing.T) {
	store := &fakeStore{created: true, instrumentErrs: map[string]error{"NOPE": domain.ErrInstrumentNotFound, "VOO": errors.New("boom")}}
	monitor := NewPriceTarget(store, store)

	require.NoError(t, monitor.Run(context.Background(), yamlParams(t, "targets:\n  - {instrument: QQQ, price: \"1\"}\n  - {instrument: NOPE, price: \"1\"}\n")))
	assert.Empty(t, store.recorded, "QQQ has no price on the latest date")

	assert.Error(t, monitor.Run(context.Background(), yamlParams(t, "targets:\n  - {instrument: VOO, price: \"1\"}\n")))

	store.priceTargets = map[string]*domain.PriceTarget{"QQQ@1": {Reached: true}}
	store.recordErr = errors.New("full")
	assert.Error(t, monitor.Run(context.Background(), yamlParams(t, "targets:\n  - {instrument: QQQ, price: \"1\"}\n")))
}

func TestPriceTargetValidatesSettings(t *testing.T) {
	monitor := NewPriceTarget(&fakeStore{}, &fakeStore{})

	assert.NoError(t, monitor.ValidateParams(yamlParams(t, targetSettings)))
	assert.NoError(t, monitor.ValidateParams(yamlParams(t, "targets:\n  - {instrument: QQQ, price: \"232\"}\n  - {instrument: VOO, price: \"232\"}\n")))

	for name, content := range map[string]string{
		"missing":       "other: 1\n",
		"empty":         "targets: []\n",
		"zero":          "targets:\n  - {instrument: QQQ, price: \"0\"}\n",
		"negative":      "targets:\n  - {instrument: QQQ, price: \"-3\"}\n",
		"bare number":   "targets:\n  - {instrument: QQQ, price: 232}\n",
		"no price":      "targets:\n  - {instrument: QQQ}\n",
		"no instrument": "targets:\n  - {price: \"232\"}\n",
		"repeated":      "targets:\n  - {instrument: QQQ, price: \"232\"}\n  - {instrument: qqq, price: \"232.0\"}\n",
		"unknown key":   "targets:\n  - {instrument: QQQ, price: \"232\", side: up}\n",
		"unknown":       "targets:\n  - {instrument: QQQ, price: \"232\"}\nwindow: 1d\n",
	} {
		assert.Error(t, monitor.ValidateParams(yamlParams(t, content)), name)
	}

	assert.Error(t, monitor.Run(context.Background(), nil))
}
