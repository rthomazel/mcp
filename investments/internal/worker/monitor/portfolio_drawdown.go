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

const incrementKey = "increment_pct"

var portfolioDrawdownAt = domain.ClockTime{Hour: 7}

// PortfolioDrawdown watches the securities value falling to multiples of an
// increment below its all-time high.
type PortfolioDrawdown struct {
	store  domain.GenericStore
	events domain.EventStore
}

// NewPortfolioDrawdown builds the monitor.
func NewPortfolioDrawdown(store domain.GenericStore, events domain.EventStore) *PortfolioDrawdown {
	return &PortfolioDrawdown{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *PortfolioDrawdown) Name() string {
	return strings.TrimPrefix(domain.Events.PortfolioDrawdown, domain.NotificationPrefix)
}

// At is 07:00 market time.
func (m *PortfolioDrawdown) At() domain.ClockTime { return portfolioDrawdownAt }

// ValidateParams requires a positive increment and rejects any other setting.
func (m *PortfolioDrawdown) ValidateParams(params Params) error {
	if err := params.Unknown(incrementKey); err != nil {
		return err
	}

	_, err := incrementFrom(params)

	return err
}

// Run records one event per level the drawdown is at or below, once for each stay.
func (m *PortfolioDrawdown) Run(ctx context.Context, params Params) error {
	increment, err := incrementFrom(params)
	if err != nil {
		return err
	}

	state, err := m.store.GetDrawdownState(ctx, increment)
	if err != nil {
		return fmt.Errorf("reading drawdown state: %w", err)
	}

	if len(state.Levels) == 0 {
		slog.InfoContext(ctx, "drawdown monitor: no level crossed")

		return nil
	}

	var recorded int

	for _, level := range state.Levels {
		data, err := json.Marshal(domain.PortfolioDrawdownData{
			Level: level.Level, CrossedOn: level.CrossedOn, Date: state.Date, DrawdownPercent: state.DrawdownPercent,
			ValueUSD: state.ValueUSD, HighDate: state.HighDate, HighValueUSD: state.HighValueUSD,
		})
		if err != nil {
			return fmt.Errorf("encoding event data: %w", err)
		}

		subject := drawdownSubject(level.Level, level.CrossedOn)

		created, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.PortfolioDrawdown, Subject: &subject, Data: data})
		if err != nil {
			return fmt.Errorf("recording event: %w", err)
		}

		if created {
			recorded++
		}
	}

	slog.InfoContext(ctx, "drawdown monitor: judged levels", "levels", len(state.Levels), "recorded", recorded)

	return nil
}

func incrementFrom(params Params) (domain.Decimal, error) {
	increment, ok, err := params.Decimal(incrementKey)
	if err != nil {
		return "", err
	}

	if !ok {
		return "", fmt.Errorf("setting %q is required", incrementKey)
	}

	if increment.Sign() <= 0 {
		return "", fmt.Errorf("setting %q must be greater than zero, got %q", incrementKey, increment)
	}

	return increment, nil
}

// drawdownSubject is the level, an at sign and the crossing date.
func drawdownSubject(level domain.Decimal, crossedOn domain.Date) string {
	return string(level) + "@" + crossedOn.Format(time.DateOnly)
}
