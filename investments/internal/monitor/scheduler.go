package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	marketTimezone = "America/New_York"
	monitorTimeout = 2 * time.Minute
)

// ClockTime is a time of day in the market timezone.
type ClockTime struct {
	Hour   int
	Minute int
}

// Monitor judges a condition on the current state and records an event when
// it holds, at most once for each subject.
type Monitor interface {
	Name() string
	At() ClockTime
	Run(ctx context.Context) error
}

// Scheduler runs each monitor daily at its time of day.
type Scheduler struct {
	monitors []Monitor
	location *time.Location
	clock    func() time.Time
}

// NewScheduler builds a Scheduler in the market timezone.
func NewScheduler(monitors []Monitor) (*Scheduler, error) {
	location, err := time.LoadLocation(marketTimezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %s: %w", marketTimezone, err)
	}

	return &Scheduler{monitors: monitors, location: location, clock: time.Now}, nil
}

// Run blocks until ctx is cancelled, with one loop for each monitor. A failing
// monitor never stops its loop or the others.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for _, monitor := range s.monitors {
		wg.Add(1)

		go func() {
			defer wg.Done()

			s.loop(ctx, monitor)
		}()
	}

	wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, monitor Monitor) {
	for {
		next := nextRun(s.clock(), s.location, monitor.At())
		slog.InfoContext(ctx, "next monitor run scheduled", "monitor", monitor.Name(), "at", next)

		timer := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			timer.Stop()
			slog.InfoContext(ctx, "monitor stopped", "monitor", monitor.Name())

			return
		case <-timer.C:
		}

		s.runOnce(ctx, monitor)
	}
}

func (s *Scheduler) runOnce(ctx context.Context, monitor Monitor) {
	ctx, cancel := context.WithTimeout(ctx, monitorTimeout)
	defer cancel()

	if err := monitor.Run(ctx); err != nil {
		slog.ErrorContext(ctx, "monitor failed", "monitor", monitor.Name(), "err", err)
	}
}

// nextRun returns the earliest occurrence of at strictly after now, built
// from the calendar date and wall clock so daylight saving keeps the local time.
func nextRun(now time.Time, location *time.Location, at ClockTime) time.Time {
	local := now.In(location)
	year, month, day := local.Date()

	for offset := range 2 {
		candidate := time.Date(year, month, day+offset, at.Hour, at.Minute, 0, 0, location)
		if candidate.After(now) {
			return candidate
		}
	}

	// Unreachable: the next day's occurrence is always after now.
	return time.Date(year, month, day+2, at.Hour, at.Minute, 0, 0, location)
}
