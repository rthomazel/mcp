package monitor

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/duration"
)

const heldWord = "held"

// Params are the settings of one monitor as decoded from YAML: text, numbers,
// booleans, lists and nested mappings. The accessors check the shape a monitor
// needs and name the setting in every error.
type Params map[string]any

// Selection is the instruments a monitor watches: every held instrument, or
// the listed tickers.
type Selection struct {
	Held    bool
	Tickers []string
}

// Unknown returns an error naming the first setting, in name order, that is
// not allowed.
func (p Params) Unknown(allowed ...string) error {
	for _, key := range slices.Sorted(maps.Keys(p)) {
		if !slices.Contains(allowed, key) {
			return fmt.Errorf("unknown setting %q", key)
		}
	}

	return nil
}

// Text reads a setting that must be text.
func (p Params) Text(key string) (text string, ok bool, err error) {
	value, ok := p[key]
	if !ok {
		return "", false, nil
	}

	text, isText := value.(string)
	if !isText {
		return "", true, fmt.Errorf("setting %q must be text, got %v", key, value)
	}

	return text, true, nil
}

// Decimal reads a setting written as quoted text and parses it as a decimal.
func (p Params) Decimal(key string) (domain.Decimal, bool, error) {
	text, ok, err := p.Text(key)
	if err != nil || !ok {
		return "", ok, err
	}

	decimal, err := domain.ParseDecimal(text)
	if err != nil {
		return "", true, fmt.Errorf("setting %q: %w", key, err)
	}

	return decimal, true, nil
}

// Duration reads a setting written as a duration with a unit. A YAML number is
// rejected as having no unit.
func (p Params) Duration(key string) (duration.Duration, bool, error) {
	value, ok := p[key]
	if !ok {
		return duration.Duration{}, false, nil
	}

	text, isText := value.(string)
	if !isText {
		return duration.Duration{}, true, fmt.Errorf("setting %q: duration %v is invalid: the unit is missing", key, value)
	}

	parsed, err := duration.Parse(key, text)

	return parsed, true, err
}

// DurationOr reads a duration setting, the fallback text when it is missing.
func (p Params) DurationOr(key, fallback string) (duration.Duration, error) {
	parsed, ok, err := p.Duration(key)
	if err != nil || ok {
		return parsed, err
	}

	return duration.Parse(key, fallback)
}

// Mappings reads a setting that must be a non-empty list of mappings.
func (p Params) Mappings(key string) ([]Params, bool, error) {
	items, ok, err := p.list(key)
	if err != nil || !ok {
		return nil, ok, err
	}

	mappings := make([]Params, len(items))

	for i, item := range items {
		switch mapping := item.(type) {
		case map[string]any:
			mappings[i] = mapping
		case Params:
			mappings[i] = mapping
		default:
			return nil, true, fmt.Errorf("setting %q item %d must be a mapping", key, i+1)
		}
	}

	return mappings, true, nil
}

// Instruments reads the word held or a non-empty list of distinct tickers,
// normalised to upper case. A single ticker is a list of one.
func (p Params) Instruments(key string) (Selection, bool, error) {
	if word, isText := p[key].(string); isText {
		if strings.ToLower(strings.TrimSpace(word)) == heldWord {
			return Selection{Held: true}, true, nil
		}

		return Selection{}, true, fmt.Errorf("setting %q must be %q or a list of tickers, got %q", key, heldWord, word)
	}

	items, ok, err := p.list(key)
	if err != nil || !ok {
		return Selection{}, ok, err
	}

	var selection Selection

	for _, item := range items {
		ticker, isText := item.(string)
		if ticker = strings.ToUpper(strings.TrimSpace(ticker)); !isText || ticker == "" {
			return Selection{}, true, fmt.Errorf("setting %q must list tickers as text", key)
		}

		if slices.Contains(selection.Tickers, ticker) {
			return Selection{}, true, fmt.Errorf("setting %q lists %q more than once", key, ticker)
		}

		selection.Tickers = append(selection.Tickers, ticker)
	}

	return selection, true, nil
}

func (p Params) list(key string) ([]any, bool, error) {
	value, ok := p[key]
	if !ok {
		return nil, false, nil
	}

	items, isList := value.([]any)
	if !isList || len(items) == 0 {
		return nil, true, fmt.Errorf("setting %q must be a non-empty list", key)
	}

	return items, true, nil
}
