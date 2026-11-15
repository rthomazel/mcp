# Types

## Holding

1. InstrumentID int
2. Symbol string
3. Exchange string
4. Quantity Decimal
5. ClosePrice *Decimal, nil when no usable price
6. PriceDate *Date, set whenever a price row exists, so a stale price shows its age
7. MarketValueUSD *Decimal, nil when no usable price
8. Allocation *Decimal, nil when the portfolio is incomplete
9. PriceIssue string, empty when usable, otherwise "missing" or "stale"
10. CostUSD Decimal, total average-cost basis of the shares held
11. AverageCostUSD Decimal, cost per share
12. UnrealizedPnLUSD *Decimal, market value minus cost, nil when no usable price
13. UnrealizedPnLPercent *Decimal, unrealized P&L as a percent of cost, nil when it is nil or cost is zero
14. RealizedPnLUSD Decimal, cumulative since the first ledger entry up to the date
15. DividendsUSD Decimal, cumulative dividends received
16. TaxUSD Decimal, cumulative tax withheld, a positive amount
17. FeesUSD Decimal, cumulative trade fees paid, for information: already inside cost and proceeds
18. TotalReturnUSD *Decimal, unrealized plus realized plus dividends minus tax, nil when unrealized is nil

Total return is a position total, not a time-weighted or money-weighted return.

## ClosedPosition

1. InstrumentID int
2. Symbol string
3. Exchange string
4. RealizedPnLUSD Decimal
5. DividendsUSD Decimal
6. TaxUSD Decimal
7. FeesUSD Decimal
8. TotalReturnUSD Decimal, realized plus dividends minus tax

A fully sold instrument stays visible with what it earned. Never needs a price.

## PnLTotals

1. CostUSD Decimal
2. UnrealizedPnLUSD *Decimal, nil when the portfolio is incomplete
3. RealizedPnLUSD Decimal
4. DividendsUSD Decimal
5. TaxUSD Decimal
6. FeesUSD Decimal
7. TotalReturnUSD *Decimal, nil when the portfolio is incomplete

Sums over every open and closed position. The figures that need a price are withheld when any holding lacks a usable price, as the total is, because a sum over only some holdings would mislead.
## Portfolio

1. Date Date, the reference session date
2. SecuritiesValueUSD *Decimal, nil when incomplete
3. Complete bool
4. Holdings []Holding
5. IntegrityErrors []IntegrityError
6. Cash CashBalance, as of the reference date
7. AccountEquityUSD *Decimal, securities value plus cash balance; nil when the portfolio is incomplete or cash is not initialized
8. ClosedPositions []ClosedPosition, ordered by symbol then exchange
9. PnL PnLTotals

A portfolio is incomplete when any active holding has no usable price. The total and allocations are then withheld, because a share computed over only some holdings would mislead. Allocation stays over securities only; cash and equity sit beside the securities value and never replace it.
