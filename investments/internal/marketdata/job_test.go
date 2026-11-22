package marketdata

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

type jobProvider struct {
	errs map[string]error
}

func (p *jobProvider) Name() string { return "fake" }

func (p *jobProvider) FetchPrices(_ context.Context, symbol string, _ *domain.Date) ([]domain.PriceBar, error) {
	if err := p.errs[symbol]; err != nil {
		return nil, err
	}

	return []domain.PriceBar{{Date: day(2), Open: "1", High: "1", Low: "1", Close: "1", AdjustedClose: "1", Volume: 1}}, nil
}

type jobStore struct {
	domain.Store

	instruments []domain.Instrument
	listErr     error
	updates     []domain.Update
	recordErr   error
	latest      *domain.Date
	latestErr   error
	snapshot    domain.SnapshotResult
	snapshotErr error
	snapshotted []domain.Date
}

func (s *jobStore) GetPositions(context.Context, *domain.Date) (domain.PositionsResult, error) {
	if s.listErr != nil {
		return domain.PositionsResult{}, s.listErr
	}

	result := domain.PositionsResult{}
	for _, instrument := range s.instruments {
		result.Positions = append(result.Positions, domain.Position{
			InstrumentID: instrument.ID, Symbol: instrument.Symbol, Quantity: "1",
		})
	}

	return result, nil
}

func (s *jobStore) LatestPriceDate(context.Context, int) (*domain.Date, error) { return nil, nil }

func (s *jobStore) UpsertPrices(context.Context, int, []domain.PriceBar) error { return nil }

func (s *jobStore) RecordUpdate(_ context.Context, update domain.Update) error {
	s.updates = append(s.updates, update)

	return s.recordErr
}

func (s *jobStore) LatestSessionDate(context.Context) (*domain.Date, error) {
	return s.latest, s.latestErr
}

func (s *jobStore) WriteSnapshot(_ context.Context, date domain.Date) (domain.SnapshotResult, error) {
	s.snapshotted = append(s.snapshotted, date)

	return s.snapshot, s.snapshotErr
}

func newJob(store *jobStore, provider *jobProvider) *Job {
	return NewJob(store, NewRefresher(store, provider), "fake")
}

func twoInstruments() []domain.Instrument {
	return []domain.Instrument{{ID: 1, Symbol: "BND"}, {ID: 2, Symbol: "VTI"}}
}

func TestJobRunWritesSnapshot(t *testing.T) {
	store := &jobStore{
		instruments: twoInstruments(), latest: new(day(2)),
		snapshot: domain.SnapshotResult{Date: day(2), Complete: true, Rows: 2},
	}

	report, err := newJob(store, &jobProvider{}).Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, report.Refreshed)
	assert.Zero(t, report.Failed)
	assert.Equal(t, []domain.Date{day(2)}, store.snapshotted)
	require.Len(t, store.updates, 1)
	assert.Equal(t, domain.UpdateDataPrices, store.updates[0].DataType)
	assert.Equal(t, domain.UpdateStatusCompleted, store.updates[0].Status)
	assert.Equal(t, 2, store.updates[0].RecordsUpdated)
	assert.Equal(t, "fake", *store.updates[0].Provider)
}

func TestJobRunPartialFailureStillSnapshots(t *testing.T) {
	store := &jobStore{instruments: twoInstruments(), latest: new(day(2)), snapshot: domain.SnapshotResult{Date: day(2)}}
	provider := &jobProvider{errs: map[string]error{"VTI": domain.ErrRateLimited}}

	report, err := newJob(store, provider).Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, report.Refreshed)
	assert.Equal(t, 1, report.Failed)
	assert.Equal(t, domain.UpdateStatusIncomplete, store.updates[0].Status)
	assert.Equal(t, "not refreshed: VTI", store.updates[0].ErrorMessage)
	assert.Len(t, store.snapshotted, 1, "the snapshot is judged by stored prices")
}

func TestJobRunListFailureRecordsFailed(t *testing.T) {
	store := &jobStore{listErr: errors.New("db down")}

	_, err := newJob(store, &jobProvider{}).Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, domain.UpdateStatusFailed, store.updates[0].Status)
	assert.Empty(t, store.snapshotted)
}

func TestJobRunNoPricesNoSnapshot(t *testing.T) {
	store := &jobStore{}

	report, err := newJob(store, &jobProvider{}).Run(context.Background())

	require.NoError(t, err)
	assert.Nil(t, report.Snapshot)
	assert.Empty(t, store.snapshotted)
}

func TestJobRunErrors(t *testing.T) {
	tests := map[string]*jobStore{
		"recording": {instruments: twoInstruments(), recordErr: errors.New("boom")},
		"latest":    {instruments: twoInstruments(), latestErr: errors.New("boom")},
		"snapshot":  {instruments: twoInstruments(), latest: new(day(2)), snapshotErr: errors.New("boom")},
	}

	for name, store := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newJob(store, &jobProvider{}).Run(context.Background())

			assert.Error(t, err)
		})
	}
}

func TestJobRunCancelled(t *testing.T) {
	store := &jobStore{instruments: twoInstruments()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newJob(store, &jobProvider{}).Run(ctx)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, store.updates, "a cancelled run records nothing")
}
