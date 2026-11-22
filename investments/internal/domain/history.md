# Types

## HistoryFilter

1. From *Date, inclusive, nil means the earliest stored snapshot
2. To *Date, inclusive, nil means the latest stored snapshot
3. WindowDays int, the number of days one point covers, zero means one

Selects the range and the spacing of the returned points. Windows are counted from the resolved range start, so the last may be shorter. The summary measures never depend on the window.

## ValuePoint

1. Date Date, the last snapshot date within the window and the range
2. SecuritiesValueUSD Decimal, the sum of the date's snapshot market values, zero when the date has only a cash snapshot
3. CashUSD *Decimal, nil without a cash snapshot for the date
4. AccountEquityUSD *Decimal, securities value plus cash, nil without a cash snapshot for the date

## Extreme

1. Date Date
2. ValueUSD Decimal

A value and the date it occurred.

## Drawdown

1. PeakDate Date
2. PeakValueUSD Decimal
3. TroughDate Date
4. TroughValueUSD Decimal
5. DrawdownUSD Decimal, trough minus peak, zero or negative
6. DrawdownPercent Decimal, as a percent of the peak, zero or negative
7. Recovered bool, true when a later date in the range has a value at or above the peak

A range that only rises has a zero drawdown, with the trough on the peak date and Recovered true.

## SeriesSummary

1. Latest Extreme, the last value in the range
2. AllTimeHigh Extreme, the highest value from the earliest stored snapshot through the range end, the earliest date winning a tie
3. FromHighUSD Decimal, latest minus the all-time high, zero or negative
4. FromHighPercent Decimal, as a percent of the all-time high, zero or negative
5. Baseline Extreme, the last value on or before the range start, or the first value in the range when none is that early
6. ChangeUSD Decimal, latest minus baseline
7. ChangePercent *Decimal, as a percent of the baseline, nil when the baseline is zero
8. RangeHigh Extreme
9. RangeLow Extreme
10. MaxDrawdown Drawdown, the largest peak-to-trough fall within the range

Measures over one series, securities value or account equity. A change in value, not an investment return: a deposit or a purchase raises securities value without being a gain.

## DrawdownLevel

1. Level Decimal, a negative percent that is a multiple of the increment, for example "-6"
2. CrossedOn Date, the first date of the current stay at or below the level

The day after the most recent date whose drawdown was shallower than the level. A drawdown of zero is shallower than every level, so a crossing always exists.

## DrawdownState

1. Date Date, the latest date of the series
2. ValueUSD Decimal, the securities value on Date
3. DrawdownPercent Decimal, the latest value as a percent of the all-time high minus one hundred, zero or negative
4. HighDate Date, the all-time high through Date, the earliest date winning a tie
5. HighValueUSD Decimal, the securities value on HighDate
6. Levels []DrawdownLevel, every level the drawdown is at or below, shallowest first, capped by the store

The securities value drawdown over the same daily series and the same all-time high as SeriesSummary, so FromHighPercent of the latest date and DrawdownPercent agree. No levels means the drawdown is shallower than the first.

## ValueHistory

1. From Date, the resolved range start
2. To Date, the resolved range end
3. WindowDays int
4. Points []ValuePoint, ordered by date ascending
5. Securities *SeriesSummary, nil when the range has no points
6. Equity *SeriesSummary, over dates with a cash snapshot, nil when the range has none
7. NetContributionsUSD *Decimal, deposits minus withdrawals dated after the equity baseline through the latest equity date, the opening balance excluded; nil exactly when Equity is nil

# Functions

## HistoryFilter.Normalize() (HistoryFilter, error)

1. if the window is zero, set it to one.
2. Return the filter.

#### Errors

- **1.** if the window is negative, return ErrInvalidInput.
- **1.1.** if From is after To, return ErrInvalidInput.
