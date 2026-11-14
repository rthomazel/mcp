# Types

## Holding

1. InstrumentID int
2. Symbol string
3. Exchange string
4. Quantity Decimal
5. ClosePrice *Decimal, nil when no usable price
6. PriceDate *Date
7. MarketValueUSD *Decimal, nil when no usable price
8. Allocation *Decimal, nil when the portfolio is incomplete
9. PriceIssue string, empty when usable, otherwise "missing" or "stale"

## Portfolio

1. Date Date, the reference session date
2. SecuritiesValueUSD *Decimal, nil when incomplete
3. Complete bool
4. Holdings []Holding
5. IntegrityErrors []IntegrityError

A portfolio is incomplete when any active holding has no usable price. The total and allocations are then withheld, because a share computed over only some holdings would mislead.
