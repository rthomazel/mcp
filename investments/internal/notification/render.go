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

// AllocationDriftRenderer announces a group outside its allocation band.
type AllocationDriftRenderer struct{}

// PriceVolatilityRenderer announces an unusual price move.
type PriceVolatilityRenderer struct{}

// PriceTargetRenderer announces a price target reached.
type PriceTargetRenderer struct{}

// InstrumentDrawdownRenderer announces a level of an instrument's drawdown crossed.
type InstrumentDrawdownRenderer struct{}

// InstrumentATHRenderer announces an instrument's new all-time high.
type InstrumentATHRenderer struct{}

// NotificationFailedRenderer announces an event whose delivery was given up on.
type NotificationFailedRenderer struct{}

// NewRenderers returns the renderer of each notifying event type by format.
func NewRenderers(updates domain.UpdateStore) map[string]map[string]Renderer {
	return map[string]map[string]Renderer{
		domain.Events.SnapshotIncomplete: {defaultFormat: &SnapshotIncompleteRenderer{updates: updates}},
		domain.Events.PortfolioDrawdown:  {defaultFormat: &PortfolioDrawdownRenderer{}},
		domain.Events.PortfolioATH:       {defaultFormat: &PortfolioATHRenderer{}},
		domain.Events.PortfolioMovement:  {defaultFormat: &PortfolioMovementRenderer{}},
		domain.Events.AllocationDrift:    {defaultFormat: &AllocationDriftRenderer{}},
		domain.Events.PriceVolatility:    {defaultFormat: &PriceVolatilityRenderer{}},
		domain.Events.PriceTarget:        {defaultFormat: &PriceTargetRenderer{}},
		domain.Events.InstrumentDrawdown: {defaultFormat: &InstrumentDrawdownRenderer{}},
		domain.Events.InstrumentATH:      {defaultFormat: &InstrumentATHRenderer{}},
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

	amounts, err := wholeDollars(data.ValueUSD, data.HighValueUSD)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Portfolio is down %s%% from its high, crossing the %s%% level on %s.\n"+
		"Securities value on %s: $%s. High on %s: $%s.\n"+
		"This value change is affected by sales and withdrawals.",
		strings.TrimPrefix(drawdown, "-"), data.Level, data.CrossedOn.Format(time.DateOnly),
		data.Date.Format(time.DateOnly), amounts[0], data.HighDate.Format(time.DateOnly), amounts[1]), nil
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

	amounts, err := dollars(data.ValueUSD, data.IncreaseUSD, data.PreviousHighValueUSD)
	if err != nil {
		return "", err
	}

	last := "none"
	if data.LastAnnouncedDate != nil {
		last = data.LastAnnouncedDate.Format(time.DateOnly)
	}

	return fmt.Sprintf("Portfolio reached a new all-time high on %s: $%s.\n"+
		"That is $%s (%s%%) above the previous high of $%s on %s. Last announced high: %s.\n"+
		"This value change is affected by sales and withdrawals.",
		data.Date.Format(time.DateOnly), amounts[0], amounts[1], increase,
		amounts[2], data.PreviousHighDate.Format(time.DateOnly), last), nil
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

	market, err := signedDollars(data.MarketUSD)
	if err != nil {
		return "", err
	}

	trades, err := signedDollars(data.TradesUSD)
	if err != nil {
		return "", err
	}

	values, err := dollars(data.StartValueUSD, data.EndValueUSD)
	if err != nil {
		return "", err
	}

	verb := "rose"
	if data.Direction == "down" {
		verb = "fell"
	}

	text := fmt.Sprintf("Market prices moved the portfolio %s: it %s %s%% (%s) over %s, from %s to %s.\n"+
		"Securities value went from $%s to $%s: market %s, trades %s.",
		data.Direction, verb, strings.TrimPrefix(percent, "-"), market, data.Window,
		data.StartDate.Format(time.DateOnly), data.Date.Format(time.DateOnly), values[0], values[1], market, trades)

	for _, line := range []struct {
		label        string
		contributors []domain.Contributor
	}{{"Largest gains", data.Gainers}, {"Largest losses", data.Losers}} {
		if len(line.contributors) == 0 {
			continue
		}

		list, err := contributorList(line.contributors)
		if err != nil {
			return "", err
		}

		text += "\n" + line.label + ": " + list + "."
	}

	return text + "\nThis value change is affected by sales and withdrawals.", nil
}

// Render names the group, the side and the bound crossed, the allocation, the start of the stay and the members.
func (r *AllocationDriftRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.AllocationDriftData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding allocation drift data: %w", err)
	}

	bound, name := data.MaxPercent, "maximum"
	if data.Side == domain.SideBelow {
		bound, name = data.MinPercent, "minimum"
	}

	if bound == nil {
		return "", fmt.Errorf("allocation drift event has no %s to compare with", name)
	}

	allocation, limit, err := percentsAgainst(data.AllocationPercent, *bound)
	if err != nil {
		return "", fmt.Errorf("rounding allocation: %w", err)
	}

	members := make([]string, len(data.Members))

	for i, member := range data.Members {
		share, err := roundedPercent(member.AllocationPercent)
		if err != nil {
			return "", fmt.Errorf("rounding allocation of %s: %w", member.Symbol, err)
		}

		members[i] = member.Symbol + " " + share + "%"
	}

	return fmt.Sprintf("Allocation of %s is %s its %s: %s%% of the securities value against %s%%, since %s (as of %s).\n"+
		"Instruments: %s.\n"+
		"This allocation changes with trades as well as with prices.",
		data.Group, data.Side, name, allocation, limit, data.StayStart.Format(time.DateOnly), data.Date.Format(time.DateOnly),
		strings.Join(members, ", ")), nil
}

// Render names the instrument, the move over the window, the variation against the threshold and the baseline.
func (r *PriceVolatilityRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.PriceVolatilityData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding price volatility data: %w", err)
	}

	move, err := roundedPercent(data.MovePercent)
	if err != nil {
		return "", fmt.Errorf("rounding move percent: %w", err)
	}

	variation, err := roundedPercent(data.Variation)
	if err != nil {
		return "", fmt.Errorf("rounding variation: %w", err)
	}

	threshold, err := roundedPercent(data.Threshold)
	if err != nil {
		return "", fmt.Errorf("rounding threshold: %w", err)
	}

	volatility, err := roundedPercent(data.VolatilityPercent)
	if err != nil {
		return "", fmt.Errorf("rounding volatility: %w", err)
	}

	verb := "rose"
	if data.Direction == "down" {
		verb = "fell"
	}

	return fmt.Sprintf("%s %s %s%% over %s, from %s to %s: %s standard deviations against a threshold of %s.\n"+
		"Its usual move over %s is %s%%, the standard deviation of %d moves in the last %s.\n"+
		"Moves use adjusted closes.",
		data.Instrument, verb, strings.TrimPrefix(move, "-"), data.Window, data.StartDate.Format(time.DateOnly),
		data.Date.Format(time.DateOnly), strings.TrimPrefix(variation, "-"), threshold,
		data.Window, volatility, data.Moves, data.Lookback), nil
}

// Render names the instrument, the target, the date and the day's low, high and close.
func (r *PriceTargetRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.PriceTargetData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding price target data: %w", err)
	}

	amounts, err := dollars(data.Price, data.Low, data.High, data.Close)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s reached its target price of $%s on %s.\n"+
		"Low $%s, high $%s, close $%s. Prices are not adjusted.",
		data.Instrument, amounts[0], data.Date.Format(time.DateOnly), amounts[1], amounts[2], amounts[3]), nil
}

// Render names the instrument, the level, the drawdown, the high, the current close and the crossing date.
func (r *InstrumentDrawdownRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.InstrumentDrawdownData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding instrument drawdown data: %w", err)
	}

	drawdown, err := roundedPercent(data.DrawdownPercent)
	if err != nil {
		return "", fmt.Errorf("rounding drawdown percent: %w", err)
	}

	amounts, err := dollars(data.Close, data.HighClose)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s is down %s%% from its high, crossing the %s%% level on %s.\n"+
		"Adjusted close on %s: $%s. High on %s: $%s.",
		data.Instrument, strings.TrimPrefix(drawdown, "-"), data.Level, data.CrossedOn.Format(time.DateOnly),
		data.Date.Format(time.DateOnly), amounts[0], data.HighDate.Format(time.DateOnly), amounts[1]), nil
}

// Render names the instrument, the date, the new adjusted close, the previous high and the last announced high.
func (r *InstrumentATHRenderer) Render(_ context.Context, event domain.Event) (string, error) {
	var data domain.InstrumentATHData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return "", fmt.Errorf("decoding instrument ATH data: %w", err)
	}

	increase, err := roundedPercent(data.IncreasePercent)
	if err != nil {
		return "", fmt.Errorf("rounding increase percent: %w", err)
	}

	amounts, err := dollars(data.Close, data.PreviousHigh)
	if err != nil {
		return "", err
	}

	last := "none"
	if data.LastAnnouncedDate != nil {
		last = data.LastAnnouncedDate.Format(time.DateOnly)
	}

	return fmt.Sprintf("%s reached a new all-time high on %s: $%s adjusted close.\n"+
		"That is %s%% above the previous high of $%s on %s. Last announced high: %s.",
		data.Instrument, data.Date.Format(time.DateOnly), amounts[0], increase,
		amounts[1], data.PreviousHighDate.Format(time.DateOnly), last), nil
}

// percentsAgainst rounds an allocation and the bound it is compared with to one decimal, and to two when
// that would show them equal, so a group outside its band never reads as exactly at the bound.
func percentsAgainst(allocation, bound domain.Decimal) (allocationText, boundText string, err error) {
	allocationText, err = roundedPercent(allocation)
	if err != nil {
		return "", "", err
	}

	boundText, err = roundedPercent(bound)
	if err != nil {
		return "", "", err
	}

	if allocationText != boundText {
		return allocationText, boundText, nil
	}

	for _, value := range []struct {
		percent domain.Decimal
		text    *string
	}{{allocation, &allocationText}, {bound, &boundText}} {
		rat, _ := new(big.Rat).SetString(string(value.percent))
		*value.text = rat.FloatString(2)
	}

	return allocationText, boundText, nil
}

// roundedPercent rounds a percent to one decimal place, halves away from zero.
func roundedPercent(percent domain.Decimal) (string, error) {
	rat, ok := new(big.Rat).SetString(string(percent))
	if !ok {
		return "", fmt.Errorf("%q is not a decimal number", percent)
	}

	return rat.FloatString(1), nil
}

// roundedDollars rounds an amount to cents, halves away from zero, never negative zero.
func roundedDollars(amount domain.Decimal) (string, error) {
	return roundedTo(amount, 2)
}

// roundedWholeDollars rounds an amount to the dollar, halves away from zero, never negative zero.
func roundedWholeDollars(amount domain.Decimal) (string, error) {
	return roundedTo(amount, 0)
}

func roundedTo(amount domain.Decimal, places int) (string, error) {
	rat, ok := new(big.Rat).SetString(string(amount))
	if !ok {
		return "", fmt.Errorf("%q is not a decimal number", amount)
	}

	text := rat.FloatString(places)
	if strings.Trim(text, "-0.") == "" {
		return rat.SetInt64(0).FloatString(places), nil
	}

	return text, nil
}

// signedDollars writes an amount rounded to cents with its sign before the dollar sign.
func signedDollars(amount domain.Decimal) (string, error) {
	text, err := roundedDollars(amount)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(text, "-") {
		return "-$" + strings.TrimPrefix(text, "-"), nil
	}

	return "+$" + text, nil
}

// dollars rounds each amount to cents, in order.
func dollars(amounts ...domain.Decimal) ([]string, error) {
	rounded := make([]string, len(amounts))

	for i, amount := range amounts {
		text, err := roundedDollars(amount)
		if err != nil {
			return nil, fmt.Errorf("rounding amount: %w", err)
		}

		rounded[i] = text
	}

	return rounded, nil
}

// wholeDollars rounds each amount to the dollar, in order.
func wholeDollars(amounts ...domain.Decimal) ([]string, error) {
	rounded := make([]string, len(amounts))

	for i, amount := range amounts {
		text, err := roundedWholeDollars(amount)
		if err != nil {
			return nil, fmt.Errorf("rounding amount: %w", err)
		}

		rounded[i] = text
	}

	return rounded, nil
}

func contributorList(contributors []domain.Contributor) (string, error) {
	parts := make([]string, len(contributors))

	for i, contributor := range contributors {
		amount, err := signedDollars(contributor.MovementUSD)
		if err != nil {
			return "", fmt.Errorf("rounding movement of %s: %w", contributor.Symbol, err)
		}

		parts[i] = contributor.Symbol + " " + amount
	}

	return strings.Join(parts, ", "), nil
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
