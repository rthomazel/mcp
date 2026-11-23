---
id: 2026-11-23-idea-portfolio-performance
type: idea
summary: Follow-up idea for a weekly report of the portfolio's time-weighted return against a benchmark, deferred until portfolio returns exist.
author: Thom
created: 2026-11-23
agents: claude
---

# Portfolio performance against a benchmark

## Status

Idea only, deferred from the monitors batch. Everything below is to be decided.

## Idea

Once a week, report how the portfolio performed over the week against a benchmark such as the Nasdaq-100.
The comparison needs the time-weighted return, which removes the timing of deposits and withdrawals.
The money-weighted and time-weighted returns idea describes that measure and its gaps; this idea
depends on it and cannot be built before it. The base spec lists portfolio returns as a non-goal,
so that feature has to be specified first.

## Benchmark

Tiingo has no index symbols: NDX, IXIC and their caret and dollar forms are all unknown to its
daily prices. It does carry ETFs, so the practical benchmark is an ETF that tracks the index, such as
QQQ for the Nasdaq-100, registered as an instrument so that the existing provider prices it.
Differences from the index are the fund's fees and tracking error, and the adjusted close includes
dividends, so it is close to a total return.

Open point: the base spec places benchmarks with the separate market-trends workflow, so the
ownership of a benchmark instrument has to be settled, along with whether a benchmark counts as a
holding or sits outside positions and valuation.

A configuration would look like this, with the benchmark a registered ticker:

```yaml
monitors:
  portfolio.performance:
    benchmark: QQQ
```

## Gaps to resolve

- Everything in the returns idea: securities moved in without cash, start of history, snapshot gaps.
- The weekly window, and whether the result is an event with the week as its subject or part of the
  weekly summary email.
- What counts as a notable result: always report, or only when the gap to the benchmark passes a
  threshold.

## Refs

- Related: 2026-11-15-portfolio-returns, the return measures this needs.
- Related: 2026-11-23-idea-weekly-summary, where the report may live.
