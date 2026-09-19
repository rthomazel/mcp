package internal

import (
	"errors"
	"fmt"
	"strings"
)

// ParseStringSlice coerces a []any (as returned by mcp-go for array params)
// into a []string. Returns false if v is not a slice or contains non-string elements.
func ParseStringSlice(v any) ([]string, bool) {
	raw, ok := v.([]any)
	if !ok {
		return nil, false
	}

	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}

	return out, true
}

// ParseCommands returns the non-empty command array supplied through exactly one
// of commands or the configured aliases.
func ParseCommands(args map[string]any, aliases []string) ([]string, error) {
	commandNames := append([]string{"commands"}, aliases...)
	missing := "missing required parameter: exactly one of " + strings.Join(commandNames, ", ") + " must be a non-empty array of strings"

	var commands []string
	provided := 0
	for _, name := range commandNames {
		value, exists := args[name]
		if !exists {
			continue
		}

		parsed, ok := ParseStringSlice(value)
		if !ok {
			return nil, fmt.Errorf("invalid parameter: %s must be an array of strings", name)
		}
		if len(parsed) == 0 {
			continue
		}
		provided++
		commands = parsed
	}

	if provided == 0 {
		return nil, errors.New(missing)
	}
	if provided > 1 {
		return nil, errors.New("only one of " + strings.Join(commandNames, ", ") + " may be provided")
	}

	return commands, nil
}
