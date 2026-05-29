package internal

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Timeout           time.Duration
	BackgroundTimeout time.Duration
	Home              string
	MiseDir           string
	EditMaxLines      int
	MaxCandidates     int
}

var defaults = Config{
	Timeout:           15 * time.Second,
	BackgroundTimeout: 5 * time.Minute,
	EditMaxLines:      50,
	MaxCandidates:     5,
}

func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home directory: %w", err)
	}

	if raw := os.Getenv("BENCH_MCP_HOME"); raw != "" {
		home = raw
	}

	miseDir := "/mise"
	if raw := os.Getenv("BENCH_MCP_MISE_DIR"); raw != "" {
		miseDir = raw
	}

	cfg := &Config{
		Timeout:           defaults.Timeout,
		BackgroundTimeout: defaults.BackgroundTimeout,
		Home:              home,
		MiseDir:           miseDir,
		EditMaxLines:      defaults.EditMaxLines,
		MaxCandidates:     defaults.MaxCandidates,
	}

	if raw := os.Getenv("BENCH_MCP_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("BENCH_MCP_TIMEOUT invalid: %w", err)
		}
		cfg.Timeout = d
	}

	if raw := os.Getenv("BENCH_MCP_BACKGROUND_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("BENCH_MCP_BACKGROUND_TIMEOUT invalid: %w", err)
		}
		cfg.BackgroundTimeout = d
	}

	if raw := os.Getenv("BENCH_MCP_EDIT_MAX_LINES"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("BENCH_MCP_EDIT_MAX_LINES invalid: must be a positive integer")
		}
		cfg.EditMaxLines = n
	}

	if raw := os.Getenv("BENCH_MCP_MAX_CANDIDATES"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("BENCH_MCP_MAX_CANDIDATES invalid: must be a positive integer")
		}
		cfg.MaxCandidates = n
	}

	return cfg, nil
}
