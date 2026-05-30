package stats

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"slices"
	"strings"
)

// ProcessedCommand is the result of running a shell command through the pipeline.
type ProcessedCommand struct {
	Normalized         string
	BaseCmd            string
	Hash               string // hex SHA-256(Normalized)
	RedactedByteCounts []int  // original byte length of each redacted/normalized token, in order
}

// ProcessCommand destroys the raw command string, passing it through the
// redaction, structural normalization, and long-token normalization passes.
// The raw command is never stored; only the normalized result and its hash are
// returned.
func ProcessCommand(cmd string, userPatterns []*regexp.Regexp) ProcessedCommand {
	var counts []int
	cmd, counts = pass2Redact(cmd, userPatterns)
	cmd, counts = pass3Structural(cmd, counts)
	counts = append(counts, longTokenByteCount(cmd)...)
	cmd = pass4LongTokens(cmd)

	sum := sha256.Sum256([]byte(cmd))
	return ProcessedCommand{
		Normalized:         cmd,
		BaseCmd:            extractBaseCmd(cmd),
		Hash:               hex.EncodeToString(sum[:]),
		RedactedByteCounts: counts,
	}
}

var wrappers = []string{"sudo", "env", "time", "command", "exec", "nice", "ionice"}

// reAnyEnvAssign matches tokens that look like env var assignments (WORD=anything).
var reAnyEnvAssign = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

func extractBaseCmd(cmd string) string {
	// Step 1: split on pipeline boundary, handle leading cd segments.
	segment := firstRealSegment(cmd)

	tokens := strings.Fields(segment)
	if len(tokens) == 0 {
		return ""
	}

	// Steps 2-4: iteratively strip env assigns and wrappers.
	for {
		progress := false

		// Strip leading env var assignments.
		for len(tokens) > 0 && reAnyEnvAssign.MatchString(tokens[0]) {
			tokens = tokens[1:]
			progress = true
		}

		if len(tokens) == 0 {
			return ""
		}

		// Strip leading wrapper and its flags.
		if slices.Contains(wrappers, tokens[0]) {
			tokens = consumeWrapper(tokens)
			progress = true
		}

		if !progress {
			break
		}
	}

	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

// firstRealSegment splits cmd on the first unquoted shell operator and returns
// the first non-cd segment.
func firstRealSegment(cmd string) string {
	segments := splitOnOperators(cmd)
	for _, seg := range segments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) > 0 && fields[0] == "cd" {
			continue
		}
		return trimmed
	}
	return ""
}

func splitOnOperators(cmd string) []string {
	var segments []string
	var current strings.Builder
	var inQuote *rune
	for i := 0; i < len(cmd); i++ {
		ch := rune(cmd[i])
		switch {
		case inQuote != nil:
			current.WriteByte(cmd[i])
			if ch == *inQuote {
				inQuote = nil
			}
		case ch == '\'' || ch == '"':
			current.WriteByte(cmd[i])
			q := ch
			inQuote = &q
		case i+1 < len(cmd) && ((cmd[i] == '&' && cmd[i+1] == '&') || (cmd[i] == '|' && cmd[i+1] == '|')):
			segments = append(segments, current.String())
			current.Reset()
			i++ // skip second char of operator
		case ch == '|' || ch == ';':
			segments = append(segments, current.String())
			current.Reset()
		default:
			current.WriteByte(cmd[i])
		}
	}
	if current.Len() > 0 {
		segments = append(segments, current.String())
	}
	return segments
}

func consumeWrapper(tokens []string) []string {
	if len(tokens) == 0 {
		return tokens
	}
	tokens = tokens[1:] // drop the wrapper itself
	for len(tokens) > 0 && strings.HasPrefix(tokens[0], "-") {
		tokens = tokens[1:]
	}
	return tokens
}
