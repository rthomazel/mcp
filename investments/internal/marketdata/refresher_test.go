package marketdata

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

func day(d int) domain.Date { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }

func bar(d int) domain.PriceBar {
	return domain.PriceBar{Date: day(d), Open: "1", High: "1", Low: "1", Close: "1", AdjustedClose: "1", Volume: 1}
}

type fetchCall struct {
	symbol string
	from   *domain.Date
}

// fakeProvider serves history per symbol, from the date inclusive.
type fakeProvider struct {
	history map[string][]domain.PriceBar
	errs    map[string]error
	calls   []fetchCall
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) FetchPrices(_ context.Context, symbol string, from *domain.Date) ([]domain.PriceBar, error) {
	p.calls = append(p.calls, fetchCall{symbol, from})

	if err := p.errs[symbol]; err != nil {
		return nil, err
	}

	history, ok := p.history[symbol]
	if !ok {
		return nil, domain.ErrSymbolNotFound
	}

	bars := []domain.PriceBar{}

	for _, b := range history {
		if from == nil || !b.Date.Before(*from) {
			bars = append(bars, b)
		}
	}

	return bars, nil
}

// fakeStore implements the four Store methods the refresher uses.
type fakeStore struct {
	domain.Store

	instruments []domain.Instrument
	prices      map[int]map[time.Time]domain.PriceBar
	registerErr error
	upsertErr   map[int]error
	listErr     error
	latestErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{prices: map[int]map[time.Time]domain.PriceBar{}, upsertErr: map[int]error{}}
}

func (s *fakeStore) add(symbol string, bars ...domain.PriceBar) domain.Instrument {
	instrument := domain.Instrument{ID: len(s.instruments) + 1, Symbol: symbol, Exchange: "NYSE"}
	s.instruments = append(s.instruments, instrument)

	s.prices[instrument.ID] = map[time.Time]domain.PriceBar{}
	for _, b := range bars {
		s.prices[instrument.ID][b.Date] = b
	}

	return instrument
}

func (s *fakeStore) RegisterInstrument(_ context.Context, draft domain.Instrument) (domain.Instrument, error) {
	if s.registerErr != nil {
		return domain.Instrument{}, s.registerErr
	}

	return s.add(draft.Symbol), nil
}

func (s *fakeStore) ListInstruments(context.Context) ([]domain.Instrument, error) {
	return s.instruments, s.listErr
}

func (s *fakeStore) UpsertPrices(_ context.Context, id int, bars []domain.PriceBar) error {
	if err := s.upsertErr[id]; err != nil {
		return err
	}

	for _, b := range bars {
		s.prices[id][b.Date] = b
	}

	return nil
}

func (s *fakeStore) LatestPriceDate(_ context.Context, id int) (*domain.Date, error) {
	if s.latestErr != nil {
		return nil, s.latestErr
	}

	var latest *domain.Date

	for d := range s.prices[id] {
		if latest == nil || d.After(*latest) {
			latest = &d
		}
	}

	return latest, nil
}

func draft(symbol string) domain.Instrument {
	return domain.Instrument{Symbol: symbol, Exchange: "nyse", Name: symbol + " fund", Currency: "usd", InstrumentType: "etf"}
}

func TestRegisterInstrumentBackfillsFullHistory(t *testing.T) {
	provider := &fakeProvider{history: map[string][]domain.PriceBar{"VOO": {bar(10), bar(11), bar(12)}}}
	store := newFakeStore()

	instrument, err := NewRefresher(store, provider).RegisterInstrument(context.Background(), draft(" voo "))
	require.NoError(t, err)

	assert.Equal(t, "VOO", instrument.Symbol)
	assert.Equal(t, []fetchCall{{"VOO", nil}}, provider.calls, "full history by normalized symbol")
	assert.Len(t, store.prices[instrument.ID], 3)
}

func TestRegisterInstrumentRejections(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid draft never reaches the provider", func(t *testing.T) {
		provider := &fakeProvider{}
		bad := draft("VOO")
		bad.Currency = "BRL"

		_, err := NewRefresher(newFakeStore(), provider).RegisterInstrument(ctx, bad)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
		assert.Empty(t, provider.calls)
	})

	t.Run("unknown symbol registers nothing", func(t *testing.T) {
		store := newFakeStore()

		_, err := NewRefresher(store, &fakeProvider{}).RegisterInstrument(ctx, draft("NOPE"))
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
		assert.ErrorIs(t, err, domain.ErrSymbolNotFound)
		assert.Empty(t, store.instruments)
	})

	t.Run("provider failure registers nothing", func(t *testing.T) {
		store := newFakeStore()
		provider := &fakeProvider{errs: map[string]error{"VOO": domain.ErrRateLimited}}

		_, err := NewRefresher(store, provider).RegisterInstrument(ctx, draft("VOO"))
		assert.ErrorIs(t, err, domain.ErrRateLimited)
		assert.NotErrorIs(t, err, domain.ErrInvalidInput)
		assert.Empty(t, store.instruments)
	})

	t.Run("store error is returned unchanged", func(t *testing.T) {
		store := newFakeStore()
		store.registerErr = domain.ErrInstrumentExists
		provider := &fakeProvider{history: map[string][]domain.PriceBar{"VOO": {bar(12)}}}

		_, err := NewRefresher(store, provider).RegisterInstrument(ctx, draft("VOO"))
		assert.Equal(t, domain.ErrInstrumentExists, err)
	})
}

func TestRegisterInstrumentBackfillFailureKeepsInstrument(t *testing.T) {
	store := newFakeStore()
	store.upsertErr[1] = errors.New("disk full")
	provider := &fakeProvider{history: map[string][]domain.PriceBar{"VOO": {bar(11), bar(12)}}}
	refresher := NewRefresher(store, provider)

	instrument, err := refresher.RegisterInstrument(context.Background(), draft("VOO"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backfill failed")
	assert.Equal(t, "VOO", instrument.Symbol, "the registered instrument is returned with the error")
	require.Len(t, store.instruments, 1)

	// The next refresh finds no prices and fetches the full history.
	delete(store.upsertErr, 1)
	provider.calls = nil

	result := refresher.RefreshInstrument(context.Background(), instrument)
	require.NoError(t, result.Err)
	assert.Nil(t, result.From)
	assert.Equal(t, 2, result.Bars)
	assert.Equal(t, []fetchCall{{"VOO", nil}}, provider.calls)
}

func TestRefreshInstrumentStartsAtLatestStoredDateInclusive(t *testing.T) {
	store := newFakeStore()
	instrument := store.add("VOO", bar(10), bar(11))

	revised := bar(11)
	revised.Volume = 999

	provider := &fakeProvider{history: map[string][]domain.PriceBar{"VOO": {bar(10), revised, bar(12), bar(13)}}}

	result := NewRefresher(store, provider).RefreshInstrument(context.Background(), instrument)
	require.NoError(t, result.Err)

	require.NotNil(t, result.From)
	assert.Equal(t, day(11), *result.From)
	assert.Equal(t, 3, result.Bars, "the latest stored day is fetched again")
	assert.EqualValues(t, 999, store.prices[instrument.ID][day(11)].Volume, "and replaced")
	assert.Len(t, store.prices[instrument.ID], 4)
	assert.Equal(t, instrument, result.Instrument)
}

func TestRefreshInstrumentFailuresNameTheSymbol(t *testing.T) {
	ctx := context.Background()

	t.Run("reading", func(t *testing.T) {
		store := newFakeStore()
		instrument := store.add("VOO")
		store.latestErr = errors.New("db down")

		result := NewRefresher(store, &fakeProvider{}).RefreshInstrument(ctx, instrument)
		require.Error(t, result.Err)
		assert.Contains(t, result.Err.Error(), "VOO")
		assert.Zero(t, result.Bars)
	})

	t.Run("fetching", func(t *testing.T) {
		store := newFakeStore()
		instrument := store.add("VOO")
		provider := &fakeProvider{errs: map[string]error{"VOO": domain.ErrRateLimited}}

		result := NewRefresher(store, provider).RefreshInstrument(ctx, instrument)
		assert.ErrorIs(t, result.Err, domain.ErrRateLimited)
		assert.Contains(t, result.Err.Error(), "VOO")
	})

	t.Run("storing", func(t *testing.T) {
		store := newFakeStore()
		instrument := store.add("VOO")
		store.upsertErr[instrument.ID] = errors.New("disk full")
		provider := &fakeProvider{history: map[string][]domain.PriceBar{"VOO": {bar(12)}}}

		result := NewRefresher(store, provider).RefreshInstrument(ctx, instrument)
		require.Error(t, result.Err)
		assert.Zero(t, result.Bars)
	})
}

func TestRefreshAll(t *testing.T) {
	ctx := context.Background()

	t.Run("one failure does not stop the others", func(t *testing.T) {
		store := newFakeStore()
		store.add("AAA")
		store.add("BBB")
		store.add("CCC")

		provider := &fakeProvider{
			history: map[string][]domain.PriceBar{"AAA": {bar(12)}, "CCC": {bar(12)}},
			errs:    map[string]error{"BBB": errors.New("boom")},
		}

		results, err := NewRefresher(store, provider).RefreshAll(ctx)
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.NoError(t, results[0].Err)
		assert.Error(t, results[1].Err)
		assert.NoError(t, results[2].Err)
		assert.Equal(t, 1, results[2].Bars)
	})

	t.Run("rate limiting stops and marks the rest", func(t *testing.T) {
		store := newFakeStore()
		store.add("AAA")
		store.add("BBB")
		store.add("CCC")

		provider := &fakeProvider{
			history: map[string][]domain.PriceBar{"AAA": {bar(12)}},
			errs:    map[string]error{"BBB": domain.ErrRateLimited},
		}

		results, err := NewRefresher(store, provider).RefreshAll(ctx)
		require.NoError(t, err)
		require.Len(t, results, 3)
		assert.NoError(t, results[0].Err)
		assert.ErrorIs(t, results[1].Err, domain.ErrRateLimited)
		assert.Equal(t, domain.ErrRateLimited, results[2].Err)
		assert.Equal(t, "CCC", results[2].Instrument.Symbol)
		assert.Len(t, provider.calls, 2, "CCC was never requested")
	})

	t.Run("listing failure", func(t *testing.T) {
		store := newFakeStore()
		store.listErr = errors.New("db down")

		results, err := NewRefresher(store, &fakeProvider{}).RefreshAll(ctx)
		require.Error(t, err)
		assert.Nil(t, results)
	})

	t.Run("cancelled context stops the run", func(t *testing.T) {
		store := newFakeStore()
		store.add("AAA")

		cancelled, cancel := context.WithCancel(ctx)
		cancel()

		results, err := NewRefresher(store, &fakeProvider{}).RefreshAll(cancelled)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, results)
	})

	t.Run("no instruments", func(t *testing.T) {
		results, err := NewRefresher(newFakeStore(), &fakeProvider{}).RefreshAll(ctx)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
