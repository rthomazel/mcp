package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/postgres/internal/sqlcheck"
)

// HandlePing returns server version and connection round-trip latency. Always enabled.
func (h *Handler) HandlePing(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	start := time.Now()
	if err := h.pool.Ping(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("ping failed: %v", err)), nil
	}
	latency := time.Since(start)

	var version string
	if err := h.pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("version query failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("version: %s\nlatency: %v", version, latency.Round(time.Millisecond))), nil
}

// HandleExplain shows the query plan without executing the statement.
// The caller provides inner SQL only — do not include EXPLAIN yourself.
// Requires allow_diagnostics: true in config.
func (h *Handler) HandleExplain(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowDiagnostics {
		return mcp.NewToolResultError("explain is disabled (set POSTGRES_MCP_ALLOW_DIAGNOSTICS=true)"), nil
	}

	innerSQL := req.GetString("sql", "")
	if strings.TrimSpace(innerSQL) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}
	if sqlcheck.FirstToken(sqlcheck.StripComments(innerSQL)) == "EXPLAIN" {
		return mcp.NewToolResultError("omit EXPLAIN from the sql parameter — the tool wraps the statement itself"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, "EXPLAIN "+innerSQL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("explain failed: %v", err)), nil
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// HandleExplainAnalyze shows the query plan with actual execution stats.
// Runs inside a transaction that is unconditionally rolled back.
// The caller provides inner SQL only — do not include EXPLAIN yourself.
// Requires allow_explain_analyze: true in config. DML inner statements also require allow_mutate: true.
func (h *Handler) HandleExplainAnalyze(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowExplainAnalyze {
		return mcp.NewToolResultError("explain_analyze is disabled (set POSTGRES_MCP_ALLOW_EXPLAIN_ANALYZE=true)"), nil
	}

	innerSQL := req.GetString("sql", "")
	if strings.TrimSpace(innerSQL) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}
	if sqlcheck.FirstToken(sqlcheck.StripComments(innerSQL)) == "EXPLAIN" {
		return mcp.NewToolResultError("omit EXPLAIN from the sql parameter — the tool wraps the statement itself"), nil
	}

	token := sqlcheck.FirstToken(sqlcheck.StripComments(innerSQL))
	switch token {
	case "INSERT", "UPDATE", "DELETE", "TRUNCATE":
		if !h.cfg.AllowMutate {
			return mcp.NewToolResultError("explain_analyze with a DML statement requires POSTGRES_MCP_ALLOW_MUTATE=true"), nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	rows, err := tx.Query(ctx, "EXPLAIN (ANALYZE, FORMAT TEXT) "+innerSQL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("explain analyze failed: %v", err)), nil
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// HandleActiveConnections shows active connections from pg_stat_activity.
// Requires allow_diagnostics: true in config.
func (h *Handler) HandleActiveConnections(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowDiagnostics {
		return mcp.NewToolResultError("active_connections is disabled (set POSTGRES_MCP_ALLOW_DIAGNOSTICS=true)"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT
			pid::text,
			COALESCE(usename, ''),
			COALESCE(application_name, ''),
			COALESCE(client_addr::text, 'local'),
			COALESCE(state, ''),
			COALESCE(wait_event_type, ''),
			COALESCE(wait_event, ''),
			COALESCE(query_start::text, '')
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		ORDER BY query_start`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var connRows [][]string
	var capped bool
	for rows.Next() {
		var pid, user, app, client, state, waitType, waitEvent, queryStart string
		if err := rows.Scan(&pid, &user, &app, &client, &state, &waitType, &waitEvent, &queryStart); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		connRows = append(connRows, []string{pid, user, app, client, state, waitType, waitEvent, queryStart})
		if len(connRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(connRows) == 0 {
		return mcp.NewToolResultText("no active connections"), nil
	}
	return mcp.NewToolResultText(tableResult(
		[]string{"pid", "user", "application", "client", "state", "wait_type", "wait_event", "query_start"},
		connRows, capped, h.cfg.MaxRows)), nil
}

// HandleActiveLocks shows blocking lock chains from pg_locks.
// Requires allow_diagnostics: true in config.
func (h *Handler) HandleActiveLocks(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowDiagnostics {
		return mcp.NewToolResultError("active_locks is disabled (set POSTGRES_MCP_ALLOW_DIAGNOSTICS=true)"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT
			blocked.pid::text,
			COALESCE(blocked_activity.usename, ''),
			blocking.pid::text,
			COALESCE(blocking_activity.usename, ''),
			COALESCE(blocked_activity.query, '')
		FROM pg_catalog.pg_locks blocked
		JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked.pid
		JOIN pg_catalog.pg_locks blocking
			ON blocking.locktype = blocked.locktype
			AND blocking.relation IS NOT DISTINCT FROM blocked.relation
			AND blocking.page IS NOT DISTINCT FROM blocked.page
			AND blocking.tuple IS NOT DISTINCT FROM blocked.tuple
			AND blocking.pid != blocked.pid
			AND blocking.granted
		JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking.pid
		WHERE NOT blocked.granted
		ORDER BY blocked.pid`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var lockRows [][]string
	var capped bool
	for rows.Next() {
		var blockedPID, blockedUser, blockingPID, blockingUser, query string
		if err := rows.Scan(&blockedPID, &blockedUser, &blockingPID, &blockingUser, &query); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		lockRows = append(lockRows, []string{blockedPID, blockedUser, blockingPID, blockingUser, query})
		if len(lockRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(lockRows) == 0 {
		return mcp.NewToolResultText("no active locks found"), nil
	}
	return mcp.NewToolResultText(tableResult(
		[]string{"blocked_pid", "blocked_user", "blocking_pid", "blocking_user", "query"},
		lockRows, capped, h.cfg.MaxRows)), nil
}
