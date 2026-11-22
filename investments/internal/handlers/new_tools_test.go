package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

type cashStore struct {
	fakeStore

	cashDraft   domain.CashTransaction
	cashResult  domain.CashRecordResult
	cashBalance domain.CashBalance
	cashFilter  domain.CashTransactionFilter
	status      domain.Status
	history     domain.ValueHistory
	historyArgs domain.HistoryFilter
	err         error
}

func (s cashOps) Record(_ context.Context, draft domain.CashTransaction) (domain.CashRecordResult, error) {
	s.cashDraft = draft
	result := s.cashResult
	result.CashTransaction = draft

	return result, s.err
}

func (s cashOps) List(_ context.Context, filter domain.CashTransactionFilter) ([]domain.CashTransaction, error) {
	s.cashFilter = filter

	return []domain.CashTransaction{{Kind: domain.CashDeposit, Amount: "5", EffectiveDate: day(3)}}, s.err
}

func (s cashOps) Get(context.Context, *domain.Date) (domain.CashBalance, error) {
	return s.cashBalance, s.err
}

func (s *cashStore) GetStatus(context.Context) (domain.Status, error) { return s.status, s.err }

func (s *cashStore) GetValueHistory(_ context.Context, filter domain.HistoryFilter) (domain.ValueHistory, error) {
	s.historyArgs = filter

	return s.history, s.err
}

// cashOps is the CashStore over a cashStore.
type cashOps struct {
	domain.CashStore
	*cashStore
}

func cashHandler(store *cashStore) *Handler {
	stores := store.bundle(store)
	stores.Cash = cashOps{cashStore: store}

	return New(stores, nil, time.Minute)
}

func TestAddCashTransactionParsesAndReportsRebuildFrom(t *testing.T) {
	stale := day(3)
	store := &cashStore{cashResult: domain.CashRecordResult{RebuildFrom: &stale}}

	result, err := cashHandler(store).HandleAddCashTransaction(context.Background(), request(map[string]any{
		"kind": "deposit", "amount": "100.5", "executed_at": "2026-11-03T15:00:00Z", "effective_date": "2026-11-03",
	}))
	body := decode(t, result, err).(map[string]any)

	assert.Equal(t, domain.CashDeposit, store.cashDraft.Kind, "the kind is upper-cased")
	assert.Equal(t, "100.5", string(store.cashDraft.Amount))
	assert.Equal(t, "2026-11-03", body["rebuild_from"])
	assert.Contains(t, body["note"], "rebuild_snapshots")
	assert.Equal(t, "100.5", body["cash_transaction"].(map[string]any)["amount"])
}

func TestAddCashTransactionRejectsBadInput(t *testing.T) {
	handler := cashHandler(&cashStore{})

	result, err := handler.HandleAddCashTransaction(context.Background(), request(map[string]any{"kind": "DEPOSIT"}))
	assert.Contains(t, toolError(t, result, err), "amount")

	result, err = handler.HandleAddCashTransaction(context.Background(), request(map[string]any{
		"kind": "DEPOSIT", "amount": "1", "executed_at": "yesterday", "effective_date": "2026-11-03",
	}))
	assert.Contains(t, toolError(t, result, err), "RFC 3339")
}

func TestListCashTransactionsPassesFilters(t *testing.T) {
	store := &cashStore{}

	result, err := cashHandler(store).HandleListCashTransactions(context.Background(), request(map[string]any{
		"kind": "fee", "from": "2026-11-01", "limit": 5,
	}))

	assert.Len(t, decode(t, result, err), 1)
	assert.Equal(t, domain.CashFee, store.cashFilter.Kind)
	assert.Equal(t, day(1), *store.cashFilter.From)
	assert.Equal(t, 5, store.cashFilter.Limit)
}

func TestGetCashHintsWhenNotInitialized(t *testing.T) {
	store := &cashStore{cashBalance: domain.CashBalance{Date: day(3)}}

	result, err := cashHandler(store).HandleGetCash(context.Background(), request(nil))
	body := decode(t, result, err).(map[string]any)

	assert.Equal(t, false, body["initialized"])
	assert.Nil(t, body["balance_usd"], "no balance rather than zero")
	assert.Nil(t, body["buying_power_usd"])
	assert.Contains(t, body["note"], "add_cash_transaction")
}

func TestGetCashReportsIntegrityErrors(t *testing.T) {
	balance, power := domain.Decimal("0"), domain.Decimal("0")
	store := &cashStore{cashBalance: domain.CashBalance{
		Date: day(3), Initialized: true, BalanceUSD: &balance, BuyingPowerUSD: &power,
		IntegrityErrors: []domain.CashIntegrityError{{FirstNegativeDate: day(2), Balance: "-5"}},
	}}

	result, err := cashHandler(store).HandleGetCash(context.Background(), request(nil))
	body := decode(t, result, err).(map[string]any)

	assert.Nil(t, body["note"])
	assert.Equal(t, "-5", body["integrity_errors"].([]any)[0].(map[string]any)["balance"])
}

func TestPortfolioViewCarriesPnLCashAndClosedPositions(t *testing.T) {
	cash := domain.Decimal("600")
	store := &cashStore{}
	store.instruments = twoInstruments()
	store.portfolio = domain.Portfolio{
		Date: day(3), Complete: false,
		Holdings: []domain.Holding{{
			InstrumentID: 1, Symbol: "VTI", Exchange: "NYSE", Quantity: "1", PriceIssue: domain.PriceIssueMissing,
			CostUSD: "10", AverageCostUSD: "10", RealizedPnLUSD: "0", DividendsUSD: "0", TaxUSD: "0", FeesUSD: "0",
		}},
		ClosedPositions: []domain.ClosedPosition{{InstrumentID: 2, Symbol: "BND", Exchange: "NYSE", RealizedPnLUSD: "5", TotalReturnUSD: "5"}},
		Cash:            domain.CashBalance{Date: day(3), Initialized: true, BalanceUSD: &cash, BuyingPowerUSD: &cash},
		PnL:             domain.PnLTotals{CostUSD: "10", RealizedPnLUSD: "5"},
	}

	result, err := cashHandler(store).HandleGetPortfolio(context.Background(), request(nil))
	body := decode(t, result, err).(map[string]any)

	holding := body["holdings"].([]any)[0].(map[string]any)
	assert.Equal(t, "10", holding["cost_usd"])
	assert.Nil(t, holding["unrealized_pnl_usd"], "withheld without a usable price")
	assert.Nil(t, holding["total_return_usd"])
	assert.Nil(t, body["account_equity_usd"])
	assert.Equal(t, "600", body["cash"].(map[string]any)["balance_usd"])
	assert.Equal(t, "Bonds", body["closed_positions"].([]any)[0].(map[string]any)["name"])
	assert.Nil(t, body["pnl"].(map[string]any)["unrealized_pnl_usd"])
}

func TestGetStatusRendersReasonsAndRuns(t *testing.T) {
	store := &cashStore{status: domain.Status{
		Date: day(3), Reasons: []string{"VTI price stale (latest 2026-10-20)"},
		Instruments:    []domain.InstrumentStatus{{Symbol: "VTI", Exchange: "NYSE", PriceIssue: domain.PriceIssueStale}},
		NeedsAttention: []domain.Update{{ID: 7, DataType: domain.UpdateDataSnapshots, Status: domain.UpdateStatusFailed}},
	}}

	result, err := cashHandler(store).HandleGetStatus(context.Background(), request(nil))
	body := decode(t, result, err).(map[string]any)

	assert.Equal(t, false, body["healthy"])
	assert.Equal(t, "stale", body["instruments"].([]any)[0].(map[string]any)["price_issue"])
	assert.Equal(t, float64(7), body["needs_attention"].([]any)[0].(map[string]any)["id"])
	assert.Nil(t, body["latest_snapshot_date"])
}

func TestGetValueHistoryPassesFilterAndStatesItIsNotAReturn(t *testing.T) {
	equityValue := domain.Decimal("150")
	store := &cashStore{history: domain.ValueHistory{
		From: day(1), To: day(3), WindowDays: 7,
		Points:     []domain.ValuePoint{{Date: day(3), SecuritiesValueUSD: "100", AccountEquityUSD: &equityValue}},
		Securities: &domain.SeriesSummary{Latest: domain.Extreme{Date: day(3), ValueUSD: "100"}},
	}}

	result, err := cashHandler(store).HandleGetValueHistory(context.Background(), request(map[string]any{
		"from": "2026-11-01", "window_days": 7,
	}))
	body := decode(t, result, err).(map[string]any)

	assert.Equal(t, 7, store.historyArgs.WindowDays)
	assert.Equal(t, day(1), *store.historyArgs.From)
	assert.Nil(t, store.historyArgs.To)
	assert.Contains(t, body["note"], "not investment returns")
	assert.Nil(t, body["equity"], "no equity summary without cash snapshots")
	assert.Nil(t, body["net_contributions_usd"])
	assert.Equal(t, "150", body["points"].([]any)[0].(map[string]any)["account_equity_usd"])
}

func TestGetValueHistoryEmptyHasNoRange(t *testing.T) {
	store := &cashStore{history: domain.ValueHistory{WindowDays: 1, Points: []domain.ValuePoint{}}}

	result, err := cashHandler(store).HandleGetValueHistory(context.Background(), request(nil))
	body := decode(t, result, err).(map[string]any)

	assert.Nil(t, body["from"])
	assert.Empty(t, body["points"])
}

func TestAddTransactionPassesTheFee(t *testing.T) {
	store := &fakeStore{instruments: twoInstruments(), positions: domain.PositionsResult{}}
	args := transactionArgs(map[string]any{"fee": " 1.25 "})

	result, err := newHandler(store, &fakeProvider{}).HandleAddTransaction(context.Background(), request(args))
	require.NoError(t, err)
	require.False(t, result.IsError, text(t, result))
	require.Len(t, store.recorded, 1)
	assert.Equal(t, "1.25", string(store.recorded[0].Fee))
}

func TestHandleRemoveEvent(t *testing.T) {
	ctx := context.Background()
	request := func(id any) mcp.CallToolRequest {
		var req mcp.CallToolRequest
		req.Params.Arguments = map[string]any{"id": id}

		return req
	}

	for name, tc := range map[string]struct {
		id      any
		err     error
		wantErr bool
		want    string
	}{
		"removed":       {id: float64(7), want: `"removed": 7`},
		"missing id":    {id: float64(0), wantErr: true, want: "invalid input"},
		"not found":     {id: float64(7), err: domain.ErrEventNotFound, wantErr: true, want: "event not found"},
		"not removable": {id: float64(7), err: domain.ErrEventNotRemovable, wantErr: true, want: "not given up on or expired"},
	} {
		store := &removeEventStore{err: tc.err}
		result, err := New(domain.Stores{Events: store}, nil, time.Minute).HandleRemoveEvent(ctx, request(tc.id))
		require.NoError(t, err, name)
		assert.Equal(t, tc.wantErr, result.IsError, name)
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, tc.want, name)
	}
}

type removeEventStore struct {
	domain.EventStore

	err error
}

func (s *removeEventStore) Remove(context.Context, int64) error { return s.err }
