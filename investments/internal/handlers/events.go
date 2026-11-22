package handlers

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// HandleRemoveEvent removes an outbox event that was given up on or has expired.
func (h *Handler) HandleRemoveEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "remove_event"

	id := req.GetInt("id", 0)
	if id < 1 {
		return h.failure(ctx, tool, fmt.Errorf("%w: missing required parameter: id", domain.ErrInvalidInput)), nil
	}

	ctx, cancel := h.withTimeout(ctx, false)
	defer cancel()

	if err := h.store.RemoveEvent(ctx, int64(id)); err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(map[string]int{"removed": id})
}
