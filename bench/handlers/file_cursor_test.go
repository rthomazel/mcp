package handlers

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestCursorHandlers(t *testing.T) {
	for _, tc := range []struct {
		name, before, anchor, content, after string
		line, count                          int
		deletion, invalid                    bool
	}{
		{name: "insert", before: "hello world", anchor: "hello", content: " brave", after: "hello brave world", line: 1},
		{name: "first match", before: "aa aa", anchor: "aa", content: "!", after: "aa! aa", line: 1},
		{name: "empty", content: "hi\n", after: "hi\n", line: 1},
		{name: "line start", before: "a\r\nb", content: " x\n", after: "a\r\n x\nb", line: 2},
		{name: "empty insertion", before: "abc", after: "abc", line: 1},
		{name: "delete", before: "hello world", anchor: "hello", after: "hello", line: 1, count: 6, deletion: true},
		{name: "overshoot", before: "hello world", anchor: "hello", after: "hello", line: 1, count: 100, deletion: true},
		{name: "unicode", before: "界é🙂", after: "é🙂", line: 1, count: 1, deletion: true},
		{name: "combining", before: "é🙂", anchor: "e", after: "e🙂", line: 1, count: 1, deletion: true},
		{name: "crlf", before: "a\r\nb", anchor: "a", after: "ab", line: 1, count: 2, deletion: true},
		{name: "partial crlf", before: "a\r\nb", anchor: "a", after: "a\nb", line: 1, count: 1, deletion: true},
		{name: "cross lines", before: "a\nb\nc", anchor: "a", after: "ac", line: 1, count: 3, deletion: true},
		{name: "eof", before: "abc", anchor: "abc", after: "abc", line: 1, count: 1, deletion: true},
		{name: "empty eof", line: 1, count: 1, deletion: true},
		{name: "trailing LF no extra line", before: "a\n", line: 2, invalid: true},
		{name: "missing anchor no-op", before: "a", anchor: "z", line: 1, invalid: true},
		{name: "no other line", before: "a\nb", anchor: "b", line: 1, invalid: true},
		{name: "binary", before: "a\x00", line: 1, invalid: true},
		{name: "invalid utf8", before: "\xff", line: 1, invalid: true},
	} {
		for _, dry := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{true: " preview"}[dry], func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(path, []byte(tc.before), 0o600); err != nil {
					t.Fatal(err)
				}
				args := map[string]any{"path": path, "line": float64(tc.line), "anchor": tc.anchor, "content": tc.content, "count": float64(tc.count), "dry_run": dry}
				var req mcp.CallToolRequest
				req.Params.Arguments = args
				h := &Handler{}
				call := h.HandleFileInsert
				if tc.deletion {
					call = h.HandleFileDelete
				}
				result, err := call(context.Background(), req)
				if err != nil || result.IsError != tc.invalid {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.after
				if dry || tc.invalid {
					want = tc.before
				}
				if string(raw) != want {
					t.Fatalf("got %q want %q", raw, want)
				}
				if !tc.invalid {
					text := result.Content[0].(mcp.TextContent).Text
					if tc.before == tc.after && !strings.Contains(text, "no-op") {
						t.Fatalf("missing no-op: %s", text)
					}
					if dry && !strings.Contains(text, "not written") {
						t.Fatalf("missing preview: %s", text)
					}
				}
			})
		}
	}
}

func TestPositiveInteger(t *testing.T) {
	for _, value := range []any{nil, true, "1", 0, -1, 0.5, math.NaN(), math.Inf(1), float64(1 << 53), float64(1 << 63)} {
		if _, err := parsePositiveInteger(value, "count"); err == "" {
			t.Errorf("accepted %v", value)
		}
	}
	for _, value := range []any{1, float64(1), float64(100)} {
		if _, err := parsePositiveInteger(value, "line"); err != "" {
			t.Error(err)
		}
	}
}

func TestCursorValidation(t *testing.T) {
	for _, field := range []string{"path", "anchor", "line", "content", "dry_run", "count"} {
		for _, value := range []any{nil, true, "\x00", "\xff"} {
			args := map[string]any{"path": "/missing", "anchor": "", "line": float64(1), "content": "", "count": float64(1)}
			args[field] = value
			var req mcp.CallToolRequest
			req.Params.Arguments = args
			h := &Handler{}
			call := h.HandleFileInsert
			if field == "count" {
				call = h.HandleFileDelete
			}
			result, err := call(context.Background(), req)
			if err != nil || !result.IsError {
				t.Fatalf("%s=%v: %+v %v", field, value, result, err)
			}
		}
	}
}
