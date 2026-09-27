package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal/file"
	"github.com/rthomazel/mcp/bench/internal/stats"
)

// HandleFileCreate writes a new file, optionally overwriting an existing file.
// It creates any missing parent directories and returns a short one-line
// message on success, or a unified diff when dry_run is set.
func (h *Handler) HandleFileCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	dryRun, _ := args["dry_run"].(bool)
	overwrite, _ := args["overwrite"].(bool)

	if _, ok := args["content"]; !ok {
		return mcp.NewToolResultError("content must not be empty."), nil
	}

	start := time.Now()
	result, toolErr := h.handleFileCreate(path, content, dryRun, overwrite)

	errorKind := ""
	if toolErr != "" {
		errorKind = "write_error"
	}
	h.record(stats.ToolCall{
		Tool:      "file_create",
		StartedAt: start,
		Duration:  time.Since(start),
		ErrorKind: errorKind,
		FilePath:  path,
		DryRun:    &dryRun,
		Overwrite: &overwrite,
	})

	if toolErr != "" {
		return mcp.NewToolResultError(toolErr), nil
	}
	return mcp.NewToolResultText(result), nil
}

//nolint:cyclop
func (h *Handler) handleFileCreate(path, content string, dryRun, overwrite bool) (result, toolErr string) {
	// 1. Input guards (no lock needed — pure validation).
	if !filepath.IsAbs(path) {
		return "", "path must be absolute."
	}
	if strings.Contains(content, "\x00") {
		return "", "null bytes detected; binary files are not supported."
	}
	if !utf8.ValidString(content) {
		return "", "content must be valid UTF-8."
	}

	// 2. Create any missing parent directories before resolving symlinks, so a
	// deeply missing path can be created in one shot.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Sprintf("create parent directory: %v", err)
	}

	// 3. Resolve symlinks and take the per-file lock so two concurrent creates
	// cannot both proceed. A missing final element resolves against its parent.
	realPath, errStr := resolveTarget(path)
	if errStr != "" {
		return "", errStr
	}

	lock := file.AcquireLock(realPath)
	defer file.ReleaseLock(realPath, lock)

	// 4. Stat the target. Missing is fine; non-regular files are always rejected.
	// Non-empty files require explicit overwrite permission.
	info, statErr := os.Lstat(realPath)
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", fmt.Sprintf("stat: %v", statErr)
	}
	if info != nil {
		if !info.Mode().IsRegular() {
			return "", "path must point to a regular file."
		}
		if info.Size() != 0 && !overwrite {
			return "", fmt.Sprintf("refusing to overwrite existing non-empty file: %s (set overwrite=true to replace it)", realPath)
		}
	}

	// 5. Write the file. Parent directories were created in step 2.
	return commitCreate(realPath, content, dryRun, overwrite, info)
}

// commitCreate writes content to realPath on success, or reports the diff on
// dry_run. info is non-nil when the target already existed.
func commitCreate(realPath, content string, dryRun, overwrite bool, info os.FileInfo) (string, string) {
	if dryRun {
		return file.ComputeDiff(realPath, "", content), ""
	}
	reason := "file did not exist"
	if info != nil {
		reason = "file was empty"
		if overwrite {
			reason = "file was overwritten"
		}
	}
	if err := file.AtomicWrite(realPath, content, 0o644); err != nil {
		return "", fmt.Sprintf("write failed: %v", err)
	}
	msg := fmt.Sprintf("created %s — %s, wrote %d bytes", realPath, reason, len(content))
	return msg, ""
}
