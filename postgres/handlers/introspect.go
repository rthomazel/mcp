package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/samber/lo"
)

// HandleListSchemas lists all allowed schemas.
func (h *Handler) HandleListSchemas(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, "SELECT schema_name FROM information_schema.schemata ORDER BY schema_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var schemas []string
	var capped bool
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		if h.schemaAllowed(schema) {
			schemas = append(schemas, schema)
			if len(schemas) == h.cfg.MaxRows {
				capped = rows.Next()
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(schemas) == 0 {
		return mcp.NewToolResultText("no schemas found"), nil
	}
	out := strings.Join(schemas, "\n")
	if capped {
		out = h.cfg.CapNote() + out
	}
	return mcp.NewToolResultText(out), nil
}

// HandleListTables lists tables in a schema.
func (h *Handler) HandleListTables(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx,
		"SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name",
		schema)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var tableRows [][]string
	var capped bool
	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		tableRows = append(tableRows, []string{tableName, tableType})
		if len(tableRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(tableRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no tables found in schema %s", schema)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"table", "type"}, tableRows, capped, h.cfg.CapNote())), nil
}

// HandleDescribeTable describes columns of a table.
func (h *Handler) HandleDescribeTable(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table := req.GetString("table", "")
	if table == "" {
		return mcp.NewToolResultError("table parameter is required"), nil
	}
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT
			column_name,
			data_type,
			is_nullable,
			column_default,
			col_description(
				(table_schema||'.'||table_name)::regclass::oid, ordinal_position
			) AS comment
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var colRows [][]string
	var capped bool
	for rows.Next() {
		var colName, dataType, isNullable string
		var colDefault, comment *string
		if err := rows.Scan(&colName, &dataType, &isNullable, &colDefault, &comment); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		colRows = append(colRows, []string{colName, dataType, isNullable, lo.FromPtr(colDefault), lo.FromPtr(comment)})
		if len(colRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(colRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("table %s.%s not found or has no columns", schema, table)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"column", "type", "nullable", "default", "comment"}, colRows, capped, h.cfg.CapNote())), nil
}

// HandleListIndexes lists indexes on a table.
func (h *Handler) HandleListIndexes(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table := req.GetString("table", "")
	if table == "" {
		return mcp.NewToolResultError("table parameter is required"), nil
	}
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT indexname, indexdef,
			CASE WHEN indexdef ILIKE '%UNIQUE%' THEN 'true' ELSE 'false' END AS unique
		FROM pg_catalog.pg_indexes
		WHERE schemaname = $1 AND tablename = $2
		ORDER BY indexname`, schema, table)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var idxRows [][]string
	var capped bool
	for rows.Next() {
		var name, def, unique string
		if err := rows.Scan(&name, &def, &unique); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		idxRows = append(idxRows, []string{name, def, unique})
		if len(idxRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(idxRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no indexes found on %s.%s", schema, table)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"index", "definition", "unique"}, idxRows, capped, h.cfg.CapNote())), nil
}

// HandleListForeignKeys lists foreign key constraints on a table.
func (h *Handler) HandleListForeignKeys(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table := req.GetString("table", "")
	if table == "" {
		return mcp.NewToolResultError("table parameter is required"), nil
	}
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT
			tc.constraint_name,
			kcu.column_name,
			ccu.table_schema AS foreign_schema,
			ccu.table_name AS foreign_table,
			ccu.column_name AS foreign_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		ORDER BY tc.constraint_name`, schema, table)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var fkRows [][]string
	var capped bool
	for rows.Next() {
		var constraint, col, fSchema, fTable, fCol string
		if err := rows.Scan(&constraint, &col, &fSchema, &fTable, &fCol); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		fkRows = append(fkRows, []string{constraint, col, fSchema, fTable, fCol})
		if len(fkRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(fkRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no foreign keys found on %s.%s", schema, table)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"constraint", "column", "foreign_schema", "foreign_table", "foreign_column"}, fkRows, capped, h.cfg.CapNote())), nil
}

// HandleListViews lists views in a schema.
func (h *Handler) HandleListViews(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx,
		"SELECT table_name, view_definition FROM information_schema.views WHERE table_schema = $1 ORDER BY table_name",
		schema)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var viewRows [][]string
	var capped bool
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		viewRows = append(viewRows, []string{name, def})
		if len(viewRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(viewRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no views found in schema %s", schema)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"view", "definition"}, viewRows, capped, h.cfg.CapNote())), nil
}

// HandleListFunctions lists functions and stored procedures in a schema.
func (h *Handler) HandleListFunctions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx,
		"SELECT routine_name, routine_type, data_type FROM information_schema.routines WHERE routine_schema = $1 ORDER BY routine_name",
		schema)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var funcRows [][]string
	var capped bool
	for rows.Next() {
		var name, routineType string
		var returnType *string
		if err := rows.Scan(&name, &routineType, &returnType); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		funcRows = append(funcRows, []string{name, routineType, lo.FromPtr(returnType)})
		if len(funcRows) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(funcRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no functions found in schema %s", schema)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"function", "type", "returns"}, funcRows, capped, h.cfg.CapNote())), nil
}

// HandleTableStats returns row count and maintenance stats for a table.
func (h *Handler) HandleTableStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table := req.GetString("table", "")
	if table == "" {
		return mcp.NewToolResultError("table parameter is required"), nil
	}
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	var liveRows, deadRows, lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze string
	err := h.pool.QueryRow(ctx, `
		SELECT
			n_live_tup::text,
			n_dead_tup::text,
			COALESCE(last_vacuum::text, 'never'),
			COALESCE(last_autovacuum::text, 'never'),
			COALESCE(last_analyze::text, 'never'),
			COALESCE(last_autoanalyze::text, 'never')
		FROM pg_catalog.pg_stat_user_tables
		WHERE schemaname = $1 AND relname = $2`, schema, table).
		Scan(&liveRows, &deadRows, &lastVacuum, &lastAutovacuum, &lastAnalyze, &lastAutoanalyze)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return mcp.NewToolResultText(fmt.Sprintf("no stats found for table %s.%s", schema, table)), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	return mcp.NewToolResultText(tableResult(
		[]string{"live_rows", "dead_rows", "last_vacuum", "last_autovacuum", "last_analyze", "last_autoanalyze"},
		[][]string{{liveRows, deadRows, lastVacuum, lastAutovacuum, lastAnalyze, lastAutoanalyze}},
		false, h.cfg.CapNote(),
	)), nil
}

// HandleDatabaseSize returns total database size and per-table sizes.
func (h *Handler) HandleDatabaseSize(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	var totalSize string
	if err := h.pool.QueryRow(ctx, "SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&totalSize); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get database size: %v", err)), nil
	}

	rows, err := h.pool.Query(ctx, `
		SELECT schemaname, tablename,
			pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
		FROM pg_catalog.pg_tables
		ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC`)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get table sizes: %v", err)), nil
	}
	defer rows.Close()

	var tableRows [][]string
	var capped bool
	for rows.Next() {
		var schemaName, tableName, size string
		if err := rows.Scan(&schemaName, &tableName, &size); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		if h.schemaAllowed(schemaName) {
			tableRows = append(tableRows, []string{schemaName, tableName, size})
			if len(tableRows) == h.cfg.MaxRows {
				capped = rows.Next()
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}

	tableSizes := formatTable([]string{"schema", "table", "size"}, tableRows)
	if capped {
		tableSizes = h.cfg.CapNote() + tableSizes
	}
	return mcp.NewToolResultText(fmt.Sprintf("Total database size: %s\n\ntable_sizes:\n%s", totalSize, tableSizes)), nil
}

// HandleSearchSchema searches table, column, and view names by keyword.
func (h *Handler) HandleSearchSchema(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	term := req.GetString("term", "")
	if term == "" {
		return mcp.NewToolResultError("term parameter is required"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT 'table' AS kind, table_schema, table_name, ''
		FROM information_schema.tables WHERE table_name ILIKE '%' || $1 || '%'
		UNION ALL
		SELECT 'column', table_schema, table_name, column_name
		FROM information_schema.columns WHERE column_name ILIKE '%' || $1 || '%'
		UNION ALL
		SELECT 'view', table_schema, table_name, ''
		FROM information_schema.views WHERE table_name ILIKE '%' || $1 || '%'
		ORDER BY 1, 2, 3`, term)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var searchRows [][]string
	var capped bool
	for rows.Next() {
		var kind, schema, name, detail string
		if err := rows.Scan(&kind, &schema, &name, &detail); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		if h.schemaAllowed(schema) {
			searchRows = append(searchRows, []string{kind, schema, name, detail})
			if len(searchRows) == h.cfg.MaxRows {
				capped = rows.Next()
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(searchRows) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no matches for term %q", term)), nil
	}
	return mcp.NewToolResultText(tableResult([]string{"kind", "schema", "name", "detail"}, searchRows, capped, h.cfg.CapNote())), nil
}

// HandleERDiagram generates a Mermaid ERD from FK relationships.
func (h *Handler) HandleERDiagram(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := req.GetString("schema", "")
	if schema == "" {
		schema = h.cfg.DefaultSchema
	}
	if !h.schemaAllowed(schema) {
		return mcp.NewToolResultError(fmt.Sprintf("schema %q is not allowed", schema)), nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.cfg.QueryTimeout)
	defer cancel()

	rows, err := h.pool.Query(ctx, `
		SELECT
			tc.table_name,
			kcu.column_name,
			ccu.table_name AS foreign_table,
			ccu.column_name AS foreign_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = $1
		ORDER BY tc.table_name, kcu.column_name`, schema)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query failed: %v", err)), nil
	}
	defer rows.Close()

	var lines []string
	var capped bool
	for rows.Next() {
		var fromTable, fromCol, toTable, toCol string
		if err := rows.Scan(&fromTable, &fromCol, &toTable, &toCol); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scan failed: %v", err)), nil
		}
		lines = append(lines, fmt.Sprintf("    %s ||--o{ %s : \"%s -> %s\"", fromTable, toTable, fromCol, toCol))
		if len(lines) == h.cfg.MaxRows {
			capped = rows.Next()
			break
		}
	}
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rows error: %v", err)), nil
	}
	if len(lines) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("no foreign key relationships found in schema %s", schema)), nil
	}
	out := "erDiagram\n" + strings.Join(lines, "\n")
	if capped {
		out = h.cfg.CapNote() + out
	}
	return mcp.NewToolResultText(out), nil
}
