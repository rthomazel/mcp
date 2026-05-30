// Package sqlcheck validates raw SQL strings before database execution.
package sqlcheck

import (
	"fmt"
	"strings"
	"unicode"
)

// txnControlKeywords lists first tokens that are always blocked across all tools.
var txnControlKeywords = map[string]bool{
	"BEGIN":     true,
	"COMMIT":    true,
	"ROLLBACK":  true,
	"SAVEPOINT": true,
	"RELEASE":   true,
	"START":     true,
}

// StripComments removes -- and /* */ comments from sql.
// Dollar-quoted strings and complex quoted identifiers are not handled — known v1 limitation.
func StripComments(sql string) string {
	var result strings.Builder
	i := 0
	for i < len(sql) {
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			for i+1 < len(sql) {
				if sql[i] == '*' && sql[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		result.WriteByte(sql[i])
		i++
	}
	return strings.TrimSpace(result.String())
}

// FirstToken returns the first whitespace-delimited token from sql, uppercased.
func FirstToken(sql string) string {
	sql = strings.TrimLeftFunc(sql, unicode.IsSpace)
	end := strings.IndexFunc(sql, unicode.IsSpace)
	if end == -1 {
		return strings.ToUpper(sql)
	}
	return strings.ToUpper(sql[:end])
}

// Validate strips comments from sql, rejects multiple statements,
// rejects transaction-control keywords, then verifies the first token
// is in allowlist (entries are matched case-insensitively).
// Returns the stripped SQL (original casing) and any error.
func Validate(sql string, allowlist []string) (string, error) {
	stripped := StripComments(sql)

	// Detect multiple statements: a ; that is not the last non-whitespace character.
	lastIdx := strings.LastIndexFunc(stripped, func(r rune) bool { return !unicode.IsSpace(r) })
	for i, ch := range stripped {
		if ch == ';' && i != lastIdx {
			return "", fmt.Errorf("multiple statements are not allowed")
		}
	}

	token := FirstToken(stripped)

	if txnControlKeywords[token] {
		return "", fmt.Errorf("transaction-control statements are not allowed")
	}

	for _, allowed := range allowlist {
		if token == strings.ToUpper(allowed) {
			return stripped, nil
		}
	}

	return "", fmt.Errorf("statement type %q is not allowed by this tool", token)
}
