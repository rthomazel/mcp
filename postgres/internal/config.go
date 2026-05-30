// Package internal holds configuration and shared utilities for postgres-mcp.
package internal

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL         string
	QueryTimeout        time.Duration
	PoolSize            int
	MaxRows             int
	Transport           string
	AllowDML            bool
	AllowDDL            bool
	AllowDCL            bool
	AllowTransactions   bool
	AllowDiagnostics    bool
	AllowExplainAnalyze bool
	AllowedSchemas      []string
	DeniedSchemas       []string
}

var defaults = Config{
	QueryTimeout:  30 * time.Second,
	PoolSize:      5,
	MaxRows:       500,
	DeniedSchemas: []string{"pg_toast", "pg_catalog", "information_schema"},
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		QueryTimeout:  defaults.QueryTimeout,
		PoolSize:      defaults.PoolSize,
		MaxRows:       defaults.MaxRows,
		DeniedSchemas: defaults.DeniedSchemas,
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if raw := os.Getenv("POSTGRES_MCP_QUERY_TIMEOUT"); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("POSTGRES_MCP_QUERY_TIMEOUT invalid: %w", err)
		}
		cfg.QueryTimeout = d
	}

	if raw := os.Getenv("POSTGRES_MCP_POOL_SIZE"); raw != "" {
		size, err := strconv.Atoi(raw)
		if err != nil || size < 1 {
			return nil, fmt.Errorf("POSTGRES_MCP_POOL_SIZE invalid: must be a positive integer")
		}
		cfg.PoolSize = size
	}

	if raw := os.Getenv("POSTGRES_MCP_MAX_ROWS"); raw != "" {
		maxRows, err := strconv.Atoi(raw)
		if err != nil || maxRows < 1 {
			return nil, fmt.Errorf("POSTGRES_MCP_MAX_ROWS invalid: must be a positive integer")
		}
		cfg.MaxRows = maxRows
	}

	cfg.Transport = os.Getenv("POSTGRES_MCP_TRANSPORT")

	for _, pair := range []struct {
		env string
		dst *bool
	}{
		{"POSTGRES_MCP_ALLOW_DML", &cfg.AllowDML},
		{"POSTGRES_MCP_ALLOW_DDL", &cfg.AllowDDL},
		{"POSTGRES_MCP_ALLOW_DCL", &cfg.AllowDCL},
		{"POSTGRES_MCP_ALLOW_TRANSACTIONS", &cfg.AllowTransactions},
		{"POSTGRES_MCP_ALLOW_DIAGNOSTICS", &cfg.AllowDiagnostics},
		{"POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE", &cfg.AllowExplainAnalyze},
	} {
		if raw := os.Getenv(pair.env); raw != "" {
			val, err := strconv.ParseBool(raw)
			if err != nil {
				return nil, fmt.Errorf("%s invalid: %w", pair.env, err)
			}
			*pair.dst = val
		}
	}

	parseSchemas := func(raw string) []string {
		var out []string
		for _, s := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}

	if raw := os.Getenv("POSTGRES_MCP_ALLOWED_SCHEMAS"); raw != "" {
		cfg.AllowedSchemas = parseSchemas(raw)
	}
	if raw := os.Getenv("POSTGRES_MCP_DENIED_SCHEMAS"); raw != "" {
		cfg.DeniedSchemas = parseSchemas(raw)
	}

	return cfg, nil
}
