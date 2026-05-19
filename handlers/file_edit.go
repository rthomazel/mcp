package handlers

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rthomazel/jail-mcp/internal/file"
)

// editedFile holds the resolved state of a file opened for in-place editing.
// The caller must defer file.ReleaseLock(ef.realPath, ef.lock) after a
// successful openFileForEdit call.
type editedFile struct {
	realPath string
	content  string
	checksum [32]byte
	lines    int
	mode     os.FileMode
	lock     *file.LockEntry
}

// openFileForEdit resolves symlinks, verifies the path is a regular file,
// acquires an exclusive per-file lock, reads the file, and rejects binary
// content. On success the caller owns the lock and must release it.
func openFileForEdit(path string) (*editedFile, string) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Sprintf("resolve path: %v", err)
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return nil, fmt.Sprintf("stat: %v", err)
	}
	if !info.Mode().IsRegular() {
		return nil, "path must point to a regular file."
	}

	lock := file.AcquireLock(realPath)

	raw, err := os.ReadFile(realPath)
	if err != nil {
		file.ReleaseLock(realPath, lock)
		return nil, fmt.Sprintf("read file: %v", err)
	}
	content := string(raw)
	if strings.Contains(content, "\x00") || !utf8.ValidString(content) {
		file.ReleaseLock(realPath, lock)
		return nil, "Binary files are not supported."
	}

	return &editedFile{
		realPath: realPath,
		content:  content,
		checksum: sha256.Sum256([]byte(content)),
		lines:    file.CountLines(content),
		mode:     info.Mode(),
		lock:     lock,
	}, ""
}

// commit finalises an edit: on dry_run it returns the diff without writing;
// otherwise it checks for external modifications, writes atomically, and
// returns the unified diff.
func (ef *editedFile) commit(working string, dryRun bool) (string, string) {
	if dryRun {
		return file.ComputeDiff(ef.realPath, ef.content, working), ""
	}
	recheck, err := os.ReadFile(ef.realPath)
	if err != nil {
		return "", fmt.Sprintf("re-read for checksum: %v", err)
	}
	if sha256.Sum256([]byte(recheck)) != ef.checksum {
		return "", "Edit aborted: file was modified externally between read and write."
	}
	if err = file.AtomicWrite(ef.realPath, working, ef.mode); err != nil {
		return "", fmt.Sprintf("write failed: %v", err)
	}
	return file.ComputeDiff(ef.realPath, ef.content, working), ""
}

// partialMatchDiagnostic checks whether the first non-empty line of find
// matches anywhere in fileContent. Returns a hint message when it does, "" otherwise.
func partialMatchDiagnostic(find, fileContent string, maxCandidates int) string {
	firstLine := file.FirstNonEmptyLine(find)
	if firstLine == "" {
		return ""
	}
	partial := file.FindMatches(fileContent, firstLine)
	if len(partial) == 0 {
		return ""
	}
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
		"first line of find matched at [%s]%s but full find did not match (check indentation or whitespace).\n%s",
		strings.Join(locs, ", "), suffix, strings.Join(snippets, ""),
	)
}
