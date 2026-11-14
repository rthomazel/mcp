package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("INVESTMENTS_MCP_DSN", "postgres://x")
		t.Setenv("INVESTMENTS_MCP_TIINGO_API_KEY", "key")
		t.Setenv("INVESTMENTS_MCP_QUERY_TIMEOUT", "")
		t.Setenv("INVESTMENTS_MCP_STALE_PRICE_DAYS", "")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "postgres://x", cfg.DSN)
		assert.Equal(t, 30*time.Second, cfg.QueryTimeout)
		assert.Equal(t, 5, cfg.StalePriceDays)
		assert.Equal(t, "key", cfg.TiingoAPIKey)
	})

	t.Run("overrides", func(t *testing.T) {
		t.Setenv("INVESTMENTS_MCP_DSN", "postgres://x")
		t.Setenv("INVESTMENTS_MCP_TIINGO_API_KEY", "key")
		t.Setenv("INVESTMENTS_MCP_QUERY_TIMEOUT", "5s")
		t.Setenv("INVESTMENTS_MCP_STALE_PRICE_DAYS", "3")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, cfg.QueryTimeout)
		assert.Equal(t, 3, cfg.StalePriceDays)
	})

	for name, env := range map[string]map[string]string{
		"missing dsn":      {"INVESTMENTS_MCP_DSN": ""},
		"missing api key":  {"INVESTMENTS_MCP_TIINGO_API_KEY": ""},
		"bad timeout":      {"INVESTMENTS_MCP_QUERY_TIMEOUT": "soon"},
		"zero timeout":     {"INVESTMENTS_MCP_QUERY_TIMEOUT": "0s"},
		"negative timeout": {"INVESTMENTS_MCP_QUERY_TIMEOUT": "-1s"},
		"bad days":         {"INVESTMENTS_MCP_STALE_PRICE_DAYS": "many"},
		"zero days":        {"INVESTMENTS_MCP_STALE_PRICE_DAYS": "0"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("INVESTMENTS_MCP_DSN", "postgres://x")
			t.Setenv("INVESTMENTS_MCP_TIINGO_API_KEY", "key")
			t.Setenv("INVESTMENTS_MCP_QUERY_TIMEOUT", "")
			t.Setenv("INVESTMENTS_MCP_STALE_PRICE_DAYS", "")

			for k, v := range env {
				t.Setenv(k, v)
			}

			_, err := Load()
			require.Error(t, err)
		})
	}
}
