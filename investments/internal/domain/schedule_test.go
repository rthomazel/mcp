package domain

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newYork(t *testing.T) *time.Location {
	t.Helper()

	location, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	return location
}

var twoTimes = []ClockTime{{Hour: 6}, {Hour: 20, Minute: 30}}

func TestNextRun(t *testing.T) {
	ny := newYork(t)

	tests := map[string]struct {
		now  time.Time
		at   []ClockTime
		want time.Time
	}{
		"before morning":  {time.Date(2026, 11, 14, 1, 0, 0, 0, ny), twoTimes, time.Date(2026, 11, 14, 6, 0, 0, 0, ny)},
		"between runs":    {time.Date(2026, 11, 14, 12, 0, 0, 0, ny), twoTimes, time.Date(2026, 11, 14, 20, 30, 0, 0, ny)},
		"after evening":   {time.Date(2026, 11, 14, 21, 0, 0, 0, ny), twoTimes, time.Date(2026, 11, 15, 6, 0, 0, 0, ny)},
		"exactly on time": {time.Date(2026, 11, 14, 6, 0, 0, 0, ny), twoTimes, time.Date(2026, 11, 14, 20, 30, 0, 0, ny)},
		"a single time":   {time.Date(2026, 11, 14, 7, 0, 0, 0, ny), []ClockTime{{Hour: 7}}, time.Date(2026, 11, 15, 7, 0, 0, 0, ny)},
		// Spring forward on 2026-03-08: 20:30 EDT is 00:30Z, not 01:30Z.
		"across spring forward": {time.Date(2026, 3, 7, 21, 0, 0, 0, ny), twoTimes, time.Date(2026, 3, 8, 6, 0, 0, 0, ny)},
		// Fall back on 2026-11-01.
		"across fall back": {time.Date(2026, 10, 31, 21, 0, 0, 0, ny), twoTimes, time.Date(2026, 11, 1, 6, 0, 0, 0, ny)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got := nextRun(test.now, ny, test.at)

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

	assert.True(t, want.Equal(nextRun(instant.In(brasilia), ny, twoTimes)))
	assert.True(t, want.Equal(nextRun(instant, ny, twoTimes)))
}

func TestNewSchedulerUsesTheGivenZone(t *testing.T) {
	scheduler, err := NewScheduler(nil, "America/New_York")
	require.NoError(t, err)
	assert.Equal(t, "America/New_York", scheduler.location.String())

	_, err = NewScheduler(nil, "Nowhere/Land")
	assert.Error(t, err)
}

func TestSchedulerStopsWhenCancelled(t *testing.T) {
	scheduler, err := NewScheduler([]ScheduledTask{{Name: "x", At: twoTimes, Timeout: time.Second, Run: func(context.Context) error { return nil }}}, "America/New_York")
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

// secondBeforeRun returns a zone in which the 06:00 run is within a second.
func secondBeforeRun(now time.Time) *time.Location {
	const day = 24 * 60 * 60

	offset := ((6*60*60-1-int(now.Unix()%day))%day + day) % day

	return time.FixedZone("test", offset)
}

func TestSchedulerRunsEachTaskAndSurvivesAFailure(t *testing.T) {
	var failing, healthy atomic.Int32

	tasks := []ScheduledTask{
		{Name: "failing", At: []ClockTime{{Hour: 6}}, Timeout: time.Second, Run: func(context.Context) error {
			failing.Add(1)

			return errors.New("boom")
		}},
		{Name: "healthy", At: []ClockTime{{Hour: 6}}, Timeout: time.Second, Run: func(context.Context) error {
			healthy.Add(1)

			return nil
		}},
	}

	scheduler := &Scheduler{tasks: tasks, location: secondBeforeRun(time.Now()), clock: time.Now}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool { return failing.Load() == 1 && healthy.Load() == 1 }, 10*time.Second, 50*time.Millisecond)

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func TestRunOnceGivesTheTaskItsTimeout(t *testing.T) {
	scheduler := &Scheduler{location: time.UTC, clock: time.Now}

	var deadlineSet bool

	scheduler.runOnce(context.Background(), ScheduledTask{Name: "x", Timeout: time.Minute, Run: func(ctx context.Context) error {
		_, deadlineSet = ctx.Deadline()

		return nil
	}})

	assert.True(t, deadlineSet)
}
