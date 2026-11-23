package monitor

import (
	"fmt"
	"strings"
)

const (
	directionKey  = "direction"
	directionUp   = "up"
	directionDown = "down"
	directionBoth = "both"
)

// direction is the side of a move a monitor watches.
type direction string

// directionFrom reads the optional direction setting, both when omitted.
func directionFrom(params Params) (direction, error) {
	text, ok, err := params.Text(directionKey)
	if err != nil {
		return "", err
	}

	if !ok {
		return directionBoth, nil
	}

	switch value := direction(strings.ToLower(strings.TrimSpace(text))); value {
	case directionUp, directionDown, directionBoth:
		return value, nil
	default:
		return "", fmt.Errorf("setting %q must be up, down or both, got %q", directionKey, text)
	}
}

// watches reports whether the monitor watches a move up or down.
func (d direction) watches(side direction) bool { return d == directionBoth || d == side }
