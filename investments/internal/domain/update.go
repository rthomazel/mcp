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
	ID             int64   // zero for an update not yet stored
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

// UpdateFilter selects updates log rows.
type UpdateFilter struct {
	DataType string // empty for any
	Status   string // empty for any
	Limit    int    // zero means defaultListLimit, capped at maxListLimit
}

// Validate checks the data type, status and limit.
func (f UpdateFilter) Validate() error {
	if f.DataType != "" && !validUpdateDataType(f.DataType) {
		return fmt.Errorf("%w: unknown update data type %q", ErrInvalidInput, f.DataType)
	}

	if f.Status != "" && !validUpdateStatus(f.Status) {
		return fmt.Errorf("%w: unknown update status %q", ErrInvalidInput, f.Status)
	}

	return validateListFilter(f.Limit, nil, nil)
}

// ResolvedLimit applies the default and the cap to Limit.
func (f UpdateFilter) ResolvedLimit() int { return resolveLimit(f.Limit) }

func validUpdateDataType(value string) bool {
	return value == UpdateDataPrices || value == UpdateDataSnapshots || value == UpdateDataSnapshotRebuild
}

func validUpdateStatus(value string) bool {
	return value == UpdateStatusCompleted || value == UpdateStatusIncomplete || value == UpdateStatusFailed
}

// Validate mirrors the updates table checks.
func (u Update) Validate() error {
	if !validUpdateDataType(u.DataType) {
		return fmt.Errorf("%w: unknown update data type %q", ErrInvalidInput, u.DataType)
	}

	if !validUpdateStatus(u.Status) {
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
