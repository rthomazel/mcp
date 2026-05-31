package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "postgres-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfig_Defaults(t *testing.T) {
	path := writeConfig(t, "dsn: postgres://localhost/test\n")
	cfg, err := LoadConfig(path)
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
	path := writeConfig(t, "max_rows: 50\n")
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing dsn")
	}
	if err.Error() != "dsn is required" {
		t.Errorf("error = %q, want %q", err.Error(), "dsn is required")
	}
}

func TestLoadConfig_MissingFileReturnsError(t *testing.T) {
	_, err := LoadConfig("/no/such/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_QueryTimeout(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		path := writeConfig(t, "dsn: postgres://localhost/test\nquery_timeout: 1m\n")
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.QueryTimeout != time.Minute {
			t.Errorf("QueryTimeout = %v, want 1m", cfg.QueryTimeout)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		path := writeConfig(t, "dsn: postgres://localhost/test\nquery_timeout: bad\n")
		_, err := LoadConfig(path)
		if err == nil {
			t.Fatal("expected error for invalid duration")
		}
	})
}

func TestLoadConfig_MaxRows(t *testing.T) {
	t.Run("explicit value", func(t *testing.T) {
		path := writeConfig(t, "dsn: postgres://localhost/test\nmax_rows: 250\n")
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.MaxRows != 250 {
			t.Errorf("MaxRows = %d, want 250", cfg.MaxRows)
		}
	})
	t.Run("zero uses default", func(t *testing.T) {
		path := writeConfig(t, "dsn: postgres://localhost/test\nmax_rows: 0\n")
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.MaxRows != 100 {
			t.Errorf("MaxRows = %d, want 100 (default)", cfg.MaxRows)
		}
	})
	t.Run("negative uses default", func(t *testing.T) {
		path := writeConfig(t, "dsn: postgres://localhost/test\nmax_rows: -1\n")
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}
		if cfg.MaxRows != 100 {
			t.Errorf("MaxRows = %d, want 100 (default)", cfg.MaxRows)
		}
	})
}

func TestLoadConfig_BoolFlags(t *testing.T) {
	type boolCase struct {
		yamlKey string
		field   func(*Config) bool
		name    string
	}
	flags := []boolCase{
		{"allow_mutate", func(c *Config) bool { return c.AllowMutate }, "AllowMutate"},
		{"allow_mutate_schema", func(c *Config) bool { return c.AllowMutateSchema }, "AllowMutateSchema"},
		{"allow_mutate_permissions", func(c *Config) bool { return c.AllowMutatePermissions }, "AllowMutatePermissions"},
		{"allow_transactions", func(c *Config) bool { return c.AllowTransactions }, "AllowTransactions"},
		{"allow_diagnostics", func(c *Config) bool { return c.AllowDiagnostics }, "AllowDiagnostics"},
		{"allow_explain_analyze", func(c *Config) bool { return c.AllowExplainAnalyze }, "AllowExplainAnalyze"},
	}
	for _, ff := range flags {
		t.Run(ff.name+"=true", func(t *testing.T) {
			path := writeConfig(t, "dsn: postgres://localhost/test\n"+ff.yamlKey+": true\n")
			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}
			if !ff.field(cfg) {
				t.Errorf("%s = false, want true", ff.name)
			}
		})
		t.Run(ff.name+"=false", func(t *testing.T) {
			path := writeConfig(t, "dsn: postgres://localhost/test\n"+ff.yamlKey+": false\n")
			cfg, err := LoadConfig(path)
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
	path := writeConfig(t, "dsn: postgres://localhost/test\nallowed_schemas:\n  - public\n  - app\n")
	cfg, err := LoadConfig(path)
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
	path := writeConfig(t, "dsn: postgres://localhost/test\ndenied_schemas:\n  - myschema\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if len(cfg.DeniedSchemas) != 1 || cfg.DeniedSchemas[0] != "myschema" {
		t.Errorf("DeniedSchemas = %v, want [myschema]", cfg.DeniedSchemas)
	}
}

func TestLoadConfig_DefaultSchemaOverride(t *testing.T) {
	path := writeConfig(t, "dsn: postgres://localhost/test\ndefault_schema: myapp\n")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.DefaultSchema != "myapp" {
		t.Errorf("DefaultSchema = %q, want myapp", cfg.DefaultSchema)
	}
}
