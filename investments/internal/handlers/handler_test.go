package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
)

func day(d int) domain.Date { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }

// fakeStore serves canned data and records the arguments of the calls under test.
type fakeStore struct {
	domain.Store

	instruments  []domain.Instrument
	positions    domain.PositionsResult
	listErr      error // ListInstruments
	positionsErr error // GetPositions

	recordErr    error
	recordResult domain.RecordResult
	recorded     []domain.DraftTransaction
	upserted     map[int][]domain.PriceBar
	upsertErr    error

	transactionFilter domain.TransactionFilter
	snapshotFilter    domain.SnapshotFilter
	updateFilter      domain.UpdateFilter

	snapshots    []domain.SnapshotRow
	updates      []domain.Update
	portfolio    domain.Portfolio
	rebuildErr   error
	rebuildCalls int
}

func (s *fakeStore) ListInstruments(context.Context) ([]domain.Instrument, error) {
	return s.instruments, s.listErr
}

func (s *fakeStore) RegisterInstrument(_ context.Context, draft domain.Instrument) (domain.Instrument, error) {
	draft.ID = len(s.instruments) + 1
	s.instruments = append(s.instruments, draft)

	return draft, nil
}

func (s *fakeStore) UpsertPrices(_ context.Context, id int, bars []domain.PriceBar) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}

	if s.upserted == nil {
		s.upserted = map[int][]domain.PriceBar{}
	}

	s.upserted[id] = append(s.upserted[id], bars...)

	return nil
}

func (s *fakeStore) LatestPriceDate(context.Context, int) (*domain.Date, error) { return nil, nil }

func (s *fakeStore) GetPositions(context.Context, *domain.Date) (domain.PositionsResult, error) {
	return s.positions, s.positionsErr
}

func (s *fakeStore) GetPortfolio(context.Context, *domain.Date) (domain.Portfolio, error) {
	return s.portfolio, nil
}

func (s *fakeStore) RecordTransaction(_ context.Context, draft domain.DraftTransaction) (domain.RecordResult, error) {
	s.recorded = append(s.recorded, draft)
	if s.recordErr != nil {
		return domain.RecordResult{}, s.recordErr
	}

	result := s.recordResult
	result.Transaction = domain.Transaction{
		ID: 1, InstrumentID: draft.InstrumentID, EventType: draft.EventType,
		QuantityChange: draft.QuantityChange, CashAmount: draft.CashAmount,
		ExecutedAt: draft.ExecutedAt, EffectiveDate: draft.EffectiveDate, Notes: draft.Notes,
	}

	return result, nil
}

func (s *fakeStore) ListTransactions(_ context.Context, filter domain.TransactionFilter) ([]domain.Transaction, error) {
	s.transactionFilter = filter

	return []domain.Transaction{{InstrumentID: 1, EventType: domain.EventDividend, QuantityChange: "0", CashAmount: "5", EffectiveDate: day(3)}}, nil
}

func (s *fakeStore) ListPrices(context.Context, domain.PriceFilter) ([]domain.PriceBar, error) {
	return []domain.PriceBar{{Date: day(3), Open: "1.50", High: "2", Low: "1", Close: "1.75", AdjustedClose: "1.7", Volume: 10}}, nil
}

func (s *fakeStore) ListSnapshots(_ context.Context, filter domain.SnapshotFilter) ([]domain.SnapshotRow, error) {
	s.snapshotFilter = filter

	return s.snapshots, nil
}

func (s *fakeStore) ListUpdates(_ context.Context, filter domain.UpdateFilter) ([]domain.Update, error) {
	s.updateFilter = filter

	return s.updates, nil
}

func (s *fakeStore) RebuildSnapshots(_ context.Context, from, to domain.Date, _ string) (domain.RebuildResult, error) {
	s.rebuildCalls++
	if s.rebuildErr != nil {
		return domain.RebuildResult{}, s.rebuildErr
	}

	return domain.RebuildResult{From: from, To: to, Dates: 2, Rows: 4}, nil
}

type fakeProvider struct {
	history map[string][]domain.PriceBar
	errs    map[string]error
	fetched []string
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) FetchPrices(_ context.Context, symbol string, _ *domain.Date) ([]domain.PriceBar, error) {
	p.fetched = append(p.fetched, symbol)

	if err := p.errs[symbol]; err != nil {
		return nil, err
	}

	history, ok := p.history[symbol]
	if !ok {
		return nil, domain.ErrSymbolNotFound
	}

	return history, nil
}

// cancelingProvider cancels the run after the first fetch.
type cancelingProvider struct{ cancel context.CancelFunc }

func (p *cancelingProvider) Name() string { return "canceling" }

func (p *cancelingProvider) FetchPrices(context.Context, string, *domain.Date) ([]domain.PriceBar, error) {
	p.cancel()

	return []domain.PriceBar{newBar(2)}, nil
}

func newBar(d int) domain.PriceBar {
	return domain.PriceBar{Date: day(d), Open: "1", High: "1", Low: "1", Close: "1", AdjustedClose: "1", Volume: 1}
}

func newHandler(store *fakeStore, provider *fakeProvider) *Handler {
	return New(store, marketdata.NewRefresher(store, provider), time.Minute)
}

func twoInstruments() []domain.Instrument {
	return []domain.Instrument{
		{ID: 1, Symbol: "VTI", Exchange: "NYSE", Name: "Total Market", Currency: "USD", InstrumentType: "ETF"},
		{ID: 2, Symbol: "BND", Exchange: "NYSE", Name: "Bonds", Currency: "USD", InstrumentType: "ETF"},
	}
}

func request(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func text(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	require.NotEmpty(t, result.Content)

	content, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	return content.Text
}

// decode asserts a successful result and returns its JSON body.
func decode(t *testing.T, result *mcp.CallToolResult, err error) any {
	t.Helper()
	require.NoError(t, err)
	require.False(t, result.IsError, text(t, result))

	var body any
	require.NoError(t, json.Unmarshal([]byte(text(t, result)), &body))

	return body
}

func toolError(t *testing.T, result *mcp.CallToolResult, err error) string {
	t.Helper()
	require.NoError(t, err)
	require.True(t, result.IsError, "expected a tool error, got %s", text(t, result))

	return text(t, result)
}

func TestRegisterAddsTheElevenTools(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	newHandler(&fakeStore{}, &fakeProvider{}).Register(s)

	names := []string{}
	for name := range s.ListTools() {
		names = append(names, name)
	}

	assert.ElementsMatch(t, []string{
		"add_instrument", "list_instruments", "add_transaction", "list_transactions", "get_positions",
		"get_portfolio", "get_prices", "refresh_prices", "get_snapshots", "rebuild_snapshots", "list_updates",
	}, names)

	for name, tool := range s.ListTools() {
		assert.NotEmpty(t, tool.Tool.Description, name)
	}
}

func TestFailureMapping(t *testing.T) {
	handler := newHandler(&fakeStore{}, &fakeProvider{})
	ctx := context.Background()

	tests := map[string]struct {
		err  error
		want string
	}{
		"invalid input":             {domain.ErrInvalidInput, "invalid input"},
		"plain text is not matched": {errors.New("fetching: provider rate limited"), internalErrorText},
		"wrapped limited":           {errors.Join(domain.ErrRateLimited), "provider rate limited"},
		"negative balance":          {&domain.NegativeBalanceError{InstrumentID: 1, OnDate: day(2), Quantity: "-1"}, "would hold -1 on 2026-11-02"},
		"incomplete":                {&domain.IncompleteSnapshotError{Date: day(2)}, "snapshot incomplete"},
		"unexpected":                {errors.New("connection string postgres://user:secret@host"), internalErrorText},
		"deadline":                  {context.DeadlineExceeded, "timed out"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := handler.failure(ctx, "tool", test.err)

			assert.True(t, result.IsError)
			assert.Contains(t, text(t, result), test.want)
			assert.NotContains(t, text(t, result), "secret", "unexpected errors are never shown")
		})
	}
}

func TestFindInstrument(t *testing.T) {
	instruments := append(twoInstruments(), domain.Instrument{ID: 3, Symbol: "VTI", Exchange: "NASDAQ"})

	found, err := findInstrument(instruments, " bnd ", "")
	require.NoError(t, err)
	assert.Equal(t, 2, found.ID)

	found, err = findInstrument(instruments, "vti", "nasdaq")
	require.NoError(t, err)
	assert.Equal(t, 3, found.ID)

	_, err = findInstrument(instruments, "VTI", "")
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "an ambiguous symbol needs an exchange")
	assert.Contains(t, err.Error(), "NYSE, NASDAQ")

	_, err = findInstrument(instruments, "QQQ", "")
	assert.ErrorIs(t, err, domain.ErrInstrumentNotFound)
}

func TestParseDate(t *testing.T) {
	date, err := parseDate("from", " 2026-11-03 ")
	require.NoError(t, err)
	assert.Equal(t, day(3), *date)

	date, err = parseDate("from", "")
	require.NoError(t, err)
	assert.Nil(t, date)

	_, err = parseDate("from", "11/03/2026")
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestAddInstrumentRegistersAndBackfills(t *testing.T) {
	store := &fakeStore{}
	provider := &fakeProvider{history: map[string][]domain.PriceBar{"VTI": {newBar(2), newBar(3)}}}

	result, err := newHandler(store, provider).HandleAddInstrument(context.Background(), request(map[string]any{
		"symbol": "vti", "exchange": "nyse", "name": "Total Market", "instrument_type": "etf",
	}))

	body := decode(t, result, err).(map[string]any)
	assert.Equal(t, "VTI", body["symbol"])
	assert.Equal(t, "USD", body["currency"])
	assert.Len(t, store.upserted[1], 2)
}

func TestAddInstrumentFailures(t *testing.T) {
	ctx := context.Background()
	args := map[string]any{"symbol": "VTI", "exchange": "NYSE", "name": "n", "instrument_type": "ETF"}

	t.Run("missing parameter", func(t *testing.T) {
		result, err := newHandler(&fakeStore{}, &fakeProvider{}).HandleAddInstrument(ctx, request(map[string]any{"symbol": "VTI"}))

		assert.Contains(t, toolError(t, result, err), "missing required parameter: exchange")
	})

	t.Run("symbol not covered registers nothing", func(t *testing.T) {
		store := &fakeStore{}
		result, err := newHandler(store, &fakeProvider{}).HandleAddInstrument(ctx, request(args))

		assert.Contains(t, toolError(t, result, err), "not covered")
		assert.Empty(t, store.instruments)
	})

	t.Run("backfill failure keeps the instrument and says how to retry", func(t *testing.T) {
		store := &fakeStore{upsertErr: errors.New("disk full")}
		provider := &fakeProvider{history: map[string][]domain.PriceBar{"VTI": {newBar(2)}}}

		result, err := newHandler(store, provider).HandleAddInstrument(ctx, request(args))

		message := toolError(t, result, err)
		assert.Contains(t, message, "registered but its price download failed")
		assert.Contains(t, message, "refresh_prices")
		assert.Len(t, store.instruments, 1)
	})
}

func TestListInstruments(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments()}

	result, err := newHandler(store, &fakeProvider{}).HandleListInstruments(context.Background(), request(nil))

	body := decode(t, result, err).([]any)
	require.Len(t, body, 2)
	assert.Equal(t, "VTI", body[0].(map[string]any)["symbol"])
	assert.NotContains(t, text(t, result), `"id"`, "internal ids are not exposed")
}

func transactionArgs(extra map[string]any) map[string]any {
	args := map[string]any{
		"symbol": "vti", "event_type": "buy", "quantity_change": "3", "cash_amount": "-300",
		"executed_at": "2026-11-03T15:00:00Z", "effective_date": "2026-11-03",
	}
	for key, value := range extra {
		args[key] = value
	}

	return args
}

func TestAddTransactionOpeningAPositionFetchesPrices(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments()}
	provider := &fakeProvider{history: map[string][]domain.PriceBar{"VTI": {newBar(2), newBar(3)}}}

	result, err := newHandler(store, provider).HandleAddTransaction(context.Background(), request(transactionArgs(nil)))

	body := decode(t, result, err).(map[string]any)
	assert.Equal(t, "BUY", body["transaction"].(map[string]any)["event_type"])
	assert.Equal(t, []string{"VTI"}, provider.fetched)
	assert.EqualValues(t, 2, body["price_refresh"].(map[string]any)["bars"])
	assert.Nil(t, body["rebuild_from"])
}

func TestAddTransactionForAHeldInstrumentDoesNotFetch(t *testing.T) {
	store := &fakeStore{
		instruments: twoInstruments(),
		positions:   domain.PositionsResult{Positions: []domain.Position{{InstrumentID: 1, Symbol: "VTI", Quantity: "5"}}},
	}
	provider := &fakeProvider{history: map[string][]domain.PriceBar{"VTI": {newBar(2)}}}

	result, err := newHandler(store, provider).HandleAddTransaction(context.Background(), request(transactionArgs(nil)))

	body := decode(t, result, err).(map[string]any)
	assert.Empty(t, provider.fetched)
	assert.Nil(t, body["price_refresh"])
}

func TestAddTransactionThatCannotOpenAPositionDoesNotCheckHoldings(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments(), positionsErr: errors.New("positions would fail")}
	args := transactionArgs(map[string]any{"event_type": "dividend", "quantity_change": "0", "cash_amount": "5"})

	result, err := newHandler(store, &fakeProvider{}).HandleAddTransaction(context.Background(), request(args))

	decode(t, result, err)
}

func TestAddTransactionRefreshFailureDoesNotFailTheEntry(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments()}
	provider := &fakeProvider{errs: map[string]error{"VTI": domain.ErrRateLimited}}

	result, err := newHandler(store, provider).HandleAddTransaction(context.Background(), request(transactionArgs(nil)))

	body := decode(t, result, err).(map[string]any)
	assert.Len(t, store.recorded, 1)
	assert.Contains(t, body["price_refresh"].(map[string]any)["error"], "rate limited")
}

func TestAddTransactionReportsRebuildFrom(t *testing.T) {
	rebuild := day(3)
	store := &fakeStore{
		instruments:  twoInstruments(),
		positions:    domain.PositionsResult{Positions: []domain.Position{{InstrumentID: 1}}},
		recordResult: domain.RecordResult{RebuildFrom: &rebuild},
	}

	result, err := newHandler(store, &fakeProvider{}).HandleAddTransaction(context.Background(), request(transactionArgs(nil)))

	body := decode(t, result, err).(map[string]any)
	assert.Equal(t, "2026-11-03", body["rebuild_from"])
	assert.Contains(t, body["note"], "rebuild_snapshots")
}

func TestAddTransactionErrors(t *testing.T) {
	ctx := context.Background()

	tests := map[string]struct {
		store *fakeStore
		args  map[string]any
		want  string
	}{
		"missing quantity": {&fakeStore{instruments: twoInstruments()}, map[string]any{"symbol": "VTI", "event_type": "BUY"}, "quantity_change"},
		"bad timestamp":    {&fakeStore{instruments: twoInstruments()}, transactionArgs(map[string]any{"executed_at": "yesterday"}), "RFC 3339"},
		"bad date":         {&fakeStore{instruments: twoInstruments()}, transactionArgs(map[string]any{"effective_date": "3 Nov"}), "YYYY-MM-DD"},
		"unknown symbol":   {&fakeStore{instruments: twoInstruments()}, transactionArgs(map[string]any{"symbol": "QQQ"}), "instrument not found"},
		"negative balance": {
			&fakeStore{instruments: twoInstruments(), recordErr: &domain.NegativeBalanceError{InstrumentID: 1, OnDate: day(2), Quantity: "-1"}},
			transactionArgs(map[string]any{"event_type": "sell", "quantity_change": "-9"}), "would hold -1",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := newHandler(test.store, &fakeProvider{}).HandleAddTransaction(ctx, request(test.args))

			assert.Contains(t, toolError(t, result, err), test.want)
		})
	}
}

func TestListTransactionsPassesFilters(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments()}

	result, err := newHandler(store, &fakeProvider{}).HandleListTransactions(context.Background(), request(map[string]any{
		"symbol": "bnd", "event_type": "dividend", "from": "2026-11-01", "limit": 5,
	}))

	body := decode(t, result, err).([]any)
	require.Len(t, body, 1)
	assert.Equal(t, "VTI", body[0].(map[string]any)["symbol"])
	assert.Equal(t, 2, *store.transactionFilter.InstrumentID)
	assert.Equal(t, domain.EventDividend, store.transactionFilter.EventType)
	assert.Equal(t, 5, store.transactionFilter.Limit)
	assert.Equal(t, day(1), *store.transactionFilter.From)
}

func TestListTransactionsErrors(t *testing.T) {
	handler := newHandler(&fakeStore{instruments: twoInstruments()}, &fakeProvider{})

	result, err := handler.HandleListTransactions(context.Background(), request(map[string]any{"symbol": "QQQ"}))
	assert.Contains(t, toolError(t, result, err), "instrument not found")

	result, err = handler.HandleListTransactions(context.Background(), request(map[string]any{"to": "soon"}))
	assert.Contains(t, toolError(t, result, err), "YYYY-MM-DD")
}

func TestGetPositionsCarriesNamesAndIntegrityErrors(t *testing.T) {
	store := &fakeStore{
		instruments: twoInstruments(),
		positions: domain.PositionsResult{
			Positions:       []domain.Position{{InstrumentID: 1, Symbol: "VTI", Exchange: "NYSE", Quantity: "3.5"}},
			IntegrityErrors: []domain.IntegrityError{{Symbol: "BND", Exchange: "NYSE", FirstNegativeDate: day(2), Quantity: "-1"}},
		},
	}

	result, err := newHandler(store, &fakeProvider{}).HandleGetPositions(context.Background(), request(nil))

	body := decode(t, result, err).(map[string]any)
	position := body["positions"].([]any)[0].(map[string]any)
	assert.Equal(t, "Total Market", position["name"])
	assert.Equal(t, "ETF", position["instrument_type"])
	assert.Equal(t, "3.5", position["quantity"])
	assert.Nil(t, body["date"])
	assert.Equal(t, "2026-11-02", body["integrity_errors"].([]any)[0].(map[string]any)["first_negative_date"])
}

func TestGetPortfolioIncompleteWithholdsTotals(t *testing.T) {
	closePrice := domain.Decimal("100")
	store := &fakeStore{
		instruments: twoInstruments(),
		portfolio: domain.Portfolio{
			Date: day(3),
			Holdings: []domain.Holding{
				{InstrumentID: 1, Symbol: "VTI", Exchange: "NYSE", Quantity: "3", ClosePrice: &closePrice, PriceDate: new(day(3))},
				{InstrumentID: 2, Symbol: "BND", Exchange: "NYSE", Quantity: "2", PriceIssue: domain.PriceIssueMissing},
			},
		},
	}

	result, err := newHandler(store, &fakeProvider{}).HandleGetPortfolio(context.Background(), request(map[string]any{"date": "2026-11-03"}))

	body := decode(t, result, err).(map[string]any)
	assert.Equal(t, false, body["complete"])
	assert.Nil(t, body["securities_value_usd"])

	holdings := body["holdings"].([]any)
	assert.Equal(t, "100", holdings[0].(map[string]any)["close_price"])
	assert.Equal(t, "Bonds", holdings[1].(map[string]any)["name"])
	assert.Equal(t, "missing", holdings[1].(map[string]any)["price_issue"])
	assert.Nil(t, holdings[1].(map[string]any)["close_price"])
}

func TestGetPrices(t *testing.T) {
	handler := newHandler(&fakeStore{instruments: twoInstruments()}, &fakeProvider{})

	result, err := handler.HandleGetPrices(context.Background(), request(map[string]any{"symbol": "VTI"}))

	bar := decode(t, result, err).([]any)[0].(map[string]any)
	assert.Equal(t, "1.75", bar["close"])
	assert.Equal(t, "1.50", bar["open"], "decimals keep their exact text")

	result, err = handler.HandleGetPrices(context.Background(), request(nil))
	assert.Contains(t, toolError(t, result, err), "missing required parameter: symbol")
}

func TestRefreshPrices(t *testing.T) {
	ctx := context.Background()
	held := domain.PositionsResult{Positions: []domain.Position{
		{InstrumentID: 1, Symbol: "VTI", Exchange: "NYSE", Quantity: "1"},
		{InstrumentID: 2, Symbol: "BND", Exchange: "NYSE", Quantity: "1"},
	}}

	t.Run("all held instruments", func(t *testing.T) {
		store := &fakeStore{instruments: twoInstruments(), positions: held}
		provider := &fakeProvider{history: map[string][]domain.PriceBar{"VTI": {newBar(2)}, "BND": {newBar(2), newBar(3)}}}

		result, err := newHandler(store, provider).HandleRefreshPrices(ctx, request(nil))

		results := decode(t, result, err).(map[string]any)["results"].([]any)
		require.Len(t, results, 2)
		assert.EqualValues(t, 2, results[1].(map[string]any)["bars"])
	})

	t.Run("a named instrument even when not held", func(t *testing.T) {
		store := &fakeStore{instruments: twoInstruments()}
		provider := &fakeProvider{history: map[string][]domain.PriceBar{"BND": {newBar(2)}}}

		result, err := newHandler(store, provider).HandleRefreshPrices(ctx, request(map[string]any{"symbol": "bnd", "from": "2026-11-01"}))

		results := decode(t, result, err).(map[string]any)["results"].([]any)
		require.Len(t, results, 1)
		assert.Equal(t, []string{"BND"}, provider.fetched)
	})

	t.Run("a rate limit is part of a successful result", func(t *testing.T) {
		store := &fakeStore{instruments: twoInstruments(), positions: held}
		provider := &fakeProvider{errs: map[string]error{"VTI": domain.ErrRateLimited}}

		result, err := newHandler(store, provider).HandleRefreshPrices(ctx, request(nil))

		results := decode(t, result, err).(map[string]any)["results"].([]any)
		assert.Contains(t, results[0].(map[string]any)["error"], "rate limited")
		assert.Contains(t, results[1].(map[string]any)["error"], "rate limited", "the rest are reported not refreshed")
	})

	t.Run("ending early keeps the results gathered", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		store := &fakeStore{instruments: twoInstruments(), positions: held}
		provider := &cancelingProvider{cancel: cancel}

		handler := New(store, marketdata.NewRefresher(store, provider), time.Minute)

		result, err := handler.HandleRefreshPrices(ctx, request(nil))

		message := toolError(t, result, err)
		assert.Contains(t, message, `"results"`, "the finished instruments are still reported")
		assert.Contains(t, message, `"error"`)
		assert.Contains(t, message, "VTI")
	})

	t.Run("ending early before any result is a plain failure", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()

		store := &fakeStore{instruments: twoInstruments(), positions: held}

		result, err := newHandler(store, &fakeProvider{}).HandleRefreshPrices(ctx, request(nil))

		assert.NotContains(t, toolError(t, result, err), `"results"`)
	})

	t.Run("future from is rejected", func(t *testing.T) {
		future := time.Now().UTC().AddDate(0, 0, 5).Format(dateFormat)

		result, err := newHandler(&fakeStore{}, &fakeProvider{}).HandleRefreshPrices(ctx, request(map[string]any{"from": future}))

		assert.Contains(t, toolError(t, result, err), "future")
	})

	t.Run("reading held instruments fails", func(t *testing.T) {
		store := &fakeStore{positionsErr: errors.New("db down")}

		result, err := newHandler(store, &fakeProvider{}).HandleRefreshPrices(ctx, request(nil))

		assert.Equal(t, internalErrorText, toolError(t, result, err))
	})
}

func TestGetSnapshotsGroupsByDate(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments(), snapshots: []domain.SnapshotRow{
		{Date: day(4), Symbol: "BND", Exchange: "NYSE", Quantity: "2", ClosePrice: "10", PriceDate: day(4), MarketValueUSD: "20", DateTotalUSD: "50", Allocation: "0.4"},
		{Date: day(4), Symbol: "VTI", Exchange: "NYSE", Quantity: "3", ClosePrice: "10", PriceDate: day(4), MarketValueUSD: "30", DateTotalUSD: "50", Allocation: "0.6"},
		{Date: day(3), Symbol: "VTI", Exchange: "NYSE", Quantity: "3", ClosePrice: "9", PriceDate: day(3), MarketValueUSD: "27", DateTotalUSD: "27", Allocation: "1"},
	}}

	result, err := newHandler(store, &fakeProvider{}).HandleGetSnapshots(context.Background(), request(map[string]any{"symbol": "VTI"}))

	days := decode(t, result, err).([]any)
	require.Len(t, days, 2)
	assert.Equal(t, "2026-11-04", days[0].(map[string]any)["date"])
	assert.Equal(t, "50", days[0].(map[string]any)["securities_value_usd"])
	assert.Len(t, days[0].(map[string]any)["holdings"], 2)
	assert.Len(t, days[1].(map[string]any)["holdings"], 1)
	assert.Equal(t, 1, *store.snapshotFilter.InstrumentID)
}

func TestGetSnapshotsEmptyIsAnEmptyList(t *testing.T) {
	result, err := newHandler(&fakeStore{}, &fakeProvider{}).HandleGetSnapshots(context.Background(), request(nil))

	assert.Equal(t, []any{}, decode(t, result, err))
}

func TestRebuildSnapshots(t *testing.T) {
	ctx := context.Background()
	args := map[string]any{"from": "2026-11-02", "to": "2026-11-04", "reason": " backdated buy "}

	t.Run("success", func(t *testing.T) {
		result, err := newHandler(&fakeStore{}, &fakeProvider{}).HandleRebuildSnapshots(ctx, request(args))

		body := decode(t, result, err).(map[string]any)
		assert.Equal(t, "2026-11-02", body["from"])
		assert.EqualValues(t, 4, body["rows"])
	})

	t.Run("missing reason never reaches the store", func(t *testing.T) {
		store := &fakeStore{}

		result, err := newHandler(store, &fakeProvider{}).HandleRebuildSnapshots(ctx, request(map[string]any{"from": "2026-11-02", "to": "2026-11-04"}))

		assert.Contains(t, toolError(t, result, err), "reason")
		assert.Zero(t, store.rebuildCalls)
	})

	t.Run("incomplete names the date and instruments", func(t *testing.T) {
		store := &fakeStore{rebuildErr: &domain.IncompleteSnapshotError{
			Date: day(3), Issues: []domain.SnapshotIssue{{Symbol: "BND", Issue: domain.PriceIssueMissing}},
		}}

		result, err := newHandler(store, &fakeProvider{}).HandleRebuildSnapshots(ctx, request(args))

		message := toolError(t, result, err)
		assert.Contains(t, message, "2026-11-03")
		assert.Contains(t, message, "BND missing")
	})
}

func TestListUpdates(t *testing.T) {
	provider := "tiingo"
	store := &fakeStore{updates: []domain.Update{
		{ID: 7, Provider: &provider, DataType: domain.UpdateDataPrices, Status: domain.UpdateStatusIncomplete,
			StartedAt: time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC), CompletedAt: time.Date(2026, 11, 3, 1, 1, 0, 0, time.UTC),
			RecordsUpdated: 4, ErrorMessage: "not refreshed: VTI"},
		{ID: 6, DataType: domain.UpdateDataSnapshotRebuild, Status: domain.UpdateStatusCompleted,
			StartedAt: time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC), DateFrom: new(day(1)), DateTo: new(day(2)), Reason: "fix"},
	}}

	result, err := newHandler(store, &fakeProvider{}).HandleListUpdates(context.Background(), request(map[string]any{
		"data_type": "prices", "status": "incomplete", "limit": 10,
	}))

	entries := decode(t, result, err).([]any)
	require.Len(t, entries, 2)
	assert.Equal(t, "not refreshed: VTI", entries[0].(map[string]any)["error_message"])
	assert.Equal(t, "tiingo", entries[0].(map[string]any)["provider"])
	assert.Nil(t, entries[1].(map[string]any)["provider"])
	assert.Nil(t, entries[1].(map[string]any)["completed_at"], "an absent value is null, not a zero time")
	assert.Equal(t, "fix", entries[1].(map[string]any)["reason"])
	assert.Equal(t, domain.UpdateFilter{DataType: "prices", Status: "incomplete", Limit: 10}, store.updateFilter)
}
