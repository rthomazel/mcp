package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/postgres/internal/sqlcheck"
)

// dryRunPrefix is the standard prefix on all dry_run results.
const dryRunPrefix = "[dry run — changes rolled back]\n"

// sqlClassify returns the allowlist and flag check for a SQL statement based on its first token.
func (h *Handler) sqlClassify(sql string) (allowlist []string, flagAllowed bool, flagName string, err error) {
	token := sqlcheck.FirstToken(sqlcheck.StripComments(sql))
	switch token {
	case "INSERT", "UPDATE", "DELETE", "TRUNCATE":
		return dmlAllowlist, h.cfg.AllowMutate, "POSTGRES_MCP_ALLOW_MUTATE", nil
	case "CREATE", "ALTER", "DROP":
		return ddlAllowlist, h.cfg.AllowMutateSchema, "POSTGRES_MCP_ALLOW_MUTATE_SCHEMA", nil
	case "GRANT", "REVOKE":
		return dclAllowlist, h.cfg.AllowMutatePermissions, "POSTGRES_MCP_ALLOW_MUTATE_PERMISSIONS", nil
	case "SELECT", "SHOW", "TABLE", "WITH":
		return dqlAllowlist, true, "", nil
	default:
		if token == "" {
			return nil, false, "", fmt.Errorf("empty statement")
		}
		return nil, false, "", fmt.Errorf("statement type %q is not recognized", token)
	}
}

// HandleMutateBatch executes multiple SQL statements in a single transaction.
// Requires allow_transactions: true in config. Class flags still apply per statement.
func (h *Handler) HandleMutateBatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowTransactions {
		return mcp.NewToolResultError("mutate_batch is disabled (set POSTGRES_MCP_ALLOW_TRANSACTIONS=true)"), nil
	}

	rawStatements, _ := req.GetArguments()["statements"].([]any)
	if len(rawStatements) == 0 {
		return mcp.NewToolResultError("no statements provided"), nil
	}

	statements := make([]string, len(rawStatements))
	for i, raw := range rawStatements {
		stmt, ok := raw.(string)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d is not a string", i)), nil
		}
		statements[i] = stmt
	}

	// Validate all statements before opening the transaction.
	cleanedStmts := make([]string, len(statements))
	for i, stmt := range statements {
		allowlist, flagAllowed, flagName, err := h.sqlClassify(stmt)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d: %v", i, err)), nil
		}
		if !flagAllowed {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d: requires %s=true", i, flagName)), nil
		}
		cleaned, err := sqlcheck.Validate(stmt, allowlist)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d validation failed: %v", i, err)), nil
		}
		cleanedStmts[i] = cleaned
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	for i, stmt := range cleanedStmts {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d failed: %v", i, err)), nil
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("commit error: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("mutate_batch executed %d statements successfully", len(statements))), nil
}

// HandleDryRun executes a statement in a transaction then unconditionally rolls back.
// Requires allow_transactions: true in config plus the class flag for the statement type.
func (h *Handler) HandleDryRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowTransactions {
		return mcp.NewToolResultError("dry_run is disabled (set POSTGRES_MCP_ALLOW_TRANSACTIONS=true)"), nil
	}

	sql := req.GetString("sql", "")
	if strings.TrimSpace(sql) == "" {
		return mcp.NewToolResultError("sql parameter is required"), nil
	}

	allowlist, flagAllowed, flagName, err := h.sqlClassify(sql)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("classification failed: %v", err)), nil
	}
	if !flagAllowed {
		return mcp.NewToolResultError(fmt.Sprintf("dry_run with this statement requires %s=true", flagName)), nil
	}

	cleaned, err := sqlcheck.Validate(sql, allowlist)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("validation failed: %v", err)), nil
	}

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("begin transaction: %v", err)), nil
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", h.cfg.QueryTimeout)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set timeout: %v", err)), nil
	}

	token := sqlcheck.FirstToken(sqlcheck.StripComments(sql))
	switch token {
	case "SELECT", "SHOW", "TABLE", "WITH":
		rows, err := tx.Query(ctx, cleaned)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
		}
		headers, rowData, capped, err := collectRows(rows, h.cfg.MaxRows)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read rows: %v", err)), nil
		}
		if len(rowData) == 0 {
			return mcp.NewToolResultText(dryRunPrefix + "query returned no results"), nil
		}
		result := dryRunPrefix + formatTable(headers, rowData)
		if capped {
			result = fmt.Sprintf("[results capped at %d rows]\n\n", h.cfg.MaxRows) + result
		}
		return mcp.NewToolResultText(result), nil

	case "INSERT", "UPDATE", "DELETE", "TRUNCATE":
		tag, err := tx.Exec(ctx, cleaned)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%sdry_run: would affect %d rows", dryRunPrefix, tag.RowsAffected())), nil

	default:
		if _, err := tx.Exec(ctx, cleaned); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
		}
		return mcp.NewToolResultText(dryRunPrefix + "dry_run: statement executed"), nil
	}
}
