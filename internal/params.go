package internal

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
