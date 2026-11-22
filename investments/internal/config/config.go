package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultQueryTimeout   = 30 * time.Second
	defaultStalePriceDays = 5
)

// Config is the server configuration. Every tunable has a default, so only the
// required values need to be provided.
type Config struct {
	DSN            string
	QueryTimeout   time.Duration
	StalePriceDays int
	TiingoAPIKey   string // server-side secret, never logged or returned

	// NotificationsConfigFile is the path of the notifications YAML file; empty means unconfigured.
	NotificationsConfigFile string
	// TelegramBotToken is a server-side secret, never logged or returned; empty means Telegram is unconfigured.
	TelegramBotToken string

	// MarketTimezone is the IANA zone of the market day, shared by the daily job, the monitors and today's date.
	MarketTimezone string
	// MaxHistoryPoints is the most points one value history query returns.
	MaxHistoryPoints int

	// EventMaxFailures is the failed attempts after which an event is given up on.
	EventMaxFailures int
	// EventBackoffBase is the delay after the first failure; each further failure doubles it.
	EventBackoffBase time.Duration
	// EventMaxAge is how long after it was recorded an event is still considered for handling.
	EventMaxAge time.Duration
	// EventRetention is how long after it was fully handled an event is kept.
	EventRetention time.Duration

	// WorkerPollInterval is how often the notifications worker looks for due events.
	WorkerPollInterval time.Duration
	// WorkerPruneInterval is how often the notifications worker prunes handled events.
	WorkerPruneInterval time.Duration
	// WorkerBatchLimit is the events taken per pass.
	WorkerBatchLimit int
	// WorkerHandlerTimeout is the deadline for one handler call.
	WorkerHandlerTimeout time.Duration
	// MonitorTimeout is the deadline for one monitor run.
	MonitorTimeout time.Duration
}

// Defaults returns the configuration with every default and no required value.
func Defaults() Config {
	return Config{
		QueryTimeout:         defaultQueryTimeout,
		StalePriceDays:       defaultStalePriceDays,
		MarketTimezone:       "America/New_York",
		MaxHistoryPoints:     1000,
		EventMaxFailures:     10,
		EventBackoffBase:     30 * time.Second,
		EventMaxAge:          7 * 24 * time.Hour,
		EventRetention:       365 * 24 * time.Hour,
		WorkerPollInterval:   30 * time.Second,
		WorkerPruneInterval:  24 * time.Hour,
		WorkerBatchLimit:     100,
		WorkerHandlerTimeout: 30 * time.Second,
		MonitorTimeout:       2 * time.Minute,
	}
}

// Load reads the configuration from INVESTMENTS_MCP_* environment variables.
func Load() (*Config, error) {
	cfg := Defaults()
	cfg.DSN = os.Getenv("INVESTMENTS_MCP_DSN")
	cfg.TiingoAPIKey = os.Getenv("INVESTMENTS_MCP_TIINGO_API_KEY")
	cfg.NotificationsConfigFile = os.Getenv("INVESTMENTS_MCP_NOTIFICATIONS_CONFIG_FILE")
	cfg.TelegramBotToken = os.Getenv("INVESTMENTS_MCP_TELEGRAM_BOT_TOKEN")

	if cfg.DSN == "" {
		return nil, errors.New("INVESTMENTS_MCP_DSN is required")
	}

	if cfg.TiingoAPIKey == "" {
		return nil, errors.New("INVESTMENTS_MCP_TIINGO_API_KEY is required")
	}

	if v := os.Getenv("INVESTMENTS_MCP_MARKET_TIMEZONE"); v != "" {
		cfg.MarketTimezone = v
	}

	durations := []struct {
		name  string
		field *time.Duration
	}{
		{"INVESTMENTS_MCP_QUERY_TIMEOUT", &cfg.QueryTimeout},
		{"INVESTMENTS_MCP_EVENT_BACKOFF_BASE", &cfg.EventBackoffBase},
		{"INVESTMENTS_MCP_EVENT_MAX_AGE", &cfg.EventMaxAge},
		{"INVESTMENTS_MCP_EVENT_RETENTION", &cfg.EventRetention},
		{"INVESTMENTS_MCP_WORKER_POLL_INTERVAL", &cfg.WorkerPollInterval},
		{"INVESTMENTS_MCP_WORKER_PRUNE_INTERVAL", &cfg.WorkerPruneInterval},
		{"INVESTMENTS_MCP_WORKER_HANDLER_TIMEOUT", &cfg.WorkerHandlerTimeout},
		{"INVESTMENTS_MCP_MONITOR_TIMEOUT", &cfg.MonitorTimeout},
	}

	for _, d := range durations {
		if err := readDuration(d.name, d.field); err != nil {
			return nil, err
		}
	}

	integers := []struct {
		name  string
		field *int
	}{
		{"INVESTMENTS_MCP_STALE_PRICE_DAYS", &cfg.StalePriceDays},
		{"INVESTMENTS_MCP_MAX_HISTORY_POINTS", &cfg.MaxHistoryPoints},
		{"INVESTMENTS_MCP_EVENT_MAX_FAILURES", &cfg.EventMaxFailures},
		{"INVESTMENTS_MCP_WORKER_BATCH_LIMIT", &cfg.WorkerBatchLimit},
	}

	for _, i := range integers {
		if err := readPositiveInt(i.name, i.field); err != nil {
			return nil, err
		}
	}

	return &cfg, nil
}

// readDuration overrides *field with the positive duration in the variable, when set.
func readDuration(name string, field *time.Duration) error {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}

	value, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", name, err)
	}

	if value <= 0 {
		return fmt.Errorf("%s must be positive, got %s", name, v)
	}

	*field = value

	return nil
}

// readPositiveInt overrides *field with the integer of at least 1 in the variable, when set.
func readPositiveInt(name string, field *int) error {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}

	value, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", name, err)
	}

	if value < 1 {
		return fmt.Errorf("%s must be at least 1, got %s", name, v)
	}

	*field = value

	return nil
}
