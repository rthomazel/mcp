package handlers

import (
	"strings"
	"testing"
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
