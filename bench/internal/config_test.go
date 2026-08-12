package internal

import (
	"testing"
)

func TestLoadConfig_ToolCallWorkers(t *testing.T) {
	useCases := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "unset defaults to 1", raw: "", want: 1},
		{name: "explicit 1", raw: "1", want: 1},
		{name: "explicit greater than 1", raw: "5", want: 5},
		{name: "zero rejected", raw: "0", wantErr: true},
		{name: "negative rejected", raw: "-1", wantErr: true},
		{name: "non-numeric rejected", raw: "nope", wantErr: true},
	}

	for _, uc := range useCases {
		t.Run(uc.name, func(t *testing.T) {
			if uc.raw != "" {
				t.Setenv("BENCH_MCP_TOOL_CALL_WORKERS", uc.raw)
			}

			cfg, err := LoadConfig()
			if uc.wantErr {
				if err == nil {
					t.Fatalf("LoadConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("LoadConfig() unexpected error: %v", err)
			}
			if cfg.ToolCallWorkers != uc.want {
				t.Errorf("ToolCallWorkers = %d, want %d", cfg.ToolCallWorkers, uc.want)
			}
		})
	}
}
