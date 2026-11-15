---
id: 2026-11-15-value-history
type: spec
summary: Portfolio value history query with a day window, all-time high, drawdown and change over a range.
author: Thom
created: 2026-11-15
agents: claude
---

# Value history

## Intent

Answer what the securities value was, whether it fell from its previous peak and where the peak was.
The all-time high scope is every stored snapshot from the earliest one through the end of the range.

## Behavior

A value history query returns the securities value over a date range, one point per window of a
given number of days, one day by default; 7 or 30 give weekly or monthly spacing. Windows are counted from the
range start, so the last may be shorter. Each point is the value on the last snapshot date within its window,
so weekends and holidays never appear. Dates without a completed snapshot are absent, never interpolated.
When a cash snapshot exists for the point, the point also carries cash and account equity beside it.

Summary measures cover the requested range, except the all-time high:

- latest value and date;
- all-time high and its date, over every stored snapshot from the earliest one through the end of the range,
  whatever the range start; the earliest date wins a tie;
- dollar and percentage drawdown of the latest value from the all-time high;
- change over the range, dollar and percent, from the baseline to the latest point. The baseline is the
  last snapshot on or before the range start, so a range starting on a weekend or holiday keeps the first
  day's move; when no snapshot exists that early, it is the first point in the range;
- the high and the low within the range, each with its date;
- the maximum drawdown within the range, peak to trough, in dollars and percent, with the peak and trough
  dates, and whether the value has since recovered to the peak.

A longer window such as the past six months since the all-time high takes two queries: the first reports the
all-time high date, the second starts from it. Results always name the baseline date used.

The same measures are also reported on account equity, over the dates that have a cash snapshot, together with the net contributions in the range, deposits minus withdrawals, so a change in equity can be read against money added or removed.

All measures are changes in value, not investment return: a deposit
or a new purchase raises value without being a gain, and results say so.

## Acceptance criteria

- Value history returns one point per interval at the last snapshot date, skips non-snapshot dates, and the all-time high and drawdown cover every stored snapshot through the range end regardless of the range start.
- Change over the range uses the last snapshot on or before the start as baseline and names its date.
- The range high, low and maximum drawdown, with their dates and a recovered flag, match hand-calculated values; a range that only rises has zero maximum drawdown.
- When cash snapshots exist, points carry cash and account equity, and equity measures and net contributions are reported; see the cash feature.


## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
