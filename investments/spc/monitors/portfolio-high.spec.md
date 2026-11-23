---
id: 2026-11-23-monitor-portfolio-high
type: spec
summary: Monitor that notifies the operator when the securities value reaches a new all-time high, at most once in a configured number of days.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitor: portfolio high

## Description

The counterpart of the portfolio drawdown monitor, built on the same monitor mechanism and
the same daily securities value series, and configured in the monitors configuration. It
tells the operator when the securities value is at a new all-time high. In a rising market a
new high can come on most days, so announcements are spaced by a configurable number of days.

## Concepts

**New high.** The latest completed snapshot date's securities value is strictly greater than
the value of every earlier date, over all stored snapshots, the series the value history and
the drawdown monitor use. The first snapshot has nothing before it and is not a new high.

**Spacing.** The number of days that must have passed since the date of the last announced
high before another is announced.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges the latest completed snapshot date.

- When the date is a new high and no portfolio high event exists whose date is within the
  spacing days before it, it records one portfolio high event. The subject of the event is
  the date of the high.
- Otherwise it records nothing. A high that the spacing suppresses is not announced later
  by itself; only a date that is a new high when judged is announced.

A date is judged once however often the monitor runs, because the date is the subject.
Weekends and holidays judge the same date again and record nothing new. When the monitor is
first enabled on a date that is a high, that high is announced. Removing the latest event
makes the monitor announce a new high within the spacing again.

The measure is a change in value, not return: a deposit or a purchase of shares funded from
outside raises the value, and a new high can follow without a market move. The message says
so. It names the date, the new value, the previous high with its date, how far above it the
new value is in dollars and percent, and the date of the last announced high. The wording is
the renderer's.

### Configuration

The monitor is named portfolio.high in the monitors configuration and records
notification.portfolio.high. It has one optional setting:

```yaml
monitors:
  portfolio.high:
    min_days_between: 15         # optional, 15 by default
```

- `min_days_between` is a whole number of days, zero or more, which may be written as a
  plain number. Zero announces every new high.
- Any other setting is invalid, whether or not the entry is disabled.

## Acceptance criteria

- With the default spacing, a new high on a date records one event; a new high on each of the
  next fourteen days records nothing; a new high fifteen days or more after the announced
  one records an event.
- A value that is equal to the previous high, or below it, records nothing.
- Running the monitor again for the same date records nothing; the first snapshot alone
  records nothing.
- With a spacing of zero, every date that is a new high records an event.
- A negative or non-numeric spacing makes the monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- Spacing is measured from the date of the last announced high and not from when its
  event was created, so a late run does not shift the window.
- The default of fifteen days is the operator's choice; the portfolio is expected to
  set new highs often in an uptrend.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets the spacing.
- Related: 2026-11-22-monitor-portfolio-drawdown, whose series it shares.
- Related: 2026-11-15-value-history, whose all-time high it matches.
- Related: 2026-11-23-monitor-instrument-extremes, the same idea for instrument prices.
