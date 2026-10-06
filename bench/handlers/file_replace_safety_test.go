package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rthomazel/mcp/bench/internal/file"
)

func TestFileReplaceOriginalSnapshotAndSymlink(t *testing.T) {
	h := newTestHandler(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	before := "anchor\nold\nend\n"
	mustSucceed(t, os.WriteFile(target, []byte(before), 0o640))
	mustSucceed(t, os.Symlink(target, link))
	result, toolErr := h.handleFileReplace(link, []replacement{
		{find: "old", replace: "new"},
		{find: "new", replace: "chained"},
		{find: "anchor\nmissing\n", replace: "preview\n"},
	}, false)
	if toolErr != "" {
		t.Fatal(toolErr)
	}
	got, err := os.ReadFile(target)
	mustSucceed(t, err)
	if string(got) != "anchor\nnew\nend\n" {
		t.Fatalf("got %q", got)
	}
	info, err := os.Lstat(link)
	mustSucceed(t, err)
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("replaced symlink")
	}
	info, err = os.Stat(target)
	mustSucceed(t, err)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %v", info.Mode())
	}
	preview := file.ComputeDiff(target, before, "preview\nend\n")
	if !strings.Contains(result, indent(strings.TrimRight(preview, "\n"), "  ")) {
		t.Fatalf("preview not against original target: %s", result)
	}
	if !strings.Contains(result, "Replacement 2: not_matched") || !strings.Contains(result, "Replacement 3: not_matched") {
		t.Fatal(result)
	}
}

func TestFileReplaceBoundedNoCandidateExcerpt(t *testing.T) {
	content := strings.Repeat("line\n", 25)
	s := resolveReplacement("/file", replacement{find: "missing", replace: "X"}, content, 25, 5)
	if s.outcome != outcomeNotMatched || !strings.Contains(s.diagnostics, "No preview candidate") || strings.Count(s.diagnostics, "line\n") != 10 {
		t.Fatalf("%+v", s)
	}
}

func TestFileReplaceMalformedHunkAbortsBeforeWrites(t *testing.T) {
	h := newTestHandler(t)
	path := filepath.Join(t.TempDir(), "file")
	mustSucceed(t, os.WriteFile(path, []byte("abc"), 0o644))
	_, toolErr := h.handleFileReplace(path, []replacement{{find: "a", replace: "X"}, {find: "", replace: "Y"}}, false)
	if toolErr == "" {
		t.Fatal("expected validation error")
	}
	got, err := os.ReadFile(path)
	mustSucceed(t, err)
	if string(got) != "abc" {
		t.Fatalf("wrote %q", got)
	}
}
