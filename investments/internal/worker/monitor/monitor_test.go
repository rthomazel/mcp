package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func newMonitor(store *fakeStore) *SnapshotIncomplete { return NewSnapshotIncomplete(store, store) }

func TestTaskRunsTheMonitorAtItsTime(t *testing.T) {
	monitor := newMonitor(&fakeStore{})
	task := Task(monitor, time.Minute)

	assert.Equal(t, []domain.ClockTime{{Hour: 7}}, task.At)
	assert.Equal(t, time.Minute, task.Timeout)
	assert.Contains(t, task.Name, domain.Events.SnapshotIncomplete)
	assert.NoError(t, task.Run(context.Background()))
}

type fakeStore struct {
	domain.Store
	domain.EventStore

	status    domain.Status
	statusErr error
	recorded  []domain.DraftEvent
	created   bool
}

func (s *fakeStore) GetStatus(context.Context) (domain.Status, error) { return s.status, s.statusErr }

func (s *fakeStore) Record(_ context.Context, draft domain.DraftEvent) (bool, error) {
	s.recorded = append(s.recorded, draft)

	return s.created, nil
}

func TestSnapshotIncompleteRecordsTheDateOnce(t *testing.T) {
	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{status: domain.Status{LatestPriceDate: &date, SnapshotMissing: true}, created: true}
	monitor := newMonitor(store)

	require.NoError(t, monitor.Run(context.Background()))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.SnapshotIncomplete, store.recorded[0].Type)
	assert.Equal(t, "2026-01-05", *store.recorded[0].Subject)

	var data domain.SnapshotIncompleteData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, date, data.Date)

	assert.Equal(t, domain.ClockTime{Hour: 7}, monitor.At())
	assert.Equal(t, domain.Events.SnapshotIncomplete, monitor.Name())
}

func TestSnapshotIncompleteRecordsNothingWhenResolved(t *testing.T) {
	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	for name, status := range map[string]domain.Status{
		"no prices":      {},
		"has a snapshot": {LatestPriceDate: &date},
	} {
		store := &fakeStore{status: status}
		require.NoError(t, newMonitor(store).Run(context.Background()), name)
		assert.Empty(t, store.recorded, name)
	}
}

func TestSnapshotIncompleteReturnsStatusErrors(t *testing.T) {
	store := &fakeStore{statusErr: errors.New("db down")}

	assert.Error(t, newMonitor(store).Run(context.Background()))
}
