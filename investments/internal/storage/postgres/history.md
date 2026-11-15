# Constants

historyBoundsSQL = select the earliest and latest dates over snapshots and cash_snapshots together, null when both are empty.
dailySeriesSQL = the daily series as one relation: every distinct date in snapshots or cash_snapshots, with securities value as the sum of snapshots market values for the date (zero when it has none), the cash balance from cash_snapshots (null when absent) and account equity as their sum when cash exists. Shared by every history query, so the point set and the summaries cannot disagree.
historyPointsSQL = from dailySeriesSQL within the range, group dates into windows of the filter's day count, the window number being the whole days from the range start divided by it, and keep the last date of each window, ordered by date ascending.
seriesSummarySQL = over one value column of dailySeriesSQL, null values excluded: the latest row in the range; the maximum over all rows on or before the range end with the earliest date winning a tie; latest minus that maximum, absolute and as a percent; the baseline, the last row on or before the range start, else the first row in the range; change from the baseline, absolute and as a percent; the highest and lowest rows in the range; and the largest peak-to-trough fall within the range from a running maximum, with its peak and trough dates and whether a later row reaches the peak again. All arithmetic in SQL, always over daily rows whatever the window.
netContributionsSQL = sum the amounts of DEPOSIT and WITHDRAWAL cash_transactions whose effective date is after the equity baseline date through the latest equity date, zero when there are none.

# Functions

## Store.GetValueHistory(ctx Context, filter domain.HistoryFilter) (domain.ValueHistory, error)

1. Normalize filter with HistoryFilter.Normalize.
2. Read the earliest and latest dates via historyBoundsSQL.
3. if there are none, return an empty history with the window and no summaries.
4. Resolve From to the earliest date and To to the latest date when absent.
5. Select the points via historyPointsSQL.
6. if there are more than maxHistoryPoints, reject.
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
