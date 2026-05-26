package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/bench-mcp/internal/file"
	"github.com/rthomazel/bench-mcp/stats"
)

// HandleFileReplaceAll replaces every occurrence of find in a file, optionally
// restricted to a line range. Returns a unified diff on success.
func (h *Handler) HandleFileReplaceAll(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	path, _ := args["path"].(string)
	find, _ := args["find"].(string)
	replace, _ := args["replace"].(string)
	dryRun, _ := args["dry_run"].(bool)

	var startLine, endLine int
	if v, ok := args["start_line"]; ok && v != nil {
		f, ok2 := v.(float64)
		if !ok2 {
			return mcp.NewToolResultError("start_line must be an integer"), nil
		}
		startLine = int(f)
	}
	if v, ok := args["end_line"]; ok && v != nil {
		f, ok2 := v.(float64)
		if !ok2 {
			return mcp.NewToolResultError("end_line must be an integer"), nil
		}
		endLine = int(f)
	}

	start := time.Now()
	result, toolErr := h.handleFileReplaceAll(path, find, replace, startLine, endLine, dryRun)

	errorKind := ""
	if toolErr != "" {
		errorKind = "write_error"
	}
	h.record(stats.ToolCall{
		Tool:             "file_replace_all",
		StartedAt:        start,
		Duration:         time.Since(start),
		ErrorKind:        errorKind,
		FilePath:         path,
		ReplacementCount: 1,
		ReplacementBytes: [][2]int{{len(find), len(replace)}},
		DryRun:           &dryRun,
	})

	if toolErr != "" {
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}

//nolint:cyclop
func (h *Handler) handleFileReplaceAll(path, find, replace string, startLine, endLine int, dryRun bool) (result, toolErr string) {
	maxLines := h.cfg.EditMaxLines
	maxCandidates := h.cfg.MaxCandidates

	// 1. Input guards.
	if !filepath.IsAbs(path) {
		return "", "path must be absolute."
	}
	if msg := validateFindReplace(find, replace, maxLines); msg != "" {
		return "", msg
	}
	if startLine != 0 && startLine < 1 {
		return "", "start_line must be ≥ 1."
	}
	if endLine != 0 && endLine < 1 {
		return "", "end_line must be ≥ 1."
	}
	if startLine != 0 && endLine != 0 && endLine < startLine {
		return "", "end_line must be ≥ start_line."
	}

	// 2–5. Resolve symlinks, stat, lock, read, validate binary.
	theFile, err := openFileForEdit(path)
	if err != "" {
		return "", err
	}
	defer file.ReleaseLock(theFile.realPath, theFile.lock)

	// 6. Validate scope against file length.
	if startLine != 0 && startLine > theFile.lines {
		return "", fmt.Sprintf("start_line %d out of range (file has %d lines).", startLine, theFile.lines)
	}
	if endLine != 0 && endLine > theFile.lines {
		return "", fmt.Sprintf("end_line %d out of range (file has %d lines).", endLine, theFile.lines)
	}

	// 7. Find all matches within scope.
	hasScope := startLine != 0 || endLine != 0
	allMatches := file.FindMatches(theFile.content, find)
	candidates := make([]file.Match, 0, len(allMatches))
	for _, m := range allMatches {
		if startLine != 0 && m.StartLine < startLine {
			continue
		}
		if endLine != 0 && m.EndLine > endLine {
			continue
		}
		candidates = append(candidates, m)
	}

	if len(candidates) == 0 {
		if hasScope {
			sl, el := startLine, endLine
			if sl == 0 {
				sl = 1
			}
			if el == 0 {
				el = theFile.lines
			}
			snip := file.ExcerptRange(theFile.content, sl, el, 10)
			return "", fmt.Sprintf("find not found between lines %d–%d.\n%s", sl, el, snip)
		}
		if hint := partialMatchDiagnostic(find, theFile.content, maxCandidates); hint != "" {
			return "", hint
		}
		return "", "find not found in file (check whitespace or CRLF endings)."
	}

	// 8. Apply in descending byte order.
	working := theFile.content
	for i := len(candidates) - 1; i >= 0; i-- {
		m := candidates[i]
		working = working[:m.StartByte] + replace + working[m.EndByte:]
	}

	// 9–12. Dry-run, external-mod check, atomic write, return diff.
	return theFile.commit(working, dryRun)
}
