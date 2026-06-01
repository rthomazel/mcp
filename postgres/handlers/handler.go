// Package handlers implements MCP tool handlers for postgres-mcp.
package handlers

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rthomazel/mcp/postgres/internal"
)

// Handler holds shared state for all tool handlers.
type Handler struct {
	cfg  *internal.Config
	pool *pgxpool.Pool
}

// New constructs a Handler.
func New(cfg *internal.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{cfg: cfg, pool: pool}
}

// schemaAllowed returns true if schema passes the AllowedSchemas/DeniedSchemas filters.
func (h *Handler) schemaAllowed(schema string) bool {
	if len(h.cfg.AllowedSchemas) > 0 {
		found := false
		for _, s := range h.cfg.AllowedSchemas {
			if s == schema {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, s := range h.cfg.DeniedSchemas {
		if s == schema {
			return false
		}
	}
	return true
}

// allowlists for each SQL class.
var (
	dqlAllowlist = []string{"SELECT", "SHOW", "TABLE", "WITH"}
	dmlAllowlist = []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE"}
	ddlAllowlist = []string{"CREATE", "ALTER", "DROP"}
	dclAllowlist = []string{"GRANT", "REVOKE"}
)

// tableResult formats data as a tab-separated table, prepending note when capped is true.
// note is the cap-warning string from Config.CapNote().
// Returns an empty string if data is empty — callers should emit their own context-specific empty message.
func tableResult(headers []string, data [][]string, capped bool, note string) string {
	if len(data) == 0 {
		return ""
	}
	out := formatTable(headers, data)
	if capped {
		out = note + out
	}
	return out
}

// formatTable returns a plain-text tab-separated table from headers and rows.
func formatTable(headers []string, rows [][]string) string {
	var buf strings.Builder
	buf.WriteString(strings.Join(headers, "\t"))
	for _, row := range rows {
		buf.WriteByte('\n')
		buf.WriteString(strings.Join(row, "\t"))
	}
	return buf.String()
}

// collectRows reads all columns from pgx.Rows generically, capping at maxRows.
// Returns column names, row data as [][]string, whether the cap was hit, and any error.
func collectRows(rows pgx.Rows, maxRows int) (headers []string, data [][]string, capped bool, err error) {
	for _, fd := range rows.FieldDescriptions() {
		headers = append(headers, fd.Name)
	}
	for rows.Next() {
		vals, scanErr := rows.Values()
		if scanErr != nil {
			return nil, nil, false, scanErr
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = formatValue(v)
		}
		data = append(data, row)
		if len(data) == maxRows {
			capped = rows.Next() // peek: true means more rows exist beyond the cap
			break
		}
	}
	return headers, data, capped, rows.Err()
}

// formatValue converts a pgx generic value to a display string.
// Handles [16]byte as a UUID string; all others use fmt.Sprintf.
func formatValue(v any) string {
	if v == nil {
		return ""
	}
	if b, ok := v.([16]byte); ok {
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	return fmt.Sprintf("%v", v)
}
