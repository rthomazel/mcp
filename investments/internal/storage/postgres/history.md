# Constants

historyBoundsSQL = select the earliest and latest dates over snapshots and cash_snapshots together, null when both are empty.
dailySeriesSQL = the daily series as one relation: every distinct date in snapshots or cash_snapshots, with securities value as the sum of snapshots market values for the date (zero when it has none), the cash balance from cash_snapshots (null when absent) and account equity as their sum when cash exists. Shared by every history query, so the point set and the summaries cannot disagree.
historyPointsSQL = from dailySeriesSQL within the range, group dates into windows of the filter's day count, the window number being the whole days from the range start divided by it, and keep the last date of each window, ordered by date ascending.
seriesSummarySQL = over one value column of dailySeriesSQL, null values excluded: the latest row in the range; the maximum over all rows on or before the range end with the earliest date winning a tie; latest minus that maximum, absolute and as a percent; the baseline, the last row on or before the range start, else the first row in the range; change from the baseline, absolute and as a percent; the highest and lowest rows in the range; and the largest peak-to-trough fall within the range, the deepest percent below a running maximum with the earliest trough winning a tie, with its peak and trough dates and whether a later row reaches the peak again. All arithmetic in SQL, always over daily rows whatever the window.
netContributionsSQL = sum the amounts of DEPOSIT and WITHDRAWAL cash_transactions whose effective date is after the equity baseline date through the latest equity date, zero when there are none.
highStateSQL = over dailySeriesSQL securities value, take the latest row and the highest value on an earlier date, the earliest date winning a tie; report the latest, the previous high, whether the latest is strictly greater, and the difference in dollars and as a percent of the previous high, null without a previous row.
marketMovesCTE = the movement of each instrument held on two consecutive snapshot dates, the earlier date's quantity times the change in close price, for the dates after $1, the window start. Consecutive means consecutive among the dates that have snapshot rows; instruments missing on either date contribute nothing.
marketMovementSQL = from marketMovesCTE, take the base, the securities value on the last snapshot date on or before $1, and the latest, the value on the newest snapshot date; return their dates and values, the market total as the sum of movements, trades as the latest minus the base minus the market total, the market total as a percent of the base, and whether it is at or above $2 or at or below minus $2, by cross multiplication and false when the base is zero. No row when there is no base date.
marketContributorsSQL = from marketMovesCTE, sum the movements by instrument symbol, largest first, symbol breaking ties.
drawdownStateBody = the part of drawdownLevelsSQL after its series, over a relation daily(date, securities), so another series can reuse it; drawdownLevelsSQL is dailySeriesSQL followed by it.
highStateBody = the part of highStateSQL after its series, over the same relation; highStateSQL is dailySeriesSQL followed by it.
maxDrawdownLevels = 100, the most levels one state reports.
drawdownLevelsSQL = over dailySeriesSQL securities value, take each row's percent below the running maximum of the values up to and including its date, zero when the maximum is zero; take the latest row and the all-time high, its date, the earliest date reaching the maximum, and its value; generate the levels, minus the increment times one up to the deepest multiple the latest drawdown reaches, at most maxDrawdownLevels; and for each level the crossing, the date of the first row after the most recent row whose percent is above the level. Percent values are in percent units, computed by Postgres with trim_scale, as in seriesSummarySQL.

# Functions

## Store.GetValueHistory(ctx Context, filter domain.HistoryFilter) (domain.ValueHistory, error)

1. Normalize filter with HistoryFilter.Normalize.
2. Read the earliest and latest dates via historyBoundsSQL.
3. if there are none, return an empty history with the window and no summaries.
4. Resolve From to the earliest date and To to the latest date when absent.
5. Select the points via historyPointsSQL.
6. if there are more than the configured maximum number of points, a Store setting,, reject.
7. if there are no points, return the empty history for the resolved range.
8. Compute the securities summary via seriesSummarySQL over securities value.
9. if any point in the range has a cash snapshot, compute the equity summary via seriesSummarySQL over account equity, and the net contributions via netContributionsSQL.
10. Return the history.

#### Errors

- **1.** if normalizing fails, return the domain.ErrInvalidInput error.

---

- **2.** if the query fails, return the wrapped error.

---

- **5.** if the query fails, return the wrapped error.

---

- **6.** return domain.ErrInvalidInput saying the series has too many points and to use a larger window or a narrower range.

---

- **8.** if the query fails, return the wrapped error.

---

- **9.** if a query fails, return the wrapped error.

The date set is every date with a securities or cash snapshot, so an all-cash date counts and has securities value zero. Dates without either are absent and never interpolated. A date after everything was sold and with no cash snapshot is therefore absent rather than zero.
All-time high is bounded by the range end so that a past range is not measured against a later peak; it is not bounded by the range start.
Percent values are in percent units, such as -12.5, computed by Postgres. A zero baseline gives no change percent rather than an error.
Reads are independent selects on one handle, no transaction, as in GetStatus.
Decimals are rendered with trim_scale, so a division does not leave trailing zeros. The summary SQL is one template over the value column, securities or equity, so both series use the same definitions.

## Monitors.GetHighState(ctx Context) (*domain.HighState, error)

1. Read the state with Store.highState over highStateSQL.
2. if there is no row, return nil.
3. Return the state, with Previous and the increases only when there is a previous row.

#### Errors

- **1.** if the query fails, return the wrapped error.

## Monitors.GetMarketMovement(ctx Context, window duration.Duration, threshold domain.Decimal) (*domain.MarketMovement, error)

1. if threshold is not positive, return domain.ErrInvalidInput.
2. Read the latest snapshot date.
3. if there is none, return nil.
4. Compute the window start with window.Before(latest), so the calendar arithmetic is the duration package's alone.
5. Select the totals via marketMovementSQL.
6. if there is no row, the window starts before the first snapshot, return nil.
7. List the instrument movements via marketContributorsSQL, then keep as Gainers the first MaxContributors with a positive movement and as Losers the last MaxContributors with a negative one, largest loss first.
8. Return the movement.

#### Errors

- **1.** return domain.ErrInvalidInput naming the threshold.

---

- **2.** if the query fails, return the wrapped error.

---

- **5.** if the query fails, return the wrapped error.

---

- **7.** if the query fails, return the wrapped error.

Dates are snapshot dates, so a weekend has none and a window starting on one uses the Friday before it as its base. Instruments are matched by id, so a position that is sold and bought back contributes on the days it was held on both dates. Reads are independent selects on one handle, no transaction.

## Monitors.GetDrawdownState(ctx Context, increment domain.Decimal) (domain.DrawdownState, error)

1. if increment is not positive, return domain.ErrInvalidInput.
2. Read the state with Store.drawdownState over drawdownLevelsSQL.
3. if the series is empty, return an empty state.
4. Return the state, its levels ordered shallowest first.

#### Errors

- **1.** return domain.ErrInvalidInput naming the increment.

---

- **2.** if the query fails, return the wrapped error.

It reads the same dailySeriesSQL as the value history, so an all-cash date counts with securities value zero and the figure agrees with GetValueHistory. The percent of a row is judged against the running maximum, so a new all-time high is zero and clears every level. At most maxDrawdownLevels levels are returned, so a tiny increment cannot produce an unbounded result; the deeper levels are not reported. Reads are independent selects on one handle, no transaction.

## Store.drawdownState(ctx Context, query string, args ...any) (domain.DrawdownState, error)

1. Run the query, a drawdown query over any series, with the arguments.
2. Collect its rows into a state: the first row's date, value, percent and high, and a level for each row that has one, in order.
3. Return the state, with no levels and a zero date when there are no rows.

#### Errors

- **1.** if the query fails, return the wrapped error.

---

- **2.** if scanning fails, return the wrapped error.

Shared by the portfolio's and the instruments' drawdown reads.

## Store.highState(ctx Context, query string, args ...any) (*domain.HighState, error)

1. Run the query, a high state query over any series, with the arguments.
2. if there is no row, return nil.
3. Return the state, with Previous and the increases only when there is a previous row.

#### Errors

- **1.** if the query fails, return the wrapped error.

Shared by the portfolio's and the instruments' high reads.
