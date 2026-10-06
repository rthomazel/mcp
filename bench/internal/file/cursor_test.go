package file

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestResolveCursor(t *testing.T) {
	for _, tc := range []struct {
		content, anchor string
		line, cursor    int
		invalid         bool
	}{
		{"", "", 1, 0, false},
		{"", "x", 1, 0, true},
		{"\n", "", 1, 0, false},
		{"\n", "", 2, 0, true},
		{"a\n\nb", "", 2, 2, false},
		{"a\r\nb", "a", 1, 1, false},
		{"a\rb", "b", 1, 3, false},
		{"aa aa", "aa", 1, 2, false},
		{"界🙂", "界", 1, 3, false},
		{"a\nb", "b", 1, 0, true},
		{"a", "", 0, 0, true},
		{"a", "\r", 1, 0, true},
		{"a", "\n", 1, 0, true},
		{"a", "\x00", 1, 0, true},
		{"a", "\xff", 1, 0, true},
	} {
		cursor, err := ResolveCursor(tc.content, tc.line, tc.anchor)
		if (err != "") != tc.invalid || (!tc.invalid && cursor != tc.cursor) {
			t.Errorf("%+v: cursor=%d err=%s", tc, cursor, err)
		}
	}
	_, err := ResolveCursor(strings.Repeat("界", 10000), 1, strings.Repeat("x", 10000))
	if !strings.Contains(err, "truncated") || strings.Count(err, "界") != 200 || strings.Contains(err, "xxx") || !utf8.ValidString(err) {
		t.Fatalf("unbounded or invalid diagnostic: %.100s", err)
	}
}

func TestAdvanceCodePoints(t *testing.T) {
	text := "界\t\r\né🙂"
	for count := 1; count <= 10; count++ {
		end := AdvanceCodePoints(text, 0, count)
		if !utf8.ValidString(text[:end]) || utf8.RuneCountInString(text[:end]) != min(count, utf8.RuneCountInString(text)) {
			t.Fatalf("count %d: end %d", count, end)
		}
	}
	if end := AdvanceCodePoints(text, len(text), int(^uint(0)>>1)); end != len(text) {
		t.Fatal(end)
	}
}
