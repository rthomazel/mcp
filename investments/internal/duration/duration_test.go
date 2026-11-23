package duration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAccepts(t *testing.T) {
	for text, want := range map[string]Duration{
		"1d": {Days: 1}, "5 days": {Days: 5}, "1 day": {Days: 1}, "2 day": {Days: 2}, "3days": {Days: 3},
		"1w": {Days: 7}, "2 weeks": {Days: 14}, "72h": {Days: 3}, "24 hours": {Days: 1}, "1w48h": {Days: 9},
		"3mo": {Months: 3}, "12 months": {Months: 12}, "1y": {Months: 12}, "1 year": {Months: 12},
		"2 WEEKS": {Days: 14}, "1Y": {Months: 12}, "1y2mo": {Months: 14}, "1y2mo3d": {Months: 14, Days: 3},
		"3d1w": {Days: 10}, "05d": {Days: 5}, "1y0d": {Months: 12}, " 5d ": {Days: 5}, "9999d": {Days: 9999},
		"1 d": {Days: 1},
	} {
		got, err := Parse("window", text)

		require.NoError(t, err, text)
		assert.Equal(t, want.Months, got.Months, text)
		assert.Equal(t, want.Days, got.Days, text)
	}
}

func TestParseRejects(t *testing.T) {
	for text, reason := range map[string]string{
		"15": "unit is missing", "15.0": "fraction", "": "empty", "   ": "empty", "d": "number is missing",
		"days": "number is missing", "0d": "zero", "0y0mo": "zero", "-5d": "sign", "+5d": "sign",
		"1.5y": "fraction", "0.5d": "fraction", "1,000d": "separator", "1_000d": "separator",
		"10000d": "longer than 4 digits", "1h": "shorter than a day", "12h": "shorter than a day",
		"23h": "shorter than a day", "36h": "whole number of days", "90h": "whole number of days",
		"1d12h": "whole number of days", "90m": "shorter than a day", "30s": "shorter than a day",
		"1h30m": "shorter than a day", "72h30m": "shorter than a day", "1q": "not known",
		"2 quarters": "not known", "1 fortnight": "not known", "1y 2mo": "written together",
		"1 year 2 months": "written together", "1year2months": "long unit", "1y1y": "repeated",
		"2d3d": "repeated", "1d2": "unit is missing", "5 d ays": "unexpected text", "5 dd": "not known",
		"5 days ago": "unexpected text", "past 5d": "number is missing", "1y,2mo": "separator",
		"5d; 5d": "unexpected text", "P1Y2M": "number is missing", "５d": "unexpected text",
		"1 y2mo": "written together",
	} {
		_, err := Parse("window", text)

		require.Error(t, err, text)
		assert.Contains(t, err.Error(), `"window"`, text)
		assert.Contains(t, err.Error(), reason, text)
	}
}

func TestParseKeepsTheTrimmedText(t *testing.T) {
	got, err := Parse("window", "  5 Days ")

	require.NoError(t, err)
	assert.Equal(t, "5 Days", got.String())
}

func TestBeforeAndAfter(t *testing.T) {
	date := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}

	for name, tc := range map[string]struct {
		text      string
		from      time.Time
		before    time.Time
		afterTime time.Time
	}{
		"month clamps":        {"1mo", date(2026, 3, 31), date(2026, 2, 28), date(2026, 4, 30)},
		"leap month clamps":   {"1mo", date(2028, 3, 31), date(2028, 2, 29), date(2028, 4, 30)},
		"leap year clamps":    {"1y", date(2028, 2, 29), date(2027, 2, 28), date(2029, 2, 28)},
		"months then days":    {"1mo2w", date(2026, 3, 20), date(2026, 2, 6), date(2026, 5, 4)},
		"across year":         {"3mo", date(2026, 1, 15), date(2025, 10, 15), date(2026, 4, 15)},
		"weeks and days":      {"1w", date(2026, 3, 3), date(2026, 2, 24), date(2026, 3, 10)},
		"twelve months, year": {"12mo", date(2026, 5, 9), date(2025, 5, 9), date(2027, 5, 9)},
	} {
		d, err := Parse("window", tc.text)
		require.NoError(t, err, name)

		assert.Equal(t, tc.before, d.Before(tc.from), name)
		assert.Equal(t, tc.afterTime, d.After(tc.from), name)
	}
}

func TestEquivalentSpansApplyIdentically(t *testing.T) {
	from := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	for _, pair := range [][2]string{{"7d", "1w"}, {"168h", "1w"}, {"12mo", "1y"}, {"72h", "3d"}} {
		a, err := Parse("w", pair[0])
		require.NoError(t, err)

		b, err := Parse("w", pair[1])
		require.NoError(t, err)

		assert.Equal(t, a.Before(from), b.Before(from), pair)
	}

	thirty, _ := Parse("w", "30d")
	month, _ := Parse("w", "1mo")

	assert.NotEqual(t, thirty.Before(from), month.Before(from))
}
