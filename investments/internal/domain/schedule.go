package domain

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

// ClockTime is a time of day in the market timezone.
type ClockTime struct {
	Hour   int
	Minute int
}

// String is the time as HH:MM.
func (c ClockTime) String() string { return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute) }

// ParseClockTime reads a 24 hour HH:MM time of day.
func ParseClockTime(text string) (ClockTime, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(text))
	if err != nil {
		return ClockTime{}, fmt.Errorf("%w: %q is not a time of day written HH:MM", ErrInvalidInput, text)
	}

	return ClockTime{Hour: parsed.Hour(), Minute: parsed.Minute()}, nil
}

// ParseClockTimes reads a comma separated list of HH:MM times, returned
// ascending. An empty list or a repeated time is invalid.
func ParseClockTimes(text string) ([]ClockTime, error) {
	var times []ClockTime

	for part := range strings.SplitSeq(text, ",") {
		clock, err := ParseClockTime(part)
		if err != nil {
			return nil, err
		}

		times = append(times, clock)
	}

	slices.SortFunc(times, func(a, b ClockTime) int { return cmp.Or(cmp.Compare(a.Hour, b.Hour), cmp.Compare(a.Minute, b.Minute)) })

	for i := 1; i < len(times); i++ {
		if times[i] == times[i-1] {
			return nil, fmt.Errorf("%w: the time %s is listed more than once", ErrInvalidInput, times[i])
		}
	}

	return times, nil
}

// ScheduledTask is work that runs daily at fixed times of day.
type ScheduledTask struct {
	Name    string
	At      []ClockTime // ascending
	Timeout time.Duration
	Run     func(ctx context.Context) error
}

// Scheduler runs each task at its times of day in the market timezone. It
// never uses the host's local zone.
type Scheduler struct {
	tasks    []ScheduledTask
	location *time.Location
	clock    func() time.Time
}

// NewScheduler builds a Scheduler for the tasks in the given IANA timezone.
func NewScheduler(tasks []ScheduledTask, timezone string) (*Scheduler, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %s: %w", timezone, err)
	}

	return &Scheduler{tasks: tasks, location: location, clock: time.Now}, nil
}

// Run blocks until ctx is cancelled, with one loop for each task. A failing
// task never stops its loop or the others.
func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup

	for _, task := range s.tasks {
		wg.Go(func() { s.loop(ctx, task) })
	}

	wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, task ScheduledTask) {
	for {
		next := nextRun(s.clock(), s.location, task.At)
		slog.InfoContext(ctx, "next run scheduled", "task", task.Name, "at", next)

		timer := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			timer.Stop()
			slog.InfoContext(ctx, "scheduled task stopped", "task", task.Name)

			return
		case <-timer.C:
		}

		s.runOnce(ctx, task)
	}
}

func (s *Scheduler) runOnce(ctx context.Context, task ScheduledTask) {
	runCtx, cancel := context.WithTimeout(ctx, task.Timeout)
	defer cancel()

	started := s.clock()
	slog.InfoContext(ctx, "scheduled run starting", "task", task.Name)

	if err := task.Run(runCtx); err != nil {
		slog.ErrorContext(ctx, "scheduled run failed", "task", task.Name, "elapsed", time.Since(started), "err", err)

		return
	}

	slog.InfoContext(ctx, "scheduled run finished", "task", task.Name, "elapsed", time.Since(started))
}

// nextRun returns the earliest of the times of day that is strictly after now,
// today or tomorrow in location. It is built from the calendar date and wall
// clock, so daylight saving changes keep the same local times.
func nextRun(now time.Time, location *time.Location, at []ClockTime) time.Time {
	local := now.In(location)
	year, month, day := local.Date()

	for offset := range 2 {
		for _, clockTime := range at {
			candidate := time.Date(year, month, day+offset, clockTime.Hour, clockTime.Minute, 0, 0, location)
			if candidate.After(now) {
				return candidate
			}
		}
	}

	// Unreachable for a non-empty list: the next day always has a time after now.
	return time.Date(year, month, day+2, 0, 0, 0, 0, location)
}
