package handlers

import (
	"crypto/sha256"
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

// substringMatch describes a single non-overlapping occurrence of find within file content.
type substringMatch struct {
	startByte int
	endByte   int
	startLine int // 1-based line of the first byte
	endLine   int // 1-based line of the last byte; a trailing \n terminates its own line
	startChar int // byte offset from the start of startLine (for same-line reporting)
}

// findSubstringMatches returns all non-overlapping matches of find in content,
// left-to-right, consistent with strings.Index / strings.Count behavior.
func findSubstringMatches(content, find string) []substringMatch {
	if find == "" {
		return nil
	}
	var matches []substringMatch
	offset := 0
	for {
		idx := strings.Index(content[offset:], find)
		if idx == -1 {
			break
		}
		start := offset + idx
		end := start + len(find)

		startLine := strings.Count(content[:start], "\n") + 1
		// endLine: line of the last byte. A trailing \n terminates its own line.
		endLine := strings.Count(content[:end-1], "\n") + 1

		lineStart := strings.LastIndex(content[:start], "\n") + 1
		startChar := start - lineStart

		matches = append(matches, substringMatch{
			startByte: start,
			endByte:   end,
			startLine: startLine,
			endLine:   endLine,
			startChar: startChar,
		})
		offset = end
	}
	return matches
}

// countLines returns the editor-style line count of s: strings.Count(s, "\n") plus 1
// if s is non-empty and does not end with "\n". Returns 0 for empty s.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// countNewlines returns strings.Count(s, "\n"). Used for the replace line-limit guard.
func countNewlines(s string) int {
	return strings.Count(s, "\n")
}

// isValidUTF8 reports whether s is valid UTF-8.
func isValidUTF8(s string) bool {
	return utf8.ValidString(s)
}

// containsNullBytes reports whether s contains a null byte.
func containsNullBytes(s string) bool {
	return strings.ContainsRune(s, 0)
}

// sha256sum returns the SHA-256 digest of s.
func sha256sum(s string) [32]byte {
	return sha256.Sum256([]byte(s))
}

// firstNonemptyLineOf returns the first line of s containing non-whitespace,
// stripped of its trailing newline. Returns "" if no such line exists.
func firstNonemptyLineOf(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// splitLines splits content into lines, dropping the trailing empty element
// produced when content ends with "\n".
func splitLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// excerpt returns lines from content centered on lineNum, with radius lines of
// context before and after. Each output line is prefixed with its 1-based number.
func excerpt(content string, lineNum, radius int) string {
	lines := splitLines(content)
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

// excerptRange returns up to maxLines lines from content between startLine and
// endLine (inclusive, 1-based), each prefixed with its line number.
func excerptRange(content string, startLine, endLine, maxLines int) string {
	lines := splitLines(content)
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

// computeDiff returns a unified diff of the changes from before to after,
// using the Myers diff algorithm.
func computeDiff(path, before, after string) string {
	edits := myers.ComputeEdits(span.URIFromPath(path), before, after)
	unified := gotextdiff.ToUnified(path, path, before, edits)
	return fmt.Sprint(unified)
}

// atomicWrite writes content to path atomically: creates a temp file in the
// same directory (guaranteeing same filesystem), writes and closes, chmods to
// mode, then renames. On any failure after temp creation the temp file is removed.
func atomicWrite(path, content string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".jail-mcp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	if _, err = tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write: %w", err)
	}
	if err = tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close: %w", err)
	}
	if err = os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// fileLockEntry is a reference-counted per-file mutex.
type fileLockEntry struct {
	mu   sync.Mutex
	refs int
}

var (
	fileLocksMapMu sync.Mutex
	fileLocksPool  = make(map[string]*fileLockEntry)
)

// acquireFileLock acquires an exclusive per-file lock keyed on path.
// The caller must pass the returned entry to releaseFileLock when done.
func acquireFileLock(path string) *fileLockEntry {
	fileLocksMapMu.Lock()
	e, ok := fileLocksPool[path]
	if !ok {
		e = &fileLockEntry{}
		fileLocksPool[path] = e
	}
	e.refs++
	fileLocksMapMu.Unlock()
	e.mu.Lock()
	return e
}

// releaseFileLock releases the per-file lock and removes the pool entry when
// no other goroutines hold a reference.
func releaseFileLock(path string, e *fileLockEntry) {
	e.mu.Unlock()
	fileLocksMapMu.Lock()
	e.refs--
	if e.refs == 0 {
		delete(fileLocksPool, path)
	}
	fileLocksMapMu.Unlock()
}
