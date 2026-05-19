package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sergi/go-diff/diffmatchpatch"
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
	for _, line := range strings.Split(s, "\n") {
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
	from := lineNum - 1 - radius
	if from < 0 {
		from = 0
	}
	to := lineNum + radius
	if to > total {
		to = total
	}
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
	from := startLine - 1
	if from < 0 {
		from = 0
	}
	to := endLine
	if to > total {
		to = total
	}
	if to-from > maxLines {
		to = from + maxLines
	}
	var b strings.Builder
	for i := from; i < to; i++ {
		fmt.Fprintf(&b, "%4d: %s\n", i+1, lines[i])
	}
	return b.String()
}

// ComputeDiff returns a unified diff of the changes from before to after.
func ComputeDiff(path, before, after string) string {
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(before, after)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)
	return unifiedDiff(path, diffs)
}

// unifiedDiff formats line-level diffs as a standard unified diff string.
// Returns an empty string when before == after.
func unifiedDiff(path string, diffs []diffmatchpatch.Diff) string {
	const ctx = 3

	type lineEntry struct {
		op   diffmatchpatch.Operation
		text string
	}

	// Flatten multi-line diff entries into individual lines.
	var flat []lineEntry
	for _, d := range diffs {
		for _, l := range strings.SplitAfter(d.Text, "\n") {
			if l == "" {
				continue
			}
			flat = append(flat, lineEntry{d.Type, l})
		}
	}

	// Compute 1-based line numbers in the original and new file for each entry.
	origNums := make([]int, len(flat))
	newNums := make([]int, len(flat))
	o, n := 1, 1
	for i, e := range flat {
		origNums[i] = o
		newNums[i] = n
		if e.op != diffmatchpatch.DiffInsert {
			o++
		}
		if e.op != diffmatchpatch.DiffDelete {
			n++
		}
	}

	// Build hunk spans: each changed line expands ctx lines on each side.
	// Adjacent or overlapping spans are merged.
	type span struct{ lo, hi int }
	var spans []span
	for i, e := range flat {
		if e.op == diffmatchpatch.DiffEqual {
			continue
		}
		lo := i - ctx
		if lo < 0 {
			lo = 0
		}
		hi := i + ctx + 1
		if hi > len(flat) {
			hi = len(flat)
		}
		if len(spans) > 0 && spans[len(spans)-1].hi >= lo {
			spans[len(spans)-1].hi = hi
		} else {
			spans = append(spans, span{lo, hi})
		}
	}

	if len(spans) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", path, path)

	for _, s := range spans {
		origCount, newCount := 0, 0
		for _, e := range flat[s.lo:s.hi] {
			if e.op != diffmatchpatch.DiffInsert {
				origCount++
			}
			if e.op != diffmatchpatch.DiffDelete {
				newCount++
			}
		}
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", origNums[s.lo], origCount, newNums[s.lo], newCount)
		for _, e := range flat[s.lo:s.hi] {
			switch e.op {
			case diffmatchpatch.DiffEqual:
				sb.WriteByte(' ')
			case diffmatchpatch.DiffInsert:
				sb.WriteByte('+')
			case diffmatchpatch.DiffDelete:
				sb.WriteByte('-')
			}
			sb.WriteString(e.text)
		}
	}

	return sb.String()
}

// AtomicWrite writes content to path atomically: creates a temp file in the
// same directory (guaranteeing same filesystem), writes and closes, chmods to
// mode, then renames. On any failure after temp creation the temp file is removed.
func AtomicWrite(path, content string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".jail-mcp-*")
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
