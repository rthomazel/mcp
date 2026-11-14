# Types

## SnapshotIssue

1. InstrumentID int
2. Symbol string
3. Exchange string
4. PriceDate *Date, the latest stored price on or before the snapshot date, nil when none
5. Issue string, "missing" when no price exists, "stale" when the latest price is older than the snapshot date

An active instrument that blocks a snapshot.

## SnapshotResult

1. Date Date
2. Skipped bool, true when the date was already completed and nothing was done
3. Complete bool
4. Rows int, snapshot rows written
5. Issues []SnapshotIssue, empty when complete

Complete with zero rows means no active positions on the date.

## RebuildResult

1. From Date
2. To Date
3. Dates int, the number of dates rebuilt
4. Rows int, snapshot rows written

## IncompleteSnapshotError

1. Date Date, the first date that could not be valued
2. Issues []SnapshotIssue

Is(ErrSnapshotIncomplete) is true. Returned by a rebuild, which then changes nothing.

## SnapshotRow

1. Date Date
2. InstrumentID int
3. Symbol string
4. Exchange string
5. Quantity Decimal
6. ClosePrice Decimal
7. PriceDate Date
8. MarketValueUSD Decimal
9. DateTotalUSD Decimal, the total over every instrument snapshotted that date, whatever the filter
10. Allocation Decimal, market value divided by the date total

One stored snapshot row with its date's total and the instrument's share. Both come from SQL.

## SnapshotFilter

1. InstrumentID *int
2. From *Date, inclusive
3. To *Date, inclusive
4. Limit int, zero means defaultListLimit, capped at maxListLimit

Selects snapshot rows.

# Functions

## SnapshotFilter.Validate() error

1. if a limit is negative or From is after To, return ErrInvalidInput.
2. Return nil.


## IncompleteSnapshotError.Error() string

Return a message naming the date and the symbols with issues.
