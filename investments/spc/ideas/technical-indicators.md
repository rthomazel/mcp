---
id: 2026-11-15-technical-indicators
type: idea
summary: Follow-up idea for classic technical indicators such as moving averages computed over stored prices and portfolio value series.
author: Thom
created: 2026-11-15
agents: claude
---

# Technical indicators

## Status

Idea only. Everything below is to be decided.

## Idea

Offer classic indicators over what the server already stores: simple and exponential moving averages,
RSI, MACD, Bollinger bands, ATR, volume averages, and distance from a moving average. They would serve the
weekly review, for example whether a holding trades below its 200 day average, and could also apply to the
portfolio value series.

## What already exists

- Daily open, high, low, close, adjusted close and volume per instrument, with full history backfilled on
  registration, which is enough for every indicator above.
- Daily securities value and, once the cash feature ships, account equity per snapshot date.
- Calculations already run in SQL; moving averages and bands are plain window functions.

## Gaps to resolve

- **Adjusted versus real close.** The spec values holdings on the real close and never rewrites it, so raw
  closes jump at splits. Indicators on raw closes would show false signals; they should use the adjusted close,
  an explicit exception to the rule that adjusted close is never used for holdings, since this is analysis, not valuation.
- **Recursive indicators.** Exponential averages, RSI and MACD depend on their previous value, which fits SQL
  poorly and invites floating point. A recursive query, or an accepted floating point exception for display-only
  indicators, has to be chosen.
- **Held instruments only.** The refresh covers held instruments, so a sold instrument's indicators stop at its last price.
- **Portfolio series.** Indicators on portfolio value are distorted by deposits and purchases; account equity
  is better, and a return series from the portfolio returns idea would be better still.
- **Mutual funds.** No volume, and prices a day late, so volume indicators do not apply.
- **Warm-up.** Each indicator needs a minimum history; short histories must report not enough data, never a partial value.
- **Boundaries.** The base spec places news, fundamentals and broad market research with the separate
  market-trends workflow; indicators sit on the edge of it, so ownership has to be settled.

## To be decided

- Which indicators are worth having; moving averages and drawdown-style measures first is a likely start.
- Whether indicators are stored or computed on request; on request avoids storage and corrections drift.
- One generic tool with an indicator name and parameters, or a tool per indicator.
- Whether signals such as crossovers are reported, or only the series, leaving interpretation to the agent.
