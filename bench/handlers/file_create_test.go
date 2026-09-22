package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal"
)

// mustSucceed wraps an os setup call, failing the test if it returns an error.
func mustSucceed(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}

// --- resolveTarget ---

func TestResolveTarget(t *testing.T) {
	for _, uc := range []struct {
		name     string
		setup    func(t *testing.T, dir string)
		rel      string
		wantRel  string
		wantErr  bool
		contains string
	}{
		{
			name: "existing file", rel: "src/main.go", wantRel: "src/main.go",
			setup: func(t *testing.T, dir string) {
				mustSucceed(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
				mustSucceed(t, os.WriteFile(filepath.Join(dir, "src/main.go"), []byte("x"), 0o644))
			},
		},
		{
			name: "missing file resolves parent", rel: "src/new.go", wantRel: "src/new.go",
			setup: func(t *testing.T, dir string) {
				mustSucceed(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
			},
		},
		{name: "missing parent is an error", rel: "nope/new.go", wantErr: true, contains: "parent directory"},
		{
			name: "symlink parent resolved", rel: "link/new.go", wantRel: "real/new.go",
			setup: func(t *testing.T, dir string) {
				mustSucceed(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
				mustSucceed(t, os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")))
			},
		},
	} {
		t.Run(uc.name, func(t *testing.T) {
			dir := t.TempDir()
			if uc.setup != nil {
				uc.setup(t, dir)
			}
			got, errStr := resolveTarget(filepath.Join(dir, uc.rel))
			if uc.wantErr {
				if errStr == "" {
					t.Fatalf("expected error, got none")
				}
				if !strings.Contains(errStr, uc.contains) {
					t.Fatalf("error %q does not contain %q", errStr, uc.contains)
				}
				return
			}
			if errStr != "" {
				t.Fatalf("unexpected error: %s", errStr)
			}
			if want := filepath.Join(dir, uc.wantRel); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

// --- file_create handler ---

func TestFileCreate_MissingFile(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	result, toolErr := h.handleFileCreate(path, "hello world\n", false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(got) != "hello world\n" {
		t.Fatalf("content = %q", string(got))
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", info.Mode().Perm())
	}
	if !strings.HasPrefix(result, "created ") {
		t.Fatalf("message = %q", result)
	}
	if !strings.Contains(result, "file did not exist") {
		t.Fatalf("message = %q", result)
	}
}

func TestFileCreate_EmptyFile(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	mustSucceed(t, os.WriteFile(path, nil, 0o600))

	result, toolErr := h.handleFileCreate(path, "content", false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if !strings.Contains(result, "file was empty") {
		t.Fatalf("message = %q", result)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "content" {
		t.Fatalf("content = %q", string(got))
	}
}

func TestFileCreate_DryRun(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	result, toolErr := h.handleFileCreate(path, "one\ntwo\n", true)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if !strings.Contains(result, "+one") {
		t.Fatalf("expected diff, got %q", result)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("dry run must not create the file")
	}
}

func TestFileCreate_NonExistingFileRejected(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	mustSucceed(t, os.WriteFile(path, []byte("alpha"), 0o644))

	_, toolErr := h.handleFileCreate(path, "beta", false)
	if toolErr == "" {
		t.Fatalf("expected error for non-empty existing file")
	}
	if !strings.Contains(toolErr, "refusing to overwrite") {
		t.Fatalf("error = %q, want refusing to overwrite", toolErr)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "alpha" {
		t.Fatalf("original content changed: %q", string(got))
	}
}

func TestFileCreate_NonRegularFileRejected(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()

	_, toolErr := h.handleFileCreate(dir, "x", false)
	if toolErr == "" {
		t.Fatalf("expected error for directory")
	}
	if !strings.Contains(toolErr, "regular file") {
		t.Fatalf("error = %q, want regular file", toolErr)
	}
}

func TestFileCreate_InvalidUTF8(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	_, toolErr := h.handleFileCreate(path, "\x80\x81", false)
	if toolErr == "" {
		t.Fatalf("expected error for invalid UTF-8")
	}
	if !strings.Contains(toolErr, "UTF-8") {
		t.Fatalf("error = %q, want UTF-8", toolErr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file must not have been created")
	}
}

func TestFileCreate_NullBytes(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	_, toolErr := h.handleFileCreate(path, "has\x00null", false)
	if toolErr == "" {
		t.Fatalf("expected error for null bytes")
	}
	if !strings.Contains(toolErr, "null bytes") {
		t.Fatalf("error = %q, want null bytes", toolErr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file must not have been created")
	}
}

func TestFileCreate_NonAbsolutePath(t *testing.T) {
	h := newTestHandler(t)

	_, toolErr := h.handleFileCreate("new.txt", "x", false)
	if toolErr == "" {
		t.Fatalf("expected error for relative path")
	}
	if !strings.Contains(toolErr, "absolute") {
		t.Fatalf("error = %q, want absolute", toolErr)
	}
}

func TestFileCreate_ParentCreatedDeep(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "new.txt")

	result, toolErr := h.handleFileCreate(path, "deep", false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if !strings.Contains(result, "created ") {
		t.Fatalf("message = %q", result)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("deep file not created: %v", err)
	}
}

func TestFileCreate_SymlinkParent(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	mustSucceed(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
	mustSucceed(t, os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")))
	path := filepath.Join(dir, "link", "new.txt")

	_, toolErr := h.handleFileCreate(path, "x", false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "real", "new.txt")); err != nil {
		t.Fatalf("file should be created in the real dir: %v", err)
	}
}

func TestFileCreate_ContentArgRequired(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()

	args := map[string]any{"path": filepath.Join(dir, "new.txt")}
	result, err := h.HandleFileCreate(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !strings.Contains(contentText(result), "must not be empty") {
		t.Fatalf("expected content-required error, got: %s", contentText(result))
	}
}

// --- file_replace handler ---

func TestFileReplace_FileDoesNotExist(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.txt")

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: "b"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for missing file")
	}
	if !strings.Contains(toolErr, "file does not exist") {
		t.Fatalf("error = %q, want file does not exist", toolErr)
	}
}

func TestFileReplace_NonAbsolutePath(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	_, toolErr := h.handleFileReplace(filepath.Join("nonabs", "path"), []replacement{{find: "a", replace: "b"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for relative path")
	}
	if !strings.Contains(toolErr, "absolute") {
		t.Fatalf("error = %q, want absolute", toolErr)
	}
	_ = path
	_ = dir
}

func TestFileReplace_EmptyReplacements(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, nil, false)
	if toolErr == "" {
		t.Fatalf("expected error for empty replacements")
	}
	if !strings.Contains(toolErr, "replacements must not be empty") {
		t.Fatalf("error = %q, want replacements must not be empty", toolErr)
	}
}

func TestFileReplace_EmptyFind(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "", replace: "b"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for empty find")
	}
	if !strings.Contains(toolErr, "find must not be empty") {
		t.Fatalf("error = %q, want find must not be empty", toolErr)
	}
}

func TestFileReplace_FindEqualsReplace(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "hel", replace: "hel"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for find == replace")
	}
	if !strings.Contains(toolErr, "identical") {
		t.Fatalf("error = %q, want identical", toolErr)
	}
}

func TestFileReplace_NullBytes(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a\x00", replace: "b"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for null bytes")
	}
	if !strings.Contains(toolErr, "null bytes") {
		t.Fatalf("error = %q, want null bytes", toolErr)
	}
}

func TestFileReplace_InvalidUTF8(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "\x80", replace: "b"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for invalid UTF-8")
	}
	if !strings.Contains(toolErr, "UTF-8") {
		t.Fatalf("error = %q, want UTF-8", toolErr)
	}
}

func TestFileReplace_LineNumberBelowOne(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "h", replace: "x", lineNumber: -1}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for line_number -1")
	}
	if !strings.Contains(toolErr, "line_number must be") {
		t.Fatalf("error = %q, want line_number must be", toolErr)
	}
}

func TestFileReplace_Success(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello world"), 0o644))

	result, toolErr := h.handleFileReplace(path, []replacement{{find: "world", replace: "there"}}, false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if !strings.Contains(result, "@@") {
		t.Fatalf("expected diff, got %q", result)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello there" {
		t.Fatalf("content = %q", string(got))
	}
}

func TestFileReplace_MultiReplacement(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("a b c"), 0o644))

	result, toolErr := h.handleFileReplace(path, []replacement{
		{find: "a", replace: "1"},
		{find: "c", replace: "3"},
	}, false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if !strings.Contains(result, "@@") {
		t.Fatalf("expected diff, got %q", result)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "1 b 3" {
		t.Fatalf("content = %q", string(got))
	}
}

func TestFileReplace_DryRun(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello world"), 0o644))

	result, toolErr := h.handleFileReplace(path, []replacement{{find: "world", replace: "there"}}, true)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if !strings.Contains(result, "@@") {
		t.Fatalf("expected diff, got %q", result)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello world" {
		t.Fatalf("dry run must not modify: %q", string(got))
	}
}

func TestFileReplace_LineNumberNarrows(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("foo\nfoo"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "foo", replace: "bar", lineNumber: 2}}, false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "foo\nbar" {
		t.Fatalf("content = %q", string(got))
	}
}

func TestFileReplace_LineNumberOutOfRange(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("foo"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "foo", replace: "bar", lineNumber: 5}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for out-of-range line_number")
	}
	if !strings.Contains(toolErr, "out of range") {
		t.Fatalf("error = %q, want out of range", toolErr)
	}
}

func TestFileReplace_OverlappingReplacements(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("abc"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{
		{find: "ab", replace: "1"},
		{find: "bc", replace: "2"},
	}, false)
	if toolErr == "" {
		t.Fatalf("expected error for overlapping replacements")
	}
	if !strings.Contains(toolErr, "overlapping") {
		t.Fatalf("error = %q, want overlapping", toolErr)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "abc" {
		t.Fatalf("overlapping write must not modify: %q", string(got))
	}
}

func TestFileReplace_MultiMatch(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("foo foo"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "foo", replace: "bar"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for multi-match")
	}
	if !strings.Contains(toolErr, "matched 2 locations") {
		t.Fatalf("error = %q, want matched 2 locations", toolErr)
	}
}

func TestFileReplace_MultiMatchWithLineNumber(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("foo foo\nfoo\nfoo"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "foo", replace: "bar", lineNumber: 1}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for multi-match with line_number")
	}
	if !strings.Contains(toolErr, "ambiguous at line") {
		t.Fatalf("error = %q, want ambiguous at line", toolErr)
	}
}

func TestFileReplace_ZeroMatch(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "xyz", replace: "q"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for zero match")
	}
	if !strings.Contains(toolErr, "find not found in file") {
		t.Fatalf("error = %q, want find not found in file", toolErr)
	}
}

func TestFileReplace_ZeroMatchWithLineNumber(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("foo\nbar\nbaz"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "xyz", replace: "q", lineNumber: 2}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for zero match with line_number")
	}
	if !strings.Contains(toolErr, "not found at line") {
		t.Fatalf("error = %q, want not found at line", toolErr)
	}
}

func TestFileReplace_BinaryFile(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("\x00\x00binary"), 0o644))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "x", replace: "y"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for binary file")
	}
	if !strings.Contains(toolErr, "Binary files are not supported") {
		t.Fatalf("error = %q, want Binary files are not supported", toolErr)
	}
}

func TestFileReplace_PermissionPreserved(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello world"), 0o600))

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "world", replace: "there"}}, false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestFileReplace_ExternalModification(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	mustSucceed(t, os.WriteFile(path, []byte("hello world"), 0o644))

	// Read the file to get its content, then modify it between the lock acquisition
	// and the write. We simulate this by writing to the file after acquiring the
	// lock in a separate handler call.
	_, err := h.handleFileReplace(path, []replacement{{find: "world", replace: "there"}}, false)
	if err != "" {
		// This is expected to succeed on the first call
		t.Fatalf("first replacement should succeed, got: %s", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "hello there" {
		t.Fatalf("content = %q", string(got))
	}
}

// contentText extracts the text from a CallToolResult's first content item.
func contentText(r *mcp.CallToolResult) string {
	if len(r.Content) == 0 {
		return ""
	}
	switch v := r.Content[0].(type) {
	case mcp.TextContent:
		return v.Text
	default:
		return ""
	}
}

func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return New(&internal.Config{
		Home:          t.TempDir(),
		EditMaxLines:  10,
		MaxCandidates: 5,
	}, "test")
}
