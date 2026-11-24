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
	"github.com/rthomazel/mcp/investments/internal/notification"
	"github.com/rthomazel/mcp/investments/internal/provider/tiingo"
	"github.com/rthomazel/mcp/investments/internal/storage/postgres"
	"github.com/rthomazel/mcp/investments/internal/worker/events"
	"github.com/rthomazel/mcp/investments/internal/worker/marketdata"
	"github.com/rthomazel/mcp/investments/internal/worker/monitor"
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

	stores, cleanup, err := postgres.Open(cfg.DSN, postgres.Settings{
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

	if err := migrate(stores); err != nil {
		return fmt.Errorf("migrating: %w", err)
	}

	provider := tiingo.NewClient(cfg.TiingoAPIKey, "")
	refresher := marketdata.NewPriceRefresher(stores.Store, stores.Prices, provider)
	job := marketdata.NewJob(stores.Prices, stores.Snapshots, stores.Updates, refresher, provider.Name())

	tasks := []domain.ScheduledTask{marketdata.RefreshTask(job, cfg.PriceRefreshRunTimes)}

	monitors := []monitor.Monitor{
		monitor.NewSnapshotIncomplete(stores.Store, stores.Events),
		monitor.NewPortfolioDrawdown(stores.Monitors, stores.Events),
		monitor.NewPortfolioATH(stores.Monitors, stores.Events),
		monitor.NewPortfolioMovement(stores.Monitors, stores.Events),
		monitor.NewAllocationDrift(stores.Monitors, stores.Events),
		monitor.NewPriceVolatility(stores.Monitors, stores.Events),
		monitor.NewPriceTarget(stores.Monitors, stores.Events),
		monitor.NewInstrumentDrawdown(stores.Monitors, stores.Events),
		monitor.NewInstrumentATH(stores.Monitors, stores.Events),
	}
	monitorsConfig := monitor.NewConfigLoader(cfg.MonitorsConfigFile, monitors)

	for _, m := range monitors {
		tasks = append(tasks, monitor.Task(m, cfg.MonitorsRunTime, cfg.MonitorTimeout, monitorsConfig))
	}

	scheduler, err := domain.NewScheduler(tasks, cfg.MarketTimezone)
	if err != nil {
		return fmt.Errorf("building scheduler: %w", err)
	}

	slog.Info("price refresh job enabled", "provider", provider.Name(), "run_times", marketdata.RunTimesDescription(cfg.PriceRefreshRunTimes, cfg.MarketTimezone))

	ctx, stop := context.WithCancel(context.Background())

	var running sync.WaitGroup

	running.Go(func() { scheduler.Run(ctx) })

	source := notification.NewSource(notification.NewConfigLoader(cfg.NotificationsConfigFile), stores.Updates, cfg.TelegramBotToken)
	eventsWorker := events.NewWorker(stores.Events, source, events.Settings{
		TypePrefix:   domain.NotificationPrefix,
		PollInterval: cfg.WorkerPollInterval, PruneInterval: cfg.WorkerPruneInterval,
		BatchLimit: cfg.WorkerBatchLimit, HandlerTimeout: cfg.WorkerHandlerTimeout,
	})

	running.Go(func() { eventsWorker.Run(ctx) })

	defer func() {
		stop()
		running.Wait()
	}()

	mcpServer := server.NewMCPServer("investments-mcp", version)
	handlers.New(stores.Domain(), refresher, cfg.QueryTimeout).Register(mcpServer)

	return server.ServeStdio(mcpServer)
}
