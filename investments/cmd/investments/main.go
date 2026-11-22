package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	_ "time/tzdata" // embed zone data so lookups do not depend on the host

	"github.com/mark3labs/mcp-go/server"

	"github.com/rthomazel/mcp/investments/internal/config"
	"github.com/rthomazel/mcp/investments/internal/domain"
	"github.com/rthomazel/mcp/investments/internal/handlers"
	"github.com/rthomazel/mcp/investments/internal/marketdata"
	"github.com/rthomazel/mcp/investments/internal/monitor"
	"github.com/rthomazel/mcp/investments/internal/notification"
	"github.com/rthomazel/mcp/investments/internal/provider/tiingo"
	"github.com/rthomazel/mcp/investments/internal/storage/postgres"
	"github.com/rthomazel/mcp/investments/internal/worker"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "local"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	store, cleanup, err := postgres.Open(cfg.DSN, postgres.Settings{
		StalePriceDays: cfg.StalePriceDays, MarketTimezone: cfg.MarketTimezone, MaxHistoryPoints: cfg.MaxHistoryPoints,
		Events: domain.EventPolicy{
			MaxFailures: cfg.EventMaxFailures, BackoffBase: cfg.EventBackoffBase,
			MaxAge: cfg.EventMaxAge, Retention: cfg.EventRetention,
		},
	})
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer func() { _ = cleanup() }()

	if err := migrate(store); err != nil {
		return fmt.Errorf("migrating: %w", err)
	}

	provider := tiingo.NewClient(cfg.TiingoAPIKey, "")
	refresher := marketdata.NewRefresher(store, provider)
	job := marketdata.NewJob(store, refresher, provider.Name())

	scheduler, err := marketdata.NewScheduler(job, cfg.MarketTimezone)
	if err != nil {
		return fmt.Errorf("building scheduler: %w", err)
	}

	slog.Info("daily job enabled", "provider", provider.Name(), "run_times", marketdata.RunTimesDescription(cfg.MarketTimezone))

	ctx, stop := context.WithCancel(context.Background())

	var running sync.WaitGroup

	running.Go(func() { scheduler.Run(ctx) })

	monitors, err := monitor.NewScheduler([]monitor.Monitor{monitor.NewSnapshotIncomplete(store)}, cfg.MarketTimezone, cfg.MonitorTimeout)
	if err != nil {
		stop()

		return fmt.Errorf("building monitor scheduler: %w", err)
	}

	source := notification.NewSource(notification.NewLoader(cfg.NotificationsConfigFile), store, cfg.TelegramBotToken)
	worker := worker.NewNotifications(store, source, worker.Settings{
		PollInterval: cfg.WorkerPollInterval, PruneInterval: cfg.WorkerPruneInterval,
		BatchLimit: cfg.WorkerBatchLimit, HandlerTimeout: cfg.WorkerHandlerTimeout,
	})

	running.Go(func() { monitors.Run(ctx) })
	running.Go(func() { worker.Run(ctx) })

	defer func() {
		stop()
		running.Wait()
	}()

	mcpServer := server.NewMCPServer("investments-mcp", version)
	handlers.New(store, refresher, cfg.QueryTimeout).Register(mcpServer)

	return server.ServeStdio(mcpServer)
}
