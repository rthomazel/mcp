---
id: 2026-11-15-position-pnl
type: spec
summary: Average-cost position P&L with separate realized, unrealized, dividend, tax and fee reporting, and trade fees on buys and sells.
author: Thom
created: 2026-11-15
agents: claude
---

# Position P&L

## Intent

Show what each position has earned, distinguishing unrealized P&L on open holdings from
realized P&L on sales, with dividends, taxes and fees shown separately. Market value minus
purchase cost would mislead for positions with several buys, sales, splits or reinvestments,
so a cost-basis method is fixed first.

## Concepts

A buy or sell may carry a fee, a non-negative amount that is part of the recorded cash
amount, never in addition to it. It is included in cost on a buy and deducted from proceeds on
a sell, so cash counts it once. Account fees that belong to no trade are cash transactions; see the cash feature.

## Behavior

Cost basis is average cost, computed by the backend over the ledger in chronological order,
per instrument. There are no tax lots.

- Buy: the cash paid, fee included, is added to cost; quantity increases.
- Sell: realized P&L is the cash received, net of fee, minus the average cost of the shares sold.
  Cost falls by that average cost; the average cost per share is unchanged.
- Split: quantity changes, cost is unchanged, so the average cost per share adjusts.
- Adjustment: a quantity increase with cash paid adds that cash to cost, which is how a
  reinvested dividend enters basis; a quantity increase with no cash adds shares at zero cost; a
  quantity decrease realizes like a sell using its cash, if any.
- Dividends and taxes do not touch cost basis.

P&L is part of the portfolio valuation read, not a separate report. For each holding as of a date the portfolio reports quantity, total cost, average cost, market value,
unrealized P&L in dollars and percent of cost, and cumulative realized P&L, dividends received,
tax withheld and fees paid, each separately. Fees are already inside cost and sale proceeds,
so they are reported for information and are never subtracted again. An optional total-return view sums
unrealized and realized P&L plus dividends minus taxes. It is a position
total, not a time-weighted or money-weighted return. A position that is fully sold stays
visible as a closed position with its realized P&L, dividends and tax. Holdings with
no usable price withhold market value and unrealized P&L, and the totals that need a price are withheld likewise, as the securities total is.
The totals add up every open and closed position. Realized P&L is cumulative since the first ledger entry up to the date.

## Non-goals

- Tax lots and any tax reporting.
- Time-weighted or money-weighted returns.

## Acceptance criteria

- Average cost: after two buys and a partial sell, realized P&L, remaining cost and average cost match hand-calculated values; a split changes quantity and average cost but not total cost and realizes nothing; a reinvested dividend adds to cost.
- A trade fee is within the recorded cash amount, counted once in cash, in cost on a buy and in net proceeds on a sell, and reported separately.
- A fully sold position shows its realized P&L; a position with no usable price withholds unrealized P&L; the total-return view equals unrealized plus realized plus dividends minus taxes.

## Refs

- Related: 2026-11-13-investments-mcp, the base spec this extends.
