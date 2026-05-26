package stats

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// NormalizerVersion is incremented whenever normalization rules change.
// Rows with different versions should not be grouped by cmd_hash.
const NormalizerVersion = 1

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

// --- base_cmd extraction ---

var wrappers = map[string]bool{
	"sudo": true, "env": true, "time": true,
	"command": true, "exec": true, "nice": true, "ionice": true,
}

// wrapperArgs holds flags that consume the next token for each wrapper.
var wrapperArgs = map[string]map[string]bool{
	"sudo":   {"-u": true, "-g": true, "-C": true, "-r": true, "-t": true, "-T": true, "-p": true},
	"nice":   {"-n": true},
	"ionice": {"-c": true, "-n": true, "-p": true},
}

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
		if wrappers[tokens[0]] {
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
	inQuote := rune(0)
	for i := 0; i < len(cmd); i++ {
		ch := rune(cmd[i])
		switch {
		case inQuote != 0:
			current.WriteByte(cmd[i])
			if ch == inQuote {
				inQuote = 0
			}
		case ch == '\'' || ch == '"':
			current.WriteByte(cmd[i])
			inQuote = ch
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
	wrapper := tokens[0]
	tokens = tokens[1:]
	args := wrapperArgs[wrapper]
	for len(tokens) > 0 {
		tok := tokens[0]
		if !strings.HasPrefix(tok, "-") {
			break
		}
		// Flag-only (no next-token arg)
		if args[tok] && len(tokens) > 1 {
			tokens = tokens[2:] // consume flag + its argument
		} else {
			tokens = tokens[1:] // consume flag only
		}
	}
	return tokens
}
