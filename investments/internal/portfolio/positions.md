# Constants

positionsSQL = per instrument, sum quantity_change over transactions whose effective_date is on or before the as-of date (all dates when none), keep sums above zero, join instruments for symbol and exchange, order by symbol then exchange.
integritySQL = per instrument, order the ledger restricted to the as-of date by effective date, execution time, id; compute the running sum of quantity_change; return the first row at which the running sum is below zero, with its effective date and running sum.

# Types

## Store

1. pool *pgxpool.Pool
2. stalePriceDays int

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

1. AsOf *Date, nil means the whole ledger
2. Positions []Position, active positions only
3. IntegrityErrors []IntegrityError

# Functions

## NewStore(pool *pgxpool.Pool, stalePriceDays int) *Store

Build a Store around the pool and the stale price threshold.

## Store.GetPositions(ctx Context, asOf *Date) (PositionsResult, error)

1. Select active positions via positionsSQL.
2. Select ledger integrity errors via integritySQL.
3. Return both with the as-of date.

#### Errors

- **1.** if the query fails, return the wrapped error.

---

- **2.** if the query fails, return the wrapped error.

An instrument whose quantity is zero after a full sale is not a position and not an error.
An instrument whose ledger ever goes negative is reported as an integrity error even if its final quantity is back to positive; an instrument whose final quantity is negative is not a position but still appears as an integrity error, so it is never silently omitted.
Negative balances can only come from administrative database edits, since writes are validated.
Sums are computed by Postgres, so the quantity strings keep full NUMERIC precision.
