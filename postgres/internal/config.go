// Package internal holds configuration and shared utilities for postgres-mcp.
package internal

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds validated, parsed configuration for postgres-mcp.
type Config struct {
	DSN                    string
	QueryTimeout           time.Duration
	MaxRows                int
	DefaultSchema          string
	AllowMutate            bool
	AllowMutateSchema      bool
	AllowMutatePermissions bool
	AllowTransactions      bool
	AllowDiagnostics       bool
	AllowExplainAnalyze    bool
	AllowedSchemas         []string
	DeniedSchemas          []string
}

// LoadConfig reads configuration from environment variables (all prefixed POSTGRES_MCP_).
func LoadConfig() (*Config, error) {
	cfg := &Config{
		DSN:                    os.Getenv("POSTGRES_MCP_DSN"),
		DefaultSchema:          os.Getenv("POSTGRES_MCP_DEFAULT_SCHEMA"),
		AllowMutate:            envBool("POSTGRES_MCP_ALLOW_MUTATE"),
		AllowMutateSchema:      envBool("POSTGRES_MCP_ALLOW_MUTATE_SCHEMA"),
		AllowMutatePermissions: envBool("POSTGRES_MCP_ALLOW_MUTATE_PERMISSIONS"),
		AllowTransactions:      envBool("POSTGRES_MCP_ALLOW_TRANSACTIONS"),
		AllowDiagnostics:       envBool("POSTGRES_MCP_ALLOW_DIAGNOSTICS"),
		AllowExplainAnalyze:    envBool("POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE"),
	}

	if v := os.Getenv("POSTGRES_MCP_QUERY_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("POSTGRES_MCP_QUERY_TIMEOUT invalid: %w", err)
		}
		cfg.QueryTimeout = d
	}

	if v := os.Getenv("POSTGRES_MCP_MAX_ROWS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("POSTGRES_MCP_MAX_ROWS invalid: %w", err)
		}
		cfg.MaxRows = n
	}

	if v := os.Getenv("POSTGRES_MCP_ALLOWED_SCHEMAS"); v != "" {
		cfg.AllowedSchemas = strings.Split(v, ",")
	}

	if v := os.Getenv("POSTGRES_MCP_DENIED_SCHEMAS"); v != "" {
		cfg.DeniedSchemas = strings.Split(v, ",")
	}

	// defaults for zero values
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 30 * time.Second
	}
	if cfg.MaxRows <= 0 {
		cfg.MaxRows = 100
	}
	if cfg.DefaultSchema == "" {
		cfg.DefaultSchema = "public"
	}
	if len(cfg.DeniedSchemas) == 0 {
		cfg.DeniedSchemas = []string{"pg_toast", "pg_catalog", "information_schema"}
	}

	if cfg.DSN == "" {
		return nil, fmt.Errorf("POSTGRES_MCP_DSN is required")
	}

	return cfg, nil
}

// envBool returns true if the named env var is set to "true" (case-insensitive).
func envBool(name string) bool {
	return strings.EqualFold(os.Getenv(name), "true")
}
