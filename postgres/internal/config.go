// Package internal holds configuration and shared utilities for postgres-mcp.
package internal

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// configFile is the raw YAML structure decoded from the config file.
// Duration fields are stored as strings and parsed separately.
type configFile struct {
	DSN                    string   `yaml:"dsn"`
	QueryTimeout           string   `yaml:"query_timeout"`
	MaxRows                int      `yaml:"max_rows"`
	DefaultSchema          string   `yaml:"default_schema"`
	AllowMutate            bool     `yaml:"allow_mutate"`
	AllowMutateSchema      bool     `yaml:"allow_mutate_schema"`
	AllowMutatePermissions bool     `yaml:"allow_mutate_permissions"`
	AllowTransactions      bool     `yaml:"allow_transactions"`
	AllowDiagnostics       bool     `yaml:"allow_diagnostics"`
	AllowExplainAnalyze    bool     `yaml:"allow_explain_analyze"`
	AllowedSchemas         []string `yaml:"allowed_schemas"`
	DeniedSchemas          []string `yaml:"denied_schemas"`
}

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

// Path returns the config file path.
// Reads POSTGRES_MCP_CONFIG env var; defaults to "postgres-mcp.yaml".
func Path() string {
	if p := os.Getenv("POSTGRES_MCP_CONFIG"); p != "" {
		return p
	}
	return "postgres-mcp.yaml"
}

// LoadConfig reads and validates the YAML config file at path.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %q: %w", path, err)
	}
	defer f.Close()

	var raw configFile
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}

	cfg := &Config{
		DSN:                    raw.DSN,
		MaxRows:                raw.MaxRows,
		DefaultSchema:          raw.DefaultSchema,
		AllowMutate:            raw.AllowMutate,
		AllowMutateSchema:      raw.AllowMutateSchema,
		AllowMutatePermissions: raw.AllowMutatePermissions,
		AllowTransactions:      raw.AllowTransactions,
		AllowDiagnostics:       raw.AllowDiagnostics,
		AllowExplainAnalyze:    raw.AllowExplainAnalyze,
		AllowedSchemas:         raw.AllowedSchemas,
		DeniedSchemas:          raw.DeniedSchemas,
	}

	if raw.QueryTimeout != "" {
		d, err := time.ParseDuration(raw.QueryTimeout)
		if err != nil {
			return nil, fmt.Errorf("query_timeout invalid: %w", err)
		}
		cfg.QueryTimeout = d
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
		return nil, fmt.Errorf("dsn is required")
	}

	return cfg, nil
}
