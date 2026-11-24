package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// InstrumentDrawdown watches instrument adjusted closes falling to multiples of an increment below their all-time highs.
type InstrumentDrawdown struct {
	store  domain.MonitorStore
	events domain.EventStore
}

// NewInstrumentDrawdown builds the monitor.
func NewInstrumentDrawdown(store domain.MonitorStore, events domain.EventStore) *InstrumentDrawdown {
	return &InstrumentDrawdown{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *InstrumentDrawdown) Name() string {
	return strings.TrimPrefix(domain.Events.InstrumentDrawdown, domain.NotificationPrefix)
}

// ValidateParams requires a positive increment and the instruments, and rejects any other setting.
func (m *InstrumentDrawdown) ValidateParams(params Params) error {
	_, _, err := instrumentDrawdownSettings(params)

	return err
}

// Run records, for each selected instrument, one event per level its drawdown is at or below, once for each stay.
func (m *InstrumentDrawdown) Run(ctx context.Context, params Params) error {
	increment, selection, err := instrumentDrawdownSettings(params)
	if err != nil {
		return err
	}

	tickers, err := tickersOf(ctx, m.store, selection)
	if err != nil {
		return err
	}

	var failures []error

	for _, ticker := range tickers {
		if err := m.judge(ctx, ticker, increment); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", ticker, err))
		}
	}

	return errors.Join(failures...)
}

// judge records the levels of one instrument. An instrument that is not registered is logged and skipped.
func (m *InstrumentDrawdown) judge(ctx context.Context, ticker string, increment domain.Decimal) error {
	state, err := m.store.GetInstrumentDrawdownState(ctx, ticker, increment)
	if errors.Is(err, domain.ErrInstrumentNotFound) {
		slog.WarnContext(ctx, "instrument drawdown monitor: ticker is not a registered instrument", "ticker", ticker)

		return nil
	}

	if err != nil {
		return fmt.Errorf("reading drawdown state: %w", err)
	}

	if len(state.Levels) == 0 {
		slog.InfoContext(ctx, "instrument drawdown monitor: no level crossed", "ticker", ticker)

		return nil
	}

	for _, level := range state.Levels {
		data, err := json.Marshal(domain.InstrumentDrawdownData{
			Instrument: ticker, Level: level.Level, CrossedOn: level.CrossedOn, Date: state.Date,
			DrawdownPercent: state.DrawdownPercent, Close: state.ValueUSD, HighDate: state.HighDate, HighClose: state.HighValueUSD,
		})
		if err != nil {
			return fmt.Errorf("encoding event data: %w", err)
		}

		subject := ticker + "@" + drawdownSubject(level.Level, level.CrossedOn)

		if _, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.InstrumentDrawdown, Subject: &subject, Data: data}); err != nil {
			return fmt.Errorf("recording event: %w", err)
		}
	}

	return nil
}

func instrumentDrawdownSettings(params Params) (domain.Decimal, Selection, error) {
	if err := params.Unknown(incrementKey, instrumentsKey); err != nil {
		return "", Selection{}, err
	}

	increment, err := incrementFrom(params)
	if err != nil {
		return "", Selection{}, err
	}

	selection, err := selectionFrom(params)

	return increment, selection, err
}
