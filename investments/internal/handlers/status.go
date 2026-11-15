package handlers

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// HandleGetStatus summarizes whether the data is healthy.
func (h *Handler) HandleGetStatus(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_status"

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	status, err := h.store.GetStatus(ctx)
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(toStatusView(status))
}
