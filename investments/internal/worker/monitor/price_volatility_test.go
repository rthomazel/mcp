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

func newVolatility(store *fakeStore) *PriceVolatility { return NewPriceVolatility(store, store) }

const volatilitySettingsYAML = `
instruments:
  - instrument: qqq
    variation_std_dev: "2"
    direction: up
  - instrument: GLD
    variation_std_dev: "3"
`

func decimalPointer(text string) *domain.Decimal {
	value := domain.Decimal(text)

	return &value
}

func volatilityResult(up, down bool) *domain.PriceVolatility {
	return &domain.PriceVolatility{
		Date: day(11, 20), StartDate: day(11, 19), MovePercent: "2.4", Moves: 60,
		VolatilityPercent: decimalPointer("1"), Variation: decimalPointer("2.4"), AtLeastUp: up, AtLeastDown: down,
	}
}

func TestPriceVolatilityIsNamedAfterItsEvent(t *testing.T) {
	assert.Equal(t, "price.volatility", newVolatility(&fakeStore{}).Name())
}

func TestPriceVolatilityRecordsTheReachedSide(t *testing.T) {
	store := &fakeStore{created: true, volatility: map[string]*domain.PriceVolatility{"QQQ": volatilityResult(true, false)}}

	require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, volatilitySettingsYAML)))

	assert.Equal(t, []string{"QQQ", "GLD"}, store.volQueries)
	assert.Equal(t, 1, store.window.Days)
	assert.Equal(t, 3, store.lookback.Months, "the default look-back")
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.PriceVolatility, store.recorded[0].Type)
	assert.Equal(t, "QQQ@2026-11-20@up", *store.recorded[0].Subject)

	var data domain.PriceVolatilityData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, domain.PriceVolatilityData{
		Instrument: "QQQ", Date: day(11, 20), Direction: "up", Window: "1d", Lookback: "3mo", StartDate: day(11, 19),
		MovePercent: "2.4", Variation: "2.4", Threshold: "2", VolatilityPercent: "1", Moves: 60,
	}, data)
}

func TestPriceVolatilityWatchesTheConfiguredDirection(t *testing.T) {
	for name, tc := range map[string]struct {
		up, down bool
		subject  string
	}{
		"up, reached up":   {true, false, "QQQ@2026-11-20@up"},
		"up, reached down": {false, true, ""},
		"neither":          {false, false, ""},
	} {
		store := &fakeStore{created: true, volatility: map[string]*domain.PriceVolatility{"QQQ": volatilityResult(tc.up, tc.down)}}

		require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, volatilitySettingsYAML)), name)

		if tc.subject == "" {
			assert.Empty(t, store.recorded, name)

			continue
		}

		require.Len(t, store.recorded, 1, name)
		assert.Equal(t, tc.subject, *store.recorded[0].Subject, name)
	}

	store := &fakeStore{created: true, volatility: map[string]*domain.PriceVolatility{"GLD": volatilityResult(false, true)}}

	require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, volatilitySettingsYAML)))
	require.Len(t, store.recorded, 1, "both directions by default")
	assert.Equal(t, "GLD@2026-11-20@down", *store.recorded[0].Subject)
}

func TestPriceVolatilitySkipsWhatCannotBeJudged(t *testing.T) {
	short := volatilityResult(true, false)
	short.Moves = 19

	flat := volatilityResult(true, false)
	flat.VolatilityPercent, flat.Variation = nil, nil

	for name, result := range map[string]*domain.PriceVolatility{"no price": nil, "short history": short, "flat prices": flat} {
		store := &fakeStore{created: true, volatility: map[string]*domain.PriceVolatility{"QQQ": result}}

		require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, volatilitySettingsYAML)), name)
		assert.Empty(t, store.recorded, name)
	}
}

func TestPriceVolatilityIntervalSpacesAnnouncementsOfAnInstrument(t *testing.T) {
	for name, tc := range map[string]struct {
		interval string
		lastDay  int
		recorded int
	}{
		"default, previous day": {"", 19, 1},
		"default, same date":    {"", 20, 0},
		"5d, four days after":   {"5d", 16, 0},
		"5d, five days after":   {"5d", 15, 1},
	} {
		content := volatilitySettingsYAML
		if tc.interval != "" {
			content = "interval: " + tc.interval + volatilitySettingsYAML
		}

		store := &fakeStore{
			created: true, latest: eventOn(day(11, tc.lastDay)),
			volatility: map[string]*domain.PriceVolatility{"QQQ": volatilityResult(true, false)},
		}

		require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, content)), name)
		assert.Len(t, store.recorded, tc.recorded, name)
	}
}

func TestPriceVolatilityUnregisteredTickerDoesNotStopTheOthers(t *testing.T) {
	store := &fakeStore{
		created:    true,
		volatility: map[string]*domain.PriceVolatility{"GLD": volatilityResult(true, false)},
		volErrs:    map[string]error{"QQQ": domain.ErrInstrumentNotFound},
	}

	require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, volatilitySettingsYAML)))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, "GLD@2026-11-20@up", *store.recorded[0].Subject)
}

func TestPriceVolatilityOtherErrorsAreReturnedAfterTheOthersRun(t *testing.T) {
	store := &fakeStore{
		created:    true,
		volatility: map[string]*domain.PriceVolatility{"GLD": volatilityResult(true, false)},
		volErrs:    map[string]error{"QQQ": errors.New("ambiguous")},
	}

	err := newVolatility(store).Run(context.Background(), yamlParams(t, volatilitySettingsYAML))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "QQQ")
	assert.Len(t, store.recorded, 1)
}

func TestPriceVolatilityLookbackDefaults(t *testing.T) {
	for name, tc := range map[string]struct {
		settings string
		lookback string
	}{
		"default":                  {"", "3mo"},
		"longer window":            {"window: 6mo\n", "6mo"},
		"shorter window":           {"window: 5d\n", "3mo"},
		"explicit":                 {"lookback: 1y\n", "1y"},
		"explicit equal to window": {"window: 2w\nlookback: 14d\n", "14d"},
	} {
		store := &fakeStore{}

		require.NoError(t, newVolatility(store).Run(context.Background(), yamlParams(t, tc.settings+volatilitySettingsYAML)), name)
		assert.Equal(t, tc.lookback, store.lookback.String(), name)
	}
}

func TestPriceVolatilityValidatesItsSettings(t *testing.T) {
	monitor := newVolatility(&fakeStore{})

	assert.NoError(t, monitor.ValidateParams(yamlParams(t, volatilitySettingsYAML)))
	assert.NoError(t, monitor.ValidateParams(yamlParams(t, "window: 5d\ninterval: 1w\nlookback: 6mo\n"+volatilitySettingsYAML)))

	for name, content := range map[string]string{
		"missing instruments": "window: 1d",
		"empty instruments":   "instruments: []",
		"unknown setting":     "other: 1\n" + volatilitySettingsYAML,
		"unknown key":         "instruments:\n  - {instrument: QQQ, variation_std_dev: \"2\", size: 1}",
		"no instrument":       "instruments:\n  - {variation_std_dev: \"2\"}",
		"no threshold":        "instruments:\n  - {instrument: QQQ}",
		"zero threshold":      "instruments:\n  - {instrument: QQQ, variation_std_dev: \"0\"}",
		"negative threshold":  "instruments:\n  - {instrument: QQQ, variation_std_dev: \"-2\"}",
		"unquoted threshold":  "instruments:\n  - {instrument: QQQ, variation_std_dev: 2}",
		"unknown direction":   "instruments:\n  - {instrument: QQQ, variation_std_dev: \"2\", direction: sideways}",
		"repeated instrument": "instruments:\n  - {instrument: QQQ, variation_std_dev: \"2\"}\n  - {instrument: qqq, variation_std_dev: \"3\"}",
		"bare window":         "window: 5\n" + volatilitySettingsYAML,
		"zero interval":       "interval: 0d\n" + volatilitySettingsYAML,
		"lookback shorter":    "window: 1mo\nlookback: 2w\n" + volatilitySettingsYAML,
		"bare lookback":       "lookback: 90\n" + volatilitySettingsYAML,
	} {
		assert.Error(t, monitor.ValidateParams(yamlParams(t, content)), name)
	}
}
