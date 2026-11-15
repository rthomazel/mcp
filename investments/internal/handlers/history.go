package handlers

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// HandleGetValueHistory reports portfolio value over time.
func (h *Handler) HandleGetValueHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_value_history"

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

	history, err := h.store.GetValueHistory(ctx, domain.HistoryFilter{From: from, To: to, WindowDays: req.GetInt("window_days", 0)})
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(toValueHistoryView(history))
}
