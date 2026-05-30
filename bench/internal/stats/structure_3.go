package stats

import (
	"fmt"
	"regexp"
	"strings"
)

// pass 3 — structural normalization
//
// Shell and interpreter constructs are replaced before the long-token rule so
// their content does not trigger spurious long-token matches. These patterns
// are best-effort; complex quoting and nesting may not be handled correctly.

var (
	reSubshell     = regexp.MustCompile(`\$\([^)]*\)`)
	reBacktick     = regexp.MustCompile("`[^`]*`")
	reHerestring   = regexp.MustCompile(`<<<\s*(?:'[^']*'|"[^"]*"|\S+)`)
	reProcessSub   = regexp.MustCompile(`<\([^)]*\)`)
	reInlineScript = regexp.MustCompile(`(python3?|perl|ruby|node|awk|sed)\s+(?:-[ce]\s+)?('[^']*'|"[^"]*")`)
	rePythonBlock  = regexp.MustCompile(`"""[\s\S]*?"""`)
)

func pass3Structural(cmd string, counts []int) (string, []int) {
	cmd, counts = normalizeInlineScripts(cmd, counts)
	cmd, counts = normalizePythonBlocks(cmd, counts)
	cmd, counts = normalizeHeredocs(cmd, counts)
	cmd, counts = normalizeHerestrings(cmd, counts)
	cmd, counts = normalizeProcessSubs(cmd, counts)
	cmd, counts = normalizeSubshells(cmd, counts)
	return cmd, counts
}

func normalizeInlineScripts(cmd string, counts []int) (string, []int) {
	cmd = reInlineScript.ReplaceAllStringFunc(cmd, func(match string) string {
		m := reInlineScript.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		quoted := m[2]
		body := quoted[1 : len(quoted)-1]
		counts = append(counts, len(body))
		return m[1] + fmt.Sprintf(" [INLINE_SCRIPT %dB]", len(body))
	})
	return cmd, counts
}

func normalizePythonBlocks(cmd string, counts []int) (string, []int) {
	cmd = rePythonBlock.ReplaceAllStringFunc(cmd, func(match string) string {
		body := match[3 : len(match)-3]
		counts = append(counts, len(body))
		return fmt.Sprintf("[PYTHON_BLOCK %dB]", len(body))
	})
	return cmd, counts
}

func normalizeHeredocs(cmd string, counts []int) (string, []int) {
	lines := strings.Split(cmd, "\n")
	result := make([]string, 0, len(lines))
	reHeredocStart := regexp.MustCompile(`<<-?\s*['"']?(\w+)['"']?`)
	i := 0
	for i < len(lines) {
		line := lines[i]
		m := reHeredocStart.FindStringSubmatchIndex(line)
		if m == nil {
			result = append(result, line)
			i++
			continue
		}
		delim := line[m[2]:m[3]]
		j := i + 1
		var bodyLen int
		for j < len(lines) && strings.TrimSpace(lines[j]) != delim {
			bodyLen += len(lines[j]) + 1
			j++
		}
		counts = append(counts, bodyLen)
		result = append(result, line[:m[0]]+fmt.Sprintf("[HEREDOC %dB]", bodyLen))
		if j < len(lines) {
			j++
		}
		i = j
	}
	return strings.Join(result, "\n"), counts
}

func normalizeHerestrings(cmd string, counts []int) (string, []int) {
	cmd = reHerestring.ReplaceAllStringFunc(cmd, func(match string) string {
		inner := strings.TrimPrefix(match, "<<<")
		inner = strings.TrimSpace(inner)
		if len(inner) >= 2 && (inner[0] == '\'' || inner[0] == '"') {
			inner = inner[1 : len(inner)-1]
		}
		counts = append(counts, len(inner))
		return fmt.Sprintf("[HERESTRING %dB]", len(inner))
	})
	return cmd, counts
}

func normalizeProcessSubs(cmd string, counts []int) (string, []int) {
	cmd = reProcessSub.ReplaceAllStringFunc(cmd, func(match string) string {
		counts = append(counts, len(match))
		return "[PROCESS_SUB]"
	})
	return cmd, counts
}

func normalizeSubshells(cmd string, counts []int) (string, []int) {
	cmd = reSubshell.ReplaceAllStringFunc(cmd, func(match string) string {
		counts = append(counts, len(match))
		return "[SUBSHELL]"
	})
	cmd = reBacktick.ReplaceAllStringFunc(cmd, func(match string) string {
		counts = append(counts, len(match))
		return "[SUBSHELL]"
	})
	return cmd, counts
}
