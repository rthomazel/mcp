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

# Functions

## IncompleteSnapshotError.Error() string

Return a message naming the date and the symbols with issues.
