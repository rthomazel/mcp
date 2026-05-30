package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/rthomazel/mcp/postgres/handlers"
	"github.com/rthomazel/mcp/postgres/internal"
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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	poolCfg.MaxConns = int32(cfg.PoolSize)

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	h := handlers.New(cfg, pool)
	s := server.NewMCPServer("postgres-mcp", version)

	// Group 1 — schema introspection (always on)
	s.AddTool(mcp.NewTool("list_schemas",
		mcp.WithDescription("List all schemas in the database."),
	), h.HandleListSchemas)

	s.AddTool(mcp.NewTool("list_tables",
		mcp.WithDescription("List all tables in a schema."),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleListTables)

	s.AddTool(mcp.NewTool("describe_table",
		mcp.WithDescription("Get columns, types, nullability, defaults, and comments for a table."),
		mcp.WithString("table", mcp.Required()),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleDescribeTable)

	s.AddTool(mcp.NewTool("list_indexes",
		mcp.WithDescription("List indexes for a table."),
		mcp.WithString("table", mcp.Required()),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleListIndexes)

	s.AddTool(mcp.NewTool("list_foreign_keys",
		mcp.WithDescription("List foreign key constraints for a table."),
		mcp.WithString("table", mcp.Required()),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleListForeignKeys)

	s.AddTool(mcp.NewTool("list_views",
		mcp.WithDescription("List views in a schema."),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleListViews)

	s.AddTool(mcp.NewTool("list_functions",
		mcp.WithDescription("List functions and stored procedures in a schema."),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleListFunctions)

	s.AddTool(mcp.NewTool("table_stats",
		mcp.WithDescription("Row count, live/dead tuple stats, and last vacuum/analyze for a table."),
		mcp.WithString("table", mcp.Required()),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleTableStats)

	s.AddTool(mcp.NewTool("database_size",
		mcp.WithDescription("Total database size and per-table sizes."),
	), h.HandleDatabaseSize)

	s.AddTool(mcp.NewTool("search_schema",
		mcp.WithDescription("Search table, column, and view names by keyword."),
		mcp.WithString("term", mcp.Required()),
	), h.HandleSearchSchema)

	s.AddTool(mcp.NewTool("er_diagram",
		mcp.WithDescription("Generate a Mermaid ERD from foreign key relationships."),
		mcp.WithString("schema", mcp.Description("Schema name. Defaults to 'public'.")),
	), h.HandleERDiagram)

	// Group 2 — query & mutation (flags checked inside handlers)
	s.AddTool(mcp.NewTool("query",
		mcp.WithDescription("Run a read-only SQL query (DQL: SELECT, SHOW, TABLE, WITH/CTE). Always enabled. Runs in a READ ONLY transaction."),
		mcp.WithString("sql", mcp.Required()),
	), h.HandleQuery)

	s.AddTool(mcp.NewTool("mutate",
		mcp.WithDescription("Run a data manipulation statement (DML: INSERT, UPDATE, DELETE, TRUNCATE). Requires POSTGRES_MCP_ALLOW_DML=true."),
		mcp.WithString("sql", mcp.Required()),
	), h.HandleMutate)

	s.AddTool(mcp.NewTool("mutate_schema",
		mcp.WithDescription("Run a schema definition statement (DDL: CREATE, ALTER, DROP). Requires POSTGRES_MCP_ALLOW_DDL=true."),
		mcp.WithString("sql", mcp.Required()),
	), h.HandleMutateSchema)

	s.AddTool(mcp.NewTool("mutate_permissions",
		mcp.WithDescription("Run a permissions statement (DCL: GRANT, REVOKE). Requires POSTGRES_MCP_ALLOW_DCL=true."),
		mcp.WithString("sql", mcp.Required()),
	), h.HandleMutatePermissions)

	// Group 3 — transactions (flag checked inside handlers)
	s.AddTool(mcp.NewTool("mutate_batch",
		mcp.WithDescription("Run multiple SQL statements in a single transaction. Requires POSTGRES_MCP_ALLOW_TRANSACTIONS=true. Each statement is validated individually."),
		mcp.WithArray("statements", mcp.Required(), mcp.Description("SQL statements to execute in order."), mcp.Items(map[string]any{"type": "string"})),
	), h.HandleMutateBatch)

	s.AddTool(mcp.NewTool("dry_run",
		mcp.WithDescription("Execute a statement inside a transaction and always roll back. Shows what would happen without committing. Requires the same flag as the equivalent single-statement tool."),
		mcp.WithString("sql", mcp.Required()),
	), h.HandleDryRun)

	// Group 4 — diagnostics (flags checked inside handlers)
	s.AddTool(mcp.NewTool("ping",
		mcp.WithDescription("Health check. Returns server version and connection round-trip latency. Always enabled."),
	), h.HandlePing)

	s.AddTool(mcp.NewTool("explain",
		mcp.WithDescription("Show the query plan for a SQL statement without executing it. Requires POSTGRES_MCP_ALLOW_DIAGNOSTICS=true."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("Inner SQL to explain (do not include EXPLAIN yourself).")),
	), h.HandleExplain)

	s.AddTool(mcp.NewTool("explain_analyze",
		mcp.WithDescription("Show the query plan with actual execution statistics. Executes the statement. Requires POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE=true."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("Inner SQL to explain and analyze (do not include EXPLAIN yourself).")),
	), h.HandleExplainAnalyze)

	s.AddTool(mcp.NewTool("active_connections",
		mcp.WithDescription("Show active database connections and their states. Requires POSTGRES_MCP_ALLOW_DIAGNOSTICS=true."),
	), h.HandleActiveConnections)

	s.AddTool(mcp.NewTool("active_locks",
		mcp.WithDescription("Show blocking lock chains. Requires POSTGRES_MCP_ALLOW_DIAGNOSTICS=true."),
	), h.HandleActiveLocks)

	slog.Info("postgres-mcp starting", "version", version)
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}
