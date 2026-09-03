package handlers

import (
	"context"
	"reflect"
	"testing"

	"github.com/rthomazel/mcp/bench/internal"
)

func TestBuildJobCommand(t *testing.T) {
	for _, uc := range []struct {
		name    string
		nice    int
		enable  bool
		wantArg []string
	}{
		{
			name: "nice enabled uses nice wrapper", nice: 10, enable: true,
			wantArg: []string{"nice", "-n", "10", "bash", "-c", "sleep 1"},
		},
		{
			name: "nice disabled uses plain bash", nice: 10, enable: false,
			wantArg: []string{"bash", "-c", "sleep 1"},
		},
		{
			name: "nice level 0 uses plain bash", nice: 0, enable: true,
			wantArg: []string{"bash", "-c", "sleep 1"},
		},
	} {
		t.Run(uc.name, func(t *testing.T) {
			cmd := buildJobCommand(context.Background(), &internal.Config{BackgroundNice: uc.nice}, "sleep 1", uc.enable)
			if !reflect.DeepEqual(cmd.Args, uc.wantArg) {
				t.Fatalf("args = %v, want %v", cmd.Args, uc.wantArg)
			}
		})
	}
}
