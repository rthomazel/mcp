package stats

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
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

// ProcessCommand runs cmd through all four pipeline passes and returns the result.
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

// --- pass 2: redaction ---

func pass2Redact(cmd string, userPatterns []*regexp.Regexp) (string, []int) {
	var counts []int
	cmd = redactEnvAssigns(cmd, &counts)
	cmd = redactFlagValues(cmd, &counts)
	cmd = redactFlagValuesSpace(cmd, &counts)
	cmd = redactURLCreds(cmd, &counts)
	cmd = redactBearerTokens(cmd, &counts)
	cmd = redactJWTs(cmd, &counts)
	cmd = redactPEMBlocks(cmd, &counts)
	cmd = redactUUIDs(cmd, &counts)
	cmd = redactLongHex(cmd, &counts)
	cmd = redactEmails(cmd, &counts)
	cmd = redactPublicIPs(cmd, &counts)
	for _, pattern := range userPatterns {
		cmd = redactCustom(cmd, pattern, &counts)
	}
	return cmd, counts
}

var (
	// Tier 1
	reEnvAssign = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)=('[^']*'|"[^"]*"|\S+)`)
	reFlagValue = regexp.MustCompile(`--([-\w]+)=(['"][^'"]*['"]|\S+)`)
	// Space-separated form: --sensitive-flag value (value is the next non-flag token)
	reFlagValueSpace = regexp.MustCompile(`--([-\w]+)\s+(\S+)`)
	reURLCreds       = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+\-.]*://)[^:@/\s]+:[^@\s]+@`)
	reBearer         = regexp.MustCompile(`(?i)(Bearer\s+)\S+`)
	reJWT            = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	rePEM            = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[^-]+-----END [A-Z ]+-----`)

	// Tier 2
	reUUID = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reHex  = regexp.MustCompile(`[0-9a-fA-F]{32,}`)

	// Tier 3
	reEmail = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	reIPv4  = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
)

var sensitiveKeywords = []string{
	"TOKEN", "KEY", "SECRET", "PASSWORD", "PASSWD", "PWD", "PASS",
	"AUTH", "CRED", "CREDENTIAL", "API_KEY", "PRIVATE", "CERT", "SIGNING",
}

func isSensitiveVarName(name string) bool {
	upper := strings.ToUpper(name)
	for _, kw := range sensitiveKeywords {
		if upper == kw {
			return true
		}
		if strings.HasSuffix(upper, "_"+kw) {
			return true
		}
		if strings.HasPrefix(upper, kw+"_") {
			return true
		}
		if strings.Contains(upper, "_"+kw+"_") {
			return true
		}
	}
	return false
}

func redactEnvAssigns(cmd string, counts *[]int) string {
	return reEnvAssign.ReplaceAllStringFunc(cmd, func(match string) string {
		m := reEnvAssign.FindStringSubmatch(match)
		if len(m) < 3 || !isSensitiveVarName(m[1]) {
			return match
		}
		*counts = append(*counts, len(m[2]))
		return m[1] + "=REDACTED"
	})
}

func redactFlagValues(cmd string, counts *[]int) string {
	return reFlagValue.ReplaceAllStringFunc(cmd, func(match string) string {
		m := reFlagValue.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		normalized := strings.ToUpper(strings.ReplaceAll(m[1], "-", "_"))
		if !isSensitiveVarName(normalized) {
			return match
		}
		*counts = append(*counts, len(m[2]))
		return "--" + m[1] + "=REDACTED"
	})
}

func redactFlagValuesSpace(cmd string, counts *[]int) string {
	return reFlagValueSpace.ReplaceAllStringFunc(cmd, func(match string) string {
		m := reFlagValueSpace.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		normalized := strings.ToUpper(strings.ReplaceAll(m[1], "-", "_"))
		if !isSensitiveVarName(normalized) {
			return match
		}
		// Do not redact if the value looks like another flag (starts with -)
		if strings.HasPrefix(m[2], "-") {
			return match
		}
		*counts = append(*counts, len(m[2]))
		return "--" + m[1] + " REDACTED"
	})
}

func redactURLCreds(cmd string, counts *[]int) string {
	return reURLCreds.ReplaceAllStringFunc(cmd, func(match string) string {
		m := reURLCreds.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		credPart := match[len(m[1]) : len(match)-1] // "user:pass" without trailing @
		*counts = append(*counts, len(credPart))
		return m[1] + "REDACTED@"
	})
}

func redactBearerTokens(cmd string, counts *[]int) string {
	return reBearer.ReplaceAllStringFunc(cmd, func(match string) string {
		m := reBearer.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		tokenPart := match[len(m[1]):]
		*counts = append(*counts, len(tokenPart))
		return m[1] + "REDACTED"
	})
}

func redactJWTs(cmd string, counts *[]int) string {
	return reJWT.ReplaceAllStringFunc(cmd, func(match string) string {
		*counts = append(*counts, len(match))
		return "[JWT]"
	})
}

func redactPEMBlocks(cmd string, counts *[]int) string {
	return rePEM.ReplaceAllStringFunc(cmd, func(match string) string {
		*counts = append(*counts, len(match))
		return "[PEM BLOCK]"
	})
}

func redactUUIDs(cmd string, counts *[]int) string {
	return reUUID.ReplaceAllStringFunc(cmd, func(match string) string {
		*counts = append(*counts, len(match))
		return "[UUID]"
	})
}

func redactLongHex(cmd string, counts *[]int) string {
	return reHex.ReplaceAllStringFunc(cmd, func(match string) string {
		*counts = append(*counts, len(match))
		return fmt.Sprintf("[HEX %dB]", len(match))
	})
}

func redactEmails(cmd string, counts *[]int) string {
	return reEmail.ReplaceAllStringFunc(cmd, func(match string) string {
		*counts = append(*counts, len(match))
		return "[EMAIL]"
	})
}

func redactPublicIPs(cmd string, counts *[]int) string {
	return reIPv4.ReplaceAllStringFunc(cmd, func(match string) string {
		parts := strings.Split(match, ".")
		if len(parts) != 4 {
			return match
		}
		octets := make([]int, 4)
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 || n > 255 {
				return match
			}
			octets[i] = n
		}
		if isPrivateIPv4(octets[0], octets[1], octets[2], octets[3]) {
			return match
		}
		*counts = append(*counts, len(match))
		return "[IP]"
	})
}

func isPrivateIPv4(a, b, _, _ int) bool {
	switch {
	case a == 127: // loopback
		return true
	case a == 10: // RFC-1918 10.0.0.0/8
		return true
	case a == 172 && b >= 16 && b <= 31: // RFC-1918 172.16.0.0/12
		return true
	case a == 192 && b == 168: // RFC-1918 192.168.0.0/16
		return true
	case a == 169 && b == 254: // link-local
		return true
	case a >= 224: // multicast / reserved
		return true
	}
	return false
}

func redactCustom(cmd string, pattern *regexp.Regexp, counts *[]int) string {
	return pattern.ReplaceAllStringFunc(cmd, func(match string) string {
		*counts = append(*counts, len(match))
		return "REDACTED"
	})
}

// --- pass 3: structural normalization ---

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
		body := quoted[1 : len(quoted)-1] // strip surrounding quotes
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
			bodyLen += len(lines[j]) + 1 // +1 for newline
			j++
		}
		counts = append(counts, bodyLen)
		result = append(result, line[:m[0]]+fmt.Sprintf("[HEREDOC %dB]", bodyLen))
		if j < len(lines) {
			j++ // skip end delimiter line
		}
		i = j
	}
	return strings.Join(result, "\n"), counts
}

func normalizeHerestrings(cmd string, counts []int) (string, []int) {
	cmd = reHerestring.ReplaceAllStringFunc(cmd, func(match string) string {
		// strip the <<< prefix and surrounding quotes
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

// --- pass 4: long token normalization ---

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

// longTokenByteCount returns the total bytes consumed by tokens that exceed longTokenThreshold.
// Used to populate RedactedByteCounts for pass 4.
func longTokenByteCount(cmd string) []int {
	var counts []int
	for _, tok := range strings.Fields(cmd) {
		if len(tok) > longTokenThreshold {
			counts = append(counts, len(tok))
		}
	}
	return counts
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
