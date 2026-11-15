---
id: 2026-11-15-value-history
type: spec
summary: Portfolio value history query with granularity, all-time high, drawdown and change over a range.
author: Thom
created: 2026-11-15
agents: claude
---

# Value history

## Intent

Answer what the securities value was, whether it fell from its previous peak and where the peak was.
The all-time high scope is every stored snapshot since the earliest one.

## Behavior

A value history query returns the securities value over a date range at daily, weekly or monthly
granularity. Each point is the value on the last snapshot date within its interval, so weekends and
holidays never appear. Dates without a completed snapshot are absent, never interpolated.
When a cash snapshot exists for the point, the point also carries cash and account equity beside it.

Summary measures cover the requested range, except the all-time high:

- latest value and date;
- all-time high and its date, over every stored snapshot since the earliest one, whatever the range;
  the earliest date wins a tie;
- dollar and percentage drawdown of the latest value from the all-time high;
- change over the range, dollar and percent, from the first point to the last.

The same measures are also reported on account equity, over the dates that have a cash snapshot, together with the net contributions in the range, deposits minus withdrawals, so a change in equity can be read against money added or removed.

All measures are changes in value, not investment return: a deposit
or a new purchase raises value without being a gain, and results say so.

## Acceptance criteria

- Value history returns one point per interval at the last snapshot date, skips non-snapshot dates, and the all-time high and drawdown cover every stored snapshot regardless of the range.
- When cash snapshots exist, points carry cash and account equity, and equity measures and net contributions are reported; see the cash feature.


## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
