package internal

import (
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.DSN != "postgres://localhost/test" {
		t.Errorf("DSN = %q, want %q", cfg.DSN, "postgres://localhost/test")
	}
	if cfg.QueryTimeout != 30*time.Second {
		t.Errorf("QueryTimeout = %v, want 30s", cfg.QueryTimeout)
	}
	if cfg.MaxRows != 100 {
		t.Errorf("MaxRows = %d, want 100", cfg.MaxRows)
	}
	if cfg.DefaultSchema != "public" {
		t.Errorf("DefaultSchema = %q, want public", cfg.DefaultSchema)
	}
	wantDenied := []string{"pg_toast", "pg_catalog", "information_schema"}
	if len(cfg.DeniedSchemas) != len(wantDenied) {
		t.Fatalf("DeniedSchemas = %v, want %v", cfg.DeniedSchemas, wantDenied)
	}
	for i, v := range wantDenied {
		if cfg.DeniedSchemas[i] != v {
			t.Errorf("DeniedSchemas[%d] = %q, want %q", i, cfg.DeniedSchemas[i], v)
		}
	}
	if cfg.AllowMutate || cfg.AllowMutateSchema || cfg.AllowMutatePermissions ||
		cfg.AllowTransactions || cfg.AllowDiagnostics || cfg.AllowExplainAnalyze {
		t.Errorf("expected all allow flags false")
	}
	if cfg.AllowedSchemas != nil {
		t.Errorf("AllowedSchemas = %v, want nil", cfg.AllowedSchemas)
	}
}

func TestLoadConfig_MissingDSNReturnsError(t *testing.T) {
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing dsn")
	}
}

func TestLoadConfig_QueryTimeout(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_QUERY_TIMEOUT", "1m")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.QueryTimeout != time.Minute {
			t.Errorf("QueryTimeout = %v, want 1m", cfg.QueryTimeout)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_QUERY_TIMEOUT", "bad")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})
}

func TestLoadConfig_MaxRows(t *testing.T) {
	t.Run("explicit value", func(t *testing.T) {
		t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_MAX_ROWS", "250")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.MaxRows != 250 {
			t.Errorf("MaxRows = %d, want 250", cfg.MaxRows)
		}
	})
	t.Run("zero uses default", func(t *testing.T) {
		t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_MAX_ROWS", "0")
		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.MaxRows != 100 {
			t.Errorf("MaxRows = %d, want 100 (default)", cfg.MaxRows)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_MAX_ROWS", "notanumber")
		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error for invalid max_rows")
		}
	})
}

func TestLoadConfig_BoolFlags(t *testing.T) {
	type boolCase struct {
		envKey string
		field  func(*Config) bool
		name   string
	}
	flags := []boolCase{
		{"POSTGRES_MCP_ALLOW_MUTATE", func(c *Config) bool { return c.AllowMutate }, "AllowMutate"},
		{"POSTGRES_MCP_ALLOW_MUTATE_SCHEMA", func(c *Config) bool { return c.AllowMutateSchema }, "AllowMutateSchema"},
		{"POSTGRES_MCP_ALLOW_MUTATE_PERMISSIONS", func(c *Config) bool { return c.AllowMutatePermissions }, "AllowMutatePermissions"},
		{"POSTGRES_MCP_ALLOW_TRANSACTIONS", func(c *Config) bool { return c.AllowTransactions }, "AllowTransactions"},
		{"POSTGRES_MCP_ALLOW_DIAGNOSTICS", func(c *Config) bool { return c.AllowDiagnostics }, "AllowDiagnostics"},
		{"POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE", func(c *Config) bool { return c.AllowExplainAnalyze }, "AllowExplainAnalyze"},
	}
	for _, ff := range flags {
		t.Run(ff.name+"=true", func(t *testing.T) {
			t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
			t.Setenv(ff.envKey, "true")
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if !ff.field(cfg) {
				t.Errorf("%s = false, want true", ff.name)
			}
		})
		t.Run(ff.name+"=false", func(t *testing.T) {
			t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
			t.Setenv(ff.envKey, "false")
			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if ff.field(cfg) {
				t.Errorf("%s = true, want false", ff.name)
			}
		})
	}
}

func TestLoadConfig_AllowedSchemas(t *testing.T) {
	t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
	t.Setenv("POSTGRES_MCP_ALLOWED_SCHEMAS", "public,app")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	want := []string{"public", "app"}
	if len(cfg.AllowedSchemas) != len(want) {
		t.Fatalf("AllowedSchemas = %v, want %v", cfg.AllowedSchemas, want)
	}
	for i, v := range want {
		if cfg.AllowedSchemas[i] != v {
			t.Errorf("AllowedSchemas[%d] = %q, want %q", i, cfg.AllowedSchemas[i], v)
		}
	}
}

func TestLoadConfig_DeniedSchemasOverridesDefault(t *testing.T) {
	t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
	t.Setenv("POSTGRES_MCP_DENIED_SCHEMAS", "myschema")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(cfg.DeniedSchemas) != 1 || cfg.DeniedSchemas[0] != "myschema" {
		t.Errorf("DeniedSchemas = %v, want [myschema]", cfg.DeniedSchemas)
	}
}

func TestLoadConfig_DefaultSchemaOverride(t *testing.T) {
	t.Setenv("POSTGRES_MCP_DSN", "postgres://localhost/test")
	t.Setenv("POSTGRES_MCP_DEFAULT_SCHEMA", "myapp")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.DefaultSchema != "myapp" {
		t.Errorf("DefaultSchema = %q, want myapp", cfg.DefaultSchema)
	}
}
