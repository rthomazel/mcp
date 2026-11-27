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

const (
	targetsKey = "targets"
	priceKey   = "price"
	noteKey    = "note"
)

// PriceTarget watches instrument daily price ranges reaching target prices.
type PriceTarget struct {
	store  domain.MonitorStore
	events domain.EventStore
}

type priceTargetEntry struct {
	ticker string
	price  domain.Decimal // normalized, so the same number is the same subject
	note   string         // why the target exists, sent with the notification
}

// NewPriceTarget builds the monitor.
func NewPriceTarget(store domain.MonitorStore, events domain.EventStore) *PriceTarget {
	return &PriceTarget{store: store, events: events}
}

// Name is the event type the monitor records without its notification prefix.
func (m *PriceTarget) Name() string {
	return strings.TrimPrefix(domain.Events.PriceTarget, domain.NotificationPrefix)
}

// ValidateParams requires a non-empty list of distinct targets and rejects any other setting.
func (m *PriceTarget) ValidateParams(params Params) error {
	_, err := priceTargetsFrom(params)

	return err
}

// Run records one event for each target within the day's range, once for each year.
func (m *PriceTarget) Run(ctx context.Context, params Params) error {
	targets, err := priceTargetsFrom(params)
	if err != nil {
		return err
	}

	var failures []error

	for _, target := range targets {
		if err := m.judge(ctx, target); err != nil {
			failures = append(failures, fmt.Errorf("%s %s: %w", target.ticker, target.price, err))
		}
	}

	return errors.Join(failures...)
}

// judge records the event of one target. A ticker that is not registered is logged and skipped.
func (m *PriceTarget) judge(ctx context.Context, target priceTargetEntry) error {
	result, err := m.store.GetPriceTarget(ctx, target.ticker, target.price)
	if errors.Is(err, domain.ErrInstrumentNotFound) {
		slog.WarnContext(ctx, "price target monitor: ticker is not a registered instrument", "ticker", target.ticker)

		return nil
	}

	if err != nil {
		return fmt.Errorf("reading price range: %w", err)
	}

	if result == nil || !result.Reached {
		return nil
	}

	data, err := json.Marshal(domain.PriceTargetData{
		Instrument: target.ticker, Price: target.price, Date: result.Date, Low: result.Low, High: result.High, Close: result.Close, Note: target.note,
	})
	if err != nil {
		return fmt.Errorf("encoding event data: %w", err)
	}

	subject := target.ticker + "@" + string(target.price)

	if _, err := m.events.Record(ctx, domain.DraftEvent{Type: domain.Events.PriceTarget, Subject: &subject, Data: data}); err != nil {
		return fmt.Errorf("recording event: %w", err)
	}

	return nil
}

func priceTargetsFrom(params Params) ([]priceTargetEntry, error) {
	if err := params.Unknown(targetsKey); err != nil {
		return nil, err
	}

	items, ok, err := params.Mappings(targetsKey)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, fmt.Errorf("setting %q is required", targetsKey)
	}

	var (
		targets []priceTargetEntry
		seen    = map[string]bool{}
	)

	for i, item := range items {
		target, err := priceTargetFrom(item)
		if err != nil {
			return nil, fmt.Errorf("target %d: %w", i+1, err)
		}

		key := target.ticker + "@" + string(target.price)

		if seen[key] {
			return nil, fmt.Errorf("target %d: %s at %s is listed more than once", i+1, target.ticker, target.price)
		}

		seen[key] = true
		targets = append(targets, target)
	}

	return targets, nil
}

func priceTargetFrom(item Params) (priceTargetEntry, error) {
	var target priceTargetEntry

	if err := item.Unknown(instrumentKey, priceKey, noteKey); err != nil {
		return target, err
	}

	ticker, ok, err := item.Text(instrumentKey)
	if err != nil {
		return target, err
	}

	if target.ticker = strings.ToUpper(strings.TrimSpace(ticker)); !ok || target.ticker == "" {
		return target, fmt.Errorf("setting %q is required and may not be empty", instrumentKey)
	}

	price, ok, err := item.Decimal(priceKey)
	if err != nil {
		return target, err
	}

	if !ok {
		return target, fmt.Errorf("setting %q is required", priceKey)
	}

	if price.Sign() <= 0 {
		return target, fmt.Errorf("setting %q must be greater than zero, got %q", priceKey, price)
	}

	target.price = price.Normalized()

	note, _, err := item.Text(noteKey)
	if err != nil {
		return target, err
	}

	target.note = strings.TrimSpace(note)

	return target, nil
}
