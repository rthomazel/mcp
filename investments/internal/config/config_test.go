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
		assert.Equal(t, Defaults().EventMaxAge, cfg.EventMaxAge)
	})

	t.Run("event, worker and market overrides", func(t *testing.T) {
		t.Setenv("INVESTMENTS_MCP_DSN", "postgres://x")
		t.Setenv("INVESTMENTS_MCP_TIINGO_API_KEY", "key")
		t.Setenv("INVESTMENTS_MCP_MARKET_TIMEZONE", "Europe/London")
		t.Setenv("INVESTMENTS_MCP_MAX_HISTORY_POINTS", "50")
		t.Setenv("INVESTMENTS_MCP_EVENT_MAX_FAILURES", "3")
		t.Setenv("INVESTMENTS_MCP_EVENT_BACKOFF_BASE", "1s")
		t.Setenv("INVESTMENTS_MCP_EVENT_MAX_AGE", "48h")
		t.Setenv("INVESTMENTS_MCP_EVENT_RETENTION", "720h")
		t.Setenv("INVESTMENTS_MCP_WORKER_POLL_INTERVAL", "5s")
		t.Setenv("INVESTMENTS_MCP_WORKER_PRUNE_INTERVAL", "1h")
		t.Setenv("INVESTMENTS_MCP_WORKER_BATCH_LIMIT", "10")
		t.Setenv("INVESTMENTS_MCP_WORKER_HANDLER_TIMEOUT", "3s")
		t.Setenv("INVESTMENTS_MCP_MONITOR_TIMEOUT", "1m")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "Europe/London", cfg.MarketTimezone)
		assert.Equal(t, 50, cfg.MaxHistoryPoints)
		assert.Equal(t, 3, cfg.EventMaxFailures)
		assert.Equal(t, time.Second, cfg.EventBackoffBase)
		assert.Equal(t, 48*time.Hour, cfg.EventMaxAge)
		assert.Equal(t, 720*time.Hour, cfg.EventRetention)
		assert.Equal(t, 5*time.Second, cfg.WorkerPollInterval)
		assert.Equal(t, time.Hour, cfg.WorkerPruneInterval)
		assert.Equal(t, 10, cfg.WorkerBatchLimit)
		assert.Equal(t, 3*time.Second, cfg.WorkerHandlerTimeout)
		assert.Equal(t, time.Minute, cfg.MonitorTimeout)
	})

	t.Run("defaults match the documented values", func(t *testing.T) {
		defaults := Defaults()
		assert.Equal(t, "America/New_York", defaults.MarketTimezone)
		assert.Equal(t, 1000, defaults.MaxHistoryPoints)
		assert.Equal(t, 10, defaults.EventMaxFailures)
		assert.Equal(t, 30*time.Second, defaults.EventBackoffBase)
		assert.Equal(t, 7*24*time.Hour, defaults.EventMaxAge)
		assert.Equal(t, 365*24*time.Hour, defaults.EventRetention)
		assert.Equal(t, 30*time.Second, defaults.WorkerPollInterval)
		assert.Equal(t, 24*time.Hour, defaults.WorkerPruneInterval)
		assert.Equal(t, 100, defaults.WorkerBatchLimit)
		assert.Equal(t, 30*time.Second, defaults.WorkerHandlerTimeout)
		assert.Equal(t, 2*time.Minute, defaults.MonitorTimeout)
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
		"bad max age":      {"INVESTMENTS_MCP_EVENT_MAX_AGE": "week"},
		"zero poll":        {"INVESTMENTS_MCP_WORKER_POLL_INTERVAL": "0s"},
		"zero failures":    {"INVESTMENTS_MCP_EVENT_MAX_FAILURES": "0"},
		"bad batch":        {"INVESTMENTS_MCP_WORKER_BATCH_LIMIT": "lots"},
		"zero points":      {"INVESTMENTS_MCP_MAX_HISTORY_POINTS": "0"},
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
