package handlers

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rthomazel/bench-mcp/internal/file"
)

// editedFile holds the resolved state of a file opened for in-place editing.
// The caller must defer file.ReleaseLock(theFile.realPath, theFile.lock) after
// a successful openFileForEdit call.
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

// commit finalizes an edit: on dry_run it returns the diff without writing;
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

// validateFindReplace checks the five guards common to both file_replace and
// file_replace_all: empty find, identical pair, null bytes, UTF-8 validity, and
// replace line-limit. Returns an error string without label prefix, or "" if valid.
func validateFindReplace(find, replace string, maxLines int) string {
	switch {
	case find == "":
		return "find must not be empty."
	case find == replace:
		return "find and replace are identical \u2014 no change would be made."
	case strings.Contains(find, "\x00") || strings.Contains(replace, "\x00"):
		return "null bytes detected; binary files are not supported."
	case !utf8.ValidString(find) || !utf8.ValidString(replace):
		return "find and replace must be valid UTF-8."
	case file.CountNewlines(replace) > maxLines:
		return fmt.Sprintf("replace exceeds the %d-newline limit.", maxLines)
	}
	return ""
}

// partialMatchDiagnostic checks whether the first non-empty line of find
// matches anywhere in content. Returns a hint message when it does, "" otherwise.
func partialMatchDiagnostic(find, content string, maxCandidates int) string {
	firstLine := file.FirstNonEmptyLine(find)
	if firstLine == "" {
		return ""
	}
	partial := file.FindMatches(content, firstLine)
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
		snippets[i] = file.Excerpt(content, m.StartLine, 1)
	}
	suffix := ""
	if len(partial) > maxCandidates {
		suffix = fmt.Sprintf(" (showing first %d of %d)", maxCandidates, len(partial))
	}
	return fmt.Sprintf(
		"first line of find matched at [%s]%s but full find did not match (check unusual characters and encoding).\n%s",
		strings.Join(locs, ", "), suffix, strings.Join(snippets, ""),
	)
}

// zeroMatchError builds the diagnostic for a find that matched zero times.
func zeroMatchError(label string, r replacement, content string, maxCandidates int) string {
	if r.lineNumber != 0 {
		snip := file.Excerpt(content, r.lineNumber, 1)
		return fmt.Sprintf("%s failed: find not found at line %d.\n%s", label, r.lineNumber, snip)
	}
	if hint := partialMatchDiagnostic(r.find, content, maxCandidates); hint != "" {
		return fmt.Sprintf("%s failed: %s", label, hint)
	}
	return fmt.Sprintf("%s failed: find not found in file (check whitespace or CRLF endings).", label)
}

// multiMatchError builds the diagnostic for a find that matched more than once.
func multiMatchError(label string, r replacement, candidates []file.Match, content string, maxCandidates int) string {
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
			snip := file.Excerpt(content, r.lineNumber, 1)
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
			snippets[i] = file.Excerpt(content, m.StartLine, 1)
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
		snippets[i] = file.Excerpt(content, m.StartLine, 1)
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
