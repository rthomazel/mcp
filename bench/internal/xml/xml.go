package xml

import (
	"fmt"
	"strings"
)

// Builder wraps strings.Builder with XML tag helpers.
type Builder struct{ strings.Builder }

// OpenTag writes an opening XML tag. Optional attrs are key-value pairs:
// b.OpenTag("command", "index", "0") writes <command index="0">.
func (b *Builder) OpenTag(tag string, attrs ...string) {
	_, _ = fmt.Fprintf(&b.Builder, "<%s", tag)

	for i := 0; i+1 < len(attrs); i += 2 {
		_, _ = fmt.Fprintf(&b.Builder, " %s=%q", attrs[i], attrs[i+1])
	}

	b.WriteString(">\n")
}

// CloseTag writes a closing XML tag with optional trailing newline.
func (b *Builder) CloseTag(tag string, newline bool) {
	_, _ = fmt.Fprintf(&b.Builder, "</%s>\n", tag)

	if newline {
		b.WriteString("\n")
	}
}

// Tag writes <name attrs...>\ncontents\n</name>\n — with optional trailing newline.
func (b *Builder) Tag(name, contents string, newline bool, attrs ...string) {
	b.OpenTag(name, attrs...)

	contents = strings.TrimRight(contents, "\n")

	b.WriteString(contents)
	b.WriteString("\n")
	b.CloseTag(name, newline)
}
