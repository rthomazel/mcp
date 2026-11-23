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
	"github.com/rthomazel/mcp/investments/internal/duration"
)

func newMonitor(store *fakeStore) *SnapshotIncomplete { return NewSnapshotIncomplete(store, store) }

type fakeStore struct {
	domain.GenericStore
	domain.EventStore

	status    domain.Status
	statusErr error
	recorded  []domain.DraftEvent
	created   bool

	drawdown    domain.DrawdownState
	drawdownErr error
	increment   domain.Decimal
	recordErr   error

	high        *domain.HighState
	movement    *domain.MarketMovement
	movementErr error
	window      duration.Duration
	threshold   domain.Decimal
	latest      *domain.Event
}

func (s *fakeStore) GetHighState(context.Context) (*domain.HighState, error) { return s.high, nil }

func (s *fakeStore) GetMarketMovement(_ context.Context, window duration.Duration, threshold domain.Decimal) (*domain.MarketMovement, error) {
	s.window, s.threshold = window, threshold

	return s.movement, s.movementErr
}

func (s *fakeStore) Latest(context.Context, string) (*domain.Event, error) { return s.latest, nil }

func (s *fakeStore) GetDrawdownState(_ context.Context, increment domain.Decimal) (domain.DrawdownState, error) {
	s.increment = increment

	return s.drawdown, s.drawdownErr
}

func (s *fakeStore) GetStatus(context.Context) (domain.Status, error) { return s.status, s.statusErr }

func (s *fakeStore) Record(_ context.Context, draft domain.DraftEvent) (bool, error) {
	s.recorded = append(s.recorded, draft)

	return s.created, s.recordErr
}

func TestSnapshotIncompleteRecordsTheDateOnce(t *testing.T) {
	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	store := &fakeStore{status: domain.Status{LatestPriceDate: &date, SnapshotMissing: true}, created: true}
	monitor := newMonitor(store)

	require.NoError(t, monitor.Run(context.Background(), nil))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, domain.Events.SnapshotIncomplete, store.recorded[0].Type)
	assert.Equal(t, "2026-01-05", *store.recorded[0].Subject)

	var data domain.SnapshotIncompleteData
	require.NoError(t, json.Unmarshal(store.recorded[0].Data, &data))
	assert.Equal(t, date, data.Date)

	assert.Equal(t, domain.ClockTime{Hour: 7}, monitor.At())
	assert.Equal(t, "snapshot.incomplete", monitor.Name())
}

func TestSnapshotIncompleteRecordsNothingWhenResolved(t *testing.T) {
	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	for name, status := range map[string]domain.Status{
		"no prices":      {},
		"has a snapshot": {LatestPriceDate: &date},
	} {
		store := &fakeStore{status: status}
		require.NoError(t, newMonitor(store).Run(context.Background(), nil), name)
		assert.Empty(t, store.recorded, name)
	}
}

func TestSnapshotIncompleteReturnsStatusErrors(t *testing.T) {
	store := &fakeStore{statusErr: errors.New("db down")}

	assert.Error(t, newMonitor(store).Run(context.Background(), nil))
}

func TestSnapshotIncompleteTakesNoSettings(t *testing.T) {
	monitor := newMonitor(&fakeStore{})

	assert.NoError(t, monitor.ValidateParams(nil))
	assert.ErrorContains(t, monitor.ValidateParams(Params{"b": "1", "a": "2"}), `"a"`)
}
