package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/rthomazel/mcp/postgres/internal/sqlcheck"
)

// sqlClassify returns the appropriate allowlist and flag-check function for a SQL statement.
// Uses sqlcheck to strip comments and get the first token.
func (h *Handler) sqlClassify(sql string) (allowlist []string, flagAllowed bool, flagName string, err error) {
	token := sqlcheck.FirstToken(sqlcheck.StripComments(sql))
	switch token {
	case "INSERT", "UPDATE", "DELETE", "TRUNCATE":
		return dmlAllowlist, h.cfg.AllowDML, "POSTGRES_MCP_ALLOW_DML", nil
	case "CREATE", "ALTER", "DROP":
		return ddlAllowlist, h.cfg.AllowDDL, "POSTGRES_MCP_ALLOW_DDL", nil
	case "GRANT", "REVOKE":
		return dclAllowlist, h.cfg.AllowDCL, "POSTGRES_MCP_ALLOW_DCL", nil
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
func (h *Handler) HandleMutateBatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowTransactions {
		return mcp.NewToolResultError("mutate_batch is disabled: set POSTGRES_MCP_ALLOW_TRANSACTIONS=true to enable"), nil
	}

	rawStatements, _ := req.Params.Arguments["statements"].([]any)
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

	// Validate all statements before starting the transaction.
	cleanedStmts := make([]string, len(statements))
	for i, stmt := range statements {
		allowlist, flagAllowed, flagName, err := h.sqlClassify(stmt)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d: %v", i, err)), nil
		}
		if !flagAllowed {
			return mcp.NewToolResultError(fmt.Sprintf("statement %d requires %s=true", i, flagName)), nil
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
func (h *Handler) HandleDryRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if !h.cfg.AllowTransactions {
		return mcp.NewToolResultError("dry_run is disabled: set POSTGRES_MCP_ALLOW_TRANSACTIONS=true to enable"), nil
	}

	sql, _ := req.Params.Arguments["sql"].(string)
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
	const prefix = "[dry run — changes rolled back]\n"

	switch token {
	case "SELECT", "SHOW", "TABLE", "WITH":
		rows, err := tx.Query(ctx, cleaned)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
		}
		defer rows.Close()
		var headers []string
		for _, fd := range rows.FieldDescriptions() {
			headers = append(headers, fd.Name)
		}
		var rowData [][]string
		for rows.Next() && len(rowData) < h.cfg.MaxRows {
			vals, err := rows.Values()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("row values: %v", err)), nil
			}
			row := make([]string, len(vals))
			for i, v := range vals {
				if v == nil {
					row[i] = ""
				} else {
					row[i] = fmt.Sprintf("%v", v)
				}
			}
			rowData = append(rowData, row)
		}
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
		}
		if len(rowData) == 0 {
			return mcp.NewToolResultText(prefix + "query returned no results"), nil
		}
		return mcp.NewToolResultText(prefix + formatTable(headers, rowData)), nil

	case "INSERT", "UPDATE", "DELETE", "TRUNCATE":
		tag, err := tx.Exec(ctx, cleaned)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%sdry_run: would affect %d rows (rolled back)", prefix, tag.RowsAffected())), nil

	default:
		if _, err := tx.Exec(ctx, cleaned); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("execution error: %v", err)), nil
		}
		return mcp.NewToolResultText(prefix + "dry_run: statement executed (rolled back)"), nil
	}
}
