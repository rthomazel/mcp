# Constants

UpdateDataPrices = "prices"
UpdateDataSnapshots = "snapshots"
UpdateDataSnapshotRebuild = "snapshot_rebuild"
UpdateStatusCompleted = "completed"
UpdateStatusIncomplete = "incomplete"
UpdateStatusFailed = "failed"

The running status exists in the schema but is not used: an outcome is recorded once, when the run ends.

# Types

## Update

1. ID int64, zero for an update not yet stored
2. Provider *string, nil for runs that call no provider
3. DataType string, one of the UpdateData constants
4. StartedAt Time
5. CompletedAt Time
6. Status string, one of the UpdateStatus constants
7. RecordsUpdated int
8. ErrorMessage string, empty when none
9. DateFrom *Date
10. DateTo *Date
11. Reason string, empty except for rebuilds

One row of the updates log. A snapshot run records DateFrom and DateTo as the same date; a completed one is the idempotency marker for that date. A rebuild records its range and reason and also counts as completed for every date in the range.

## UpdateFilter

1. DataType string, empty for any
2. Status string, empty for any
3. Limit int, zero means defaultListLimit, capped at maxListLimit

Selects updates log rows.

# Functions

## Update.Validate() error

1. if DataType or Status is not one of the constants, return ErrInvalidInput.
2. if CompletedAt is before StartedAt, return ErrInvalidInput.
3. if DataType is UpdateDataSnapshotRebuild and DateFrom, DateTo or Reason is missing, return ErrInvalidInput.
4. Return nil.

Mirrors the updates table checks.

## UpdateFilter.Validate() error

1. if DataType or Status is set and not one of its constants, or Limit is negative, return ErrInvalidInput.
2. Return nil.

## UpdateFilter.ResolvedLimit() int

Return Limit with the default applied when zero and the cap applied.
