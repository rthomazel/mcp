package handlers

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// HandleListUpdates reads the updates log, newest first.
func (h *Handler) HandleListUpdates(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	updates, err := h.store.ListUpdates(ctx, domain.UpdateFilter{
		DataType: strings.TrimSpace(req.GetString("data_type", "")),
		Status:   strings.TrimSpace(req.GetString("status", "")),
		Limit:    req.GetInt("limit", 0),
	})
	if err != nil {
		return h.failure(ctx, "list_updates", err), nil
	}

	return jsonResult(toUpdateViews(updates))
}
