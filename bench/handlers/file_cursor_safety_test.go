package handlers

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal/stats"
)

func TestCursorTelemetry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	writer, err := stats.Open(dbPath, stats.WriterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{stats: writer}
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, call := range []func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error){h.HandleFileInsert, h.HandleFileDelete} {
		for _, line := range []any{float64(1), "bad", float64(2)} {
			var req mcp.CallToolRequest
			req.Params.Arguments = map[string]any{"path": path, "line": line, "anchor": "abc", "content": "", "count": float64(1), "dry_run": true}
			if _, err := call(context.Background(), req); err != nil {
				t.Fatal(err)
			}
		}
	}
	h.Close()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	for _, kind := range []string{"", "validation_error", "edit_error"} {
		var count int
		err = db.QueryRow("SELECT count(*) FROM tool_calls WHERE coalesce(error_kind,'') = ? AND file_path = ? AND dry_run = 1 AND replacement_count IS NULL AND replacement_bytes IS NULL", kind, path).Scan(&count)
		if err != nil || count != 2 {
			t.Fatalf("%s count=%d err=%v", kind, count, err)
		}
	}
}

func TestCursorEditSafety(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := path + "-link"
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	request := cursorRequest{path: alias, line: 1, anchor: "a"}
	_, err := applyCursorEdit(request, func(original string, cursor int) string { return original[:cursor] + "!" + original[cursor:] })
	if err != "" {
		t.Fatal(err)
	}
	info, e := os.Lstat(alias)
	if e != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v", e)
	}
	info, e = os.Stat(path)
	if e != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode changed: %v", e)
	}
	_, err = applyCursorEdit(request, func(original string, cursor int) string {
		if e := os.WriteFile(path, []byte("external"), 0o600); e != nil {
			t.Fatal(e)
		}
		return "overwrite"
	})
	if !strings.Contains(err, "modified externally") {
		t.Fatal(err)
	}
	raw, e := os.ReadFile(path)
	if e != nil || string(raw) != "external" {
		t.Fatalf("external edit lost: %s %v", raw, e)
	}
	request.path = path + "-missing"
	if _, err = applyCursorEdit(request, func(string, int) string { t.Fatal("transform invoked"); return "" }); err == "" {
		t.Fatal("missing accepted")
	}
	request.path = filepath.Dir(path)
	if _, err = applyCursorEdit(request, func(string, int) string { t.Fatal("transform invoked"); return "" }); err == "" {
		t.Fatal("directory accepted")
	}
}
