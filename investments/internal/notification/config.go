package notification

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	supportedVersion       = 1
	implementationTelegram = "telegram"
	implementationStub     = "stub"
	defaultFormat          = "plain"
)

// Config is the complete notifications configuration, parsed from one YAML file.
type Config struct {
	Version  int             `yaml:"version"`
	Channels []ChannelConfig `yaml:"channels"`
	Routes   []Route         `yaml:"routes"`
}

// ChannelConfig declares one channel and its categories.
type ChannelConfig struct {
	Name           string           `yaml:"name"`
	Implementation string           `yaml:"implementation"`
	Categories     []CategoryConfig `yaml:"categories"`
}

// CategoryConfig declares one category of a channel.
type CategoryConfig struct {
	Name     string            `yaml:"name"`
	Settings map[string]string `yaml:"settings"`
}

// Route sends one event type to its targets.
type Route struct {
	EventType string   `yaml:"event_type"`
	Format    string   `yaml:"format"`
	Targets   []Target `yaml:"targets"`
}

// Target is one channel and one of its categories.
type Target struct {
	Channel  string `yaml:"channel"`
	Category string `yaml:"category"`
}

// Validate checks the configuration as a whole.
func (c Config) Validate() error {
	if c.Version != supportedVersion {
		return fmt.Errorf("unsupported configuration version %d, expected %d", c.Version, supportedVersion)
	}

	categories := map[string][]string{}

	for _, channel := range c.Channels {
		if channel.Name == "" {
			return errors.New("a channel has no name")
		}

		if _, exists := categories[channel.Name]; exists {
			return fmt.Errorf("channel %q is declared twice", channel.Name)
		}

		if channel.Implementation != implementationTelegram && channel.Implementation != implementationStub {
			return fmt.Errorf("channel %q has unknown implementation %q", channel.Name, channel.Implementation)
		}

		names := []string{}

		for _, category := range channel.Categories {
			if category.Name == "" || slices.Contains(names, category.Name) {
				return fmt.Errorf("channel %q has a blank or repeated category %q", channel.Name, category.Name)
			}

			names = append(names, category.Name)
		}

		categories[channel.Name] = names
	}

	seen := []string{}

	for _, route := range c.Routes {
		if !slices.Contains(domain.NotifyingEvents, route.EventType) {
			return fmt.Errorf("route for unknown event type %q", route.EventType)
		}

		if slices.Contains(seen, route.EventType) {
			return fmt.Errorf("event type %q is routed twice", route.EventType)
		}

		seen = append(seen, route.EventType)

		if route.Format != "" && route.Format != defaultFormat {
			return fmt.Errorf("route %q has unknown format %q", route.EventType, route.Format)
		}

		for _, target := range route.Targets {
			names, ok := categories[target.Channel]
			if !ok || !slices.Contains(names, target.Category) {
				return fmt.Errorf("route %q targets undeclared %s/%s", route.EventType, target.Channel, target.Category)
			}
		}
	}

	return nil
}

// Loader reads the configuration file and remembers the last valid result.
type Loader struct {
	path     string
	mu       sync.Mutex
	lastGood *Config
}

// NewLoader builds a Loader; an empty path means notifications are unconfigured.
func NewLoader(path string) *Loader {
	return &Loader{path: path}
}

// Current reads the file on every call. A missing file is an empty
// configuration. An invalid file is logged on every call; the last valid
// configuration is served when there is one, otherwise the error is returned.
func (l *Loader) Current() (Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.path == "" {
		l.lastGood = nil

		return Config{}, nil
	}

	raw, err := os.ReadFile(l.path)
	if errors.Is(err, fs.ErrNotExist) {
		l.lastGood = nil

		return Config{}, nil
	}

	var cfg Config

	if err == nil {
		if err = yaml.Unmarshal(raw, &cfg); err != nil {
			err = fmt.Errorf("parsing %s: %w", l.path, err)
		} else if err = cfg.Validate(); err != nil {
			err = fmt.Errorf("invalid %s: %w", l.path, err)
		}
	} else {
		err = fmt.Errorf("reading %s: %w", l.path, err)
	}

	if err != nil {
		slog.Error("notifications configuration is invalid", "path", l.path, "err", err)

		if l.lastGood != nil {
			return *l.lastGood, nil
		}

		return Config{}, err
	}

	l.lastGood = &cfg

	return cfg, nil
}
