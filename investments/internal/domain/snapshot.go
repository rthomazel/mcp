package domain

import (
	"fmt"
	"strings"
	"time"
)

// SnapshotIssue is an active instrument that blocks a snapshot.
type SnapshotIssue struct {
	InstrumentID int
	Symbol       string
	Exchange     string
	PriceDate    *Date  // latest stored price on or before the snapshot date, nil when none
	Issue        string // PriceIssueMissing, or PriceIssueStale when the latest price is older than the date
}

// SnapshotResult is the outcome of writing one date's snapshot.
type SnapshotResult struct {
	Date     Date
	Skipped  bool // the date was already completed and nothing was done
	Complete bool
	Rows     int // complete with zero rows means no active positions
	Issues   []SnapshotIssue
}

// SnapshotRow is one stored snapshot row with its date's total and its share.
type SnapshotRow struct {
	Date           Date
	InstrumentID   int
	Symbol         string
	Exchange       string
	Quantity       Decimal
	ClosePrice     Decimal
	PriceDate      Date
	MarketValueUSD Decimal
	DateTotalUSD   Decimal // over every instrument snapshotted that date, whatever the filter
	Allocation     Decimal // market value divided by the date total
}

// SnapshotFilter selects snapshot rows.
type SnapshotFilter struct {
	InstrumentID *int
	From         *Date // inclusive
	To           *Date // inclusive
	Limit        int   // zero means defaultListLimit, capped at maxListLimit
}

// Validate checks the limit and the range.
func (f SnapshotFilter) Validate() error { return validateListFilter(f.Limit, f.From, f.To) }

// ResolvedLimit applies the default and the cap to Limit.
func (f SnapshotFilter) ResolvedLimit() int { return resolveLimit(f.Limit) }

// RebuildResult is the outcome of a snapshot rebuild.
type RebuildResult struct {
	From  Date
	To    Date
	Dates int
	Rows  int
}

// IncompleteSnapshotError reports the first date a rebuild could not value.
type IncompleteSnapshotError struct {
	Date   Date
	Issues []SnapshotIssue
}

func (e *IncompleteSnapshotError) Error() string {
	return fmt.Sprintf("%s: %s: %s", ErrSnapshotIncomplete, e.Date.Format(time.DateOnly), DescribeIssues(e.Issues))
}

// Is reports a match with ErrSnapshotIncomplete.
func (e *IncompleteSnapshotError) Is(target error) bool {
	return target == ErrSnapshotIncomplete
}

// DescribeIssues renders issues as "SYMBOL missing, SYMBOL stale".
func DescribeIssues(issues []SnapshotIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		parts = append(parts, issue.Symbol+" "+issue.Issue)
	}

	return strings.Join(parts, ", ")
}
