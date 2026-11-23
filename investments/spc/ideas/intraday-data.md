---
id: 2026-11-23-idea-intraday-data
type: idea
summary: Follow-up idea for intraday prices, which would allow hourly monitors, targets checked during the day and durations shorter than a day.
author: Thom
created: 2026-11-23
agents: claude
---

# Intraday data

## Status

Idea only, not part of the monitors batch. Everything below is to be decided.

## Idea

The system stores daily prices only. Intraday prices would let monitors react within the day: a price
target that is checked hourly during market hours, a volatility or movement monitor with a window of a
few hours, and durations shorter than a day. Until then every duration shorter than one day is rejected
in configuration, so the settings can grow into intraday later without changing what a valid value means.

## What already exists

- The provider's latest-quote endpoint answers for an instrument with the last trade price and the
  previous close. It was probed from this environment and works with the existing credential; no historical
  intraday series was checked.
- The monitor scheduler runs a task daily at one clock time, and the monitors configuration says the time
  is part of the monitor. Hourly would need a schedule of several times a day or an interval.
- The price target monitor judges the daily low and high, which already catches a touch of the target during
  the day, one day late.

## Gaps to resolve

- Whether to store intraday bars or only read the latest quote when a monitor runs.
- Provider limits and licensing for intraday data, and whether the quote is real time or delayed.
- Market hours and holidays: when an hourly monitor should run, and what the last bar of a day is.
- Storage volume if bars are kept, and how long.
- How a duration shorter than a day is applied to the daily snapshots and the value series.
- Whether the portfolio value, which is valued at closes, has any intraday meaning.

## Refs

- Related: 2026-11-23-duration, which rejects spans shorter than a day.
- Related: 2026-11-23-monitor-price-target, which uses the daily range instead.
