package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

const (
	variationKey = "variation_pct"
	windowKey    = "window"
)

// PortfolioMovement watches the securities value moving over a window because
// of prices alone.
type PortfolioMovement struct {
	store  domain.GenericStore
	events domain.EventStore
}

type movementSettings struct {
	variation domain.Decimal
	direction direction
	window    duration.Duration
	interval  duration.Duration
}

// NewPortfolioMovement builds the monitor.
func NewPortfolioMovement(store domain.GenericStore, events domain.EventStore) *PortfolioMovement {
	return &PortfolioMovement{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *PortfolioMovement) Name() string {
	return strings.TrimPrefix(domain.Events.PortfolioMovement, domain.NotificationPrefix)
}

// ValidateParams requires a positive variation and accepts a direction, a
// window and an interval.
func (m *PortfolioMovement) ValidateParams(params Params) error {
	_, err := movementSettingsFrom(params)

	return err
}

// Run records one event when the market movement over the window reaches the
// threshold in a watched direction and the interval has passed.
func (m *PortfolioMovement) Run(ctx context.Context, params Params) error {
	settings, err := movementSettingsFrom(params)
	if err != nil {
		return err
	}

	movement, err := m.store.GetMarketMovement(ctx, settings.window, settings.variation)
	if err != nil {
		return fmt.Errorf("reading market movement: %w", err)
	}

	if movement == nil {
		slog.InfoContext(ctx, "movement monitor: not enough snapshots for the window")

		return nil
	}

	side := settings.sideReached(movement)
	if side == "" {
		slog.InfoContext(ctx, "movement monitor: no movement at the threshold", "date", movement.EndDate.Format(time.DateOnly))

		return nil
	}

	last, err := lastEventDate(ctx, m.events, domain.Events.PortfolioMovement)
	if err != nil {
		return err
	}

	if withinInterval(last, settings.interval, movement.EndDate) {
		slog.InfoContext(ctx, "movement monitor: movement within the interval", "date", movement.EndDate.Format(time.DateOnly))

		return nil
	}

	return m.record(ctx, settings, movement, side)
}

func (m *PortfolioMovement) record(ctx context.Context, settings movementSettings, movement *domain.MarketMovement, side direction) error {
	data, err := json.Marshal(domain.PortfolioMovementData{
		Date: movement.EndDate, Direction: string(side), Window: settings.window.String(), StartDate: movement.StartDate,
		StartValueUSD: movement.StartValueUSD, EndValueUSD: movement.EndValueUSD, MarketUSD: movement.MarketUSD,
		MarketPercent: *movement.MarketPercent, TradesUSD: movement.TradesUSD,
		Gainers: movement.Gainers, Losers: movement.Losers,
	})
	if err != nil {
		return fmt.Errorf("encoding event data: %w", err)
	}

	subject := movement.EndDate.Format(time.DateOnly) + "@" + string(side)

	if _, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.PortfolioMovement, Subject: &subject, Data: data}); err != nil {
		return fmt.Errorf("recording event: %w", err)
	}

	return nil
}

// sideReached is the watched side the movement reached, empty when none.
func (s movementSettings) sideReached(movement *domain.MarketMovement) direction {
	switch {
	case movement.AtLeastUp && s.direction.watches(directionUp):
		return directionUp
	case movement.AtLeastDown && s.direction.watches(directionDown):
		return directionDown
	default:
		return ""
	}
}

func movementSettingsFrom(params Params) (movementSettings, error) {
	var settings movementSettings

	if err := params.Unknown(variationKey, directionKey, windowKey, intervalKey); err != nil {
		return settings, err
	}

	variation, ok, err := params.Decimal(variationKey)
	if err != nil {
		return settings, err
	}

	if !ok {
		return settings, fmt.Errorf("setting %q is required", variationKey)
	}

	if variation.Sign() <= 0 {
		return settings, fmt.Errorf("setting %q must be greater than zero, got %q", variationKey, variation)
	}

	settings.variation = variation

	if settings.direction, err = directionFrom(params); err != nil {
		return settings, err
	}

	if settings.window, err = params.DurationOr(windowKey, movementDefaultSpan); err != nil {
		return settings, err
	}

	settings.interval, err = params.DurationOr(intervalKey, movementDefaultSpan)

	return settings, err
}
