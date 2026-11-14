package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// addTransactionView is the add_transaction result.
type addTransactionView struct {
	Transaction  transactionView `json:"transaction"`
	PriceRefresh *refreshView    `json:"price_refresh"`
	RebuildFrom  *string         `json:"rebuild_from"`
	Note         *string         `json:"note"`
}

// HandleAddTransaction appends one ledger entry and, when it opens a position,
// fetches the instrument's prices.
func (h *Handler) HandleAddTransaction(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "add_transaction"

	draft, symbol, err := parseDraftTransaction(req)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	queryCtx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	instrument, err := h.resolveInstrument(queryCtx, symbol, req.GetString("exchange", ""))
	if err != nil {
		return h.failure(queryCtx, tool, err), nil
	}

	draft.InstrumentID = instrument.ID

	wasHeld, err := h.opensPosition(queryCtx, draft)
	if err != nil {
		return h.failure(queryCtx, tool, err), nil
	}

	result, err := h.store.RecordTransaction(queryCtx, draft)
	if err != nil {
		return h.failure(queryCtx, tool, err), nil
	}

	view := addTransactionView{Transaction: toTransactionView(result.Transaction, instrument)}

	if wasHeld != nil && !*wasHeld {
		view.PriceRefresh = h.refreshOpenedPosition(ctx, instrument)
	}

	if result.RebuildFrom != nil {
		view.RebuildFrom = formatOptionalDate(result.RebuildFrom)
		note := fmt.Sprintf("snapshots from %s are stale until rebuild_snapshots runs", formatDate(*result.RebuildFrom))
		view.Note = &note
	}

	return jsonResult(view)
}

func parseDraftTransaction(req mcp.CallToolRequest) (domain.DraftTransaction, string, error) {
	var draft domain.DraftTransaction

	symbol, err := requireString(req, "symbol")
	if err != nil {
		return draft, "", err
	}

	eventType, err := requireString(req, "event_type")
	if err != nil {
		return draft, "", err
	}

	quantity, err := requireString(req, "quantity_change")
	if err != nil {
		return draft, "", err
	}

	executedAt, err := requireString(req, "executed_at")
	if err != nil {
		return draft, "", err
	}

	if draft.EffectiveDate, err = requireDate(req, "effective_date"); err != nil {
		return draft, "", err
	}

	if draft.ExecutedAt, err = parseTimestamp("executed_at", executedAt); err != nil {
		return draft, "", err
	}

	draft.EventType = domain.EventType(strings.ToUpper(eventType))
	draft.QuantityChange = domain.Decimal(quantity)
	draft.CashAmount = domain.Decimal(strings.TrimSpace(req.GetString("cash_amount", "")))

	if notes := req.GetString("notes", ""); notes != "" {
		draft.Notes = &notes
	}

	return draft, symbol, nil
}

// opensPosition reports whether the instrument is held now, or nil when the
// entry adds no quantity and so cannot open a position.
func (h *Handler) opensPosition(ctx context.Context, draft domain.DraftTransaction) (*bool, error) {
	quantity, err := domain.ParseDecimal(string(draft.QuantityChange))
	if err != nil || quantity.Sign() <= 0 {
		return nil, nil //nolint:nilnil // the store validates the amount
	}

	positions, err := h.store.GetPositions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reading positions: %w", err)
	}

	held := false

	for _, position := range positions.Positions {
		if position.InstrumentID == draft.InstrumentID {
			held = true
		}
	}

	return &held, nil
}

// refreshOpenedPosition catches the instrument's prices up after its position
// opened. Failure is reported in the result and never fails the transaction.
func (h *Handler) refreshOpenedPosition(ctx context.Context, instrument domain.Instrument) *refreshView {
	ctx, cancel := h.withTimeout(ctx, true)
	defer cancel()

	result := h.refresher.RefreshInstrument(ctx, instrument, nil)
	if result.Err != nil {
		slog.WarnContext(ctx, "price refresh after opening a position failed", "symbol", instrument.Symbol, "err", result.Err)
	}

	view := toRefreshView(result)

	return &view
}

// HandleListTransactions lists ledger entries, newest first.
func (h *Handler) HandleListTransactions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "list_transactions"

	from, err := optionalDate(req, "from")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	to, err := optionalDate(req, "to")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	instruments, err := h.store.ListInstruments(ctx)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	instrumentID, err := optionalInstrumentID(instruments, req.GetString("symbol", ""), req.GetString("exchange", ""))
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	entries, err := h.store.ListTransactions(ctx, domain.TransactionFilter{
		InstrumentID: instrumentID, From: from, To: to, Limit: req.GetInt("limit", 0),
		EventType: domain.EventType(strings.ToUpper(strings.TrimSpace(req.GetString("event_type", "")))),
	})
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	index := indexInstruments(instruments)
	views := make([]transactionView, 0, len(entries))

	for _, entry := range entries {
		views = append(views, toTransactionView(entry, index[entry.InstrumentID]))
	}

	return jsonResult(views)
}
