# Types

## Position

1. InstrumentID int
2. Symbol string
3. Exchange string
4. Quantity Decimal

## IntegrityError

1. InstrumentID int
2. Symbol string
3. Exchange string
4. FirstNegativeDate Date
5. Quantity Decimal, the running quantity at that entry

## PositionsResult

1. Date *Date, nil means the whole ledger
2. Positions []Position, active positions only
3. IntegrityErrors []IntegrityError
