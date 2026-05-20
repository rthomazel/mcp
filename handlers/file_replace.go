package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

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

//nolint:cyclop
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
		if strings.Contains(r.find, "\x00") || strings.Contains(r.replace, "\x00") {
			return "", fmt.Sprintf("%s: null bytes detected; binary files are not supported.", label)
		}
		if !utf8.ValidString(r.find) || !utf8.ValidString(r.replace) {
			return "", fmt.Sprintf("%s: find and replace must be valid UTF-8.", label)
		}
		if r.lineNumber != 0 && r.lineNumber < 1 {
			return "", fmt.Sprintf("%s: line_number must be ≥ 1.", label)
		}
		if file.CountNewlines(r.replace) > maxLines {
			return "", fmt.Sprintf("%s: replace exceeds the %d-newline limit.", label, maxLines)
		}
	}

	// 2–5. Resolve symlinks, stat, lock, read, validate binary.
	editedFile, err := openFileForEdit(path)
	if err != "" {
		return "", err
	}
	defer file.ReleaseLock(editedFile.realPath, editedFile.lock)

	// 6. Validate line_number ranges against actual file length.
	for i, r := range replacements {
		if r.lineNumber != 0 && r.lineNumber > editedFile.lines {
			return "", fmt.Sprintf(
				"Replacement %d: line_number %d out of range (file has %d lines).",
				i+1, r.lineNumber, editedFile.lines,
			)
		}
	}

	// 7. Pre-pass: locate each replacement’s unique candidate in original content.
	located := make([]locatedReplacement, 0, len(replacements))
	for i, r := range replacements {
		label := fmt.Sprintf("Replacement %d of %d", i+1, len(replacements))

		allMatches := file.FindMatches(editedFile.content, r.find)
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
			return "", zeroMatchError(label, r, editedFile.content, maxCandidates)
		case 1:
			located = append(located, locatedReplacement{origIdx: i, r: r, m: candidates[0]})
		default:
			return "", multiMatchError(label, r, candidates, editedFile.content, maxCandidates)
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
	working := editedFile.content
	for i := len(located) - 1; i >= 0; i-- {
		l := located[i]
		working = working[:l.m.StartByte] + l.r.replace + working[l.m.EndByte:]
	}

	// 10–13. Dry-run, external-mod check, atomic write, return diff.
	return editedFile.commit(working, dryRun)
}
