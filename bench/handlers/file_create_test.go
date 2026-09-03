package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rthomazel/mcp/bench/internal"
)

// mustSucceed wraps an os setup call, failing the test if it returns an error.
func mustSucceed(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}

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

func TestCreateMode_MissingFile(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	result, toolErr := h.handleFileReplace(path, []replacement{{find: "unused", replace: "hello world\n"}}, false)
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
	if !strings.HasPrefix(result, "created "+path+" (") {
		t.Fatalf("message = %q", result)
	}
	if !strings.Contains(result, "file did not exist") {
		t.Fatalf("message = %q", result)
	}
}

func TestCreateMode_EmptyFile(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	mustSucceed(t, os.WriteFile(path, nil, 0o600))

	result, toolErr := h.handleFileReplace(path, []replacement{{find: "x", replace: "content"}}, false)
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

func TestCreateMode_MultipleReplacementsFallThrough(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	_, toolErr := h.handleFileReplace(path, []replacement{
		{find: "a", replace: "1"},
		{find: "b", replace: "2"},
	}, false)
	if toolErr == "" {
		t.Fatalf("expected error for multiple replacements on missing file")
	}
	if !strings.Contains(toolErr, "find not found") {
		t.Fatalf("error = %q, want find not found", toolErr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should not have been created")
	}
}

func TestCreateMode_ParentMissing(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "nope", "new.txt")

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: "1"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error")
	}
	if !strings.Contains(toolErr, "parent directory") {
		t.Fatalf("error = %q", toolErr)
	}
}

func TestCreateMode_DryRun(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	result, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: "one\ntwo\n"}}, true)
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

func TestCreateMode_NoLineLimit(t *testing.T) {
	h := newTestHandler(t) // EditMaxLines = 10
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")

	big := strings.Repeat("line\n", 50) // 50 newlines, far over the limit
	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: big}}, false)
	if toolErr != "" {
		t.Fatalf("create mode must not apply the line limit: %s", toolErr)
	}
	got, _ := os.ReadFile(path)
	if string(got) != big {
		t.Fatalf("content mismatch")
	}
}

func TestCreateMode_SymlinkParent(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	mustSucceed(t, os.MkdirAll(filepath.Join(dir, "real"), 0o755))
	mustSucceed(t, os.Symlink(filepath.Join(dir, "real"), filepath.Join(dir, "link")))
	path := filepath.Join(dir, "link", "new.txt")

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: "x"}}, false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "real", "new.txt")); err != nil {
		t.Fatalf("file should be created in the real dir: %v", err)
	}
}

func TestCreateMode_ExistingNonEmptyFileUsesEditMode(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	mustSucceed(t, os.WriteFile(path, []byte("alpha beta gamma"), 0o644))

	result, toolErr := h.handleFileReplace(path, []replacement{{find: "beta", replace: "BETA"}}, false)
	if toolErr != "" {
		t.Fatalf("unexpected error: %s", toolErr)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "alpha BETA gamma" {
		t.Fatalf("content = %q", string(got))
	}
	if strings.HasPrefix(result, "created ") {
		t.Fatalf("expected a diff, got create message: %q", result)
	}
}

func TestCreateMode_DirectoryRejected(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()

	_, toolErr := h.handleFileReplace(dir, []replacement{{find: "a", replace: "x"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for directory")
	}
	if !strings.Contains(toolErr, "regular file") {
		t.Fatalf("error = %q", toolErr)
	}
}

func TestCreateMode_InvalidUTF8(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: "\x80\x81"}}, false)
	if toolErr == "" {
		t.Fatalf("expected error for invalid UTF-8")
	}
	if !strings.Contains(toolErr, "UTF-8") {
		t.Fatalf("error = %q", toolErr)
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
