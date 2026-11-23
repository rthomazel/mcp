package monitor

import (
	"fmt"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/rthomazel/mcp/investments/internal/config/fileloader"
)

const (
	supportedVersion = 1
	disabledKey      = "disabled"
	versionKey       = "version"
	monitorsKey      = "monitors"
)

// Config is the monitors configuration, parsed from one YAML file.
type Config struct {
	Version  int
	Monitors map[string]Entry
}

// Entry is the block of one monitor: whether it is disabled and its settings.
type Entry struct {
	Disabled bool
	Params   Params
}

// NewConfigLoader builds the loader of the monitors file; an empty path means
// no monitor runs.
func NewConfigLoader(path string, monitors []Monitor) *fileloader.Loader[Config] {
	return fileloader.New(path, func(cfg Config) error { return cfg.Validate(monitors) })
}

// UnmarshalYAML rejects top-level keys other than version and monitors and
// gives an entry written without a value an enabled, empty Entry.
func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var top map[string]yaml.Node
	if err := node.Decode(&top); err != nil {
		return fmt.Errorf("monitors file must be a mapping: %w", err)
	}

	for key := range top {
		if key != versionKey && key != monitorsKey {
			return fmt.Errorf("unknown top-level key %q", key)
		}
	}

	if version, ok := top[versionKey]; ok {
		if err := version.Decode(&c.Version); err != nil {
			return fmt.Errorf("decoding version: %w", err)
		}
	}

	var entries map[string]*Entry

	if monitors, ok := top[monitorsKey]; ok {
		if err := monitors.Decode(&entries); err != nil {
			return fmt.Errorf("decoding monitors: %w", err)
		}
	}

	c.Monitors = make(map[string]Entry, len(entries))

	for name, entry := range entries {
		if entry == nil {
			entry = &Entry{}
		}

		c.Monitors[name] = *entry
	}

	return nil
}

// UnmarshalYAML reads disabled as a boolean and every other key as a setting
// of any shape.
func (e *Entry) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		return nil
	}

	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return fmt.Errorf("entry must be a mapping: %w", err)
	}

	if value, ok := raw[disabledKey]; ok {
		disabled, isBool := value.(bool)
		if !isBool {
			return fmt.Errorf("%s must be true or false, got %v", disabledKey, value)
		}

		e.Disabled = disabled

		delete(raw, disabledKey)
	}

	if len(raw) > 0 {
		e.Params = raw
	}

	return nil
}

// Validate checks the file as a whole against the monitors the server has.
func (c Config) Validate(monitors []Monitor) error {
	if c.Version != supportedVersion {
		return fmt.Errorf("unsupported configuration version %d, expected %d", c.Version, supportedVersion)
	}

	byName := make(map[string]Monitor, len(monitors))
	for _, monitor := range monitors {
		byName[monitor.Name()] = monitor
	}

	names := make([]string, 0, len(c.Monitors))
	for name := range c.Monitors {
		names = append(names, name)
	}

	slices.Sort(names)

	for _, name := range names {
		monitor, ok := byName[name]
		if !ok {
			return fmt.Errorf("unknown monitor %q", name)
		}

		if err := monitor.ValidateParams(c.Monitors[name].Params); err != nil {
			return fmt.Errorf("monitor %q: %w", name, err)
		}
	}

	return nil
}
