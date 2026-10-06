package handlers

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rthomazel/mcp/bench/internal/file"
	"github.com/rthomazel/mcp/bench/internal/stats"
)

type cursorRequest struct {
	path   string
	line   int
	anchor string
	dryRun bool
}

func parseCursorRequest(args map[string]any) (request cursorRequest, toolErr string) {
	var pathOK, anchorOK bool
	request.path, pathOK = args["path"].(string)
	request.anchor, anchorOK = args["anchor"].(string)
	request.dryRun, _ = args["dry_run"].(bool)
	if !pathOK {
		return request, "path is required and must be a string."
	}
	if !anchorOK {
		return request, "anchor is required and must be a string."
	}
	request.line, toolErr = parsePositiveInteger(args["line"], "line")
	if toolErr != "" {
		return request, toolErr
	}
	if value, exists := args["dry_run"]; exists {
		var ok bool
		request.dryRun, ok = value.(bool)
		if !ok {
			return request, "dry_run must be a boolean."
		}
	}
	if !filepath.IsAbs(request.path) {
		return request, "path must be absolute."
	}
	if !utf8.ValidString(request.path) || strings.ContainsRune(request.path, 0) {
		return request, "path must be valid UTF-8 without null bytes."
	}
	if !utf8.ValidString(request.anchor) || strings.ContainsRune(request.anchor, 0) {
		return request, "anchor must be valid UTF-8 without null bytes."
	}
	if strings.ContainsAny(request.anchor, "\r\n") {
		return request, "anchor must be single-line (no CR or LF)."
	}
	return request, ""
}

func parsePositiveInteger(value any, name string) (int, string) {
	// MCP JSON numbers decode as float64; int is also useful for in-process callers.
	switch number := value.(type) {
	case int:
		if number > 0 {
			return number, ""
		}
	case float64:
		if math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 || math.Trunc(number) != number {
			return 0, name + " must be a positive integer."
		}
		if number > 9007199254740991 || number >= math.Ldexp(1, strconv.IntSize-1) {
			return 0, name + " is outside the supported integer range."
		}
		return int(number), ""
	default:
		return 0, name + " is required and must be a number."
	}
	return 0, name + " must be a positive integer."
}

func applyCursorEdit(request cursorRequest, transform func(original string, cursor int) string) (string, string) {
	opened, toolErr := openFileForEdit(request.path)
	if toolErr != "" {
		return "", toolErr
	}
	defer file.ReleaseLock(opened.realPath, opened.lock)
	cursor, toolErr := file.ResolveCursor(opened.content, request.line, request.anchor)
	if toolErr != "" {
		return "", toolErr
	}
	transformed := transform(opened.content, cursor)
	if transformed == opened.content {
		if request.dryRun {
			return "No changes (no-op; dry-run, not written).", ""
		}
		return "No changes (no-op).", ""
	}
	result, toolErr := opened.commit(transformed, request.dryRun)
	if toolErr != "" {
		return "", toolErr
	}
	if request.dryRun {
		result = "Dry-run: not written.\n" + result
	}
	return result, ""
}

func (h *Handler) recordCursorEdit(tool string, started time.Time, request cursorRequest, errorKind string) {
	h.record(stats.ToolCall{Tool: tool, StartedAt: started, Duration: time.Since(started), FilePath: request.path, DryRun: &request.dryRun, ErrorKind: errorKind})
}
