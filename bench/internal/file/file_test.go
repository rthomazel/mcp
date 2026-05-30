package file

import (
	"strings"
	"testing"
)

func TestFindMatches(t *testing.T) {
	useCases := []struct {
		name     string
		content  string
		find     string
		expected []Match
	}{
		{
			name:     "empty find",
			content:  "hello world",
			find:     "",
			expected: nil,
		},
		{
			name:     "no match",
			content:  "hello world",
			find:     "xyz",
			expected: nil,
		},
		{
			name:    "match at start",
			content: "hello world",
			find:    "hello",
			expected: []Match{
				{StartByte: 0, EndByte: 5, StartLine: 1, EndLine: 1, StartChar: 0},
			},
		},
		{
			name:    "match at end",
			content: "hello world",
			find:    "world",
			expected: []Match{
				{StartByte: 6, EndByte: 11, StartLine: 1, EndLine: 1, StartChar: 6},
			},
		},
		{
			name:    "single char find",
			content: "hello",
			find:    "l",
			expected: []Match{
				{StartByte: 2, EndByte: 3, StartLine: 1, EndLine: 1, StartChar: 2},
				{StartByte: 3, EndByte: 4, StartLine: 1, EndLine: 1, StartChar: 3},
			},
		},
		{
			name:    "non-overlapping: aa in aaa yields one match",
			content: "aaa",
			find:    "aa",
			expected: []Match{
				{StartByte: 0, EndByte: 2, StartLine: 1, EndLine: 1, StartChar: 0},
			},
		},
		{
			name:    "multi-line find",
			content: "line1\nline2\nline3",
			find:    "line2\nline3",
			expected: []Match{
				{StartByte: 6, EndByte: 17, StartLine: 2, EndLine: 3, StartChar: 0},
			},
		},
		{
			name:    "match spanning line boundary",
			content: "hello\nworld",
			find:    "o\nw",
			expected: []Match{
				{StartByte: 4, EndByte: 7, StartLine: 1, EndLine: 2, StartChar: 4},
			},
		},
		{
			name:     "find longer than content",
			content:  "hi",
			find:     "hello",
			expected: nil,
		},
		{
			name:    "multiple matches on different lines",
			content: "foo\nbar\nfoo",
			find:    "foo",
			expected: []Match{
				{StartByte: 0, EndByte: 3, StartLine: 1, EndLine: 1, StartChar: 0},
				{StartByte: 8, EndByte: 11, StartLine: 3, EndLine: 3, StartChar: 0},
			},
		},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FindMatches(tc.content, tc.find)
			if len(result) != len(tc.expected) {
				t.Fatalf("expected %d matches, got %d: %+v", len(tc.expected), len(result), result)
			}
			for i, m := range result {
				e := tc.expected[i]
				if m != e {
					t.Errorf("match %d: got %+v, want %+v", i, m, e)
				}
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	useCases := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1},
		{"two lines no trailing newline", "line1\nline2", 2},
		{"two lines with trailing newline", "line1\nline2\n", 2},
		{"only a newline", "\n", 1},
		{"three lines", "a\nb\nc", 3},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountLines(tc.input); got != tc.expected {
				t.Errorf("got %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestCountNewlines(t *testing.T) {
	useCases := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"no newlines", "hello world", 0},
		{"one newline", "hello\nworld", 1},
		{"several newlines", "a\nb\nc\nd", 3},
		{"only newlines", "\n\n\n", 3},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountNewlines(tc.input); got != tc.expected {
				t.Errorf("got %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	useCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"all whitespace lines", "   \n  \t  \n", ""},
		{"first line non-empty", "hello", "hello"},
		{"first line empty second non-empty", "\nworld", "world"},
		{"whitespace then content", "  \n  \ncontent", "content"},
		{"content with trailing newline", "first\n", "first"},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FirstNonEmptyLine(tc.input); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	useCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", []string{}},
		{"single line no newline", "hello", []string{"hello"}},
		{"single line with newline", "hello\n", []string{"hello"}},
		{"two lines no trailing newline", "line1\nline2", []string{"line1", "line2"}},
		{"two lines with trailing newline", "line1\nline2\n", []string{"line1", "line2"}},
		{"three lines", "a\nb\nc", []string{"a", "b", "c"}},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitLines(tc.input)
			if len(got) != len(tc.expected) {
				t.Fatalf("got %d lines, want %d: %v", len(got), len(tc.expected), got)
			}
			for i, line := range got {
				if line != tc.expected[i] {
					t.Errorf("line %d: got %q, want %q", i, line, tc.expected[i])
				}
			}
		})
	}
}

func TestExcerpt(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	useCases := []struct {
		name     string
		lineNum  int
		radius   int
		expected string
	}{
		{
			name:     "clamp at start",
			lineNum:  1,
			radius:   5,
			expected: "   1: line1\n   2: line2\n   3: line3\n   4: line4\n   5: line5\n",
		},
		{
			name:     "clamp at end",
			lineNum:  5,
			radius:   5,
			expected: "   1: line1\n   2: line2\n   3: line3\n   4: line4\n   5: line5\n",
		},
		{
			name:     "middle with radius 1",
			lineNum:  3,
			radius:   1,
			expected: "   2: line2\n   3: line3\n   4: line4\n",
		},
		{
			name:     "zero radius",
			lineNum:  3,
			radius:   0,
			expected: "   3: line3\n",
		},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Excerpt(content, tc.lineNum, tc.radius); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestExcerptRange(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	useCases := []struct {
		name      string
		startLine int
		endLine   int
		maxLines  int
		expected  string
	}{
		{
			name:      "basic range",
			startLine: 2, endLine: 4, maxLines: 100,
			expected: "   2: line2\n   3: line3\n   4: line4\n",
		},
		{
			name:      "maxLines cap",
			startLine: 1, endLine: 5, maxLines: 2,
			expected: "   1: line1\n   2: line2\n",
		},
		{
			name:      "clamp at file end",
			startLine: 4, endLine: 10, maxLines: 100,
			expected: "   4: line4\n   5: line5\n",
		},
		{
			name:      "single line",
			startLine: 3, endLine: 3, maxLines: 100,
			expected: "   3: line3\n",
		},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExcerptRange(content, tc.startLine, tc.endLine, tc.maxLines); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestComputeDiff(t *testing.T) {
	useCases := []struct {
		name     string
		before   string
		after    string
		wantDiff bool
	}{
		{"identical content", "hello\nworld", "hello\nworld", false},
		{"single-line change", "hello\nworld", "hello\nuniverse", true},
		{"line added", "line1\nline2", "line1\nline2\nline3", true},
		{"line removed", "line1\nline2\nline3", "line1\nline3", true},
	}

	for _, tc := range useCases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeDiff("test.go", tc.before, tc.after)
			if !tc.wantDiff {
				if got != "" {
					t.Errorf("expected empty diff, got %q", got)
				}
				return
			}
			for _, want := range []string{"---", "+++", "@@"} {
				if !strings.Contains(got, want) {
					t.Errorf("diff missing %q: %q", want, got)
				}
			}
		})
	}
}
