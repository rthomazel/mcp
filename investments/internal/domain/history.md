# Constants

maxHistoryPoints = 1000, the most points one query returns. A longer series is rejected with a hint to use a larger window or a narrower range, never silently truncated.

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
