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

1. Provider *string, nil for runs that call no provider
2. DataType string, one of the UpdateData constants
3. StartedAt Time
4. CompletedAt Time
5. Status string, one of the UpdateStatus constants
6. RecordsUpdated int
7. ErrorMessage string, empty when none
8. DateFrom *Date
9. DateTo *Date
10. Reason string, empty except for rebuilds

One row of the updates log. A snapshot run records DateFrom and DateTo as the same date; a completed one is the idempotency marker for that date. A rebuild records its range and reason and also counts as completed for every date in the range.

# Functions

## Update.Validate() error

1. if DataType or Status is not one of the constants, return ErrInvalidInput.
2. if CompletedAt is before StartedAt, return ErrInvalidInput.
3. if DataType is UpdateDataSnapshotRebuild and DateFrom, DateTo or Reason is missing, return ErrInvalidInput.
4. Return nil.

Mirrors the updates table checks.
