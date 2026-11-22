---
id: 2026-11-22-monitor-portfolio-drawdown
type: spec
summary: Monitor that notifies the operator once each time the securities value falls to a new multiple of a configured percentage below its all-time high, and again after it recovers and falls back.
author: Thom
created: 2026-11-22
agents: claude
---

# Monitor: portfolio drawdown

## Description

The first monitor with settings, built on the monitor mechanism in the notifications spec
and turned on and configured in the monitors configuration. It tells the operator how far
the securities value has fallen from its all-time high, in steps of a configured size,
without repeating itself while the value stays down.

## Concepts

**Drawdown.** How far the latest securities value is below the all-time high, as a
negative percent of the high. It is the same figure the value history reports as the
distance from the high, measured over the same daily series, so what the agent says and
what the notification says agree. A new all-time high makes it zero.

**Increment.** The configured step, a positive percent, for example 3.

**Level.** A multiple of the increment, written as the negative percent it stands for:
with an increment of 3 the levels are -3, -6, -9 and so on. The latest value is at a level
when its drawdown is at or below it, so a drawdown of -7.2 is at the -3 and -6 levels.

**Crossing.** The date the value last entered a level: the first date of the current stay at
or below it, which is the day after the most recent date that was shallower than the level.
A level is cleared when the value is shallower than it, and the next date at or below it is
a new crossing.

## Behavior

The monitor runs daily at 7:05 AM Eastern, after the snapshot-incomplete monitor, so the
latest snapshot has been settled. It reads the securities value series and judges its
latest date.

- For every level the latest drawdown is at or below, it records one drawdown event, unless
  one already exists for that level and crossing. The subject of the event is the level and
  its crossing date.
- It records nothing when the drawdown is shallower than the first level, when there are
  no snapshots, or when every level at or below has been announced for its crossing.

Because the crossing is part of the subject, the monitor announces each level once per
stay below it, with no state of its own:

- While the value stays at or below a level, the crossing does not change, so nothing more is
  announced for it, however many days or months that lasts.
- If the value recovers above a level and later falls to it again, that is a new crossing
  and the level is announced again. Levels deeper than the recovered one are cleared by
  the same recovery.
- A new all-time high clears every level.
- If the value falls past several levels in one day, each newly crossed level is announced,
  shallowest first, so a fall from zero to -7 with an increment of 3 records the -3 and
  the -6 events.
- Weekends and holidays have no new snapshot, so the same latest date is judged again and
  nothing is recorded. A latest price date without a completed snapshot is not judged: the
  monitor works from the series as stored, and the snapshot-incomplete monitor announces
  the gap.
- When the monitor is first enabled while the value is already below one or more levels,
  those levels are announced, one message each.
- Changing the increment keeps levels that are still multiples of it as they were, so a
  change from 3 to 2 does not repeat the -6, and announces the new levels the value is
  at, such as the -2 and -4.
- Removing an announced event makes the monitor announce that level again while the value
  is still at it. Fully handled events are deleted after a year, so a stay below a level
  that lasts longer than a year announces it once more.

The measure is a change in value, not investment return: selling or withdrawing money lowers
the securities value and buying raises the high, so the figure can fall or hide a decline
without a market move. The message says so. A measure that excludes money moved in and out
is a separate, later feature.

The message names the level crossed, the current drawdown and its date, the date of the high
and the crossing date, with percentages and dates only and no balances. The wording is the
renderer's.

### Configuration

The monitor is named portfolio.drawdown in the monitors configuration and records
notification.portfolio.drawdown, routed by the notifications configuration like any other
event. It has one setting:

```yaml
monitors:
  portfolio.drawdown:
    increment_pct: "3"   # required, a percent greater than zero, written as text
```

A missing or non-positive increment, or any other setting, makes the file invalid. With no
entry the monitor does not run.

## Non-goals

- A drawdown that excludes deposits, withdrawals, purchases and sales.
- A window other than the all-time high, such as the drawdown within the last year.
- Announcing recoveries or new highs.
- Several increments, or levels that are not evenly spaced.
- A buffer before a level is considered cleared; a value moving across a level repeatedly
  announces it each time it re-enters.
- Account equity, which includes cash.

## Decisions

- Once per level and stay, decided by the crossing date in the subject. The outbox's seven day
  limit only bounds delivery attempts and does not make an announcement forgotten.
- No state beyond the outbox: the crossing is derived from the stored series, so the monitor
  follows observed state, as the first monitor does.
- One event per newly crossed level, so every level has its own subject and a later easing
  never announces an older level out of order.
- The series and the all-time high are the value history's, so a drawdown notification and
  an agent answer never disagree.
- Value drawdown first, with the flow-adjusted measure left for a separate feature.
- Percentages and dates only in the message.

## Acceptance criteria

- At 7:05 AM Eastern, with an increment of 3 and a drawdown of -7.2, one event is recorded
  for the -3 level and one for the -6 level, each once.
- Running it again, on later days, while the value stays at or below those levels records
  nothing.
- After the value rises above -3 and falls to -3 again, the -3 level is announced again with
  a new crossing; the -6 level is announced only if the value reaches it again.
- A new all-time high clears every level; the next fall announces the levels again.
- With a drawdown shallower than the increment, with no snapshots, or with no entry in the
  monitors configuration, nothing is recorded.
- A missing or non-positive increment makes the monitors file invalid.
- The drawdown the monitor reads equals the distance from the all-time high that the value
  history reports for the latest date.
- The message names the level, the drawdown and the dates and contains no balances.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its increment.
- Related: 2026-11-15-value-history, whose series and all-time high it shares.
- Related: 2026-11-21-monitor-snapshot-incomplete, which runs before it.
