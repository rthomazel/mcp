package xml

import (
	"testing"
)

func TestBuilderOpenTag(t *testing.T) {
	useCases := []struct {
		name  string
		tag   string
		attrs []string
		want  string
	}{
		{
			name: "no attrs",
			tag:  "metadata",
			want: "<metadata>\n",
		},
		{
			name:  "single attr",
			tag:   "command",
			attrs: []string{"index", "0"},
			want:  `<command index="0">` + "\n",
		},
		{
			name:  "multiple attrs",
			tag:   "item",
			attrs: []string{"index", "1", "type", "job"},
			want:  `<item index="1" type="job">` + "\n",
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			var b Builder
			b.OpenTag(u.tag, u.attrs...)
			if got := b.String(); got != u.want {
				t.Errorf("got %q, want %q", got, u.want)
			}
		})
	}
}

func TestBuilderCloseTag(t *testing.T) {
	useCases := []struct {
		name    string
		newline bool
		want    string
	}{
		{
			name:    "no trailing newline",
			newline: false,
			want:    "</metadata>\n",
		},
		{
			name:    "trailing newline",
			newline: true,
			want:    "</metadata>\n\n",
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			var b Builder
			b.CloseTag("metadata", u.newline)
			if got := b.String(); got != u.want {
				t.Errorf("got %q, want %q", got, u.want)
			}
		})
	}
}

func TestBuilderTag(t *testing.T) {
	useCases := []struct {
		name     string
		tag      string
		contents string
		newline  bool
		attrs    []string
		want     string
	}{
		{
			name:     "simple contents",
			tag:      "stdout",
			contents: "hello",
			newline:  false,
			want:     "<stdout>\nhello\n</stdout>\n",
		},
		{
			name:     "trailing newlines in contents are trimmed",
			tag:      "stdout",
			contents: "hello\n\n",
			newline:  false,
			want:     "<stdout>\nhello\n</stdout>\n",
		},
		{
			name:     "trailing newline after close",
			tag:      "stderr",
			contents: "oops",
			newline:  true,
			want:     "<stderr>\noops\n</stderr>\n\n",
		},
		{
			name:     "attrs passed through",
			tag:      "command",
			contents: "body",
			newline:  false,
			attrs:    []string{"index", "2"},
			want:     "<command index=\"2\">\nbody\n</command>\n",
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			var b Builder
			b.Tag(u.tag, u.contents, u.newline, u.attrs...)
			if got := b.String(); got != u.want {
				t.Errorf("got %q, want %q", got, u.want)
			}
		})
	}
}

func TestParseStringSlice(t *testing.T) {
	useCases := []struct {
		name  string
		input any
		want  []string
		ok    bool
	}{
		{
			name:  "valid slice",
			input: []any{"foo", "bar", "baz"},
			want:  []string{"foo", "bar", "baz"},
			ok:    true,
		},
		{
			name:  "empty slice",
			input: []any{},
			want:  []string{},
			ok:    true,
		},
		{
			name:  "not a slice",
			input: "just a string",
			ok:    false,
		},
		{
			name:  "non-string element",
			input: []any{"foo", 42},
			ok:    false,
		},
	}

	for _, u := range useCases {
		t.Run(u.name, func(t *testing.T) {
			got, ok := ParseStringSlice(u.input)
			if ok != u.ok {
				t.Fatalf("ok = %v, want %v", ok, u.ok)
			}
			if !u.ok {
				return
			}
			if len(got) != len(u.want) {
				t.Fatalf("got %v, want %v", got, u.want)
			}
			for i := range got {
				if got[i] != u.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], u.want[i])
				}
			}
		})
	}
}
