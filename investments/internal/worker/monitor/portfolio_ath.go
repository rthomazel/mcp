package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	intervalKey         = "interval"
	athDefaultInterval  = "15d"
	movementDefaultSpan = "1d"
)

// PortfolioATH watches the securities value reaching a new all-time high.
type PortfolioATH struct {
	store  domain.GenericStore
	events domain.EventStore
}

// NewPortfolioATH builds the monitor.
func NewPortfolioATH(store domain.GenericStore, events domain.EventStore) *PortfolioATH {
	return &PortfolioATH{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *PortfolioATH) Name() string {
	return strings.TrimPrefix(domain.Events.PortfolioATH, domain.NotificationPrefix)
}

// ValidateParams accepts only a valid interval.
func (m *PortfolioATH) ValidateParams(params Params) error {
	if err := params.Unknown(intervalKey); err != nil {
		return err
	}

	_, err := params.DurationOr(intervalKey, athDefaultInterval)

	return err
}

// Run records one event when the latest date is a new high and the interval
// has passed since the last announced one.
func (m *PortfolioATH) Run(ctx context.Context, params Params) error {
	interval, err := params.DurationOr(intervalKey, athDefaultInterval)
	if err != nil {
		return err
	}

	state, err := m.store.GetHighState(ctx)
	if err != nil {
		return fmt.Errorf("reading high state: %w", err)
	}

	if state == nil || !state.NewHigh {
		slog.InfoContext(ctx, "all-time high monitor: no new high")

		return nil
	}

	last, err := lastEventDate(ctx, m.events, domain.Events.PortfolioATH)
	if err != nil {
		return err
	}

	if withinInterval(last, interval, state.Date) {
		slog.InfoContext(ctx, "all-time high monitor: new high within the interval", "date", state.Date.Format(time.DateOnly))

		return nil
	}

	data, err := json.Marshal(domain.PortfolioATHData{
		Date: state.Date, ValueUSD: state.ValueUSD,
		PreviousHighDate: state.Previous.Date, PreviousHighValueUSD: state.Previous.ValueUSD,
		IncreaseUSD: state.IncreaseUSD, IncreasePercent: state.IncreasePercent, LastAnnouncedDate: last,
	})
	if err != nil {
		return fmt.Errorf("encoding event data: %w", err)
	}

	subject := state.Date.Format(time.DateOnly)

	if _, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.PortfolioATH, Subject: &subject, Data: data}); err != nil {
		return fmt.Errorf("recording event: %w", err)
	}

	return nil
}
