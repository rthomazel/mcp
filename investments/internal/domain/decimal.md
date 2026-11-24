# Types

## Decimal

A string holding a plain decimal number: optional minus sign, digits, optional fractional digits. No exponent, no separators, no fractions.
Go never does arithmetic on decimals; it only checks sign and passes them to and from Postgres NUMERIC.

## Date

A calendar date, a time.Time at midnight UTC. Rendered as "YYYY-MM-DD" at the API edge.

# Functions

## ParseDecimal(s string) (Decimal, error)

1. Trim spaces.
2. Check it matches an optional minus, one or more digits, and optionally a dot with one or more digits.
3. Return it as a Decimal.

#### Errors

- **2.** if it does not match, return ErrInvalidInput naming the value.

## Decimal.Compare(other Decimal) int

Return -1, 0 or 1 by exact value, using math/big rationals, so 1.0 equals 1. For validating settings, such as a minimum that may not exceed a maximum; money is never added or divided in Go. Text that is not a decimal compares as text.

## Decimal.Sign() int

Return -1, 0 or 1 by inspecting the text: zero if every digit is zero, negative if it starts with a minus.
