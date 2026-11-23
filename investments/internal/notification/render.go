package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// Renderer turns an event into plain, channel-neutral text.
type Renderer interface {
	Render(ctx context.Context, event domain.Event) (string, error)
}

// SnapshotIncompleteRenderer describes the incomplete snapshot run for the event's date.
type SnapshotIncompleteRenderer struct {
	updates domain.UpdateStore
}

// PortfolioDrawdownRenderer announces a level of drawdown crossed.
type PortfolioDrawdownRenderer struct{}

// PortfolioATHRenderer announces a new all-time high.
type PortfolioATHRenderer struct{}

// PortfolioMovementRenderer announces a market movement over a window.
type PortfolioMovementRenderer struct{}

// NotificationFailedRenderer announces an event whose delivery was given up on.
type NotificationFailedRenderer struct{}

// NewRenderers returns the renderer of each notifying event type by format.
func NewRenderers(updates domain.UpdateStore) map[string]map[string]Renderer {
	return map[string]map[string]Renderer{
		domain.Events.SnapshotIncomplete: {defaultFormat: &SnapshotIncompleteRenderer{updates: updates}},
		domain.Events.PortfolioDrawdown:  {defaultFormat: &PortfolioDrawdownRenderer{}},
		domain.Events.PortfolioATH:       {defaultFormat: &PortfolioATHRenderer{}},
		domain.Events.PortfolioMovement:  {defaultFormat: &PortfolioMovementRenderer{}},
		domain.Events.NotificationFailed: {defaultFormat: &NotificationFailedRenderer{}},
	}
}

// Render names the date and the run's error message, or that no run was recorded.
func (r *SnapshotIncompleteRenderer) Render(ctx context.Context, event domain.Event) (string, error) {
	var data domain.SnapshotIncompleteData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding snapshot incomplete data: %w", err)
	}

	updates, err := r.updates.List(ctx, domain.UpdateFilter{
		DataType: domain.UpdateDataSnapshots, Status: domain.UpdateStatusIncomplete,
	})
	if err != nil {
		return "", fmt.Errorf("listing incomplete snapshot runs: %w", err)
	}

	date := data.Date.Format(time.DateOnly)

	for _, update := range updates {
		if update.DateFrom != nil && update.DateTo != nil && update.DateFrom.Equal(data.Date) && update.DateTo.Equal(data.Date) {
			return fmt.Sprintf("Snapshot for %s is incomplete: %s", date, update.ErrorMessage), nil
		}
	}

	return fmt.Sprintf("Snapshot for %s is incomplete: no run was recorded for it.", date), nil
}

// Render names the level, the drawdown, the high, the current value and the crossing date.
func (r *PortfolioDrawdownRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.PortfolioDrawdownData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding portfolio drawdown data: %w", err)
	}

	drawdown, err := roundedPercent(data.DrawdownPercent)
	if err != nil {
		return "", fmt.Errorf("rounding drawdown percent: %w", err)
	}

	return fmt.Sprintf("Portfolio is down %s%% from its high, crossing the %s%% level on %s.\n"+
		"Securities value on %s: $%s. High on %s: $%s.\n"+
		"This value change is affected by sales and withdrawals.",
		strings.TrimPrefix(drawdown, "-"), data.Level, data.CrossedOn.Format(time.DateOnly),
		data.Date.Format(time.DateOnly), data.ValueUSD, data.HighDate.Format(time.DateOnly), data.HighValueUSD), nil
}

// Render names the date, the new value, the previous high and the last announced high.
func (r *PortfolioATHRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.PortfolioATHData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding portfolio ATH data: %w", err)
	}

	increase, err := roundedPercent(data.IncreasePercent)
	if err != nil {
		return "", fmt.Errorf("rounding increase percent: %w", err)
	}

	last := "none"
	if data.LastAnnouncedDate != nil {
		last = data.LastAnnouncedDate.Format(time.DateOnly)
	}

	return fmt.Sprintf("Portfolio reached a new all-time high on %s: $%s.\n"+
		"That is $%s (%s%%) above the previous high of $%s on %s. Last announced high: %s.\n"+
		"This value change is affected by sales and withdrawals.",
		data.Date.Format(time.DateOnly), data.ValueUSD, data.IncreaseUSD, increase,
		data.PreviousHighValueUSD, data.PreviousHighDate.Format(time.DateOnly), last), nil
}

// Render names the window, the market movement, the values, the split between
// market and trades and the largest contributors.
func (r *PortfolioMovementRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.PortfolioMovementData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding portfolio movement data: %w", err)
	}

	percent, err := roundedPercent(data.MarketPercent)
	if err != nil {
		return "", fmt.Errorf("rounding market percent: %w", err)
	}

	verb := "rose"
	if data.Direction == "down" {
		verb = "fell"
	}

	text := fmt.Sprintf("Market prices moved the portfolio %s: it %s %s%% (%s) over %s, from %s to %s.\n"+
		"Securities value went from $%s to $%s: market %s, trades %s.",
		data.Direction, verb, strings.TrimPrefix(percent, "-"), signedDollars(data.MarketUSD), data.Window,
		data.StartDate.Format(time.DateOnly), data.Date.Format(time.DateOnly),
		data.StartValueUSD, data.EndValueUSD, signedDollars(data.MarketUSD), signedDollars(data.TradesUSD))

	if len(data.Gainers) > 0 {
		text += "\nLargest gains: " + contributorList(data.Gainers) + "."
	}

	if len(data.Losers) > 0 {
		text += "\nLargest losses: " + contributorList(data.Losers) + "."
	}

	return text + "\nThis value change is affected by sales and withdrawals.", nil
}

// roundedPercent rounds a percent to one decimal place, halves away from zero.
func roundedPercent(percent domain.Decimal) (string, error) {
	rat, ok := new(big.Rat).SetString(string(percent))
	if !ok {
		return "", fmt.Errorf("%q is not a decimal number", percent)
	}

	return rat.FloatString(1), nil
}

// signedDollars writes an amount with its sign before the dollar sign.
func signedDollars(amount domain.Decimal) string {
	if strings.HasPrefix(string(amount), "-") {
		return "-$" + strings.TrimPrefix(string(amount), "-")
	}

	return "+$" + string(amount)
}

func contributorList(contributors []domain.Contributor) string {
	parts := make([]string, len(contributors))
	for i, contributor := range contributors {
		parts[i] = contributor.Symbol + " " + signedDollars(contributor.MovementUSD)
	}

	return strings.Join(parts, ", ")
}

// Render names the failed event's type, recorded time and attempts, never its payload.
func (r *NotificationFailedRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.NotificationFailedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding notification failed data: %w", err)
	}

	return fmt.Sprintf("Delivery of a %s notification recorded at %s was given up on after %d failed attempts.",
		data.EventType, data.RecordedAt.Format(time.RFC3339), data.Failures), nil
}
