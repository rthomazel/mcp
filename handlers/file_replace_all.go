package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/jail-mcp/internal/fileops"
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

	result, toolErr := h.handleFileReplaceAll(path, find, replace, startLine, endLine, dryRun)
	if toolErr != "" {
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}

//nolint:cyclop,funlen
func (h *Handler) handleFileReplaceAll(path, find, replace string, startLine, endLine int, dryRun bool) (result, toolErr string) {
	maxLines := h.cfg.EditMaxLines
	maxCandidates := h.cfg.MaxCandidates

	// 1. Input guards.
	if !filepath.IsAbs(path) {
		return "", "path must be absolute."
	}
	if find == "" {
		return "", "find must not be empty."
	}
	if find == replace {
		return "", "find and replace are identical — no change would be made."
	}
	if fileops.ContainsNullBytes(find) || fileops.ContainsNullBytes(replace) {
		return "", "null bytes detected; binary files are not supported."
	}
	if !fileops.IsValidUTF8(find) || !fileops.IsValidUTF8(replace) {
		return "", "find and replace must be valid UTF-8."
	}
	if fileops.CountNewlines(replace) > maxLines {
		return "", fmt.Sprintf("replace exceeds the %d-newline limit.", maxLines)
	}
	if startLine != 0 && startLine < 1 {
		return "", "start_line must be \u2265 1."
	}
	if endLine != 0 && endLine < 1 {
		return "", "end_line must be \u2265 1."
	}
	if startLine != 0 && endLine != 0 && endLine < startLine {
		return "", "end_line must be \u2265 start_line."
	}

	// 2. Resolve symlinks.
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Sprintf("resolve path: %v", err)
	}

	// 3. Verify regular file.
	info, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Sprintf("stat: %v", err)
	}
	if !info.Mode().IsRegular() {
		return "", "path must point to a regular file."
	}

	// 4. Acquire exclusive per-file lock.
	lock := fileops.AcquireLock(realPath)
	defer fileops.ReleaseLock(realPath, lock)

	// 5. Read file; reject binary content.
	raw, err := os.ReadFile(realPath)
	if err != nil {
		return "", fmt.Sprintf("read file: %v", err)
	}
	fileContent := string(raw)
	if fileops.ContainsNullBytes(fileContent) || !fileops.IsValidUTF8(fileContent) {
		return "", "Binary files are not supported."
	}
	checksum := fileops.SHA256Sum(fileContent)
	totalLines := fileops.CountLines(fileContent)

	// 6. Validate scope against file length.
	if startLine != 0 && startLine > totalLines {
		return "", fmt.Sprintf("start_line %d out of range (file has %d lines).", startLine, totalLines)
	}
	if endLine != 0 && endLine > totalLines {
		return "", fmt.Sprintf("end_line %d out of range (file has %d lines).", endLine, totalLines)
	}

	// 7. Find all matches within scope.
	hasScope := startLine != 0 || endLine != 0
	allMatches := fileops.FindMatches(fileContent, find)
	candidates := make([]fileops.Match, 0, len(allMatches))
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
			sl := startLine
			if sl == 0 {
				sl = 1
			}
			el := endLine
			if el == 0 {
				el = totalLines
			}
			snip := fileops.ExcerptRange(fileContent, sl, el, 10)
			return "", fmt.Sprintf("find not found between lines %d\u2013%d.\n%s", sl, el, snip)
		}
		if firstLine := fileops.FirstNonemptyLine(find); firstLine != "" {
			partial := fileops.FindMatches(fileContent, firstLine)
			if len(partial) > 0 {
				shown := partial
				if len(shown) > maxCandidates {
					shown = shown[:maxCandidates]
				}
				locs := make([]string, len(shown))
				snippets := make([]string, len(shown))
				for i, m := range shown {
					locs[i] = fmt.Sprintf("%d", m.StartLine)
					snippets[i] = fileops.Excerpt(fileContent, m.StartLine, 1)
				}
				suffix := ""
				if len(partial) > maxCandidates {
					suffix = fmt.Sprintf(" (showing first %d of %d)", maxCandidates, len(partial))
				}
				return "", fmt.Sprintf(
					"first line of find matched at [%s]%s but full find did not match (check indentation or whitespace).\n%s",
					strings.Join(locs, ", "), suffix, strings.Join(snippets, ""),
				)
			}
		}
		return "", "find not found in file (check whitespace or CRLF endings)."
	}

	// 8. Apply in descending byte order.
	working := fileContent
	for i := len(candidates) - 1; i >= 0; i-- {
		m := candidates[i]
		working = working[:m.StartByte] + replace + working[m.EndByte:]
	}

	// 9. Dry-run exit.
	if dryRun {
		return fileops.ComputeDiff(realPath, fileContent, working), ""
	}

	// 10. External-modification check.
	recheck, err := os.ReadFile(realPath)
	if err != nil {
		return "", fmt.Sprintf("re-read for checksum: %v", err)
	}
	if fileops.SHA256Sum(string(recheck)) != checksum {
		return "", "Edit aborted: file was modified externally between read and write."
	}

	// 11. Atomic write — preserve original permissions.
	if err = fileops.AtomicWrite(realPath, working, info.Mode()); err != nil {
		return "", fmt.Sprintf("write failed: %v", err)
	}

	// 12. Return unified diff.
	return fileops.ComputeDiff(realPath, fileContent, working), ""
}
