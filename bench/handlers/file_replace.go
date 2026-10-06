package handlers

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal/file"
	"github.com/rthomazel/mcp/bench/internal/stats"
)

// Replacement outcomes reported per hunk in the tool result.
const (
	outcomeResolved      = "resolved"
	outcomeApplied       = "applied"
	outcomeWouldApply    = "would_apply"
	outcomeNotMatched    = "not_matched"
	outcomeInvalidRegion = "invalid_region"
	outcomeOverlap       = "overlap"
)

// replacement is a single find/replace pair from a file_replace call.
type replacement struct {
	find      string
	replace   string
	startLine int // 0 means not provided
	endLine   int // 0 means not provided
}

// locatedReplacement pairs a replacement with its single resolved match.
type locatedReplacement struct {
	origIdx int
	r       replacement
	m       file.Match
}

// hunkStatus is the outcome of resolving and applying a single replacement.
type hunkStatus struct {
	pos         int
	outcome     string
	located     *locatedReplacement
	diagnostics string
}

// HandleFileReplace applies each replacement independently against original
// content. A bad guess on one replacement never discards the good ones in the
// same batch; unmatched hunks receive not-applied candidate previews.
func (h *Handler) HandleFileReplace(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

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
		if v, ok := obj["start_line"]; ok && v != nil {
			f, ok2 := v.(float64)
			if !ok2 || math.IsNaN(f) || math.Trunc(f) != f || f < -math.Exp2(strconv.IntSize-1) || f >= math.Exp2(strconv.IntSize-1) {
				return mcp.NewToolResultError(fmt.Sprintf("Replacement %d: start_line must be an integer.", i+1)), nil
			}
			r.startLine = int(f)
		}
		if v, ok := obj["end_line"]; ok && v != nil {
			f, ok2 := v.(float64)
			if !ok2 || math.IsNaN(f) || math.Trunc(f) != f || f < -math.Exp2(strconv.IntSize-1) || f >= math.Exp2(strconv.IntSize-1) {
				return mcp.NewToolResultError(fmt.Sprintf("Replacement %d: end_line must be an integer.", i+1)), nil
			}
			r.endLine = int(f)
		}
		replacements = append(replacements, r)
	}

	start := time.Now()
	result, toolErr := h.handleFileReplace(path, replacements, dryRun)

	repBytes := make([][2]int, len(replacements))
	for i, r := range replacements {
		repBytes[i] = [2]int{len(r.find), len(r.replace)}
	}
	errorKind := ""
	if toolErr != "" {
		errorKind = "write_error"
	}
	h.record(stats.ToolCall{
		Tool:             "file_replace",
		StartedAt:        start,
		Duration:         time.Since(start),
		ErrorKind:        errorKind,
		FilePath:         path,
		ReplacementCount: len(replacements),
		ReplacementBytes: repBytes,
		DryRun:           &dryRun,
	})

	if toolErr != "" {
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}

//nolint:cyclop
func (h *Handler) handleFileReplace(path string, replacements []replacement, dryRun bool) (result, toolErr string) {
	maxLines := h.cfg.EditMaxLines
	maxCandidates := h.cfg.MaxCandidates

	// 1. Input guards (no lock needed — pure validation).
	if !filepath.IsAbs(path) {
		return "", "path must be absolute."
	}
	if len(replacements) == 0 {
		return "", "replacements must not be empty."
	}

	// 2. Validate the target exists before validating replacement contents, so a
	// missing file produces a direct "file does not exist" error rather than
	// "find not found in file (file does not exist)".
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return "", "file does not exist."
		}
		return "", fmt.Sprintf("stat: %v", err)
	}

	// 3. Open the file for editing: resolves symlinks, acquires the per-file
	// lock, reads, and rejects binary content.
	theFile, errStr := openFileForEdit(path)
	if errStr != "" {
		return "", errStr
	}
	defer file.ReleaseLock(theFile.realPath, theFile.lock)

	// 3. Validate every replacement before resolving any; a violation on one
	// replacement aborts the whole batch.
	for i, r := range replacements {
		label := fmt.Sprintf("Replacement %d", i+1)
		if msg := validateFindReplace(r.find, r.replace, maxLines); msg != "" {
			return "", fmt.Sprintf("%s: %s", label, msg)
		}
	}

	// 4. Resolve each replacement independently against original content. A bad
	// guess on one hunk never discards the good ones in the same batch.
	statuses := make([]hunkStatus, len(replacements))
	for i, r := range replacements {
		s := resolveReplacement(theFile.realPath, r, theFile.content, theFile.lines, maxCandidates)
		s.pos = i + 1
		if s.located != nil {
			s.located.origIdx = i
		}
		statuses[i] = s
	}

	// 5. Collect the located hunks, preserving batch order.
	located := make([]locatedReplacement, 0, len(replacements))
	for _, s := range statuses {
		if s.outcome == outcomeResolved && s.located != nil {
			located = append(located, *s.located)
		}
	}

	// 6. Sort by match start byte ascending, ties by original batch position.
	sort.SliceStable(located, func(a, b int) bool {
		if located[a].m.StartByte != located[b].m.StartByte {
			return located[a].m.StartByte < located[b].m.StartByte
		}
		return located[a].origIdx < located[b].origIdx
	})

	// 7. Keep a hunk only when its half-open byte span does not overlap a
	// previously kept hunk. The first hunk in byte order wins; later crossings
	// are dropped so one edit never invalidates another's byte offsets.
	kept := make([]locatedReplacement, 0, len(located))
	for _, l := range located {
		winner := -1
		for _, k := range kept {
			if l.m.StartByte < k.m.EndByte && k.m.StartByte < l.m.EndByte {
				winner = k.origIdx
				break
			}
		}
		if winner < 0 {
			kept = append(kept, l)
			continue
		}
		s := statuses[l.origIdx]
		s.outcome = outcomeOverlap
		s.located = nil
		s.diagnostics = fmt.Sprintf(
			"dropped: overlaps with replacement %d, which was kept.",
			winner+1,
		)
		statuses[l.origIdx] = s
	}

	// 8. Apply the kept hunks in descending byte order so earlier offsets stay valid.
	var diff string
	if len(kept) > 0 {
		working := theFile.content
		for i := len(kept) - 1; i >= 0; i-- {
			l := kept[i]
			working = working[:l.m.StartByte] + l.r.replace + working[l.m.EndByte:]
		}

		// 9. Commit the diff and optionally write; on error report nothing.
		d, commitErr := theFile.commit(working, dryRun)
		if commitErr != "" {
			return "", commitErr
		}
		diff = d
		for _, l := range kept {
			s := statuses[l.origIdx]
			if dryRun {
				s.outcome = outcomeWouldApply
			} else {
				s.outcome = outcomeApplied
			}
			statuses[l.origIdx] = s
		}
	}

	// 10. Assemble the result from the applied diff followed by per-hunk status.
	var sb strings.Builder
	sb.WriteString(diff)
	if diff != "" {
		sb.WriteString("\n")
	}
	sb.WriteString(renderStatus(statuses))
	return sb.String(), ""
}

// resolveReplacement resolves a single replacement independently against
// original content. It never aborts the batch; unmatched or ambiguous hunks are
// reported as not_matched with actionable diagnostics.
func resolveReplacement(path string, r replacement, content string, fileLines int, maxCandidates int) hunkStatus {
	regionSupplied := r.startLine != 0 || r.endLine != 0
	regionStart, regionEnd := 1, fileLines
	if regionSupplied {
		start, end := r.startLine, r.endLine
		if start == 0 {
			start = 1
		}
		if end == 0 {
			end = fileLines
		}
		clamped, clampedEnd, ok := file.RegionSpan(start, end, fileLines)
		if !ok {
			return hunkStatus{
				outcome: outcomeInvalidRegion,
				diagnostics: fmt.Sprintf(
					"region [%d-%d] is out of range for a file with %d lines.",
					r.startLine, r.endLine, fileLines,
				),
			}
		}
		regionStart, regionEnd = clamped, clampedEnd
	}

	// 2. Gather all byte-exact matches, then keep those whose line span overlaps
	// the region (or all matches when no region was supplied).
	allMatches := file.FindMatches(content, r.find)
	matches := make([]file.Match, 0, len(allMatches))
	for _, m := range allMatches {
		if !regionSupplied || (m.StartLine <= regionEnd && regionStart <= m.EndLine) {
			matches = append(matches, m)
		}
	}

	// 3. Branch on the surviving matches.
	switch len(matches) {
	case 0:
		return hunkStatus{
			outcome:     outcomeNotMatched,
			diagnostics: resolveMismatchDiagnostics(path, r, content, regionStart, regionEnd, maxCandidates),
		}
	case 1:
		return hunkStatus{
			outcome: outcomeResolved,
			located: &locatedReplacement{r: r, m: matches[0]},
		}
	default:
		return hunkStatus{
			outcome:     outcomeNotMatched,
			diagnostics: ambiguousDiagnostics(path, r, matches, content, maxCandidates),
		}
	}
}

// resolveMismatchDiagnostics builds not-applied candidate previews for a find
// that matched nowhere in the effective bounds. Candidate selection is a
// deterministic heuristic, never permission to apply.
func resolveMismatchDiagnostics(path string, r replacement, content string, startLine, endLine, maxCandidates int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "find did not match within lines %d-%d.\n", startLine, endLine)
	sb.WriteString(file.ExcerptRange(content, startLine, endLine, 10))

	// 1. Anchor on the first non-empty line of find, preserving whitespace.
	anchor := file.FirstNonEmptyLine(r.find)
	var hits []file.Match
	if anchor != "" {
		hits = findWithinBounds(content, anchor, startLine, endLine)
		if len(hits) == 0 {
			if trimmed := strings.TrimSpace(anchor); trimmed != "" {
				hits = findWithinBounds(content, trimmed, startLine, endLine)
			}
		}
	}

	// 2. An empty anchor, or one with no hits, yields no candidates.
	if len(hits) == 0 {
		sb.WriteString(noPreviewCandidate())
		return sb.String()
	}

	// 3. Infer candidate line spans: anchor hit minus the anchor's line index
	// within find, selecting the full extent of find as whole lines.
	anchorIndex := anchorLineIndex(r.find)
	nLines := file.CountLines(r.find)
	spans := dedupeSpans(hits, anchorIndex, nLines, content, startLine, endLine)
	if len(spans) == 0 {
		sb.WriteString(noPreviewCandidate())
		return sb.String()
	}

	// 4. Render not-applied previews for the first maxCandidates spans.
	sb.WriteString("Candidate previews (not applied):\n")
	shown := spans
	if len(shown) > maxCandidates {
		shown = shown[:maxCandidates]
	}
	for _, span := range shown {
		s, e, ok := lineByteSpan(content, span[0], span[1])
		if !ok {
			continue
		}
		hypothetical := content[:s] + r.replace + content[e:]
		label := fmt.Sprintf("not applied, approximate whole-line candidate, lines %d-%d", span[0], span[1])
		writeCandidatePreview(&sb, path, content, hypothetical, label)
	}
	if len(spans) > maxCandidates {
		fmt.Fprintf(&sb, "(showing first %d of %d)\n", maxCandidates, len(spans))
	}
	return sb.String()
}

// ambiguousDiagnostics builds not-applied previews for each of the first
// maxCandidates exact matches when find matched more than once.
func ambiguousDiagnostics(path string, r replacement, matches []file.Match, content string, maxCandidates int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "find matched %d locations; none applied. Provide start_line/end_line to narrow, or widen find.\n", len(matches))
	shown := matches
	if len(shown) > maxCandidates {
		shown = shown[:maxCandidates]
	}
	for _, m := range shown {
		hypothetical := content[:m.StartByte] + r.replace + content[m.EndByte:]
		label := fmt.Sprintf("not applied, matched at line %d-%d, char %d", m.StartLine, m.EndLine, m.StartChar)
		writeCandidatePreview(&sb, path, content, hypothetical, label)
	}
	if len(matches) > maxCandidates {
		fmt.Fprintf(&sb, "(showing first %d of %d)\n", maxCandidates, len(matches))
	}
	return sb.String()
}

// renderStatus tallies final outcomes and then prints each hunk's result in
// batch order. The internal resolved outcome never appears.
func renderStatus(statuses []hunkStatus) string {
	order := []string{outcomeApplied, outcomeWouldApply, outcomeNotMatched, outcomeInvalidRegion, outcomeOverlap}
	tallies := map[string]int{}
	for _, s := range statuses {
		if s.outcome != outcomeResolved {
			tallies[s.outcome]++
		}
	}
	var sb strings.Builder
	var parts []string
	for _, o := range order {
		if n := tallies[o]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, o))
		}
	}
	sb.WriteString(strings.Join(parts, ", "))
	sb.WriteString("\n\n")

	for _, s := range statuses {
		switch s.outcome {
		case outcomeApplied, outcomeWouldApply:
			m := s.located.m
			fmt.Fprintf(&sb, "Replacement %d: %s (line %d-%d, char %d)\n", s.pos, s.outcome, m.StartLine, m.EndLine, m.StartChar)
			if s.outcome == outcomeWouldApply {
				sb.WriteString("  not written\n")
			}
		default:
			fmt.Fprintf(&sb, "Replacement %d: %s\n", s.pos, s.outcome)
			sb.WriteString(indent(strings.TrimRight(s.diagnostics, "\n"), "  "))
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// writeCandidatePreview emits a labelled not-applied unified-diff preview for a
// hypothetical edit. It never commits.
func writeCandidatePreview(sb *strings.Builder, path, content, hypothetical, label string) {
	sb.WriteString(label + "\n")
	if hypothetical == content {
		sb.WriteString("empty preview: content unchanged.\n")
		return
	}
	sb.WriteString(file.ComputeDiff(path, content, hypothetical))
}

// findWithinBounds returns matches of needle whose line span overlaps the
// 1-based inclusive region [startLine, endLine].
func findWithinBounds(content, needle string, startLine, endLine int) []file.Match {
	var out []file.Match
	for _, m := range file.FindMatches(content, needle) {
		if m.StartLine <= endLine && startLine <= m.EndLine {
			out = append(out, m)
		}
	}
	return out
}

// anchorLineIndex returns the zero-based line index of the first non-empty line
// within find.
func anchorLineIndex(find string) int {
	for i, line := range strings.Split(find, "\n") {
		if strings.TrimSpace(line) != "" {
			return i
		}
	}
	return 0
}

// dedupeSpans maps anchor hits to candidate line spans, discarding those that
// fall outside the effective bounds and collapsing identical spans.
func dedupeSpans(
	hits []file.Match,
	anchorIndex, nLines int,
	content string,
	startLine, endLine int,
) [][2]int {
	var spans [][2]int
	seen := map[[2]int]bool{}
	for _, h := range hits {
		start := h.StartLine - anchorIndex
		end := start + nLines - 1
		if start < startLine || end > endLine {
			continue
		}
		span := [2]int{start, end}
		if seen[span] {
			continue
		}
		seen[span] = true
		spans = append(spans, span)
	}
	return spans
}

// lineByteSpan returns the [start,end) byte span covering the 1-based inclusive
// line range [startLine, endLine], preserving line endings.
func lineByteSpan(content string, startLine, endLine int) (int, int, bool) {
	bands := lineBoundaries(content)
	if startLine < 1 || startLine > len(bands) {
		return 0, 0, false
	}
	if endLine > len(bands) {
		endLine = len(bands)
	}
	if endLine < startLine {
		return 0, 0, false
	}
	return bands[startLine-1][0], bands[endLine-1][1], true
}

// lineBoundaries returns the [start,end) byte span of each 1-based line in
// content, including its trailing newline.
func lineBoundaries(content string) [][2]int {
	var bands [][2]int
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			bands = append(bands, [2]int{start, i + 1})
			start = i + 1
		}
	}
	if start < len(content) {
		bands = append(bands, [2]int{start, len(content)})
	}
	return bands
}

// noPreviewCandidate reports that no candidate could be inferred.
func noPreviewCandidate() string {
	return "No preview candidate. Check whitespace, indentation, or CRLF endings.\n"
}

// indent prefixes every line of s with prefix.
func indent(s, prefix string) string {
	if s == "" {
		return ""
	}
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}
