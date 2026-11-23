package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

const (
	instrumentsKey            = "instruments"
	instrumentKey             = "instrument"
	stdDevKey                 = "variation_std_dev"
	lookbackKey               = "lookback"
	defaultLookback           = "3mo"
	minVolatilityMoves        = 20
	volatilityDefaultInterval = "1d"
)

// PriceVolatility watches an instrument's price move over a window against its usual moves.
type PriceVolatility struct {
	store  domain.GenericStore
	events domain.EventStore
}

type volatilityEntry struct {
	ticker    string
	threshold domain.Decimal
	direction direction
}

type volatilitySettings struct {
	window, interval, lookback duration.Duration
	entries                    []volatilityEntry
}

// NewPriceVolatility builds the monitor.
func NewPriceVolatility(store domain.GenericStore, events domain.EventStore) *PriceVolatility {
	return &PriceVolatility{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *PriceVolatility) Name() string {
	return strings.TrimPrefix(domain.Events.PriceVolatility, domain.NotificationPrefix)
}

// ValidateParams requires a valid list of instruments and accepts a window, an interval and a look-back.
func (m *PriceVolatility) ValidateParams(params Params) error {
	_, err := volatilitySettingsFrom(params)

	return err
}

// Run records one event for each instrument whose move reaches its threshold in a watched direction.
func (m *PriceVolatility) Run(ctx context.Context, params Params) error {
	settings, err := volatilitySettingsFrom(params)
	if err != nil {
		return err
	}

	var failures []error

	for _, entry := range settings.entries {
		if err := m.judge(ctx, settings, entry); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.ticker, err))
		}
	}

	return errors.Join(failures...)
}

// judge records the event of one instrument. An instrument that cannot be judged is logged and skipped.
func (m *PriceVolatility) judge(ctx context.Context, settings volatilitySettings, entry volatilityEntry) error {
	result, err := m.store.GetPriceVolatility(ctx, entry.ticker, settings.window, settings.lookback, entry.threshold)
	if errors.Is(err, domain.ErrInstrumentNotFound) {
		slog.WarnContext(ctx, "volatility monitor: ticker is not a registered instrument", "ticker", entry.ticker)

		return nil
	}

	if err != nil {
		return fmt.Errorf("reading price volatility: %w", err)
	}

	if result == nil {
		slog.InfoContext(ctx, "volatility monitor: no price to judge", "ticker", entry.ticker)

		return nil
	}

	if result.Moves < minVolatilityMoves || result.VolatilityPercent == nil || result.Variation == nil {
		slog.InfoContext(ctx, "volatility monitor: not enough moves or no volatility to judge a variation",
			"ticker", entry.ticker, "moves", result.Moves)

		return nil
	}

	side := entry.sideReached(result)
	if side == "" {
		return nil
	}

	last, err := lastEventDate(ctx, m.events, domain.Events.PriceVolatility, entry.ticker+"@")
	if err != nil {
		return err
	}

	if withinInterval(last, settings.interval, result.Date) {
		slog.InfoContext(ctx, "volatility monitor: move within the interval", "ticker", entry.ticker)

		return nil
	}

	data, err := json.Marshal(domain.PriceVolatilityData{
		Instrument: entry.ticker, Date: result.Date, Direction: string(side), Window: settings.window.String(),
		Lookback: settings.lookback.String(), StartDate: result.StartDate, MovePercent: result.MovePercent,
		Variation: *result.Variation, Threshold: entry.threshold, VolatilityPercent: *result.VolatilityPercent, Moves: result.Moves,
	})
	if err != nil {
		return fmt.Errorf("encoding event data: %w", err)
	}

	subject := strings.Join([]string{entry.ticker, result.Date.Format(time.DateOnly), string(side)}, "@")

	if _, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.PriceVolatility, Subject: &subject, Data: data}); err != nil {
		return fmt.Errorf("recording event: %w", err)
	}

	return nil
}

// sideReached is the watched side the move reached, empty when none.
func (e volatilityEntry) sideReached(result *domain.PriceVolatility) direction {
	switch {
	case result.AtLeastUp && e.direction.watches(directionUp):
		return directionUp
	case result.AtLeastDown && e.direction.watches(directionDown):
		return directionDown
	default:
		return ""
	}
}

func volatilitySettingsFrom(params Params) (volatilitySettings, error) {
	var settings volatilitySettings

	if err := params.Unknown(windowKey, intervalKey, lookbackKey, instrumentsKey); err != nil {
		return settings, err
	}

	var err error

	if settings.window, err = params.DurationOr(windowKey, movementDefaultSpan); err != nil {
		return settings, err
	}

	if settings.interval, err = params.DurationOr(intervalKey, volatilityDefaultInterval); err != nil {
		return settings, err
	}

	if settings.lookback, err = lookbackFrom(params, settings.window); err != nil {
		return settings, err
	}

	if settings.entries, err = volatilityEntriesFrom(params); err != nil {
		return settings, err
	}

	return settings, nil
}

// lookbackFrom reads the look-back: three months by default, or the window when that is longer.
// A look-back shorter than the window is an error.
func lookbackFrom(params Params, window duration.Duration) (duration.Duration, error) {
	lookback, ok, err := params.Duration(lookbackKey)
	if err != nil {
		return lookback, err
	}

	if ok {
		if lookback.Compare(window) < 0 {
			return lookback, fmt.Errorf("setting %q (%s) may not be shorter than the window (%s)", lookbackKey, lookback, window)
		}

		return lookback, nil
	}

	lookback, err = duration.Parse(lookbackKey, defaultLookback)
	if err != nil {
		return lookback, err
	}

	if window.Compare(lookback) > 0 {
		return window, nil
	}

	return lookback, nil
}

func volatilityEntriesFrom(params Params) ([]volatilityEntry, error) {
	items, ok, err := params.Mappings(instrumentsKey)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("setting %q is required", instrumentsKey)
	}

	var (
		entries []volatilityEntry
		seen    = map[string]bool{}
	)

	for i, item := range items {
		entry, err := volatilityEntryFrom(item)
		if err != nil {
			return nil, fmt.Errorf("instrument %d: %w", i+1, err)
		}

		if seen[entry.ticker] {
			return nil, fmt.Errorf("instrument %d: %s is listed more than once", i+1, entry.ticker)
		}

		seen[entry.ticker] = true
		entries = append(entries, entry)
	}

	return entries, nil
}

func volatilityEntryFrom(item Params) (volatilityEntry, error) {
	var entry volatilityEntry

	if err := item.Unknown(instrumentKey, stdDevKey, directionKey); err != nil {
		return entry, err
	}

	ticker, ok, err := item.Text(instrumentKey)
	if err != nil {
		return entry, err
	}

	if entry.ticker = strings.ToUpper(strings.TrimSpace(ticker)); !ok || entry.ticker == "" {
		return entry, fmt.Errorf("setting %q is required and may not be empty", instrumentKey)
	}

	threshold, ok, err := item.Decimal(stdDevKey)
	if err != nil {
		return entry, err
	}

	if !ok {
		return entry, fmt.Errorf("setting %q is required", stdDevKey)
	}

	if threshold.Sign() <= 0 {
		return entry, fmt.Errorf("setting %q must be greater than zero, got %q", stdDevKey, threshold)
	}

	entry.threshold = threshold
	entry.direction, err = directionFrom(item)

	return entry, err
}
