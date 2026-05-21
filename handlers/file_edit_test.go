package handlers

import (
	"strings"
	"testing"

	"github.com/rthomazel/bench-mcp/internal/file"
)

func TestValidateFindReplace(t *testing.T) {
	useCases := []struct {
		name     string
		find     string
		replace  string
		maxLines int
		wantErr  bool
		contains string
	}{
		{"empty find", "", "valid", 10, true, "find must not be empty"},
		{"identical pair", "same", "same", 10, true, "identical"},
		{"null byte in find", "has\x00null", "replace", 10, true, "null bytes"},
		{"null byte in replace", "find", "has\x00null", 10, true, "null bytes"},
		{"invalid UTF-8 in find", "\x80\x81", "valid", 10, true, "valid UTF-8"},
		{"invalid UTF-8 in replace", "find", "\x80\x81", 10, true, "valid UTF-8"},
		{"replace exceeds line limit", "find", "a\nb\nc", 1, true, "exceeds"},
		{"valid input", "find", "replace", 10, false, ""},
		{"valid with newlines within limit", "find", "a\nb", 10, false, ""},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			got := validateFindReplace(tc.find, tc.replace, tc.maxLines)
			if tc.wantErr {
				if got == "" {
					t.Fatalf("expected error, got empty string")
				}
				if !strings.Contains(got, tc.contains) {
					t.Errorf("got %q, want it to contain %q", got, tc.contains)
				}
			} else {
				if got != "" {
					t.Errorf("expected no error, got %q", got)
				}
			}
		})
	}
}

func TestPartialMatchDiagnostic(t *testing.T) {
	useCases := []struct {
		name          string
		find          string
		content       string
		maxCandidates int
		wantHint      bool
		contains      string
	}{
		{
			name:          "find has no non-empty lines",
			find:          "   \n\t\n",
			content:       "hello\nworld",
			maxCandidates: 5,
			wantHint:      false,
		},
		{
			name:          "first line does not match",
			find:          "notfound",
			content:       "hello\nworld",
			maxCandidates: 5,
			wantHint:      false,
		},
		{
			name:          "first line matches",
			find:          "hello\nworld",
			content:       "hello\nfoo\nhello\nbar",
			maxCandidates: 5,
			wantHint:      true,
			contains:      "first line of find matched",
		},
		{
			name:          "line numbers appear in output",
			find:          "line1\nline2",
			content:       "line1\nfoo\nline1\nbar",
			maxCandidates: 5,
			wantHint:      true,
			contains:      "[1, 3]",
		},
		{
			name:          "candidate capping",
			find:          "x\ny",
			content:       "x\na\nx\nb\nx\nc\nx\nd",
			maxCandidates: 2,
			wantHint:      true,
			contains:      "showing first 2",
		},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			got := partialMatchDiagnostic(tc.find, tc.content, tc.maxCandidates)
			if tc.wantHint {
				if got == "" {
					t.Fatalf("expected hint, got empty string")
				}
				if !strings.Contains(got, tc.contains) {
					t.Errorf("got %q, want to contain %q", got, tc.contains)
				}
			} else {
				if got != "" {
					t.Errorf("expected empty string, got %q", got)
				}
			}
		})
	}
}

func TestZeroMatchError(t *testing.T) {
	useCases := []struct {
		name          string
		lineNumber    int
		find          string
		content       string
		maxCandidates int
		contains      string
	}{
		{
			name:          "no line_number, no partial match",
			lineNumber:    0,
			find:          "notfound",
			content:       "hello\nworld",
			maxCandidates: 5,
			contains:      "find not found in file",
		},
		{
			name:          "no line_number, partial first-line match",
			lineNumber:    0,
			find:          "hello\nfoo",
			content:       "hello\nworld",
			maxCandidates: 5,
			contains:      "first line of find matched",
		},
		{
			name:          "with line_number",
			lineNumber:    5,
			find:          "notfound",
			content:       "line1\nline2\nline3\nline4\nline5",
			maxCandidates: 5,
			contains:      "find not found at line 5",
		},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			r := replacement{find: tc.find, lineNumber: tc.lineNumber}
			got := zeroMatchError("TestOp", r, tc.content, tc.maxCandidates)
			if !strings.Contains(got, tc.contains) {
				t.Errorf("got %q, want to contain %q", got, tc.contains)
			}
		})
	}
}

func TestMultiMatchError(t *testing.T) {
	useCases := []struct {
		name          string
		lineNumber    int
		candidates    []file.Match
		content       string
		maxCandidates int
		contains      string
	}{
		{
			name:       "no line_number, multiple matches",
			lineNumber: 0,
			candidates: []file.Match{
				{StartByte: 0, EndByte: 3, StartLine: 1, EndLine: 1, StartChar: 0},
				{StartByte: 8, EndByte: 11, StartLine: 3, EndLine: 3, StartChar: 0},
			},
			content:       "foo\nbar\nfoo",
			maxCandidates: 5,
			contains:      "find matched 2 locations",
		},
		{
			name:       "line_number, all same line",
			lineNumber: 1,
			candidates: []file.Match{
				{StartByte: 1, EndByte: 2, StartLine: 1, EndLine: 1, StartChar: 1},
				{StartByte: 2, EndByte: 3, StartLine: 1, EndLine: 1, StartChar: 2},
			},
			content:       "foo bar",
			maxCandidates: 5,
			contains:      "ambiguous at line 1",
		},
		{
			name:       "line_number, same line, characters in output",
			lineNumber: 1,
			candidates: []file.Match{
				{StartByte: 1, EndByte: 2, StartLine: 1, EndLine: 1, StartChar: 1},
				{StartByte: 3, EndByte: 4, StartLine: 1, EndLine: 1, StartChar: 3},
			},
			content:       "banana",
			maxCandidates: 5,
			contains:      "characters",
		},
		{
			name:       "line_number, spread across lines",
			lineNumber: 1,
			candidates: []file.Match{
				{StartByte: 0, EndByte: 4, StartLine: 1, EndLine: 1, StartChar: 0},
				{StartByte: 6, EndByte: 10, StartLine: 2, EndLine: 2, StartChar: 0},
			},
			content:       "line1\nline2\nline3",
			maxCandidates: 5,
			contains:      "did not narrow to one match",
		},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			r := replacement{find: "x", lineNumber: tc.lineNumber}
			got := multiMatchError("TestOp", r, tc.candidates, tc.content, tc.maxCandidates)
			if !strings.Contains(got, tc.contains) {
				t.Errorf("got %q, want to contain %q", got, tc.contains)
			}
		})
	}
}
