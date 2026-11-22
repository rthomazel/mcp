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

func newYork(t *testing.T) *time.Location {
	t.Helper()

	location, err := time.LoadLocation(marketTimezone)
	require.NoError(t, err)

	return location
}

func TestNextRunIsStrictlyAfterNowInMarketTime(t *testing.T) {
	location := newYork(t)
	at := ClockTime{Hour: 7}

	before := time.Date(2026, 3, 1, 6, 0, 0, 0, location)
	assert.Equal(t, time.Date(2026, 3, 1, 7, 0, 0, 0, location), nextRun(before, location, at))

	exactly := time.Date(2026, 3, 1, 7, 0, 0, 0, location)
	assert.Equal(t, time.Date(2026, 3, 2, 7, 0, 0, 0, location), nextRun(exactly, location, at))
}

func TestNextRunKeepsLocalTimeAcrossDaylightSaving(t *testing.T) {
	location := newYork(t)
	// Clocks go forward on 2026-03-08.
	now := time.Date(2026, 3, 7, 8, 0, 0, 0, location)
	next := nextRun(now, location, ClockTime{Hour: 7})

	assert.Equal(t, 7, next.In(location).Hour())
	assert.Equal(t, 8, next.In(location).Day())
	assert.Equal(t, 22, int(next.Sub(now).Hours())) // the day is an hour shorter
}

type fakeStore struct {
	domain.Store

	status    domain.Status
	statusErr error
	recorded  []domain.DraftEvent
	created   bool
}

func (s *fakeStore) GetStatus(context.Context) (domain.Status, error) { return s.status, s.statusErr }

func (s *fakeStore) RecordEvent(_ context.Context, draft domain.DraftEvent) (bool, error) {
	s.recorded = append(s.recorded, draft)

	return s.created, nil
}

func TestSnapshotIncompleteRecordsTheDateOnce(t *testing.T) {
	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{status: domain.Status{LatestPriceDate: &date, SnapshotMissing: true}, created: true}
	monitor := NewSnapshotIncomplete(store)

	require.NoError(t, monitor.Run(context.Background()))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.SnapshotIncomplete, store.recorded[0].Type)
	assert.Equal(t, "2026-01-05", *store.recorded[0].Subject)

	var data domain.SnapshotIncompleteData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, date, data.Date)

	assert.Equal(t, ClockTime{Hour: 7}, monitor.At())
	assert.Equal(t, domain.Events.SnapshotIncomplete, monitor.Name())
}

func TestSnapshotIncompleteRecordsNothingWhenResolved(t *testing.T) {
	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	for name, status := range map[string]domain.Status{
		"no prices":      {},
		"has a snapshot": {LatestPriceDate: &date},
	} {
		store := &fakeStore{status: status}
		require.NoError(t, NewSnapshotIncomplete(store).Run(context.Background()), name)
		assert.Empty(t, store.recorded, name)
	}
}

func TestSnapshotIncompleteReturnsStatusErrors(t *testing.T) {
	store := &fakeStore{statusErr: errors.New("db down")}

	assert.Error(t, NewSnapshotIncomplete(store).Run(context.Background()))
}
