---
id: 2026-11-23-monitor-portfolio-movement
type: spec
summary: Monitor that notifies the operator when the securities value moved by at least a configured percentage over a configured window of days because of market prices alone, ignoring trades.
author: Thom
created: 2026-11-23
updated: 2026-11-23
agents: claude
---

# Monitor: portfolio movement

## Description

Built on the monitor mechanism in the notifications spec and configured in the monitors
configuration. It tells the operator when the market moved the portfolio unusually far over a
window of days. Buying, selling and other ledger activity change the securities value without a
market move, so they are taken out; the value history cannot tell the two apart and this
monitor must.

## Concepts

**Market movement.** For one snapshot date and the snapshot date before it, the sum over every
position held on the earlier date of the shares held then multiplied by the change in that
instrument's price. Shares bought or sold on the later date, or positions opened or closed on
it, contribute nothing for that day.

**Window.** A number of dates of the snapshot series, counted back from the latest date, so
weekends and holidays are not counted. The market movement over a window is the sum of the
daily market movements of the days it spans, in dollars; as a percent it is divided by the
securities value on the date before the window starts. A window of one day is the movement of
the latest date alone.

It is a measure of price movement on what was held, not an investment return: dividends and
cash are not part of it, and percentages of different windows are not chained. It is the same
measure the weekly summary idea intends for contributors, winners and losers.

**Interval.** The least number of days between two announcements of this monitor, which makes
it act as if it ran every that many days. It is measured between dates, from the date of the
last event this monitor recorded to the latest date.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges the window ending at the latest completed
snapshot date.

- When fewer days than the interval have passed since the date of its last event, it records
  nothing.
- When the market movement percent over the window is at least the configured threshold in a
  direction the monitor watches, it records one portfolio movement event, unless one already
  exists. The subject of the event is the latest date and the direction.
- It records nothing otherwise, with fewer completed snapshots than the window needs, or when
  the securities value before the window was zero.

Each date is judged once however often the monitor runs, and weekends and holidays judge the
same date again and record nothing new. With a window longer than the interval, consecutive
windows overlap, so the interval is what stops one move from being announced on every day it
stays inside the window.

The message names the window and its dates, the movement as a percent and in dollars, the
securities value before and after, and how much of the change between the two values was the
market and how much was trades, so the operator sees why the two differ. It names the largest
contributors in each direction by symbol and dollars. The wording is the renderer's.

### Configuration

The monitor is named portfolio.movement in the monitors configuration and records
notification.portfolio.movement. Its settings:

```yaml
monitors:
  portfolio.movement:
    variation_pct: "3"
    direction: both              # optional: up, down or both; both by default
    window_days: 1               # optional, 1 by default
    interval_days: 1             # optional, 1 by default
```

- `variation_pct` is required, a positive decimal written as quoted text.
- `direction` is up, down or both, both when omitted.
- `window_days` and `interval_days` are whole numbers of at least 1, which may be written as plain
  numbers.
- Any other setting is invalid, whether or not the entry is disabled.

## Acceptance criteria

- With the defaults, a day on which the held shares gain 3.4 percent from prices, with a
  threshold of 3, records one event for the date and direction; running again records nothing.
- With a window of 3, the market movements of the last three dates are added and divided by the
  value before them; a 3.2 percent total records an event even when no single day reached 3.
- With a window of 3 and an interval of 3, the same move is announced once, and not again on the
  next two days while the window still contains it.
- A day on which the securities value rises 5 percent only because shares were bought records
  nothing; a day on which a sale lowers the value without a price move records nothing.
- A position bought and sold on different days contributes only on the days it was held at the
  start of the day.
- A fall of the same size in an unwatched direction records nothing.
- The message splits the value change into market and trades.
- A missing, zero or negative threshold, a window or interval below 1, a non-numeric window or
  an unknown direction makes the monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- Window and interval are the operator's settings, defaulting to the daily judgement.
- Window movements add dollar movements and divide by the starting value; they are not chained
  returns, in keeping with the system's measures of value change.
- The interval is measured from the last event recorded and so needs no state of its own.
- Only this monitor and the price volatility monitor have these settings for now; others may
  gain them when needed.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its threshold.
- Related: 2026-11-15-value-history, whose value change this separates from trades.
- Related: 2026-11-23-idea-weekly-summary, which reuses the market movement measure.
