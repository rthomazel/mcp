package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/jail-mcp/internal/file"
)

// replacement is a single find/replace pair from a file_replace call.
type replacement struct {
	find       string
	replace    string
	lineNumber int // 0 means not provided
}

// HandleFileReplace replaces each find exactly once per item in a file.
// All items are validated against the original content before any write.
func (h *Handler) HandleFileReplace(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments

	path, _ := args["path"].(string)
	dryRun, _ := args["dry_run"].(bool)

	rawItems, ok := args["replacements"].([]any)
	if !ok || len(rawItems) == 0 {
		return mcp.NewToolResultError("replacements must not be empty."), nil
	}

	replacements := make([]replacement, 0, len(rawItems))
	for i, item := range rawItems {
		obj, ok := item.(map[string]any)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("Replacement %d: must be an object.", i+1)), nil
		}
		r := replacement{}
		r.find, _ = obj["find"].(string)
		r.replace, _ = obj["replace"].(string)
		if v, ok := obj["line_number"]; ok && v != nil {
			f, ok2 := v.(float64)
			if !ok2 {
				return mcp.NewToolResultError(fmt.Sprintf("Replacement %d: line_number must be an integer.", i+1)), nil
			}
			r.lineNumber = int(f)
		}
		replacements = append(replacements, r)
	}

	result, toolErr := h.handleFileReplace(path, replacements, dryRun)
	if toolErr != "" {
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}

// locatedReplacement pairs a replacement with its single resolved match.
type locatedReplacement struct {
	origIdx int
	r       replacement
	m       file.Match
}

//nolint:cyclop,funlen
func (h *Handler) handleFileReplace(path string, replacements []replacement, dryRun bool) (result, toolErr string) {
	maxCandidates := h.cfg.MaxCandidates
	maxLines := h.cfg.EditMaxLines

	// 1. Input guards (no lock needed — pure validation).
	if !filepath.IsAbs(path) {
		return "", "path must be absolute."
	}
	if len(replacements) == 0 {
		return "", "replacements must not be empty."
	}
	for i, r := range replacements {
		label := fmt.Sprintf("Replacement %d", i+1)
		if r.find == "" {
			return "", fmt.Sprintf("%s: find must not be empty.", label)
		}
		if r.find == r.replace {
			return "", fmt.Sprintf("%s: find and replace are identical — no change would be made.", label)
		}
		if file.ContainsNullBytes(r.find) || file.ContainsNullBytes(r.replace) {
			return "", fmt.Sprintf("%s: null bytes detected; binary files are not supported.", label)
		}
		if !file.IsValidUTF8(r.find) || !file.IsValidUTF8(r.replace) {
			return "", fmt.Sprintf("%s: find and replace must be valid UTF-8.", label)
		}
		if r.lineNumber != 0 && r.lineNumber < 1 {
			return "", fmt.Sprintf("%s: line_number must be \u2265 1.", label)
		}
		if file.CountNewlines(r.replace) > maxLines {
			return "", fmt.Sprintf("%s: replace exceeds the %d-newline limit.", label, maxLines)
		}
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
	lock := file.AcquireLock(realPath)
	defer file.ReleaseLock(realPath, lock)

	// 5. Read file; reject binary content.
	raw, err := os.ReadFile(realPath)
	if err != nil {
		return "", fmt.Sprintf("read file: %v", err)
	}
	fileContent := string(raw)
	if file.ContainsNullBytes(fileContent) || !file.IsValidUTF8(fileContent) {
		return "", "Binary files are not supported."
	}
	checksum := file.SHA256Sum(fileContent)
	totalLines := file.CountLines(fileContent)

	// 6. Validate line_number ranges against actual file length.
	for i, r := range replacements {
		if r.lineNumber != 0 && r.lineNumber > totalLines {
			return "", fmt.Sprintf(
				"Replacement %d: line_number %d out of range (file has %d lines).",
				i+1, r.lineNumber, totalLines,
			)
		}
	}

	// 7. Pre-pass: locate each replacement's unique candidate in original content.
	located := make([]locatedReplacement, 0, len(replacements))
	for i, r := range replacements {
		label := fmt.Sprintf("Replacement %d of %d", i+1, len(replacements))

		allMatches := file.FindMatches(fileContent, r.find)
		var candidates []file.Match
		if r.lineNumber != 0 {
			for _, m := range allMatches {
				if m.StartLine <= r.lineNumber && r.lineNumber <= m.EndLine {
					candidates = append(candidates, m)
				}
			}
		} else {
			candidates = allMatches
		}

		switch len(candidates) {
		case 0:
			return "", zeroMatchError(label, r, fileContent, maxCandidates)
		case 1:
			located = append(located, locatedReplacement{origIdx: i, r: r, m: candidates[0]})
		default:
			return "", multiMatchError(label, r, candidates, fileContent, maxCandidates)
		}
	}

	// 8. Sort by start byte; reject overlapping candidates.
	sort.Slice(located, func(a, b int) bool {
		return located[a].m.StartByte < located[b].m.StartByte
	})
	for j := 1; j < len(located); j++ {
		prev, curr := located[j-1], located[j]
		if prev.m.EndByte > curr.m.StartByte {
			return "", fmt.Sprintf(
				"Replacements %d and %d target overlapping regions in the original file.",
				prev.origIdx+1, curr.origIdx+1,
			)
		}
	}

	// 9. Apply in descending byte order.
	working := fileContent
	for i := len(located) - 1; i >= 0; i-- {
		l := located[i]
		working = working[:l.m.StartByte] + l.r.replace + working[l.m.EndByte:]
	}

	// 10. Dry-run exit.
	if dryRun {
		return file.ComputeDiff(realPath, fileContent, working), ""
	}

	// 11. External-modification check.
	recheck, err := os.ReadFile(realPath)
	if err != nil {
		return "", fmt.Sprintf("re-read for checksum: %v", err)
	}
	if file.SHA256Sum(string(recheck)) != checksum {
		return "", "Edit aborted: file was modified externally between read and write."
	}

	// 12. Atomic write — preserve original permissions.
	if err = file.AtomicWrite(realPath, working, info.Mode()); err != nil {
		return "", fmt.Sprintf("write failed: %v", err)
	}

	// 13. Return unified diff.
	return file.ComputeDiff(realPath, fileContent, working), ""
}

// zeroMatchError builds the diagnostic for a find that matched zero times.
func zeroMatchError(label string, r replacement, fileContent string, maxCandidates int) string {
	if r.lineNumber != 0 {
		snip := file.Excerpt(fileContent, r.lineNumber, 1)
		return fmt.Sprintf("%s failed: find not found at line %d.\n%s", label, r.lineNumber, snip)
	}
	if firstLine := file.FirstNonemptyLine(r.find); firstLine != "" {
		partial := file.FindMatches(fileContent, firstLine)
		if len(partial) > 0 {
			shown := partial
			if len(shown) > maxCandidates {
				shown = shown[:maxCandidates]
			}
			locs := make([]string, len(shown))
			snippets := make([]string, len(shown))
			for i, m := range shown {
				locs[i] = fmt.Sprintf("%d", m.StartLine)
				snippets[i] = file.Excerpt(fileContent, m.StartLine, 1)
			}
			suffix := ""
			if len(partial) > maxCandidates {
				suffix = fmt.Sprintf(" (showing first %d of %d)", maxCandidates, len(partial))
			}
			return fmt.Sprintf(
				"%s failed: first line of find matched at [%s]%s but full find did not match (check indentation or whitespace).\n%s",
				label, strings.Join(locs, ", "), suffix, strings.Join(snippets, ""),
			)
		}
	}
	return fmt.Sprintf("%s failed: find not found in file (check whitespace or CRLF endings).", label)
}

// multiMatchError builds the diagnostic for a find that matched more than once.
func multiMatchError(label string, r replacement, candidates []file.Match, fileContent string, maxCandidates int) string {
	if r.lineNumber != 0 {
		sameLine := true
		for _, c := range candidates {
			if c.StartLine != candidates[0].StartLine {
				sameLine = false
				break
			}
		}
		if sameLine {
			charPositions := make([]string, len(candidates))
			for i, c := range candidates {
				charPositions[i] = fmt.Sprintf("%d", c.StartChar)
			}
			snip := file.Excerpt(fileContent, r.lineNumber, 1)
			return fmt.Sprintf(
				"%s failed: ambiguous at line %d: find matched %d times at characters [%s]. Replace the whole line.\n%s",
				label, r.lineNumber, len(candidates), strings.Join(charPositions, ", "), snip,
			)
		}
		shown := candidates
		if len(shown) > maxCandidates {
			shown = shown[:maxCandidates]
		}
		locs := make([]string, len(shown))
		snippets := make([]string, len(shown))
		for i, m := range shown {
			locs[i] = fmt.Sprintf("%d", m.StartLine)
			snippets[i] = file.Excerpt(fileContent, m.StartLine, 1)
		}
		suffix := ""
		if len(candidates) > maxCandidates {
			suffix = fmt.Sprintf(" (showing first %d of %d)", maxCandidates, len(candidates))
		}
		return fmt.Sprintf(
			"%s failed: line_number %d did not narrow to one match (at lines [%s]%s).\n%s",
			label, r.lineNumber, strings.Join(locs, ", "), suffix, strings.Join(snippets, ""),
		)
	}
	shown := candidates
	if len(shown) > maxCandidates {
		shown = shown[:maxCandidates]
	}
	locs := make([]string, len(shown))
	snippets := make([]string, len(shown))
	for i, m := range shown {
		locs[i] = fmt.Sprintf("%d", m.StartLine)
		snippets[i] = file.Excerpt(fileContent, m.StartLine, 1)
	}
	suffix := ""
	if len(candidates) > maxCandidates {
		suffix = fmt.Sprintf(" (showing first %d of %d)", maxCandidates, len(candidates))
	}
	return fmt.Sprintf(
		"%s failed: find matched %d locations (lines [%s]%s). Provide line_number or widen find.\n%s",
		label, len(candidates), strings.Join(locs, ", "), suffix, strings.Join(snippets, ""),
	)
}
