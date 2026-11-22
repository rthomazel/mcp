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

// PriceRefresher orchestrates the provider and the store. It holds no state of its own.
type PriceRefresher struct {
	store    domain.Store
	provider domain.PriceProvider
}

// RefreshResult is the outcome of refreshing one instrument.
type RefreshResult struct {
	Instrument domain.Instrument
	From       *domain.Date // nil for a full history
	Bars       int
	Err        error
}

// NewPriceRefresher builds a PriceRefresher over the store and the provider.
func NewPriceRefresher(store domain.Store, provider domain.PriceProvider) *PriceRefresher {
	return &PriceRefresher{store: store, provider: provider}
}

// RegisterInstrument fetches the full history, registers the instrument and
// stores the history. The history is fetched first so a mistyped symbol never
// creates an instrument that can never be priced. If storing fails after
// registering, the instrument is returned with the error and the next refresh
// backfills it.
func (r *PriceRefresher) RegisterInstrument(ctx context.Context, draft domain.Instrument) (domain.Instrument, error) {
	draft = draft.Normalize()
	if err := draft.Validate(); err != nil {
		return domain.Instrument{}, err
	}

	bars, err := r.provider.FetchPrices(ctx, draft.Symbol, nil)
	if errors.Is(err, domain.ErrSymbolNotFound) {
		return domain.Instrument{}, fmt.Errorf("%w: %s is not covered by %s: %w",
			domain.ErrInvalidInput, draft.Symbol, r.provider.Name(), err)
	}

	if err != nil {
		return domain.Instrument{}, fmt.Errorf("fetching history for %s: %w", draft.Symbol, err)
	}

	instrument, err := r.store.RegisterInstrument(ctx, draft)
	if err != nil {
		return domain.Instrument{}, err
	}

	if err := r.store.UpsertPrices(ctx, instrument.ID, bars); err != nil {
		return instrument, fmt.Errorf("instrument %s registered but backfill failed: %w", instrument.Symbol, err)
	}

	return instrument, nil
}

// RefreshInstrument fetches from the latest stored date inclusive, since the
// provider revises a day after first publishing it, or the full history when
// the instrument has no prices. A from date earlier than that re-fetches older
// history; the start is never later than the latest stored date, so no gap opens.
// It ignores holdings: a caller can refresh any named instrument.
func (r *PriceRefresher) RefreshInstrument(ctx context.Context, instrument domain.Instrument, from *domain.Date) RefreshResult {
	result := RefreshResult{Instrument: instrument}

	fail := func(step string, err error) RefreshResult {
		result.Err = fmt.Errorf("%s for %s: %w", step, instrument.Symbol, err)

		return result
	}

	latest, err := r.store.LatestPriceDate(ctx, instrument.ID)
	if err != nil {
		return fail("reading latest price date", err)
	}

	start := latest
	if from != nil && latest != nil && from.Before(*latest) {
		start = from
	}

	bars, err := r.provider.FetchPrices(ctx, instrument.Symbol, start)
	if err != nil {
		return fail("fetching prices", err)
	}

	if err := r.store.UpsertPrices(ctx, instrument.ID, bars); err != nil {
		return fail("storing prices", err)
	}

	result.From = start
	result.Bars = len(bars)

	return result
}

// RefreshAll refreshes every held instrument, those with a positive quantity,
// in order. Rate limiting stops the run and marks the remaining instruments;
// any other failure is left in that instrument's result and the run continues.
// A sold instrument costs no provider request.
func (r *PriceRefresher) RefreshAll(ctx context.Context, from *domain.Date) ([]RefreshResult, error) {
	positions, err := r.store.GetPositions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reading held instruments: %w", err)
	}

	instruments := make([]domain.Instrument, 0, len(positions.Positions))
	for _, position := range positions.Positions {
		instruments = append(instruments, domain.Instrument{
			ID: position.InstrumentID, Symbol: position.Symbol, Exchange: position.Exchange,
		})
	}

	results := make([]RefreshResult, 0, len(instruments))

	for i, instrument := range instruments {
		if err := ctx.Err(); err != nil {
			return results, err
		}

		result := r.RefreshInstrument(ctx, instrument, from)
		results = append(results, result)

		if errors.Is(result.Err, domain.ErrRateLimited) {
			for _, skipped := range instruments[i+1:] {
				results = append(results, RefreshResult{Instrument: skipped, Err: domain.ErrRateLimited})
			}

			break
		}
	}

	return results, nil
}

const runTimeout = 30 * time.Minute

// runTimes are the price refresh run times, ascending: 06:00 picks up mutual funds,
// 20:30 follows the equity close and corrections.
var runTimes = []domain.ClockTime{{Hour: 6}, {Hour: 20, Minute: 30}}

// RunTimesDescription renders the run times for the startup log.
func RunTimesDescription(timezone string) string {
	parts := make([]string, 0, len(runTimes))
	for _, runTime := range runTimes {
		parts = append(parts, fmt.Sprintf("%02d:%02d", runTime.Hour, runTime.Minute))
	}

	return strings.Join(parts, ", ") + " " + timezone
}

// RefreshTask is the price refresh job as a task for the domain scheduler, run at
// runTimes. A failed run never stops the schedule; the next slot retries.
func RefreshTask(job *Job) domain.ScheduledTask {
	return domain.ScheduledTask{
		Name: "price refresh", At: runTimes, Timeout: runTimeout,
		Run: func(ctx context.Context) error {
			report, err := job.Run(ctx)
			if err != nil {
				return err
			}

			slog.InfoContext(ctx, "price refresh report", "refreshed", report.Refreshed, "failed", report.Failed)

			return nil
		},
	}
}
