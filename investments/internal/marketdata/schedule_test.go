package marketdata

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func TestRunTimesDescription(t *testing.T) {
	assert.Equal(t, "06:00, 20:30 America/New_York", RunTimesDescription("America/New_York"))
}

func TestDailyTaskRunsTheJob(t *testing.T) {
	fake := &jobStore{instruments: twoInstruments(), latest: new(day(2)), snapshot: domain.SnapshotResult{Date: day(2), Complete: true}}
	task := DailyTask(newJob(fake, &jobProvider{}))

	assert.Equal(t, runTimes, task.At)
	assert.Equal(t, runTimeout, task.Timeout)
	require.NoError(t, task.Run(context.Background()))
	assert.Equal(t, []domain.Date{day(2)}, fake.snapshotted)
}

func TestDailyTaskReturnsTheJobError(t *testing.T) {
	store := &jobStore{instruments: twoInstruments(), recordErr: errors.New("db down")}

	assert.Error(t, DailyTask(newJob(store, &jobProvider{})).Run(context.Background()))
	assert.Empty(t, store.snapshotted)
}
