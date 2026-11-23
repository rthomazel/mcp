---
id: 2026-11-23-monitor-portfolio-extremes
type: spec
summary: Two monitors on the securities value, drawdown in configured steps below the all-time high, and new all-time highs spaced by a configured number of days.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitors: portfolio drawdown and portfolio high

## Description

Two monitors built on the monitor mechanism in the notifications spec and configured in the
monitors configuration, which watch the extremes of the securities value: how far it has fallen
from its all-time high, and when it reaches a new one. They share one series and one set of
concepts, so one description covers both. They are separate monitors with separate entries
because an operator may want one without the other. This spec replaces the earlier portfolio
drawdown spec, whose behavior is kept unchanged.

## Concepts

**Series.** The securities value over the daily snapshots, the series the value history reports.
Dates without a completed snapshot are absent. The all-time high is its highest value, the
earliest date winning a tie, so a notification and an agent answer never disagree.

**Drawdown.** How far the latest securities value is below the all-time high, as a negative
percent of the high. It is the distance from the high that the value history reports for the
latest date. A new all-time high makes it zero.

**Increment.** The configured step of the drawdown monitor, a positive percent, for example 3.

**Level.** A multiple of the increment, written as the negative percent it stands for: with an
increment of 3 the levels are -3, -6, -9 and so on. The latest value is at a level when its
drawdown is at or below it, so a drawdown of -7.2 is at the -3 and -6 levels.

**Crossing.** The date the value last entered a level: the first date of the current stay at or
below it, which is the day after the most recent date that was shallower than the level. A level
is cleared when the value is shallower than it, and the next date at or below it is a new
crossing.

**New high.** The latest date's value is strictly greater than the value of every earlier date.
The first snapshot has nothing before it and is not a new high.

**Spacing.** The number of days that must have passed since the date of the last announced high
before another is announced.

## Behavior

Both monitors run daily at 7:00 AM Eastern, an hour after the last price refresh, so the latest
snapshot has been settled. They do not depend on the snapshot-incomplete monitor, which runs at
the same time. They judge the latest date of the series.

### Portfolio drawdown

- For every level the latest drawdown is at or below, it records one drawdown event, unless one
  already exists for that level and crossing. The subject of the event is the level and its
  crossing date.
- It records nothing when the drawdown is shallower than the first level, when there are no
  snapshots, or when every level at or below has been announced for its crossing.

Because the crossing is part of the subject, each level is announced once per stay below it,
with no state of its own:

- While the value stays at or below a level, the crossing does not change, so nothing more is
  announced for it, however many days or months that lasts.
- If the value recovers above a level and later falls to it again, that is a new crossing and
  the level is announced again. Levels deeper than the recovered one are cleared by the same
  recovery.
- A new all-time high clears every level.
- If the value falls past several levels in one day, each newly crossed level is announced,
  shallowest first, so a fall from zero to -7 with an increment of 3 records the -3 and the -6
  events.
- Weekends and holidays have no new snapshot, so the same latest date is judged again and
  nothing is recorded. A latest price date without a completed snapshot is not judged: the
  monitor works from the series as stored, and the snapshot-incomplete monitor announces the gap.
- When the monitor is first enabled while the value is already below one or more levels, those
  levels are announced, one message each.
- Changing the increment keeps levels that are still multiples of it as they were, so a change
  from 3 to 2 does not repeat the -6, and announces the new levels the value is at, such as the
  -2 and -4.
- Removing an announced event makes the monitor announce that level again while the value is
  still at it. Fully handled events are deleted after a year, so a stay below a level that lasts
  longer than a year announces it once more.

The message names the level crossed, the current drawdown and its date, the date of the high and
the crossing date, and gives the securities value at the high and the current securities value.
The wording is the renderer's.

### Portfolio high

- When the latest date is a new high and no portfolio high event exists whose date is within the
  spacing days before it, it records one portfolio high event. The subject of the event is the
  date of the high.
- Otherwise it records nothing. A high that the spacing suppresses is not announced later by
  itself; only a date that is a new high when judged is announced.

A date is judged once however often the monitor runs, because the date is the subject. Weekends
and holidays judge the same date again and record nothing new. When the monitor is first enabled
on a date that is a high, that high is announced. Removing the latest event makes the monitor
announce a new high within the spacing again.

The message names the date, the new value, the previous high with its date, how far above it the
new value is in dollars and percent, and the date of the last announced high.

### Not a return

Both measures are changes in value, not investment return: selling or withdrawing money lowers
the securities value, and buying or depositing raises the high, so the figure can fall or hide a
decline, or reach a high, without a market move. Both messages say so. A measure that excludes
money moved in and out is a separate, later feature.

### Configuration

The monitors are named portfolio.drawdown and portfolio.high and record
notification.portfolio.drawdown and notification.portfolio.high, routed by the notifications
configuration like any other event.

```yaml
monitors:
  portfolio.drawdown:
    increment_pct: "3"           # required, a percent greater than zero, written as text
  portfolio.high:
    min_days_between: 15         # optional, 15 by default
```

- A missing or non-positive increment makes the file invalid.
- `min_days_between` is a whole number of days, zero or more, which may be written as a plain
  number. Zero announces every new high.
- Any other setting is invalid, whether or not the entry is disabled.
- With no entry a monitor does not run.

## Non-goals

- A drawdown that excludes deposits, withdrawals, purchases and sales.
- A window other than the all-time high, such as the drawdown within the last year.
- Announcing recoveries.
- Several increments, or levels that are not evenly spaced.
- A buffer before a level is considered cleared; a value moving across a level repeatedly
  announces it each time it re-enters.
- Account equity, which includes cash.

## Decisions

- Once per level and stay, decided by the crossing date in the subject. The outbox's seven day
  limit only bounds delivery attempts and does not make an announcement forgotten.
- No state beyond the outbox: the crossing and the last high are derived from the stored series
  and events, so the monitors follow observed state, as the first monitor does.
- One event per newly crossed level, so every level has its own subject and a later easing never
  announces an older level out of order.
- The series and the all-time high are the value history's.
- Value drawdown first, with the flow-adjusted measure left for a separate feature.
- Spacing is measured from the date of the last announced high and not from when its event was
  created, so a late run does not shift the window. The default of fifteen days is the
  operator's choice, since an uptrend can set new highs on most days.
- The messages carry the amounts as well as the percentages, so a percentage can be read against
  them; it is the operator's own portfolio channel.

## Acceptance criteria

- At 7:00 AM Eastern, with an increment of 3 and a drawdown of -7.2, one drawdown event is
  recorded for the -3 level and one for the -6 level, each once.
- Running it again, on later days, while the value stays at or below those levels records
  nothing.
- After the value rises above -3 and falls to -3 again, the -3 level is announced again with a
  new crossing; the -6 level is announced only if the value reaches it again.
- A new all-time high clears every drawdown level; the next fall announces the levels again.
- With a drawdown shallower than the increment, with no snapshots, or with no entry in the
  monitors configuration, nothing is recorded.
- A missing or non-positive increment makes the monitors file invalid.
- The drawdown the monitor reads equals the distance from the all-time high that the value
  history reports for the latest date.
- With the default spacing, a new high on a date records one high event; a new high on each of
  the next fourteen days records nothing; a new high fifteen days or more after the announced
  one records an event.
- A value equal to the previous high, or below it, records no high event; the first snapshot
  alone records none; running again for the same date records none.
- With a spacing of zero, every date that is a new high records an event.
- A negative or non-numeric spacing makes the monitors file invalid.
- The messages name the dates and amounts described above and say the figure is a change in
  value and not a return.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns them on and sets their settings.
- Related: 2026-11-15-value-history, whose series and all-time high they share.
- Related: 2026-11-21-monitor-snapshot-incomplete, which runs at the same time.
- Related: 2026-11-23-monitor-instrument-extremes, the same monitors for instrument prices.
