package handlers

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// positionsView is the get_positions result.
type positionsView struct {
	Date            *string         `json:"date"`
	Positions       []positionView  `json:"positions"`
	IntegrityErrors []integrityView `json:"integrity_errors"`
}

// HandleGetPositions reconstructs active positions from the ledger.
func (h *Handler) HandleGetPositions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_positions"

	date, err := optionalDate(req, "date")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	result, err := h.store.GetPositions(ctx, date)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	instruments, err := h.store.ListInstruments(ctx)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(positionsView{
		Date:            formatOptionalDate(result.Date),
		Positions:       toPositionViews(result.Positions, indexInstruments(instruments)),
		IntegrityErrors: toIntegrityViews(result.IntegrityErrors),
	})
}

// HandleGetPortfolio values active positions at real closing prices.
func (h *Handler) HandleGetPortfolio(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_portfolio"

	date, err := optionalDate(req, "date")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	portfolio, err := h.store.GetPortfolio(ctx, date)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	instruments, err := h.store.ListInstruments(ctx)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(toPortfolioView(portfolio, indexInstruments(instruments)))
}
