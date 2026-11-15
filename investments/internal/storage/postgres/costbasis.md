# Constants

costBasisSQL = walk each instrument's ledger up to the reference date in ledger order (effective date, execution time, id) as a recursive query carrying running quantity, running cost and cumulative realized P&L, applying the average-cost rules below; take each instrument's final state and add cumulative dividends, tax and fees by sums over the same ledger; skip instruments whose running quantity goes negative.
portfolioPnLSQL = valuationSQL as a subquery joined to costBasisSQL by instrument, adding each open holding's cost, average cost, unrealized P&L (market value minus cost, where a usable price exists), unrealized P&L as a percent of cost, and total return (unrealized plus realized plus dividends minus tax); also returns the zero-quantity instruments from costBasisSQL as closed rows with realized P&L, dividends, tax, fees and their total return, and the totals over every open and closed row as a rollup row.

# Rules

Average-cost rules, applied per ledger row in order, all in SQL:

- BUY: cost increases by the negated cash amount, fee included; quantity increases.
- SELL: the average cost is running cost over running quantity; the cost released is the average cost times the shares sold, or the whole remaining cost when the sale takes every share, so rounding never leaves residue; realized increases by the cash amount minus the cost released; quantity and cost fall.
- SPLIT: quantity changes, cost and realized are unchanged.
- ADJUSTMENT: a quantity increase adds the cash paid, if any, to cost, which is how a reinvested dividend enters basis, and adds the shares at zero cost when no cash was paid; positive cash on an increase is treated as no cash paid. A quantity decrease realizes like a sell using its cash amount, which may be zero.
- DIVIDEND and TAX: no change to quantity, cost or realized; they are summed separately.

Fees are inside cost on buys and inside net proceeds on sells, so they are reported but never subtracted again, in realized P&L or in the total return.
Snapshots keep using valuationSQL alone, so the daily job never pays for the ledger walk.
