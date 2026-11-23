---
id: 2026-11-23-monitor-instrument-extremes
type: spec
summary: Two monitors on instrument prices, drawdown from the all-time high in configured steps and new all-time highs spaced by a configured number of days.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitors: instrument drawdown and instrument high

## Description

The instrument counterparts of the portfolio drawdown and portfolio high monitors, built on
the same monitor mechanism and configured in the monitors configuration. They watch the
price series of chosen instruments instead of the securities value, with the same rules,
so one description covers both. They are two monitors with separate entries because an
operator may want one without the other.

## Concepts

**Series.** An instrument's daily adjusted closes over every stored price, from the earliest,
which is the start of its backfilled history. Adjusted close is used so a split or a dividend
is never a fall. An all-time high is the highest value of the series, the earliest date
winning a tie.

**Drawdown, increment, level, crossing, new high and spacing.** As defined for the portfolio
drawdown and portfolio high monitors, applied to an instrument's series instead of the
securities value. A level is announced per instrument, and a crossing and a spacing are
tracked per instrument.

**Selection.** The instruments a monitor watches: a list of tickers, or the word held for
every instrument with an active position on the latest completed snapshot date.

## Behavior

Both monitors run daily at 7:00 AM Eastern and judge each selected instrument on the latest
price date of the store. An instrument with no price on that date is skipped.

### Instrument drawdown

- For every level the instrument's drawdown is at or below, it records one instrument
  drawdown event unless one exists for that instrument, level and crossing. The subject of
  the event is the instrument, the level and the crossing date.
- It records nothing for an instrument shallower than the first level or with no prices.

Each level is announced once per stay below it, as for the portfolio drawdown: a recovery above
a level clears it, a new high clears all levels, a fall past several levels in a day announces
each, and weekends judge the same date again and record nothing new. Changing the increment
keeps levels that are still multiples of it. Enabling the monitor while an instrument is
already below levels announces them, one message each.

The message names the instrument, the level, the drawdown and its date, the date of the
high, the crossing date, and the adjusted close at the high and now.

### Instrument high

- When the latest date is a new high of the instrument and no instrument high event exists
  for it within the spacing days before that date, it records one instrument high event.
  The subject of the event is the instrument and the date.
- It records nothing otherwise, and an instrument's first price is not a new high.

The message names the instrument, the date, the new adjusted close, the previous high with
its date and the percent above it, and the date of the last announced high for the
instrument.

### Configuration

The monitors are named instrument.drawdown and instrument.high and record
notification.instrument.drawdown and notification.instrument.high.

```yaml
monitors:
  instrument.drawdown:
    increment_pct: "5"
    instruments: [QQQ, VOO]      # a list of tickers, or: held
  instrument.high:
    min_days_between: 15         # optional, 15 by default
    instruments: held
```

- `increment_pct` is required for instrument.drawdown, a positive decimal written as quoted
  text.
- `min_days_between` is a whole number, zero or more, which may be written as a plain
  number.
- `instruments` is required: the word held, or a non-empty list of tickers without repeats.
- Any other setting is invalid, whether or not the entry is disabled. A ticker that is not
  registered is logged as a warning on each run.

## Acceptance criteria

- With an increment of 5, an instrument that falls from its high to -11 records its -5 and
  -10 events once; later days at or below those levels record nothing; after a recovery above
  -5 and a new fall to -5, the -5 level is announced again.
- A split or a dividend does not change the drawdown.
- With the default spacing, an instrument's new highs on consecutive days record one event;
  a new high fifteen or more days after the announced one records another; instruments are
  spaced independently.
- With the selection held, an instrument bought later is watched from its next run, and one
  sold is not.
- A missing or non-positive increment, an empty list, a repeated ticker, a negative spacing
  or an unknown key makes the monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- One spec for two monitors that differ only in what they announce.
- The same level and spacing semantics as the portfolio monitors, so the operator learns the
  behavior once.

## Risks

- The provider may restate adjusted history, for example after a dividend. A restated series
  can move a crossing date and announce a level again. The series is read as stored; the
  effect is expected to be rare.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns them on.
- Related: 2026-11-22-monitor-portfolio-drawdown, whose levels and crossings they follow.
- Related: 2026-11-23-monitor-portfolio-high, whose spacing they follow.
