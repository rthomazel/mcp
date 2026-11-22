package marketdata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// Job is the daily routine, without scheduling.
type Job struct {
	prices    domain.PriceStore
	snapshots domain.SnapshotStore
	updates   domain.UpdateStore
	refresher *PriceRefresher
	provider  string
	clock     func() time.Time
}

// RunReport is the outcome of one run.
type RunReport struct {
	Refreshed int
	Failed    int
	Snapshot  *domain.SnapshotResult // nil when no price data exists yet
}

// NewJob builds a Job using the real clock.
func NewJob(prices domain.PriceStore, snapshots domain.SnapshotStore, updates domain.UpdateStore, refresher *PriceRefresher, provider string) *Job {
	return &Job{prices: prices, snapshots: snapshots, updates: updates, refresher: refresher, provider: provider, clock: time.Now}
}

// Run refreshes prices, logs the outcome, then snapshots the latest price
// date. A failed or rate limited refresh does not stop the snapshot: its date
// is judged only by the prices stored.
func (j *Job) Run(ctx context.Context) (RunReport, error) {
	var report RunReport

	started := j.clock()

	results, err := j.refresher.RefreshAll(ctx, nil)
	if err != nil && ctx.Err() != nil {
		slog.ErrorContext(ctx, "price refresh run cancelled", "step", "refreshing prices", "err", err)

		return report, err
	}

	update := j.pricesUpdate(started, results, err)
	for _, result := range results {
		switch {
		case result.Err == nil:
			report.Refreshed++
		case errors.Is(result.Err, domain.ErrRateLimited):
			report.Failed++
			slog.WarnContext(ctx, "provider rate limited", "symbol", result.Instrument.Symbol)
		default:
			report.Failed++
			slog.ErrorContext(ctx, "refreshing prices failed", "symbol", result.Instrument.Symbol, "err", result.Err)
		}
	}

	if err != nil {
		slog.ErrorContext(ctx, "price refresh run step failed", "step", "refreshing prices", "err", err)
	}

	if err := j.updates.Record(ctx, update); err != nil {
		slog.ErrorContext(ctx, "price refresh run step failed", "step", "recording prices update", "err", err)

		return report, fmt.Errorf("recording prices update: %w", err)
	}

	slog.InfoContext(ctx, "prices refreshed", "refreshed", report.Refreshed, "failed", report.Failed,
		"status", update.Status, "bars", update.RecordsUpdated)

	latest, err := j.prices.LatestSessionDate(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "price refresh run step failed", "step", "reading latest session date", "err", err)

		return report, fmt.Errorf("reading latest session date: %w", err)
	}

	if latest == nil {
		slog.InfoContext(ctx, "no price data yet, no snapshot")

		return report, nil
	}

	snapshot, err := j.snapshots.Write(ctx, *latest)
	if err != nil {
		slog.ErrorContext(ctx, "price refresh run step failed", "step", "writing snapshot", "err", err)

		return report, fmt.Errorf("writing snapshot: %w", err)
	}

	report.Snapshot = &snapshot

	switch {
	case snapshot.Skipped:
		slog.InfoContext(ctx, "snapshot skipped, date already completed", "date", latest.Format(time.DateOnly))
	case snapshot.Complete:
		slog.InfoContext(ctx, "snapshot written", "date", latest.Format(time.DateOnly), "rows", snapshot.Rows)
	default:
		slog.WarnContext(ctx, "snapshot incomplete", "date", latest.Format(time.DateOnly),
			"issues", domain.DescribeIssues(snapshot.Issues))
	}

	return report, nil
}

// pricesUpdate builds the prices log row: completed when every instrument
// succeeded, incomplete otherwise, failed when the instruments could not be
// listed. The message carries symbols only, never the credential.
func (j *Job) pricesUpdate(started time.Time, results []RefreshResult, listErr error) domain.Update {
	provider := j.provider
	update := domain.Update{
		Provider: &provider, DataType: domain.UpdateDataPrices, StartedAt: started,
		Status: domain.UpdateStatusCompleted,
	}

	var failed []string

	for _, result := range results {
		update.RecordsUpdated += result.Bars

		if result.Err != nil {
			failed = append(failed, result.Instrument.Symbol)
		}
	}

	switch {
	case listErr != nil:
		update.Status, update.ErrorMessage = domain.UpdateStatusFailed, "listing instruments failed"
	case len(failed) > 0:
		update.Status, update.ErrorMessage = domain.UpdateStatusIncomplete, "not refreshed: "+strings.Join(failed, ", ")
	}

	update.CompletedAt = j.clock()

	return update
}
