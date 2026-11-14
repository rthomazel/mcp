package data

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	marketTimezone = "America/New_York"
	runTimeout     = 30 * time.Minute
)

// runTimes are the daily run times as minutes after local midnight, ascending:
// 06:00 picks up mutual funds, 20:30 follows the equity close and corrections.
var runTimes = []int{6 * 60, 20*60 + 30}

// RunTimesDescription renders the run times for the startup log.
func RunTimesDescription() string {
	parts := make([]string, 0, len(runTimes))
	for _, minutes := range runTimes {
		parts = append(parts, fmt.Sprintf("%02d:%02d", minutes/60, minutes%60))
	}

	return strings.Join(parts, ", ") + " " + marketTimezone
}

// Scheduler runs the job at the daily run times in the market timezone. It
// never uses the host's local zone.
type Scheduler struct {
	job      *Job
	location *time.Location
	clock    func() time.Time
}

// NewScheduler builds a Scheduler in the market timezone.
func NewScheduler(job *Job) (*Scheduler, error) {
	location, err := time.LoadLocation(marketTimezone)
	if err != nil {
		return nil, fmt.Errorf("loading timezone %s: %w", marketTimezone, err)
	}

	return &Scheduler{job: job, location: location, clock: time.Now}, nil
}

// nextRun returns the earliest run time strictly after now. It is built from
// the calendar date and wall clock in location, so daylight saving changes
// keep the same local times.
func nextRun(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	year, month, day := local.Date()

	for offset := range 2 {
		for _, minutes := range runTimes {
			candidate := time.Date(year, month, day+offset, minutes/60, minutes%60, 0, 0, location)
			if candidate.After(now) {
				return candidate
			}
		}
	}

	// Unreachable: the next day always has a run time after now.
	return time.Date(year, month, day+2, 0, 0, 0, 0, location)
}

// Run blocks until ctx is cancelled. A failed run never stops the loop.
func (s *Scheduler) Run(ctx context.Context) {
	for {
		next := nextRun(s.clock(), s.location)
		slog.InfoContext(ctx, "next daily run scheduled", "at", next)

		timer := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			timer.Stop()
			slog.InfoContext(ctx, "scheduler stopped")

			return
		case <-timer.C:
		}

		s.runOnce(ctx)
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	started := s.clock()
	slog.InfoContext(ctx, "daily run starting")

	report, err := s.job.Run(runCtx)
	if err != nil {
		slog.ErrorContext(ctx, "daily run failed", "elapsed", time.Since(started), "err", err)

		return
	}

	slog.InfoContext(ctx, "daily run finished", "elapsed", time.Since(started),
		"refreshed", report.Refreshed, "failed", report.Failed)
}
