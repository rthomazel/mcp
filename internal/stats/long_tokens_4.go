package stats

import (
	"fmt"
	"strings"
)

const longTokenThreshold = 80

func pass4LongTokens(cmd string) string {
	tokens := strings.Fields(cmd)
	for i, tok := range tokens {
		if len(tok) > longTokenThreshold {
			tokens[i] = fmt.Sprintf("[LONG STRING %dB]", len(tok))
		}
	}
	return strings.Join(tokens, " ")
}

// longTokenByteCount returns the original byte sizes of tokens that exceed
// longTokenThreshold, collected before pass4LongTokens replaces them.
func longTokenByteCount(cmd string) []int {
	var counts []int
	for _, tok := range strings.Fields(cmd) {
		if len(tok) > longTokenThreshold {
			counts = append(counts, len(tok))
		}
	}
	return counts
}
