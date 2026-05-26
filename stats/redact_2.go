package stats

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// pass 2 — redaction
//
// All substitutions are destructive. Tiers run in order; earlier matches take
// precedence. Byte counts of original matched text are appended to counts.

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
	// Tier 1 — known secret patterns
	reEnvAssign      = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)=('[^']*'|"[^"]*"|\S+)`)
	reFlagValue      = regexp.MustCompile(`--([-\w]+)=(['"][^'"]*['"]|\S+)`)
	reFlagValueSpace = regexp.MustCompile(`--([-\w]+)\s+(\S+)`) // --flag value form
	reURLCreds       = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+\-.]*://)[^:@/\s]+:[^@\s]+@`)
	reBearer         = regexp.MustCompile(`(?i)(Bearer\s+)\S+`)
	reJWT            = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	rePEM            = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[^-]+-----END [A-Z ]+-----`)

	// Tier 2 — high-entropy patterns
	reUUID = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reHex  = regexp.MustCompile(`[0-9a-fA-F]{32,}`)

	// Tier 3 — PII
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
		if strings.HasPrefix(m[2], "-") {
			return match // looks like another flag, not a value
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
		credPart := match[len(m[1]) : len(match)-1]
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
