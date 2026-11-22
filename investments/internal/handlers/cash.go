package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const initializeCashHint = "cash is not initialized: record an opening balance with add_cash_transaction"

// addCashView is the add_cash_transaction result.
type addCashView struct {
	CashTransaction cashTransactionView `json:"cash_transaction"`
	RebuildFrom     *string             `json:"rebuild_from"`
	Note            *string             `json:"note"`
}

// cashView is the get_cash result.
type cashView struct {
	cashBalanceView
	Note *string `json:"note"`
}

// HandleAddCashTransaction appends one account-level cash entry.
func (h *Handler) HandleAddCashTransaction(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "add_cash_transaction"

	draft, err := parseCashTransaction(req)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	result, err := h.stores.Cash.Record(ctx, draft)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	view := addCashView{CashTransaction: toCashTransactionView(result.CashTransaction)}

	if result.RebuildFrom != nil {
		view.RebuildFrom = formatOptionalDate(result.RebuildFrom)
		note := fmt.Sprintf("cash snapshots from %s are stale until rebuild_snapshots runs", formatDate(*result.RebuildFrom))
		view.Note = &note
	}

	return jsonResult(view)
}

func parseCashTransaction(req mcp.CallToolRequest) (domain.CashTransaction, error) {
	var draft domain.CashTransaction

	kind, err := requireString(req, "kind")
	if err != nil {
		return draft, err
	}

	amount, err := requireString(req, "amount")
	if err != nil {
		return draft, err
	}

	executedAt, err := requireString(req, "executed_at")
	if err != nil {
		return draft, err
	}

	if draft.EffectiveDate, err = requireDate(req, "effective_date"); err != nil {
		return draft, err
	}

	if draft.ExecutedAt, err = parseTimestamp("executed_at", executedAt); err != nil {
		return draft, err
	}

	draft.Kind = domain.CashKind(strings.ToUpper(kind))
	draft.Amount = domain.Decimal(amount)

	if notes := req.GetString("notes", ""); notes != "" {
		draft.Notes = &notes
	}

	return draft, nil
}

// HandleListCashTransactions lists cash entries, newest first.
func (h *Handler) HandleListCashTransactions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "list_cash_transactions"

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

	entries, err := h.stores.Cash.List(ctx, domain.CashTransactionFilter{
		Kind: domain.CashKind(strings.ToUpper(strings.TrimSpace(req.GetString("kind", "")))),
		From: from, To: to, Limit: req.GetInt("limit", 0),
	})
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	views := make([]cashTransactionView, 0, len(entries))
	for _, entry := range entries {
		views = append(views, toCashTransactionView(entry))
	}

	return jsonResult(views)
}

// HandleGetCash reports the cash balance and buying power.
func (h *Handler) HandleGetCash(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_cash"

	date, err := optionalDate(req, "date")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	balance, err := h.stores.Cash.Get(ctx, date)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	view := cashView{cashBalanceView: toCashBalanceView(balance)}

	if !balance.Initialized {
		hint := initializeCashHint
		view.Note = &hint
	}

	return jsonResult(view)
}
