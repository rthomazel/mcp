package internal

import "fmt"

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
// of the supported command parameter names.
func ParseCommands(args map[string]any) ([]string, error) {
	const missing = "missing required parameter: exactly one of commands, command, or command_paths must be a non-empty array of strings"

	var commands []string
	provided := 0
	for _, name := range []string{"commands", "command", "command_paths"} {
		value, exists := args[name]
		if !exists {
			continue
		}

		parsed, ok := ParseStringSlice(value)
		if !ok || len(parsed) == 0 {
			return nil, fmt.Errorf("invalid parameter: %s must be a non-empty array of strings", name)
		}
		provided++
		commands = parsed
	}

	if provided == 0 {
		return nil, fmt.Errorf("%s", missing)
	}
	if provided > 1 {
		return nil, fmt.Errorf("only one of commands, command, or command_paths may be provided")
	}

	return commands, nil
}
