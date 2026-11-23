---
id: 2026-11-23-monitor-price-target
type: spec
summary: Monitor that notifies the operator once, and again after a year, when an instrument's daily price range reaches a configured target price, within a tolerance.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitor: price target

## Description

Built on the monitor mechanism in the notifications spec and configured in the monitors
configuration. It tells the operator when an instrument trades at a price they are waiting
for, such as a level at which they intend to buy or sell. It uses the daily price data the
system already stores; there is no intraday price.

## Concepts

**Target.** An instrument, a price, and a tolerance in percent. The band is the price plus and
minus the tolerance, so a target of 232 with a tolerance of 3 covers 225.04 to 238.96. The
band means the operator need not know whether the price will arrive from above or below, and
a tolerance of zero is an exact price.

**Reached.** A target is reached on a price date when the day's range from low to high touches
the band. Using the range and not the close catches a price that crossed the band during the
day and closed outside it. The unadjusted prices are used, since a target is a price the
operator sees on a quote.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges each target on the latest price date of
the store. A target whose instrument has no price on that date is skipped.

- When the target is reached, it records one price target event, unless one exists for the
  instrument and price. The subject of the event is the instrument and the price.
- It records nothing otherwise.

Each target is announced once however long the price stays in the band. The outbox deletes
fully handled events after a year, so the target is announced again after that if the price
is in the band then, which gives once a year with no state of the monitor's own. Weekends and
holidays judge the same date again and record nothing new. When a target is added while the
price is already in the band, it is announced at the next run. Removing the event makes the
target announce again if the price is still in the band.

The message names the instrument, the target price and the band, the date, and the day's low,
high and close. The wording is the renderer's.

### Configuration

The monitor is named price.target in the monitors configuration and records
notification.price.target. It has one setting, targets, a non-empty list:

```yaml
monitors:
  price.target:
    targets:
      - instrument: QQQ
        price: "232"
        tolerance_pct: "3"       # optional, zero by default
      - instrument: QQQ
        price: "250"
```

- `instrument` is a ticker; `price` is a positive decimal written as quoted text.
- `tolerance_pct` is a decimal from zero up to but not including 100, written as quoted text.
- The same instrument may have several targets; the same instrument and price twice is
  invalid.
- Any other key is invalid, whether or not the entry is disabled. A ticker that is not
  registered is logged as a warning on each run.

## Acceptance criteria

- With a target of 232 and a tolerance of 3, a day whose range includes any price from 225.04
  to 238.96 records one event; later days in the band record nothing.
- A day that gaps over the band, with a low above it or a high below it, records nothing; a
  day that crosses the band and closes outside it records an event.
- After the event has been deleted, because it was handled more than a year ago or removed, a
  price still or again in the band records a new event.
- Two targets of one instrument are announced independently.
- A price that is not positive, a tolerance of 100 or more, a repeated instrument and price
  pair or an unknown key makes the monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- Daily range, not intraday quotes: the operator chose to stay with daily data, and the range
  captures a touch within the day.
- Once a year comes from the outbox's one year retention of handled events, so the subject
  carries no year and the monitor needs nothing extra.
- The tolerance is both sides, so a target can be written without a direction.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its targets.
- Related: 2026-11-13-investments-mcp, the daily prices it reads.
