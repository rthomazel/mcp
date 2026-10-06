package handlers

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal/file"
)

// HandleFileDelete deletes code points forward from a line-scoped cursor.
func (h *Handler) HandleFileDelete(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	started := time.Now()
	var request cursorRequest
	errorKind := ""
	defer func() { h.recordCursorEdit("file_delete", started, request, errorKind) }()
	args := req.GetArguments()
	var toolErr string
	request, toolErr = parseCursorRequest(args)
	if toolErr != "" {
		errorKind = "validation_error"
		return mcp.NewToolResultError(toolErr), nil
	}
	count, toolErr := parsePositiveInteger(args["count"], "count")
	if toolErr != "" {
		errorKind = "validation_error"
		return mcp.NewToolResultError(toolErr), nil
	}
	result, toolErr := applyCursorEdit(request, func(original string, cursor int) string {
		end := file.AdvanceCodePoints(original, cursor, count)
		return original[:cursor] + original[end:]
	})
	if toolErr != "" {
		errorKind = "edit_error"
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}
