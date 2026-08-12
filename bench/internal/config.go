package internal

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Timeout             time.Duration
	BackgroundTimeout   time.Duration
	Home                string
	MiseDir             string
	EditMaxLines        int
	MaxCandidates       int
	ToolCallWorkers     int
	StatsRedactPatterns []*regexp.Regexp
}

var defaults = Config{
	Timeout:           15 * time.Second,
	BackgroundTimeout: 5 * time.Minute,
	EditMaxLines:      50,
	MaxCandidates:     5,
	// ToolCallWorkers defaults to 1: mcp-go's stdio transport dequeues tool calls in
	// arrival order but, with more than one worker, processes and writes responses
	// independently -- so completion order (and any side effects an agent depends on
	// ordering, e.g. a commit followed by a push issued in the same turn) can race and
	// finish out of order. A single worker drains the queue strictly FIFO, which
	// restores deterministic ordering matching submission order. See
	// BENCH_MCP_TOOL_CALL_WORKERS to opt into concurrent (out-of-order) processing for
	// throughput on workloads with no cross-call ordering dependency.
	ToolCallWorkers: 1,
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
		ToolCallWorkers:   defaults.ToolCallWorkers,
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

	if raw := os.Getenv("BENCH_MCP_TOOL_CALL_WORKERS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("BENCH_MCP_TOOL_CALL_WORKERS invalid: must be a positive integer")
		}
		cfg.ToolCallWorkers = n
	}

	if raw := os.Getenv("BENCH_MCP_STATS_REDACT_PATTERNS"); raw != "" {
		for i, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			re, err := regexp.Compile(line)
			if err != nil {
				slog.Error("BENCH_MCP_STATS_REDACT_PATTERNS: invalid regex, skipping",
					"pattern", line, "line", i+1, "err", err)
				continue
			}
			cfg.StatsRedactPatterns = append(cfg.StatsRedactPatterns, re)
		}
	}

	return cfg, nil
}

// NormalizerVersion is incremented whenever the command normalization rules
// change. Rows with different versions should not be grouped by cmd_hash.
const NormalizerVersion = 1

// StatsEncryptionKeyPath is the Docker Secret mount path for the stats AES-256 key.
const StatsEncryptionKeyPath = "/run/secrets/bench_mcp_stats_encryption_key_v1"

// StatsP95MinSamples is the minimum number of samples required before p95 is computed.
const StatsP95MinSamples = 20

// StatsTopLines is the maximum number of commands shown in the stats top-commands table.
const StatsTopLines = 20

// HintCWDParsed is appended to the shell metadata block when a leading "cd PATH &&" prefix
// is parsed and applied as the effective working directory.
const HintCWDParsed = "cwd parsed from 'cd' prefix; pass cwd= directly instead"

// HintAndAndSplit is appended to the shell metadata block when a " && " chain is split
// into independent commands.
const HintAndAndSplit = "auto-split from && chain; commands run independently (no short-circuit on failure)"
