package notification

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"

	"github.com/rthomazel/mcp/investments/internal/domain"
)

const (
	supportedVersion       = 1
	implementationTelegram = "telegram"
	implementationStub     = "stub"
	defaultFormat          = "plain"
	defaultCategory        = "default"
	categoriesKey          = "categories"
)

// Config is the complete notifications configuration, parsed from one YAML file.
type Config struct {
	Version       int                      `yaml:"version"`
	Channels      map[string]ChannelConfig `yaml:"channels"`
	Notifications map[string]Notification  `yaml:"notifications"`
}

// UnmarshalYAML reads the file and gives a channel written without a value, which
// the YAML decoder never hands to ChannelConfig, its one default category.
func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Version       int                       `yaml:"version"`
		Channels      map[string]*ChannelConfig `yaml:"channels"`
		Notifications map[string]Notification   `yaml:"notifications"`
	}

	if err := node.Decode(&raw); err != nil {
		return err //nolint:wrapcheck // the caller names the file
	}

	c.Version, c.Notifications = raw.Version, raw.Notifications
	c.Channels = make(map[string]ChannelConfig, len(raw.Channels))

	for name, channel := range raw.Channels {
		if channel == nil {
			channel = &ChannelConfig{Categories: map[string]map[string]string{defaultCategory: {}}}
		}

		c.Channels[name] = *channel
	}

	return nil
}

// ChannelConfig holds the settings of each category of a channel. It is
// written either as a categories map or as the settings of one category given
// inline, which becomes the category defaultCategory.
type ChannelConfig struct {
	Categories map[string]map[string]string
}

// Notification routes one event type to one category of each of its channels.
type Notification struct {
	Format   string            `yaml:"format"`
	Channels map[string]string `yaml:"channels"` // channel to category; empty means the channel's only category
}

// UnmarshalYAML accepts a categories map or inline settings of one category.
func (c *ChannelConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		c.Categories = map[string]map[string]string{defaultCategory: {}}

		return nil
	}

	var raw map[string]yaml.Node
	if err := node.Decode(&raw); err != nil {
		return fmt.Errorf("channel must be a mapping: %w", err)
	}

	categories, hasCategories := raw[categoriesKey]
	if hasCategories {
		if len(raw) > 1 {
			return errors.New("channel mixes a categories map with inline settings")
		}

		var decoded map[string]map[string]string
		if err := categories.Decode(&decoded); err != nil {
			return fmt.Errorf("decoding categories: %w", err)
		}

		c.Categories = decoded

		return nil
	}

	var settings map[string]string
	if err := node.Decode(&settings); err != nil {
		return fmt.Errorf("decoding channel settings: %w", err)
	}

	if settings == nil {
		settings = map[string]string{}
	}

	c.Categories = map[string]map[string]string{defaultCategory: settings}

	return nil
}

// Validate checks the configuration as a whole.
func (c Config) Validate() error {
	if c.Version != supportedVersion {
		return fmt.Errorf("unsupported configuration version %d, expected %d", c.Version, supportedVersion)
	}

	for name, channel := range c.Channels {
		if name != implementationTelegram && name != implementationStub {
			return fmt.Errorf("unknown channel implementation %q", name)
		}

		if len(channel.Categories) == 0 {
			return fmt.Errorf("channel %q has no categories", name)
		}

		for category, settings := range channel.Categories {
			if strings.TrimSpace(category) == "" {
				return fmt.Errorf("channel %q has a blank category name", name)
			}

			if name == implementationTelegram && strings.TrimSpace(settings[settingChatID]) == "" {
				return fmt.Errorf("telegram category %q has no chat_id", category)
			}
		}
	}

	for eventType, notification := range c.Notifications {
		if !domain.IsNotification(eventType) {
			return fmt.Errorf("notification for unknown event type %q", eventType)
		}

		if notification.Format != defaultFormat {
			return fmt.Errorf("notification %q has format %q, expected %q", eventType, notification.Format, defaultFormat)
		}

		if len(notification.Channels) == 0 {
			return fmt.Errorf("notification %q has no channels; remove it to silence the event type", eventType)
		}

		for channelName, category := range notification.Channels {
			if _, err := c.resolveCategory(eventType, channelName, category); err != nil {
				return err
			}
		}
	}

	return nil
}

// resolveCategory returns the category a notification delivers to on a
// channel: the one named, or the channel's only one when none is named.
func (c Config) resolveCategory(eventType, channelName, category string) (string, error) {
	channel, ok := c.Channels[channelName]
	if !ok {
		return "", fmt.Errorf("notification %q names undeclared channel %q", eventType, channelName)
	}

	if category == "" {
		if len(channel.Categories) != 1 {
			return "", fmt.Errorf("notification %q must name a category of channel %q", eventType, channelName)
		}

		for only := range channel.Categories {
			return only, nil
		}
	}

	if _, ok := channel.Categories[category]; !ok {
		return "", fmt.Errorf("notification %q names undeclared category %q of channel %q", eventType, category, channelName)
	}

	return category, nil
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
