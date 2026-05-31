package main

import (
	"strings"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rthomazel/mcp/keys/internal/config"
	"github.com/rthomazel/mcp/keys/internal/proxy"
	"github.com/rthomazel/mcp/keys/internal/secrets"
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
	configPath := flag.String("config", "/etc/keys/config.yaml", "path to YAML config file")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(
		os.Stderr,
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)))

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("config load failed", "error", err)
		return err
	}

	store, err := secrets.Load(cfg.Secrets)
	if err != nil {
		slog.Error("secrets load failed", "error", err)
		return err
	}

	toolNames := make([]string, 0, len(cfg.Tools))
	for name := range cfg.Tools {
		toolNames = append(toolNames, name)
	}

	slog.Info("keys starting",
		"version", version,
		"timeout_seconds", cfg.TimeoutSeconds,
		"tools", toolNames,
		"secrets", store.Names(),
	)

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	theProxy := proxy.New(timeout, cfg.MaxResponseBytes, cfg.MaxRequestBytes, store)

	theServer := server.NewMCPServer("keys", version, server.WithToolCapabilities(false))

	for toolName, toolCfg := range cfg.Tools {
		var desc strings.Builder; desc.WriteString(toolCfg.Description)

		if len(toolCfg.Docs) > 0 {
			desc.WriteString("\nDocs:\n")
			for _, doc := range toolCfg.Docs {
				desc.WriteString(doc + "\n")
			}
		}

		theServer.AddTool(
			mcp.NewTool(toolName,
				mcp.WithDescription(desc.String()),
				mcp.WithString("path", mcp.Required(), mcp.Description("Relative API path, e.g. /repos/owner/repo/pulls")),
				mcp.WithString("method", mcp.Required(), mcp.Description("HTTP method")),
				mcp.WithString("body", mcp.Description("Optional request body. Set Content-Type in headers if needed.")),
				mcp.WithObject("headers", mcp.Description("Optional non-secret headers, e.g. {\"Content-Type\": \"application/json\"}")),
			),
			func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				reqPath, _ := req.Params.Arguments["path"].(string)
				method, _ := req.Params.Arguments["method"].(string)
				body, _ := req.Params.Arguments["body"].(string)

				agentHeaders := make(map[string]string)
				if raw, ok := req.Params.Arguments["headers"].(map[string]any); ok {
					for k, v := range raw {
						if sv, ok := v.(string); ok {
							agentHeaders[k] = sv
						}
					}
				}

				if reqPath == "" {
					return mcp.NewToolResultError("path is required"), nil
				}
				if method == "" {
					return mcp.NewToolResultError("method is required"), nil
				}

				resp, err := theProxy.Do(ctx, toolName, toolCfg, reqPath, method, body, agentHeaders)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				b, err := json.Marshal(resp)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("marshal response: %v", err)), nil
				}

				return mcp.NewToolResultText(string(b)), nil
			},
		)
	}

	slog.Info("serving on stdio")
	if err := server.ServeStdio(theServer); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}
