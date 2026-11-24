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

// Normalized returns the number without a plus sign, trailing zeros of its fraction or leading zeros of its
// whole part, so equal numbers written differently read the same: 232.50 and 0232.5 are 232.5. Text only.
func (d Decimal) Normalized() Decimal {
	s := strings.TrimSpace(string(d))

	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}

	whole, fraction, _ := strings.Cut(s, ".")
	if whole = strings.TrimLeft(whole, "0"); whole == "" {
		whole = "0"
	}

	if fraction = strings.TrimRight(fraction, "0"); fraction != "" {
		whole += "." + fraction
	}

	if sign != "" && strings.Trim(whole, "0.") == "" {
		sign = ""
	}

	return Decimal(sign + whole)
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
