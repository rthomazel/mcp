package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

// Match describes a single non-overlapping occurrence of a substring within file content.
type Match struct {
	StartByte int
	EndByte   int
	StartLine int // 1-based line of the first byte
	EndLine   int // 1-based line of the last byte; a trailing \n terminates its own line
	StartChar int // byte offset from the start of StartLine (for same-line reporting)
}

// FindMatches returns all non-overlapping matches of find in content,
// left-to-right, consistent with strings.Index / strings.Count behavior.
func FindMatches(content, find string) []Match {
	if find == "" {
		return nil
	}
	var matches []Match
	offset := 0
	for {
		idx := strings.Index(content[offset:], find)
		if idx == -1 {
			break
		}
		start := offset + idx
		end := start + len(find)

		startLine := strings.Count(content[:start], "\n") + 1
		// EndLine: line of the last byte. A trailing \n terminates its own line.
		endLine := strings.Count(content[:end-1], "\n") + 1

		lineStart := strings.LastIndex(content[:start], "\n") + 1
		startChar := start - lineStart

		matches = append(matches, Match{
			StartByte: start,
			EndByte:   end,
			StartLine: startLine,
			EndLine:   endLine,
			StartChar: startChar,
		})
		offset = end
	}
	return matches
}

// RegionSpan clamps a 1-based inclusive line range to the file's line count.
// The caller fills omitted bounds before calling; the returned bounds are what
// callers use for filtering. ok is false only for an empty (invalid) range,
// never permission to fall back to an unrestricted search.
func RegionSpan(startLine, endLine, fileLines int) (start, end int, ok bool) {
	start = startLine
	if start < 1 {
		start = 1
	}
	end = endLine
	if end > fileLines {
		end = fileLines
	}
	if start > end {
		return 0, 0, false
	}
	return start, end, true
}

// CountLines returns the editor-style line count of s: strings.Count(s, "\n") plus 1
// if s is non-empty and does not end with "\n". Returns 0 for empty s.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// CountNewlines returns strings.Count(s, "\n"). Used for the replace line-limit guard.
func CountNewlines(s string) int {
	return strings.Count(s, "\n")
}

// FirstNonEmptyLine returns the first line of s containing non-whitespace,
// stripped of its trailing newline. Returns "" if no such line exists.
func FirstNonEmptyLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// SplitLines splits content into lines, dropping the trailing empty element
// produced when content ends with "\n".
func SplitLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Excerpt returns lines from content centered on lineNum, with radius lines of
// context before and after. Each output line is prefixed with its 1-based number.
func Excerpt(content string, lineNum, radius int) string {
	lines := SplitLines(content)
	total := len(lines)
	from := max(lineNum-1-radius, 0)
	to := min(lineNum+radius, total)
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%4d: %s\n", i+1, lines[i])
	}
	return b.String()
}

// ExcerptRange returns up to maxLines lines from content between startLine and
// endLine (inclusive, 1-based), each prefixed with its line number.
func ExcerptRange(content string, startLine, endLine, maxLines int) string {
	lines := SplitLines(content)
	total := len(lines)
	from := max(startLine-1, 0)
	to := min(endLine, total)
	if to-from > maxLines {
		to = from + maxLines
	}
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%4d: %s\n", i+1, lines[i])
	}
	return b.String()
}

// ComputeDiff returns a unified diff of the changes from before to after,
// using the Myers diff algorithm.
func ComputeDiff(path, before, after string) string {
	edits := myers.ComputeEdits(span.URIFromPath(path), before, after)
	unified := gotextdiff.ToUnified(path, path, before, edits)
	return fmt.Sprint(unified)
}

// AtomicWrite writes content to path atomically: creates a temp file in the
// same directory (guaranteeing same filesystem), writes and closes, chmods to
// mode, then renames. On any failure after temp creation the temp file is removed.
func AtomicWrite(path, content string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".bench-mcp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err = tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close: %w", err)
	}
	if err = os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// LockEntry is a reference-counted per-file mutex.
type LockEntry struct {
	mu   sync.Mutex
	refs int
}

var (
	fileLocksMapMu sync.Mutex
	fileLocksPool  = make(map[string]*LockEntry)
)

// AcquireLock acquires an exclusive per-file lock keyed on path.
// The caller must pass the returned entry to ReleaseLock when done.
func AcquireLock(path string) *LockEntry {
	fileLocksMapMu.Lock()
	e, ok := fileLocksPool[path]
	if !ok {
		e = &LockEntry{}
		fileLocksPool[path] = e
	}
	e.refs++
	fileLocksMapMu.Unlock()
	e.mu.Lock()
	return e
}

// ReleaseLock releases the per-file lock and removes the pool entry when
// no other goroutines hold a reference.
func ReleaseLock(path string, e *LockEntry) {
	e.mu.Unlock()
	fileLocksMapMu.Lock()
	e.refs--
	if e.refs == 0 {
		delete(fileLocksPool, path)
	}
	fileLocksMapMu.Unlock()
}

// ResolveCursor finds the byte after the first anchor on the selected line.
func ResolveCursor(content string, line int, anchor string) (int, string) {
	if line <= 0 {
		return 0, "line must be a positive integer."
	}
	if !utf8.ValidString(anchor) || strings.ContainsAny(anchor, "\x00\r\n") {
		return 0, "anchor must be valid UTF-8 without null bytes, CR, or LF."
	}
	lines := CountLines(content)
	if line > lines && (content != "" || line != 1) {
		return 0, fmt.Sprintf("line %d out of range (file has %d lines).", line, lines)
	}
	start := 0
	for current := 1; current < line; current++ {
		start += strings.IndexByte(content[start:], '\n') + 1
	}
	end := len(content)
	if offset := strings.IndexByte(content[start:], '\n'); offset >= 0 {
		end = start + offset
		if end > start && content[end-1] == '\r' {
			end--
		}
	}
	if anchor == "" {
		return start, ""
	}
	text := content[start:end]
	if offset := strings.Index(text, anchor); offset >= 0 {
		return start + offset + len(anchor), ""
	}
	excerptEnd := AdvanceCodePoints(text, 0, 200)
	suffix := ""
	if excerptEnd < len(text) {
		suffix = " (truncated)"
	}
	return 0, fmt.Sprintf("anchor not found on line %d: %q%s", line, text[:excerptEnd], suffix)
}

// AdvanceCodePoints advances a valid byte cursor without splitting UTF-8.
func AdvanceCodePoints(content string, cursor int, count int) int {
	for count > 0 && cursor < len(content) {
		_, width := utf8.DecodeRuneInString(content[cursor:])
		cursor += width
		count--
	}
	return cursor
}
