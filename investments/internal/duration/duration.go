package duration

import (
	"cmp"
	"fmt"
	"strings"
	"time"
)

const (
	maxDigits    = 4
	hoursPerDay  = 24
	daysPerWeek  = 7
	monthsInYear = 12
	daysPerMonth = 30.4375
)

// Duration is a calendar span: months and days, applied to a date.
type Duration struct {
	Months int
	Days   int
	text   string
}

type unit struct {
	short string
	long  []string
}

var units = []unit{
	{"h", []string{"hour", "hours"}},
	{"d", []string{"day", "days"}},
	{"w", []string{"week", "weeks"}},
	{"mo", []string{"month", "months"}},
	{"y", []string{"year", "years"}},
}

// Shorter than a day, rejected with their own reason.
var subDayUnits = []string{"ms", "s", "sec", "secs", "second", "seconds", "m", "min", "mins", "minute", "minutes"}

type component struct {
	number int
	unit   string // the short form
	long   bool
	spaced bool
}

// Parse reads text as a duration. Errors name the setting and the text.
func Parse(setting, text string) (Duration, error) {
	trimmed := strings.TrimSpace(text)

	d, err := parse(strings.ToLower(trimmed))
	if err != nil {
		return Duration{}, fmt.Errorf("setting %q: duration %q is invalid: %w", setting, text, err)
	}

	d.text = trimmed

	return d, nil
}

// String is the text as written, trimmed.
func (d Duration) String() string { return d.text }

// Compare orders two durations, -1, 0 or 1, counting a month as 30.4375 days.
// It is approximate, for checking that one span is not shorter than another;
// it is never used to move a date.
func (d Duration) Compare(other Duration) int {
	return cmp.Compare(d.approxDays(), other.approxDays())
}

func (d Duration) approxDays() float64 { return float64(d.Months)*daysPerMonth + float64(d.Days) }

// Before is the date one duration earlier: months first, clamped to the end of
// the month, then days.
func (d Duration) Before(date time.Time) time.Time { return d.shift(date, -1) }

// After is the date one duration later, by the same rules.
func (d Duration) After(date time.Time) time.Time { return d.shift(date, 1) }

func (d Duration) shift(date time.Time, sign int) time.Time {
	year, month, day := date.Date()
	index := int(month) - 1 + sign*d.Months
	year += floorDiv(index, monthsInYear)
	month = time.Month(index-floorDiv(index, monthsInYear)*monthsInYear) + 1

	if last := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day(); day > last {
		day = last
	}

	moved := time.Date(year, month, day, date.Hour(), date.Minute(), date.Second(), date.Nanosecond(), date.Location())

	return moved.AddDate(0, 0, sign*d.Days)
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b < 0 {
		q--
	}

	return q
}

func parse(s string) (Duration, error) {
	if s == "" {
		return Duration{}, fmt.Errorf("it is empty")
	}

	comps, err := scan(s)
	if err != nil {
		return Duration{}, err
	}

	if len(comps) > 1 {
		for _, c := range comps {
			if c.spaced {
				return Duration{}, fmt.Errorf("several components must be written together, such as 1y2mo")
			}

			if c.long {
				return Duration{}, fmt.Errorf("long unit names are only allowed in a single component")
			}
		}
	}

	return total(comps)
}

// scan splits s into components.
func scan(s string) ([]component, error) {
	var comps []component

	for i := 0; i < len(s); {
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}

		if i == start {
			return nil, unexpected(s, i)
		}

		if i-start > maxDigits {
			return nil, fmt.Errorf("the number is longer than %d digits", maxDigits)
		}

		number := 0
		for _, digit := range s[start:i] {
			number = number*10 + int(digit-'0')
		}

		gap := i
		for i < len(s) && s[i] == ' ' {
			i++
		}

		spaced := i > gap

		word := i
		for i < len(s) && s[i] >= 'a' && s[i] <= 'z' {
			i++
		}

		if i == word {
			if i == len(s) {
				return nil, fmt.Errorf("the unit is missing")
			}

			return nil, unexpected(s, i)
		}

		short, long, err := lookup(s[word:i])
		if err != nil {
			return nil, err
		}

		comps = append(comps, component{number: number, unit: short, long: long, spaced: spaced})

		if i < len(s) && s[i] == ' ' {
			rest := strings.TrimLeft(s[i:], " ")
			if rest != "" && rest[0] >= '0' && rest[0] <= '9' {
				return nil, fmt.Errorf("several components must be written together, such as 1y2mo")
			}

			return nil, fmt.Errorf("unexpected text %q", rest)
		}
	}

	return comps, nil
}

func unexpected(s string, i int) error {
	switch c := s[i]; {
	case c >= 'a' && c <= 'z':
		return fmt.Errorf("the number is missing")
	case c == '-' || c == '+':
		return fmt.Errorf("a sign is not allowed")
	case c == '.':
		return fmt.Errorf("a fraction is not allowed")
	case c == ',' || c == '_':
		return fmt.Errorf("a separator is not allowed")
	default:
		return fmt.Errorf("unexpected text %q", s[i:])
	}
}

func lookup(word string) (short string, long bool, err error) {
	for _, u := range units {
		if word == u.short {
			return u.short, false, nil
		}

		for _, name := range u.long {
			if word == name {
				return u.short, true, nil
			}
		}
	}

	for _, name := range subDayUnits {
		if word == name {
			return "", false, fmt.Errorf("it is shorter than a day, which is not supported")
		}
	}

	return "", false, fmt.Errorf("the unit %q is not known", word)
}

func total(comps []component) (Duration, error) {
	var d Duration

	seen := map[string]bool{}
	hours := 0

	for _, c := range comps {
		if seen[c.unit] {
			return Duration{}, fmt.Errorf("the unit %q is repeated", c.unit)
		}

		seen[c.unit] = true

		switch c.unit {
		case "h":
			hours = c.number
		case "d":
			d.Days += c.number
		case "w":
			d.Days += c.number * daysPerWeek
		case "mo":
			d.Months += c.number
		case "y":
			d.Months += c.number * monthsInYear
		}
	}

	if hours%hoursPerDay != 0 {
		if hours < hoursPerDay && d.Days == 0 && d.Months == 0 {
			return Duration{}, fmt.Errorf("it is shorter than a day, which is not supported")
		}

		return Duration{}, fmt.Errorf("hours must be a whole number of days")
	}

	d.Days += hours / hoursPerDay

	if d.Months == 0 && d.Days == 0 {
		return Duration{}, fmt.Errorf("the duration is zero")
	}

	return d, nil
}
