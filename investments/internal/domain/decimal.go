package domain

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

var decimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

// Decimal is a plain decimal number held as text; Go never does arithmetic on it.
type Decimal string

// Date is a calendar date, a time.Time at midnight UTC.
type Date = time.Time

// ParseDecimal validates s as a plain decimal number.
func ParseDecimal(s string) (Decimal, error) {
	s = strings.TrimSpace(s)
	if !decimalPattern.MatchString(s) {
		return "", fmt.Errorf("%w: %q is not a decimal number", ErrInvalidInput, s)
	}

	return Decimal(s), nil
}

// Compare returns -1, 0 or 1 by exact value, for validating settings. Money is
// never added or divided in Go.
func (d Decimal) Compare(other Decimal) int {
	left, _ := new(big.Rat).SetString(strings.TrimSpace(string(d)))
	right, _ := new(big.Rat).SetString(strings.TrimSpace(string(other)))

	if left == nil || right == nil {
		return strings.Compare(string(d), string(other))
	}

	return left.Cmp(right)
}

// Sign returns -1, 0 or 1.
func (d Decimal) Sign() int {
	s := strings.TrimSpace(string(d))
	negative := strings.HasPrefix(s, "-")
	if negative {
		s = s[1:]
	}

	if strings.Trim(s, "0.") == "" {
		return 0
	}

	if negative {
		return -1
	}

	return 1
}
