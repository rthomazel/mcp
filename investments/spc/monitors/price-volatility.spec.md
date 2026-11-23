---
id: 2026-11-23-monitor-price-volatility
type: spec
summary: Monitor that notifies the operator when an instrument's price move over a configured window of days is at least a configured number of standard deviations of its recent moves over that window.
author: Thom
created: 2026-11-23
updated: 2026-11-23
agents: claude
---

# Monitor: price volatility

## Description

Built on the monitor mechanism in the notifications spec and configured in the monitors
configuration. It tells the operator when an instrument moved unusually far over a window of
days, judged against how much that instrument usually moves over the same window rather than
against a fixed percentage, so two percent is news for a bond fund and noise for a volatile
stock.

## Concepts

**Window.** A duration, one day by default, ending on the latest price date.

**Move.** The percent change of the adjusted close from the close on the last price date on or
before the window's start, one window before the latest date, to the latest price date's. Adjusted
close is used so a split or a dividend never looks like a move. With a window of one day it is the
daily move.

**Look-back.** A duration. It defaults to three months, or to the window when the window is longer.
It may not be shorter than the window.

**Volatility.** The sample standard deviation of the moves of the same window length that end on
each price date within the look-back before the latest date. The judged move is excluded so a large
move cannot raise its own baseline.

**Variation.** The move divided by the volatility, in standard deviations, signed.

**Interval.** The least time between two announcements for an instrument, a duration, one day by
default, measured from the date of the last event recorded for it. It makes the monitor act as if it
ran that often for that instrument.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges the latest price date in the store, for
each configured instrument that has a price on it.

- When less time than the interval has passed since the instrument's last event date, it
  records nothing for it.
- It records one price volatility event when the variation is at least the entry's threshold
  in a direction the entry watches: up for a rise, down for a fall. The subject of the event
  is the instrument, the date and the direction.
- It records nothing otherwise, including when the instrument has no price on the latest date.
- An instrument with fewer than 20 moves in the look-back, or with zero volatility, is
  skipped and logged at info level, because a variation cannot be judged.

A day is judged once however often the monitor runs, because the date is in the subject.
Weekends and holidays judge the same date again and record nothing new. With a window longer
than the interval, consecutive windows overlap and the interval is what stops one move from being
announced on every day it stays inside the window.

The message names the instrument, the date, the window, the move, the variation against the
threshold, the volatility and the look-back used. The wording is the renderer's.

### Configuration

The monitor is named price.volatility in the monitors configuration and records
notification.price.volatility. Its settings:

```yaml
monitors:
  price.volatility:
    window: 1d                   # optional, 1 day by default
    interval: 1d                 # optional, 1 day by default
    lookback: 3mo                # optional, 3 months or the window if longer
    instruments:
      - instrument: QQQ
        variation_std_dev: "2"
        direction: up            # optional: up, down or both; both by default
      - instrument: GLD
        variation_std_dev: "3"
```

- `window`, `interval` and `lookback` are durations as the duration spec defines them, written with
  a unit, so a bare number is invalid. They apply to every entry. A look-back shorter than the
  window is invalid.
- `instruments` is a non-empty list; an instrument appears at most once.
- `instrument` is a ticker; `variation_std_dev` is a positive decimal written as quoted text.
- Any other key, or an unknown direction, is invalid, whether or not the entry is disabled.
- A ticker that is not registered is logged as a warning on each run.

## Acceptance criteria

- With the defaults, a threshold of 2 and a daily move that is 2.4 standard deviations of the daily
  moves over the preceding three months, one event is recorded for the instrument, date and
  direction; running again records nothing.
- With a window of 5d, the move is the change from the close on or before five calendar days earlier,
  and the volatility comes from five day moves, so a steady rise of one percent a day is judged
  against how often five day moves are that large.
- With a window of 6mo and no look-back, the look-back is six months.
- With a window of 5d and an interval of 5d, a move that stays inside the window on the following
  days is announced once.
- A move of the same size in the unwatched direction records nothing; with both directions watched,
  either direction records an event.
- A split or a dividend alone does not produce an event.
- An instrument with a short history or flat prices records nothing and is logged.
- A threshold that is zero or negative, a window, interval or look-back that is not a valid
  duration, a look-back shorter than the window, a repeated instrument or an unknown key makes the
  monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- One move over the window against the volatility, not stepped levels: the threshold is the
  operator's own number per instrument.
- Window and interval are settings of the monitor, not of an entry, and default to the daily
  judgement. The look-back default of three months gives about sixty daily moves.
- Percent change rather than log return, to match every other percentage the system shows. Over
  a day the two differ negligibly.
- Adjusted close over close, so corporate actions are not moves.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its entries.
- Related: 2026-11-13-investments-mcp, the daily prices it reads.
- Related: 2026-11-23-monitor-portfolio-movement, which has the same window and interval settings.
