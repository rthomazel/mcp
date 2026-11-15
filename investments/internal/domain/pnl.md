# Types

## PositionPnL

1. InstrumentID int
2. Symbol string
3. Exchange string
4. Quantity Decimal, zero for a fully sold position
5. CostUSD Decimal, total average-cost basis of the open shares, zero when closed
6. AverageCostUSD *Decimal, cost per open share, nil when closed
7. ClosePrice *Decimal, nil when closed or no usable price
8. PriceDate *Date, set whenever a price row exists for an open position
9. MarketValueUSD *Decimal, nil when closed or no usable price
10. UnrealizedPnLUSD *Decimal, market value minus cost, nil when closed or no usable price
11. UnrealizedPnLPercent *Decimal, unrealized P&L as a percent of cost, nil when it is nil or cost is zero
12. RealizedPnLUSD Decimal, cumulative since the first ledger entry up to the date
13. DividendsUSD Decimal, cumulative dividends received
14. TaxUSD Decimal, cumulative tax withheld, a positive amount
15. FeesUSD Decimal, cumulative trade fees paid, for information: already inside cost and proceeds
16. TotalReturnUSD *Decimal, unrealized plus realized plus dividends minus tax, nil when unrealized is nil for an open position
17. PriceIssue string, empty when usable or closed, otherwise "missing" or "stale"

One instrument's profit and loss under average cost as of a date. A position total, not a time-weighted or money-weighted return.

## PnLTotals

1. CostUSD Decimal
2. UnrealizedPnLUSD *Decimal, nil when the report is incomplete
3. RealizedPnLUSD Decimal
4. DividendsUSD Decimal
5. TaxUSD Decimal
6. FeesUSD Decimal
7. TotalReturnUSD *Decimal, nil when the report is incomplete

Sums over every position. The figures that need a price are withheld when any open position lacks a usable price, as in the portfolio, because a total over only some holdings would mislead.

## PnLResult

1. Date Date, the reference date
2. Complete bool, false when any open position has no usable price
3. Positions []PositionPnL, open positions first, then closed ones, each ordered by symbol then exchange
4. Totals PnLTotals
5. IntegrityErrors []IntegrityError

An instrument whose ledger goes negative has no defined cost, so it is left out of Positions and reported in IntegrityErrors, never silently omitted.
