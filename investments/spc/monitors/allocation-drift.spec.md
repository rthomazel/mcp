---
id: 2026-11-23-monitor-allocation-drift
type: spec
summary: Monitor that notifies the operator once each time a configured group of instruments leaves its allocation band, and again after it returns inside the band and leaves it.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitor: allocation drift

## Description

Built on the monitor mechanism in the notifications spec and configured in the monitors
configuration. It tells the operator when the share of the securities value held in a
group of instruments is above its maximum or below its minimum, without repeating itself
while the group stays outside the band.

## Concepts

**Allocation.** An instrument's value divided by the securities value, as the base spec
defines it: cash is excluded and the allocations of all instruments sum to one hundred
percent.

**Group.** A named set of instruments whose allocations are added together and judged as one,
so a band can cover several instruments at once. A group of one instrument is the ordinary
single-instrument case. Instruments are named by ticker, and an instrument belongs to one
group at most. Instruments in no group are not watched.

**Band.** A minimum, a maximum or both, in percent of the securities value. A group is above
the band when its allocation is greater than the maximum and below it when it is less than
the minimum; being exactly at a bound is inside the band.

**Stay.** The run of consecutive snapshot dates, ending at the latest, on which the group has
been outside the band on the same side. Its start is the day after the most recent date the
group was inside the band, or the first snapshot date when it never was. A date without a
completed snapshot is absent from the series and does not break a stay, as in the value
history. A date on which the securities value is zero has no allocation and is inside the
band.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges the latest completed snapshot date.

- For every group outside its band on that date, it records one allocation drift event,
  unless one already exists for that group, side and stay start. The subject of the event
  is the group name, the side and the start date.
- It records nothing for a group inside its band, when there are no snapshots, or when the
  portfolio holds no securities on the latest date.

As with the drawdown monitor, the stay start in the subject makes each excursion announced
once with no state of its own: while the group stays outside the band the start does not
change; if it returns inside and leaves again, that is a new stay and is announced again.
Moving from above to below the band, which takes at least one date inside it, is a new stay on
the other side. Weekends and holidays judge the same date again and record nothing.

- A group whose instruments are not registered or are not held has allocation zero. A group
  with a minimum and no holdings is therefore below its band and is announced; this is
  intended, so a missing holding is visible. A ticker that is not registered is logged as a
  warning on each run.
- A ticker that matches more than one registered instrument fails the run and is logged, so
  the operator resolves the ambiguity.
- Retuning a bound keeps the events already recorded; a stay that is still outside the band
  keeps its start and is not announced again, and a stay that now falls outside a tighter
  band is announced as a new event with the same start.
- Removing an announced event makes the monitor announce that stay again while it lasts.

The allocation changes with trades as well as with prices: buying or selling moves it
without any market move. The message says so. The message names the group, the side, the
group allocation against the bound that was crossed, the start of the stay, and each member
instrument with its allocation. The wording is the renderer's.

### Configuration

The monitor is named allocation.drift in the monitors configuration and records
notification.allocation.drift. It has one setting, groups, a non-empty list:

```yaml
monitors:
  allocation.drift:
    groups:
      - name: us-core
        instruments: [VOO, VTI]
        max_pct: "15"
      - name: gold
        instruments: [GLD]
        min_pct: "5"
        max_pct: "10"
```

- `name` is required, not empty and unique among the groups.
- `instruments` is a non-empty list of tickers with no repeats, within the group and across
  all groups.
- `min_pct` and `max_pct` are decimals between 0 and 100 written as quoted text; at least one
  must be present, and the minimum may not exceed the maximum.
- Any other key in a group, or any other setting, is invalid, whether or not the entry is
  disabled.

## Acceptance criteria

- A group of two instruments whose combined allocation exceeds its maximum records one event
  for the first date of the stay; running again on later dates, and on weekends, records
  nothing while it stays above.
- After the group returns inside the band and exceeds the maximum again, a new event is
  recorded; a drop below the minimum records an event for the other side.
- A group exactly at a bound records nothing.
- Instruments outside every group never produce an event; a group with a minimum and no
  holdings produces an event.
- A ticker in two groups, a group with neither bound, a minimum above the maximum, a bound
  outside 0 to 100 or a repeated group name makes the monitors file invalid.
- With no snapshots, or with zero securities value on the latest date, nothing is recorded.
- With no enabled entry in the monitors configuration, nothing is recorded.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its groups.
- Related: 2026-11-13-investments-mcp, the allocation concept.
- Related: 2026-11-22-monitor-portfolio-drawdown, whose crossing technique the stay start follows.
