package handlers

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/bench/internal/file"
)

func TestFileReplaceCoordinateValidation(t *testing.T) {
	for _, key := range []string{"start_line", "end_line"} {
		for _, value := range []any{1.5, -0.5, math.Inf(1), math.Inf(-1), math.NaN(), math.Exp2(63), "2", true} {
			h := newTestHandler(t)
			path := filepath.Join(t.TempDir(), "file")
			mustSucceed(t, os.WriteFile(path, []byte("a\na\n"), 0o644))
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"path": path, "replacements": []any{map[string]any{"find": "a", "replace": "b", key: value}}}
			result, err := h.HandleFileReplace(context.Background(), req)
			if err != nil || !result.IsError {
				t.Fatalf("%s=%v: result=%+v err=%v", key, value, result, err)
			}
			got, err := os.ReadFile(path)
			mustSucceed(t, err)
			if string(got) != "a\na\n" {
				t.Fatalf("invalid coordinate wrote %q", got)
			}
		}
	}
}

func TestFileReplaceApprovedRegions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		r       replacement
		outcome string
		line    int
	}{
		{"start only", replacement{find: "a", replace: "x", startLine: 3}, outcomeResolved, 3},
		{"end only", replacement{find: "a", replace: "x", endLine: 1}, outcomeResolved, 1},
		{"clamp", replacement{find: "b", replace: "x", startLine: -8, endLine: 99}, outcomeResolved, 2},
		{"outside", replacement{find: "a", replace: "x", startLine: 4}, outcomeInvalidRegion, 0},
		{"negative end", replacement{find: "a", replace: "x", endLine: -1}, outcomeInvalidRegion, 0},
		{"spanning", replacement{find: "a\nb", replace: "x", startLine: 2, endLine: 2}, outcomeResolved, 1},
		{"newline belongs to prior line", replacement{find: "a\n", replace: "x", startLine: 2, endLine: 2}, outcomeNotMatched, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := resolveReplacement("/file", tc.r, "a\nb\na\n", 3, 2)
			if s.outcome != tc.outcome {
				t.Fatalf("%+v", s)
			}
			if s.located != nil && s.located.m.StartLine != tc.line {
				t.Fatalf("%+v", s.located)
			}
		})
	}
}

func TestFileReplaceCandidatePreviews(t *testing.T) {
	for _, tc := range []struct {
		name, content, find, replace, after, label string
		start, end                                 int
	}{
		{"trimmed anchor CRLF", "\tanchor\r\nold\r\n", "  anchor\nnew\n", "X\n", "X\n", "approximate whole-line candidate, lines 1-2", 1, 2},
		{"leading blank anchor offset", "\nanchor\nold", "\nanchor\nnew", "X", "X", "approximate whole-line candidate, lines 1-3", 1, 3},
		{"empty preview", "anchor\nold\n", "anchor\nnew\n", "anchor\nold\n", "anchor\nold\n", "empty preview: content unchanged.", 1, 2},
		{"out of region", "anchor\nold\n", "anchor\nnew\n", "X", "", "No preview candidate", 1, 1},
		{"blank anchor", "a\nb\n", " \n\n", "X", "", "No preview candidate", 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := replacement{find: tc.find, replace: tc.replace, startLine: tc.start, endLine: tc.end}
			s := resolveReplacement("/file", r, tc.content, file.CountLines(tc.content), 2)
			if s.outcome != outcomeNotMatched || !strings.Contains(s.diagnostics, tc.label) {
				t.Fatalf("%+v", s)
			}
			if tc.after != "" && tc.after != tc.content && !strings.Contains(s.diagnostics, file.ComputeDiff("/file", tc.content, tc.after)) {
				t.Fatalf("wrong preview: %s", s.diagnostics)
			}
			h := newTestHandler(t)
			path := filepath.Join(t.TempDir(), "file")
			mustSucceed(t, os.WriteFile(path, []byte(tc.content), 0o644))
			_, toolErr := h.handleFileReplace(path, []replacement{r}, false)
			if toolErr != "" {
				t.Fatal(toolErr)
			}
			got, err := os.ReadFile(path)
			mustSucceed(t, err)
			if string(got) != tc.content {
				t.Fatalf("preview wrote %q", got)
			}
		})
	}
	for _, find := range []string{"anchor", "anchor\nmissing"} {
		s := resolveReplacement("/file", replacement{find: find, replace: "X"}, "anchor\na\nanchor\nb\nanchor\nc\n", 6, 2)
		if !strings.Contains(s.diagnostics, "showing first 2 of 3") || strings.Count(s.diagnostics, "--- /file") != 2 {
			t.Fatalf("uncapped previews: %s", s.diagnostics)
		}
	}
	s := resolveReplacement("/file", replacement{find: "anchor\nmissing", replace: "X"}, "anchor anchor\na\n", 2, 5)
	if strings.Count(s.diagnostics, "approximate whole-line candidate") != 1 {
		t.Fatalf("not deduplicated: %s", s.diagnostics)
	}
}

func TestFileReplaceOverlapAndDryRunContract(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		h := newTestHandler(t)
		path := filepath.Join(t.TempDir(), "file")
		mustSucceed(t, os.WriteFile(path, []byte("abcdef\nz\n"), 0o644))
		result, toolErr := h.handleFileReplace(path, []replacement{
			{find: "bc", replace: "loser"},
			{find: "ab", replace: "A"},
			{find: "abc", replace: "tie loser"},
			{find: "de", replace: "B"},
			{find: "z", replace: "Z", startLine: 99},
			{find: "missing", replace: "M"},
		}, dryRun)
		if toolErr != "" {
			t.Fatal(toolErr)
		}
		outcome := outcomeApplied
		want := "AcBf\nz\n"
		if dryRun {
			outcome = outcomeWouldApply
			want = "abcdef\nz\n"
			if !strings.Contains(result, "not written") {
				t.Fatal(result)
			}
		}
		for _, expected := range []string{"2 " + outcome, "2 overlap", "1 invalid_region", "1 not_matched", "Replacement 2: " + outcome, "Replacement 3: overlap", "overlaps with replacement 2"} {
			if !strings.Contains(result, expected) {
				t.Fatalf("missing %q: %s", expected, result)
			}
		}
		got, err := os.ReadFile(path)
		mustSucceed(t, err)
		if string(got) != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}
