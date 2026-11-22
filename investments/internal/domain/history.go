package domain

import "fmt"

// HistoryFilter selects the range and the spacing of the returned points.
// Windows are counted from the resolved range start, so the last may be shorter.
type HistoryFilter struct {
	From       *Date // inclusive, nil means the earliest stored snapshot
	To         *Date // inclusive, nil means the latest stored snapshot
	WindowDays int   // days one point covers, zero means one
}

// ValuePoint is one point of the series: the last snapshot date in its window.
type ValuePoint struct {
	Date               Date
	SecuritiesValueUSD Decimal  // zero when the date has only a cash snapshot
	CashUSD            *Decimal // nil without a cash snapshot
	AccountEquityUSD   *Decimal // nil without a cash snapshot
}

// Extreme is a value and the date it occurred.
type Extreme struct {
	Date     Date
	ValueUSD Decimal
}

// Drawdown is a peak-to-trough fall.
type Drawdown struct {
	PeakDate        Date
	PeakValueUSD    Decimal
	TroughDate      Date
	TroughValueUSD  Decimal
	DrawdownUSD     Decimal // trough minus peak, zero or negative
	DrawdownPercent Decimal // percent of the peak, zero or negative
	Recovered       bool    // a later date in the range is at or above the peak
}

// SeriesSummary holds the measures over one series. A change in value, not a return.
type SeriesSummary struct {
	Latest          Extreme
	AllTimeHigh     Extreme // earliest snapshot through the range end, earliest date wins a tie
	FromHighUSD     Decimal
	FromHighPercent Decimal
	Baseline        Extreme // last value on or before the range start, else the first in range
	ChangeUSD       Decimal
	ChangePercent   *Decimal // nil when the baseline is zero
	RangeHigh       Extreme
	RangeLow        Extreme
	MaxDrawdown     Drawdown
}

// DrawdownLevel is a level the drawdown is at or below.
type DrawdownLevel struct {
	Level     Decimal // negative percent, a multiple of the increment
	CrossedOn Date    // first date of the current stay at or below the level
}

// DrawdownState is the drawdown of the latest securities value from the
// all-time high, over the series SeriesSummary reads.
type DrawdownState struct {
	Date            Date
	ValueUSD        Decimal
	DrawdownPercent Decimal // zero or negative
	HighDate        Date
	HighValueUSD    Decimal
	Levels          []DrawdownLevel // shallowest first, capped by the store
}

// ValueHistory is the series and its summaries.
type ValueHistory struct {
	From                Date
	To                  Date
	WindowDays          int
	Points              []ValuePoint
	Securities          *SeriesSummary // nil when the range has no points
	Equity              *SeriesSummary // nil when no date in the range has a cash snapshot
	NetContributionsUSD *Decimal       // nil exactly when Equity is nil
}

// Normalize defaults the window to one day and checks the range.
func (f HistoryFilter) Normalize() (HistoryFilter, error) {
	if f.WindowDays < 0 {
		return f, fmt.Errorf("%w: window_days must not be negative", ErrInvalidInput)
	}

	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return f, fmt.Errorf("%w: from is after to", ErrInvalidInput)
	}

	if f.WindowDays == 0 {
		f.WindowDays = 1
	}

	return f, nil
}
