package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/postgres/internal/sqlcheck"
)

// HandleQuery executes a read-only DQL statement (SELECT, SHOW, TABLE, WITH).
// Always runs inside BEGIN READ ONLY ... ROLLBACK.
func (h *Handler) HandleQuery(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sql := req.GetString("sql", "")
	if strings.TrimSpace(sql) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}

	cleaned, err := sqlcheck.Validate(sql, dqlAllowlist)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	rows, err := tx.Query(ctx, cleaned)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
	}

	headers, rowData, capped, err := collectRows(rows, h.cfg.MaxRows)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read rows: %v", err)), nil
	}
	if len(rowData) == 0 {
		return mcp.NewToolResultText("query returned no results"), nil
	}
	return mcp.NewToolResultText(tableResult(headers, rowData, capped, h.cfg.MaxRows)), nil
}

// HandleMutate executes a DML statement (INSERT, UPDATE, DELETE, TRUNCATE).
// Requires POSTGRES_MCP_ALLOW_MUTATE=true.
func (h *Handler) HandleMutate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowMutate {
		return mcp.NewToolResultError("mutate is disabled (set POSTGRES_MCP_ALLOW_MUTATE=true)"), nil
	}

	sql := req.GetString("sql", "")
	if strings.TrimSpace(sql) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}

	cleaned, err := sqlcheck.Validate(sql, dmlAllowlist)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	tag, err := tx.Exec(ctx, cleaned)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
	}
	if err = tx.Commit(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("commit error: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("mutate executed successfully. rows affected: %d", tag.RowsAffected())), nil
}

// HandleMutateSchema executes a DDL statement (CREATE, ALTER, DROP).
// Requires POSTGRES_MCP_ALLOW_MUTATE_SCHEMA=true.
func (h *Handler) HandleMutateSchema(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowMutateSchema {
		return mcp.NewToolResultError("mutate_schema is disabled (set POSTGRES_MCP_ALLOW_MUTATE_SCHEMA=true)"), nil
	}

	sql := req.GetString("sql", "")
	if strings.TrimSpace(sql) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}

	cleaned, err := sqlcheck.Validate(sql, ddlAllowlist)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	if _, err = tx.Exec(ctx, cleaned); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
	}
	if err = tx.Commit(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("commit error: %v", err)), nil
	}
	return mcp.NewToolResultText("schema statement executed successfully"), nil
}

// HandleMutatePermissions executes a DCL statement (GRANT, REVOKE).
// Requires POSTGRES_MCP_ALLOW_MUTATE_PERMISSIONS=true.
func (h *Handler) HandleMutatePermissions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowMutatePermissions {
		return mcp.NewToolResultError("mutate_permissions is disabled (set POSTGRES_MCP_ALLOW_MUTATE_PERMISSIONS=true)"), nil
	}

	sql := req.GetString("sql", "")
	if strings.TrimSpace(sql) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}

	cleaned, err := sqlcheck.Validate(sql, dclAllowlist)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation error: %v", err)), nil
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	if _, err = tx.Exec(ctx, cleaned); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
	}
	if err = tx.Commit(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("commit error: %v", err)), nil
	}
	return mcp.NewToolResultText("permissions statement executed successfully"), nil
}
