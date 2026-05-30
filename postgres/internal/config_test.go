package internal

import (
	"testing"
	"time"
)

func TestLoadConfig_DefaultsAppliedWhenOnlyDatabaseURLSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://localhost/test")
	}
	if cfg.QueryTimeout != 30*time.Second {
		t.Errorf("QueryTimeout = %v, want %v", cfg.QueryTimeout, 30*time.Second)
	}
	if cfg.PoolSize != 5 {
		t.Errorf("PoolSize = %d, want 5", cfg.PoolSize)
	}
	if cfg.MaxRows != 500 {
		t.Errorf("MaxRows = %d, want 500", cfg.MaxRows)
	}

	wantDenied := []string{"pg_toast", "pg_catalog", "information_schema"}
	if len(cfg.DeniedSchemas) != len(wantDenied) {
		t.Fatalf("DeniedSchemas length = %d, want %d", len(cfg.DeniedSchemas), len(wantDenied))
	}
	for i, v := range wantDenied {
		if cfg.DeniedSchemas[i] != v {
			t.Errorf("DeniedSchemas[%d] = %q, want %q", i, cfg.DeniedSchemas[i], v)
		}
	}

	if cfg.AllowDML {
		t.Errorf("AllowDML = true, want false")
	}
	if cfg.AllowDDL {
		t.Errorf("AllowDDL = true, want false")
	}
	if cfg.AllowDCL {
		t.Errorf("AllowDCL = true, want false")
	}
	if cfg.AllowTransactions {
		t.Errorf("AllowTransactions = true, want false")
	}
	if cfg.AllowDiagnostics {
		t.Errorf("AllowDiagnostics = true, want false")
	}
	if cfg.AllowExplainAnalyze {
		t.Errorf("AllowExplainAnalyze = true, want false")
	}
	if cfg.AllowedSchemas != nil {
		t.Errorf("AllowedSchemas = %v, want nil", cfg.AllowedSchemas)
	}
}

func TestLoadConfig_MissingDatabaseURLReturnsError(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want non-nil error")
	}
	if cfg != nil {
		t.Errorf("LoadConfig() returned non-nil config, want nil")
	}
	if err.Error() != "DATABASE_URL is required" {
		t.Errorf("error = %q, want %q", err.Error(), "DATABASE_URL is required")
	}
}

func TestLoadConfig_QueryTimeout(t *testing.T) {
	t.Run("valid duration", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_QUERY_TIMEOUT", "1m")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v, want nil", err)
		}
		if cfg.QueryTimeout != 1*time.Minute {
			t.Errorf("QueryTimeout = %v, want 1m", cfg.QueryTimeout)
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_QUERY_TIMEOUT", "bad")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want non-nil error")
		}
	})
}

func TestLoadConfig_PoolSize(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_POOL_SIZE", "10")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v, want nil", err)
		}
		if cfg.PoolSize != 10 {
			t.Errorf("PoolSize = %d, want 10", cfg.PoolSize)
		}
	})

	t.Run("zero returns error", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_POOL_SIZE", "0")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want non-nil")
		}
		if err.Error() != "POSTGRES_MCP_POOL_SIZE invalid: must be a positive integer" {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("non-integer returns error", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_POOL_SIZE", "abc")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want non-nil")
		}
	})
}

func TestLoadConfig_MaxRows(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_MAX_ROWS", "100")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v, want nil", err)
		}
		if cfg.MaxRows != 100 {
			t.Errorf("MaxRows = %d, want 100", cfg.MaxRows)
		}
	})

	t.Run("zero returns error", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_MAX_ROWS", "0")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want non-nil")
		}
		if err.Error() != "POSTGRES_MCP_MAX_ROWS invalid: must be a positive integer" {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("non-integer returns error", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_MAX_ROWS", "abc")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want non-nil")
		}
	})
}

func TestLoadConfig_BoolVariables(t *testing.T) {
	type boolField struct {
		env   string
		field func(*Config) bool
		name  string
	}

	fields := []boolField{
		{"POSTGRES_MCP_ALLOW_DML", func(c *Config) bool { return c.AllowDML }, "AllowDML"},
		{"POSTGRES_MCP_ALLOW_DDL", func(c *Config) bool { return c.AllowDDL }, "AllowDDL"},
		{"POSTGRES_MCP_ALLOW_DCL", func(c *Config) bool { return c.AllowDCL }, "AllowDCL"},
		{"POSTGRES_MCP_ALLOW_TRANSACTIONS", func(c *Config) bool { return c.AllowTransactions }, "AllowTransactions"},
		{"POSTGRES_MCP_ALLOW_DIAGNOSTICS", func(c *Config) bool { return c.AllowDiagnostics }, "AllowDiagnostics"},
		{"POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE", func(c *Config) bool { return c.AllowExplainAnalyze }, "AllowExplainAnalyze"},
	}

	for _, ff := range fields {
		t.Run(ff.name+"=true", func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv(ff.env, "true")

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if !ff.field(cfg) {
				t.Errorf("%s = false, want true", ff.name)
			}
		})

		t.Run(ff.name+"=false", func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv(ff.env, "false")

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if ff.field(cfg) {
				t.Errorf("%s = true, want false", ff.name)
			}
		})

		t.Run(ff.name+"=invalid", func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://localhost/test")
			t.Setenv(ff.env, "bad")

			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("LoadConfig() error = nil, want non-nil")
			}
		})
	}
}

func TestLoadConfig_AllowedSchemas(t *testing.T) {
	t.Run("parsed and trimmed", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://localhost/test")
		t.Setenv("POSTGRES_MCP_ALLOWED_SCHEMAS", "public, tenant_a ")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		want := []string{"public", "tenant_a"}
		if len(cfg.AllowedSchemas) != len(want) {
			t.Fatalf("AllowedSchemas = %v, want %v", cfg.AllowedSchemas, want)
		}
		for i, v := range want {
			if cfg.AllowedSchemas[i] != v {
				t.Errorf("AllowedSchemas[%d] = %q, want %q", i, cfg.AllowedSchemas[i], v)
			}
		}
	})
}

func TestLoadConfig_DeniedSchemasOverridesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POSTGRES_MCP_DENIED_SCHEMAS", "myschema")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(cfg.DeniedSchemas) != 1 || cfg.DeniedSchemas[0] != "myschema" {
		t.Errorf("DeniedSchemas = %v, want [myschema]", cfg.DeniedSchemas)
	}
}
