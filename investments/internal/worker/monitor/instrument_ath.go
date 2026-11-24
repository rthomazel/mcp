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

// InstrumentATH watches instrument adjusted closes reaching new all-time highs.
type InstrumentATH struct {
	store  domain.MonitorStore
	events domain.EventStore
}

// NewInstrumentATH builds the monitor.
func NewInstrumentATH(store domain.MonitorStore, events domain.EventStore) *InstrumentATH {
	return &InstrumentATH{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *InstrumentATH) Name() string {
	return strings.TrimPrefix(domain.Events.InstrumentATH, domain.NotificationPrefix)
}

// ValidateParams requires the instruments, accepts an interval and rejects any other setting.
func (m *InstrumentATH) ValidateParams(params Params) error {
	_, _, err := instrumentATHSettings(params)

	return err
}

// Run records, for each selected instrument at a new high, one event unless one was recorded within the interval.
func (m *InstrumentATH) Run(ctx context.Context, params Params) error {
	interval, selection, err := instrumentATHSettings(params)
	if err != nil {
		return err
	}

	tickers, err := tickersOf(ctx, m.store, selection)
	if err != nil {
		return err
	}

	var failures []error

	for _, ticker := range tickers {
		if err := m.judge(ctx, ticker, interval); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", ticker, err))
		}
	}

	return errors.Join(failures...)
}

// judge records the event of one instrument. An instrument that is not registered is logged and skipped.
func (m *InstrumentATH) judge(ctx context.Context, ticker string, interval duration.Duration) error {
	state, err := m.store.GetInstrumentHighState(ctx, ticker)
	if errors.Is(err, domain.ErrInstrumentNotFound) {
		slog.WarnContext(ctx, "instrument all-time high monitor: ticker is not a registered instrument", "ticker", ticker)

		return nil
	}

	if err != nil {
		return fmt.Errorf("reading high state: %w", err)
	}

	if state == nil || !state.NewHigh {
		return nil
	}

	last, err := lastEventDate(ctx, m.events, domain.Events.InstrumentATH, ticker+"@")
	if err != nil {
		return err
	}

	if withinInterval(last, interval, state.Date) {
		slog.InfoContext(ctx, "instrument all-time high monitor: new high within the interval", "ticker", ticker)

		return nil
	}

	data, err := json.Marshal(domain.InstrumentATHData{
		Instrument: ticker, Date: state.Date, Close: state.ValueUSD,
		PreviousHighDate: state.Previous.Date, PreviousHigh: state.Previous.ValueUSD,
		IncreasePercent: state.IncreasePercent, LastAnnouncedDate: last,
	})
	if err != nil {
		return fmt.Errorf("encoding event data: %w", err)
	}

	subject := ticker + "@" + state.Date.Format(time.DateOnly)

	if _, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.InstrumentATH, Subject: &subject, Data: data}); err != nil {
		return fmt.Errorf("recording event: %w", err)
	}

	return nil
}

func instrumentATHSettings(params Params) (duration.Duration, Selection, error) {
	if err := params.Unknown(intervalKey, instrumentsKey); err != nil {
		return duration.Duration{}, Selection{}, err
	}

	interval, err := params.DurationOr(intervalKey, athDefaultInterval)
	if err != nil {
		return interval, Selection{}, err
	}

	selection, err := selectionFrom(params)

	return interval, selection, err
}
