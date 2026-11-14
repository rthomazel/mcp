package handlers

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

// HandleGetSnapshots reads stored snapshot rows grouped by date.
func (h *Handler) HandleGetSnapshots(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "get_snapshots"

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

	rows, err := h.store.ListSnapshots(ctx, domain.SnapshotFilter{
		InstrumentID: instrumentID, From: from, To: to, Limit: req.GetInt("limit", 0),
	})
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(toSnapshotDays(rows))
}

// HandleRebuildSnapshots replaces the snapshots of every price date in a range.
func (h *Handler) HandleRebuildSnapshots(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "rebuild_snapshots"

	from, err := requireDate(req, "from")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	to, err := requireDate(req, "to")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	reason, err := requireString(req, "reason")
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	ctx, cancel := h.withTimeout(ctx, true)
	defer cancel()

	result, err := h.store.RebuildSnapshots(ctx, from, to, strings.TrimSpace(reason))
	if err != nil {
		return h.failure(ctx, tool, err), nil
	}

	return jsonResult(rebuildView{From: formatDate(result.From), To: formatDate(result.To), Dates: result.Dates, Rows: result.Rows})
}
