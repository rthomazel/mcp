package handlers

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
)

// HandleFileInsert inserts verbatim content at a line-scoped cursor.
func (h *Handler) HandleFileInsert(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	started := time.Now()
	var request cursorRequest
	errorKind := ""
	defer func() { h.recordCursorEdit("file_insert", started, request, errorKind) }()
	args := req.GetArguments()
	var toolErr string
	request, toolErr = parseCursorRequest(args)
	if toolErr != "" {
		errorKind = "validation_error"
		return mcp.NewToolResultError(toolErr), nil
	}
	content, ok := args["content"].(string)
	if !ok {
		errorKind = "validation_error"
		return mcp.NewToolResultError("content is required and must be a string."), nil
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		errorKind = "validation_error"
		return mcp.NewToolResultError("content must be valid UTF-8 without null bytes."), nil
	}
	result, toolErr := applyCursorEdit(request, func(original string, cursor int) string { return original[:cursor] + content + original[cursor:] })
	if toolErr != "" {
		errorKind = "edit_error"
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}
