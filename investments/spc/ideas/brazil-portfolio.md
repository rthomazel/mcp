---
id: 2026-11-13-brazil-portfolio
type: idea
summary: Future integration of a fragmented, larger Brazilian portfolio with performance tracking.
author: Thom
created: 2026-11-13
agents: claude
---

# Brazil portfolio

## Status

Idea only, not part of v1. The Investments MCP spec excludes Brazilian assets,
non-USD instruments and FX. This captures the intent to integrate them later with
an appropriate design, not by bolting them onto the v1 model.

## Intent

Track the performance of a Brazilian portfolio in the same place as the USD one.
It is much larger than the USD positions, so it matters more than v1 does, and
performance is the point, not only valuation.

## What makes it different

- It is fragmented across many products rather than a handful of listed tickers.
- Some holdings follow Brazil's base interest rate, so their value grows by accrual
  and is not quoted by a market.
- Other holdings are securities that can be sold to other investors, with a price
  marked daily.
- Programmatic retrieval of values may not exist for some or most of it. Manual
  entry, statements or periodic imports may be the only source.
- Currency is BRL. Keeping it separate avoids FX, since the two portfolios are not combined.

## Tensions with the v1 design

- v1 is USD only and has no FX.
- v1 values holdings as quantity times a provider closing price. Accrual-based
  holdings do not fit that model, and manually reported values do not come from a
  price provider.
- v1 excludes portfolio-level returns, tax lots and cost basis. Tracking
  performance here needs a cash-flow model v1 deliberately avoids.
- v1 snapshots assume daily market prices; some holdings may only have periodic values.

## Direction

Brazil is a separate portfolio, not merged into the USD one. Its data does not need
to mix with the USD data, so there is no combined total and no FX requirement
unless a later need appears. The aim is to reuse design elements already built for
the USD portfolio where they fit, not to share its tables or force its model onto
holdings it does not suit.

## To be decided

- Which v1 design elements can be reused, such as the ledger of events, daily
  snapshots, the updates log, the ingestion job structure and the access path,
  without sharing data where sharing makes no sense.
- Which valuation approaches are needed: accrual against a rate index, daily marked
  prices, and manually reported values.
- How performance is defined and what cash-flow records it requires.
- Where each holding's data can come from, and what is realistically retrievable.
- Whether the v1 design needs any accommodation now, such as keeping currency a
  first-class field, or whether a later redesign is acceptable.

The v1 spec should not grow to cover this. If the design question affects v1
choices, bring it back to the spec explicitly.
