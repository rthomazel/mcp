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

// Config is the server configuration.
type Config struct {
	DSN            string
	QueryTimeout   time.Duration
	StalePriceDays int
}

// Load reads the configuration from INVESTMENTS_MCP_* environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		DSN:            os.Getenv("INVESTMENTS_MCP_DSN"),
		QueryTimeout:   defaultQueryTimeout,
		StalePriceDays: defaultStalePriceDays,
	}

	if cfg.DSN == "" {
		return nil, errors.New("INVESTMENTS_MCP_DSN is required")
	}

	if v := os.Getenv("INVESTMENTS_MCP_QUERY_TIMEOUT"); v != "" {
		timeout, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parsing INVESTMENTS_MCP_QUERY_TIMEOUT: %w", err)
		}

		if timeout <= 0 {
			return nil, fmt.Errorf("INVESTMENTS_MCP_QUERY_TIMEOUT must be positive, got %s", v)
		}

		cfg.QueryTimeout = timeout
	}

	if v := os.Getenv("INVESTMENTS_MCP_STALE_PRICE_DAYS"); v != "" {
		days, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("parsing INVESTMENTS_MCP_STALE_PRICE_DAYS: %w", err)
		}

		if days < 1 {
			return nil, fmt.Errorf("INVESTMENTS_MCP_STALE_PRICE_DAYS must be at least 1, got %s", v)
		}

		cfg.StalePriceDays = days
	}

	return cfg, nil
}
