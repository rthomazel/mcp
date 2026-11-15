# Constants

pnlSQL = walk each instrument's ledger up to the reference date in ledger order (effective date, execution time, id) as a recursive query carrying running quantity, running cost and cumulative realized P&L, applying the average-cost rules per event type below; take each instrument's final state, join its latest prices row on or before the reference date (lateral join, newest first, one row) with the days between them, add cumulative dividends, tax and fees by sums over the same ledger, compute market value, unrealized P&L and its percent of cost, and the totals as a rollup row; skip instruments whose running quantity goes negative.

# Functions

## Store.GetPnL(ctx Context, date *domain.Date) (domain.PnLResult, error)

1. Derive the reference date: date, or today in Store.marketLocation.
2. Run pnlSQL for the reference date.
3. Mark each open position unusable when it has no price row ("missing") or its price is Store.stalePriceDays or more older than the reference date ("stale").
4. Select integrity errors via integritySQL for the reference date.
5. if no open position is unusable, set Complete, and keep the position and total unrealized and total return figures.
6. otherwise leave Complete false and withhold the figures that need a price on the unusable positions and in the totals; realized, dividends, tax, fees and cost stay.
7. Return the result with open positions first.

#### Errors

- **2.** if the query fails, return the wrapped error.

---

- **4.** if the query fails, return the wrapped error.

Average-cost rules, applied per ledger row in order, all in SQL:

- BUY: cost increases by the negated cash amount, fee included; quantity increases.
- SELL: the average cost is running cost over running quantity; the cost released is the average cost times the shares sold, or the whole remaining cost when the sale takes every share, so rounding never leaves residue; realized increases by the cash amount minus the cost released; quantity and cost fall.
- SPLIT: quantity changes, cost and realized are unchanged.
- ADJUSTMENT: a quantity increase adds the cash paid, if any, to cost, which is how a reinvested dividend enters basis, and adds the shares at zero cost when no cash was paid; positive cash on an increase is treated as no cash paid. A quantity decrease realizes like a sell using its cash amount, which may be zero.
- DIVIDEND and TAX: no change to quantity, cost or realized; they are summed separately.

Fees are inside cost on buys and inside net proceeds on sells, so they are reported but never subtracted again, in realized P&L or in the total return.
Valuation uses close, never adjusted close, and the stale price threshold is the same as the portfolio's.
All arithmetic is in SQL; Go only decides what to expose.
