package main

import (
	"fmt"
	"log/slog"
	"os"
	_ "time/tzdata" // embed zone data so lookups do not depend on the host

	"github.com/mark3labs/mcp-go/server"

	"github.com/rthomazel/mcp/investments/internal/config"
	"github.com/rthomazel/mcp/investments/internal/storage/postgres"
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

	store, cleanup, err := postgres.Open(cfg.DSN, cfg.StalePriceDays)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer func() { _ = cleanup() }()

	if err := migrate(store); err != nil {
		return fmt.Errorf("migrating: %w", err)
	}

	mcpServer := server.NewMCPServer("investments-mcp", version)

	return server.ServeStdio(mcpServer)
}
