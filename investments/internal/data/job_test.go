package data

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
)

func day(d int) domain.Date { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }

type fakeProvider struct {
	errs map[string]error
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) FetchPrices(_ context.Context, symbol string, _ *domain.Date) ([]domain.PriceBar, error) {
	if err := p.errs[symbol]; err != nil {
		return nil, err
	}

	return []domain.PriceBar{{Date: day(2), Open: "1", High: "1", Low: "1", Close: "1", AdjustedClose: "1", Volume: 1}}, nil
}

type fakeStore struct {
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

func (s *fakeStore) ListInstruments(context.Context) ([]domain.Instrument, error) {
	return s.instruments, s.listErr
}

func (s *fakeStore) LatestPriceDate(context.Context, int) (*domain.Date, error) { return nil, nil }

func (s *fakeStore) UpsertPrices(context.Context, int, []domain.PriceBar) error { return nil }

func (s *fakeStore) RecordUpdate(_ context.Context, update domain.Update) error {
	s.updates = append(s.updates, update)

	return s.recordErr
}

func (s *fakeStore) LatestSessionDate(context.Context) (*domain.Date, error) {
	return s.latest, s.latestErr
}

func (s *fakeStore) WriteSnapshot(_ context.Context, date domain.Date) (domain.SnapshotResult, error) {
	s.snapshotted = append(s.snapshotted, date)

	return s.snapshot, s.snapshotErr
}

func newJob(store *fakeStore, provider *fakeProvider) *Job {
	return NewJob(store, marketdata.NewRefresher(store, provider), "fake")
}

func twoInstruments() []domain.Instrument {
	return []domain.Instrument{{ID: 1, Symbol: "BND"}, {ID: 2, Symbol: "VTI"}}
}

func TestJobRunWritesSnapshot(t *testing.T) {
	store := &fakeStore{
		instruments: twoInstruments(), latest: new(day(2)),
		snapshot: domain.SnapshotResult{Date: day(2), Complete: true, Rows: 2},
	}

	report, err := newJob(store, &fakeProvider{}).Run(context.Background())

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
	store := &fakeStore{instruments: twoInstruments(), latest: new(day(2)), snapshot: domain.SnapshotResult{Date: day(2)}}
	provider := &fakeProvider{errs: map[string]error{"VTI": domain.ErrRateLimited}}

	report, err := newJob(store, provider).Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, report.Refreshed)
	assert.Equal(t, 1, report.Failed)
	assert.Equal(t, domain.UpdateStatusIncomplete, store.updates[0].Status)
	assert.Equal(t, "not refreshed: VTI", store.updates[0].ErrorMessage)
	assert.Len(t, store.snapshotted, 1, "the snapshot is judged by stored prices")
}

func TestJobRunListFailureRecordsFailed(t *testing.T) {
	store := &fakeStore{listErr: errors.New("db down")}

	_, err := newJob(store, &fakeProvider{}).Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, domain.UpdateStatusFailed, store.updates[0].Status)
	assert.Empty(t, store.snapshotted)
}

func TestJobRunNoPricesNoSnapshot(t *testing.T) {
	store := &fakeStore{}

	report, err := newJob(store, &fakeProvider{}).Run(context.Background())

	require.NoError(t, err)
	assert.Nil(t, report.Snapshot)
	assert.Empty(t, store.snapshotted)
}

func TestJobRunErrors(t *testing.T) {
	tests := map[string]*fakeStore{
		"recording": {instruments: twoInstruments(), recordErr: errors.New("boom")},
		"latest":    {instruments: twoInstruments(), latestErr: errors.New("boom")},
		"snapshot":  {instruments: twoInstruments(), latest: new(day(2)), snapshotErr: errors.New("boom")},
	}

	for name, store := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := newJob(store, &fakeProvider{}).Run(context.Background())

			assert.Error(t, err)
		})
	}
}

func TestJobRunCancelled(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newJob(store, &fakeProvider{}).Run(ctx)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, store.updates, "a cancelled run records nothing")
}
