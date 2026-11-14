package data

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
)

func newYork(t *testing.T) *time.Location {
	t.Helper()

	location, err := time.LoadLocation(marketTimezone)
	require.NoError(t, err)

	return location
}

func TestNextRun(t *testing.T) {
	ny := newYork(t)

	tests := map[string]struct {
		now  time.Time
		want time.Time
	}{
		"before morning":  {time.Date(2026, 11, 14, 1, 0, 0, 0, ny), time.Date(2026, 11, 14, 6, 0, 0, 0, ny)},
		"between runs":    {time.Date(2026, 11, 14, 12, 0, 0, 0, ny), time.Date(2026, 11, 14, 20, 30, 0, 0, ny)},
		"after evening":   {time.Date(2026, 11, 14, 21, 0, 0, 0, ny), time.Date(2026, 11, 15, 6, 0, 0, 0, ny)},
		"exactly on time": {time.Date(2026, 11, 14, 6, 0, 0, 0, ny), time.Date(2026, 11, 14, 20, 30, 0, 0, ny)},
		// Spring forward on 2026-03-08: 20:30 EDT is 00:30Z, not 01:30Z.
		"across spring forward": {time.Date(2026, 3, 7, 21, 0, 0, 0, ny), time.Date(2026, 3, 8, 6, 0, 0, 0, ny)},
		// Fall back on 2026-11-01.
		"across fall back": {time.Date(2026, 10, 31, 21, 0, 0, 0, ny), time.Date(2026, 11, 1, 6, 0, 0, 0, ny)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := nextRun(test.now, ny)

			assert.True(t, test.want.Equal(got), "want %s got %s", test.want, got)
			assert.Equal(t, test.want.Hour(), got.In(ny).Hour())
		})
	}
}

// The host zone, such as Brasilia time, must not change the run times.
func TestNextRunIgnoresHostZone(t *testing.T) {
	ny := newYork(t)
	instant := time.Date(2026, 11, 14, 12, 0, 0, 0, ny)
	want := time.Date(2026, 11, 14, 20, 30, 0, 0, ny)

	brasilia, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)

	previous := time.Local
	time.Local = brasilia

	t.Cleanup(func() { time.Local = previous })

	assert.True(t, want.Equal(nextRun(instant.In(brasilia), ny)))
	assert.True(t, want.Equal(nextRun(instant, ny)))
}

func TestRunTimesDescription(t *testing.T) {
	assert.Equal(t, "06:00, 20:30 America/New_York", RunTimesDescription())
}

func TestNewSchedulerUsesTheMarketZone(t *testing.T) {
	scheduler, err := NewScheduler(nil)

	require.NoError(t, err)
	assert.Equal(t, marketTimezone, scheduler.location.String())
}

func TestSchedulerStopsWhenCancelled(t *testing.T) {
	scheduler, err := NewScheduler(nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})

	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}

// signalStore reports when the daily job reaches the snapshot step.
type signalStore struct {
	*fakeStore

	reached chan struct{}
	once    sync.Once
}

func (s *signalStore) WriteSnapshot(ctx context.Context, date domain.Date) (domain.SnapshotResult, error) {
	defer s.once.Do(func() { close(s.reached) })

	return s.fakeStore.WriteSnapshot(ctx, date)
}

// secondBeforeRun returns a zone in which the next run time is within a second.
func secondBeforeRun(now time.Time) *time.Location {
	const day = 24 * 60 * 60

	runSeconds := runTimes[0] * 60
	offset := ((runSeconds-1-int(now.Unix()%day))%day + day) % day

	return time.FixedZone("test", offset)
}

func TestSchedulerFiresTheJobAndKeepsRunning(t *testing.T) {
	fake := &fakeStore{instruments: twoInstruments(), latest: new(day(2)), snapshot: domain.SnapshotResult{Date: day(2), Complete: true}}
	store := &signalStore{fakeStore: fake, reached: make(chan struct{})}
	job := NewJob(store, marketdata.NewRefresher(store, &fakeProvider{}), "fake")

	scheduler := &Scheduler{job: job, location: secondBeforeRun(time.Now()), clock: time.Now}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	select {
	case <-store.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the job did not run at the scheduled time")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop")
	}

	assert.Equal(t, []domain.Date{day(2)}, fake.snapshotted)
}

func TestRunOnceSurvivesAFailedRun(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}
	scheduler := &Scheduler{job: newJob(store, &fakeProvider{}), location: time.UTC, clock: time.Now}

	assert.NotPanics(t, func() { scheduler.runOnce(context.Background()) })
	assert.Empty(t, store.snapshotted)
}
