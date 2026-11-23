---
id: 2026-11-23-monitor-price-volatility
type: spec
summary: Monitor that notifies the operator when an instrument's daily price move is at least a configured number of standard deviations of its recent daily moves.
author: Thom
created: 2026-11-23
agents: claude
---

# Monitor: price volatility

## Description

Built on the monitor mechanism in the notifications spec and configured in the monitors
configuration. It tells the operator when an instrument moved unusually far in one day,
judged against how much that instrument usually moves rather than against a fixed
percentage, so two percent is news for a bond fund and noise for a volatile stock.

## Concepts

**Daily move.** The percent change of the adjusted close from the previous trading day's to
the latest price date's. Adjusted close is used so a split or a dividend never looks like
a move.

**Volatility.** The sample standard deviation of the daily moves over the lookback window:
the configured number of trading days that precede the day being judged. The judged day is
excluded so a large move cannot raise its own baseline.

**Variation.** The daily move divided by the volatility, in standard deviations, signed.

## Behavior

The monitor runs daily at 7:00 AM Eastern and judges the latest price date in the store, for
each configured instrument that has a price on it.

- It records one price volatility event when the variation is at least the entry's threshold
  in a direction the entry watches: up for a rise, down for a fall. The subject of the
  event is the instrument, the date and the direction.
- It records nothing otherwise, including when the instrument has no price on the latest
  date.
- An instrument with fewer prices than the lookback window needs, or with zero volatility,
  is skipped and logged at info level, because a variation cannot be judged.

A day is judged once however often the monitor runs, because the date is in the subject.
Weekends and holidays judge the same date again and record nothing new.

The message names the instrument, the date, the daily move, the variation against the
threshold, the volatility and the lookback used. The wording is the renderer's.

### Configuration

The monitor is named price.volatility in the monitors configuration and records
notification.price.volatility. Its settings:

```yaml
monitors:
  price.volatility:
    lookback_days: 60            # optional, 60 by default
    instruments:
      - instrument: QQQ
        variation_std_dev: "2"
        direction: up            # optional: up, down or both; both by default
      - instrument: GLD
        variation_std_dev: "3"
```

- `lookback_days` is a whole number of at least 20 and may be written as a plain number.
- `instruments` is a non-empty list; an instrument appears at most once.
- `instrument` is a ticker; `variation_std_dev` is a positive decimal written as quoted text.
- Any other key, or an unknown direction, is invalid, whether or not the entry is disabled.
- A ticker that is not registered is logged as a warning on each run.

## Acceptance criteria

- With a lookback of 60, a threshold of 2 and a daily move that is 2.4 standard deviations
  of the preceding 60 moves, one event is recorded for the instrument, date and direction;
  running again records nothing.
- A move of the same size in the unwatched direction records nothing; with both directions
  watched, either direction records an event.
- A split or a dividend alone does not produce an event.
- An instrument with a short history or flat prices records nothing and is logged.
- A threshold that is zero or negative, a lookback below 20, a repeated instrument or an
  unknown key makes the monitors file invalid.
- With no enabled entry, nothing is recorded.

## Decisions

- One daily move against the volatility, not stepped levels: the threshold is the operator's
  own number per instrument.
- Percent change rather than log return, to match every other percentage the system shows.
  Over a day the two differ negligibly.
- Adjusted close over close, so corporate actions are not moves.

## Refs

- Related: 2026-11-21-notifications, the outbox and monitor mechanism this uses.
- Related: 2026-11-22-monitors-configuration, the file that turns it on and sets its entries.
- Related: 2026-11-13-investments-mcp, the daily prices it reads.
