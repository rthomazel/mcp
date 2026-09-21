package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/rthomazel/mcp/bench/internal/file"
)

// createFile is the create-mode fast path of file_replace: the target file
// does not exist (or is empty) and a single replacement carries the entire
// new content. find and line_number are ignored; no line limit applies.
type createFile struct {
	target  string // resolved absolute path (parent already resolved)
	content string
	lock    *file.LockEntry
}

// resolveTarget resolves symlinks in path, returning the real absolute path.
// For a path whose final element does not exist yet, the parent directory is
// resolved instead — a missing final element makes EvalSymlinks fail outright.
func resolveTarget(path string) (string, string) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		parent, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Sprintf("parent directory %s does not exist.", filepath.Dir(path))
			}
			return "", fmt.Sprintf("resolve path: %v", err)
		}
		return filepath.Join(parent, filepath.Base(path)), ""
	}

	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Sprintf("resolve path: %v", err)
	}
	return realPath, ""
}

// openCreateFile decides whether file_replace takes the create path: the
// target is missing, or exists and is empty. Existing non-regular files are
// rejected up front so create mode never points at a directory or device.
// On success the returned *createFile owns the per-file lock; the caller
// must release it via file.ReleaseLock(creation.target, creation.lock).
func openCreateFile(path, content string) (*createFile, string) {
	realPath, errStr := resolveTarget(path)
	if errStr != "" {
		return nil, errStr
	}

	info, statErr := os.Lstat(realPath)
	if statErr != nil && !os.IsNotExist(statErr) {
		return nil, fmt.Sprintf("stat: %v", statErr)
	}
	if info != nil {
		if !info.Mode().IsRegular() {
			return nil, "path must point to a regular file."
		}
		if info.Size() != 0 {
			return nil, "" // non-empty: normal edit mode
		}
	}

	if strings.Contains(content, "\x00") {
		return nil, "null bytes detected; binary files are not supported."
	}
	if !utf8.ValidString(content) {
		return nil, "replace must be valid UTF-8."
	}

	lock := file.AcquireLock(realPath)
	return &createFile{
		target:  realPath,
		content: content,
		lock:    lock,
	}, ""
}

// commit writes (or on dry_run reports the diff of) the created file.
func (cf *createFile) commit(dryRun bool) (string, string) {
	if dryRun {
		return file.ComputeDiff(cf.target, "", cf.content), ""
	}
	reason := "file did not exist"
	if _, err := os.Lstat(cf.target); err == nil {
		reason = "file was empty"
	}
	if err := file.AtomicWrite(cf.target, cf.content, 0o644); err != nil {
		return "", fmt.Sprintf("write failed: %v", err)
	}
	msg := fmt.Sprintf(
		"created %s (%s) — %s, find ignored, wrote %d bytes",
		cf.target, humanSize(len(cf.content)), reason, len(cf.content),
	)
	return msg, ""
}

func humanSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}
