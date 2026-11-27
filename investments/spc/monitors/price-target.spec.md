---
id: 2026-11-23-monitor-price-target
type: spec
summary: Monitor that notifies the operator once, and again after a year, when an instrument's daily price range reaches a configured target price.
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

**Target.** An instrument and a price.

**Reached.** A target is reached on a price date when the price lies within the day's range
from low to high. Using the range and not the close catches a price that was touched during the
day and closed away from it, so no tolerance is needed. The unadjusted prices are used, since a target is a price the
operator sees on a quote.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges each target on the latest price date of
the store. A target whose instrument has no price on that date is skipped.

- When the target is reached, it records one price target event, unless one exists for the
  instrument and price. The subject of the event is the instrument and the price.
- It records nothing otherwise.

Each target is announced once however long the price stays within the day's range. The outbox deletes
fully handled events after a year, so the target is announced again after that if the price
is within the range then, which gives once a year with no state of the monitor's own. Weekends and
holidays judge the same date again and record nothing new. When a target is added while the
price is already within the range, it is announced at the next run. Removing the event makes the
target announce again if the price is still within the range.

The message names the instrument, the target price, the date, and the day's low,
high and close, and ends with the target's note when it has one. The wording is the renderer's.

### Configuration

The monitor is named price.target in the monitors configuration and records
notification.price.target. It has one setting, targets, a non-empty list:

```yaml
monitors:
  price.target:
    targets:
      - instrument: QQQ
        price: "232"
      - instrument: QQQ
        price: "250"
        note: sell a third, see the March plan
```

- `instrument` is a ticker; `price` is a positive decimal written as quoted text.
- `note` is optional text, the reason the target exists. It is sent with the notification and
  documents the file; it plays no part in what makes a target the same, the subject or the
  validation beyond being text.
- The same instrument may have several targets; the same instrument and price twice is
  invalid.
- Any other key is invalid, whether or not the entry is disabled. A ticker that is not
  registered is logged as a warning on each run.

## Acceptance criteria

- With a target of 232, a day with a low of 230 and a high of 236 records one event; later days
  whose range includes 232 record nothing.
- A day that gaps over the target, with a low above it or a high below it, records nothing; a
  day that touches the target and closes away from it records an event.
- After the event has been deleted, because it was handled more than a year ago or removed, a
  price still or again within the range records a new event.
- Two targets of one instrument are announced independently.
- A price that is not positive, a repeated instrument and price
  pair or an unknown key makes the monitors file invalid.
- A target with a note records an event whose message ends with the note; a target without one
  has no note line. A note that is not text makes the monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- Daily range, not intraday quotes: the operator chose to stay with daily data, and the range
  captures a touch within the day.
- Once a year comes from the outbox's one year retention of handled events, so the subject
  carries no year and the monitor needs nothing extra.
- The note is per target, since each target has its own reason, and lives in the event data so the
  renderer needs nothing but the event.
- The range needs no direction or tolerance: a target is reached whether the price comes from
  above or below.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its targets.
- Related: 2026-11-13-investments-mcp, the daily prices it reads.
