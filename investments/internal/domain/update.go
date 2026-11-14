package domain

import (
	"fmt"
	"time"
)

// Update data types and statuses. The running status exists in the schema but
// is unused: an outcome is recorded once, when the run ends.
const (
	UpdateDataPrices          = "prices"
	UpdateDataSnapshots       = "snapshots"
	UpdateDataSnapshotRebuild = "snapshot_rebuild"

	UpdateStatusCompleted  = "completed"
	UpdateStatusIncomplete = "incomplete"
	UpdateStatusFailed     = "failed"
)

// Update is one row of the updates log. A snapshot run records DateFrom and
// DateTo as the same date; a completed one is the idempotency marker for that
// date. A rebuild records its range and reason.
type Update struct {
	Provider       *string // nil for runs that call no provider
	DataType       string
	StartedAt      time.Time
	CompletedAt    time.Time
	Status         string
	RecordsUpdated int
	ErrorMessage   string
	DateFrom       *Date
	DateTo         *Date
	Reason         string
}

// Validate mirrors the updates table checks.
func (u Update) Validate() error {
	switch u.DataType {
	case UpdateDataPrices, UpdateDataSnapshots, UpdateDataSnapshotRebuild:
	default:
		return fmt.Errorf("%w: unknown update data type %q", ErrInvalidInput, u.DataType)
	}

	switch u.Status {
	case UpdateStatusCompleted, UpdateStatusIncomplete, UpdateStatusFailed:
	default:
		return fmt.Errorf("%w: unknown update status %q", ErrInvalidInput, u.Status)
	}

	if u.CompletedAt.Before(u.StartedAt) {
		return fmt.Errorf("%w: update completed before it started", ErrInvalidInput)
	}

	if u.DataType == UpdateDataSnapshotRebuild && (u.DateFrom == nil || u.DateTo == nil || u.Reason == "") {
		return fmt.Errorf("%w: a rebuild needs a date range and a reason", ErrInvalidInput)
	}

	return nil
}
