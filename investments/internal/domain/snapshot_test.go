package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdateValidate(t *testing.T) {
	from := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	valid := Update{DataType: UpdateDataSnapshots, Status: UpdateStatusCompleted, StartedAt: now, CompletedAt: now}

	tests := map[string]struct {
		mutate func(*Update)
		ok     bool
	}{
		"valid":                 {func(*Update) {}, true},
		"unknown data type":     {func(u *Update) { u.DataType = "other" }, false},
		"unknown status":        {func(u *Update) { u.Status = "running" }, false},
		"completed before":      {func(u *Update) { u.CompletedAt = now.Add(-time.Second) }, false},
		"rebuild without range": {func(u *Update) { u.DataType, u.Reason = UpdateDataSnapshotRebuild, "fix" }, false},
		"rebuild without reason": {func(u *Update) {
			u.DataType, u.DateFrom, u.DateTo = UpdateDataSnapshotRebuild, &from, &from
		}, false},
		"rebuild complete": {func(u *Update) {
			u.DataType, u.DateFrom, u.DateTo, u.Reason = UpdateDataSnapshotRebuild, &from, &from, "fix"
		}, true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			update := valid
			test.mutate(&update)

			err := update.Validate()
			if test.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrInvalidInput)
			}
		})
	}
}

func TestIncompleteSnapshotError(t *testing.T) {
	err := error(&IncompleteSnapshotError{
		Date:   time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Issues: []SnapshotIssue{{Symbol: "VTI", Issue: PriceIssueMissing}, {Symbol: "BND", Issue: PriceIssueStale}},
	})

	assert.True(t, errors.Is(err, ErrSnapshotIncomplete))
	assert.Equal(t, "snapshot incomplete: 2026-11-02: VTI missing, BND stale", err.Error())
}
