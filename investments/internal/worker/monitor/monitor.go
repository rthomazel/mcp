package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rthomazel/mcp/investments/internal/config/fileloader"
	"github.com/rthomazel/mcp/investments/internal/domain"
)

// Monitor judges a condition on the current state and records an event when
// it holds, at most once for each subject.
type Monitor interface {
	// Name is the key of the monitor's entry in the monitors configuration.
	Name() string
	At() domain.ClockTime
	// ValidateParams rejects settings the monitor cannot run with.
	ValidateParams(params Params) error
	Run(ctx context.Context, params Params) error
}

// Task is the monitor as a task for the domain scheduler, run daily at its
// time of day when the monitors configuration enables it.
func Task(monitor Monitor, timeout time.Duration, loader *fileloader.Loader[Config]) domain.ScheduledTask {
	return domain.ScheduledTask{
		Name: "monitor " + monitor.Name(), At: []domain.ClockTime{monitor.At()}, Timeout: timeout,
		Run: func(ctx context.Context) error { return run(ctx, monitor, loader) },
	}
}

func run(ctx context.Context, monitor Monitor, loader *fileloader.Loader[Config]) error {
	cfg, err := loader.Current()
	if err != nil {
		return fmt.Errorf("reading monitors configuration: %w", err)
	}

	entry, ok := cfg.Monitors[monitor.Name()]
	if !ok {
		slog.WarnContext(ctx, "monitor is not configured, skipping", "monitor", monitor.Name())

		return nil
	}

	if entry.Disabled {
		slog.InfoContext(ctx, "monitor is disabled, skipping", "monitor", monitor.Name())

		return nil
	}

	return monitor.Run(ctx, entry.Params)
}
