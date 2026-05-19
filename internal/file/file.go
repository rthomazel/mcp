package file

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

// IsValidUTF8 reports whether s is valid UTF-8.
func IsValidUTF8(s string) bool {
	return utf8.ValidString(s)
}

// ContainsNullBytes reports whether s contains a null byte.
func ContainsNullBytes(s string) bool {
	return strings.ContainsRune(s, 0)
}

// SHA256Sum returns the SHA-256 digest of s.
func SHA256Sum(s string) [32]byte {
	return sha256.Sum256([]byte(s))
}

// FirstNonemptyLine returns the first line of s containing non-whitespace,
// stripped of its trailing newline. Returns "" if no such line exists.
func FirstNonemptyLine(s string) string {
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
