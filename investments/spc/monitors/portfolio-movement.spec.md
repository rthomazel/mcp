---
id: 2026-11-23-monitor-portfolio-movement
type: spec
summary: Monitor that notifies the operator when the securities value moved by at least a configured percentage in one day because of market prices alone, ignoring trades.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitor: portfolio movement

## Description

Built on the monitor mechanism in the notifications spec and configured in the monitors
configuration. It tells the operator when the market moved the portfolio unusually far in
a day. Buying, selling and other ledger activity change the securities value without a
market move, so they are taken out; the value history cannot tell the two apart and this
monitor must.

## Concepts

**Market movement.** For a snapshot date and the snapshot date before it, the sum over every
position held on the earlier date of the shares held then multiplied by the change in that
instrument's price. Shares bought or sold on the later date, or positions opened or closed
on it, contribute nothing for that day. The result is a dollar amount; as a percent it is
divided by the securities value of the earlier date.

It is a measure of price movement on what was held, not an investment return: dividends and
cash are not part of it, and it is not chained across days. It is the same measure the
weekly summary idea intends for contributors, winners and losers.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges the latest completed snapshot date
against the completed snapshot date before it.

- When the market movement percent is at least the configured threshold in a direction the
  monitor watches, it records one portfolio movement event, unless one already exists. The
  subject of the event is the date and the direction.
- It records nothing otherwise, with fewer than two completed snapshots, or when the earlier
  date held no securities.

Each day is judged once however often the monitor runs, and weekends and holidays judge the
same date again and record nothing new.

The message names the date, the movement as a percent and in dollars, the securities value
before and after, and how much of the change between the two values was the market and how
much was trades, so the operator sees why the two differ. It names the largest contributors
in each direction by symbol and dollars. The wording is the renderer's.

### Configuration

The monitor is named portfolio.movement in the monitors configuration and records
notification.portfolio.movement. Its settings:

```yaml
monitors:
  portfolio.movement:
    variation_pct: "3"
    direction: both              # optional: up, down or both; both by default
```

- `variation_pct` is required, a positive decimal written as quoted text.
- `direction` is up, down or both, both when omitted.
- Any other setting is invalid, whether or not the entry is disabled.

## Acceptance criteria

- A day on which the held shares gain 3.4 percent from prices, with a threshold of 3, records
  one event for the date and direction; running again records nothing.
- A day on which the securities value rises 5 percent only because shares were bought records
  nothing; a day on which a sale lowers the value without a price move records nothing.
- A position bought and sold on different days contributes only on the days it was held at
  the start of the day.
- A fall of the same size in an unwatched direction records nothing.
- The message splits the value change into market and trades.
- A missing, zero or negative threshold or an unknown direction makes the monitors file
  invalid.
- With no enabled entry, nothing is recorded.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its threshold.
- Related: 2026-11-15-value-history, whose value change this separates from trades.
- Related: 2026-11-23-idea-weekly-summary, which reuses the market movement measure.
