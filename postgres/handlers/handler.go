// Package handlers implements MCP tool handlers for postgres-mcp.
package handlers

import (
	"slices"
	"strings"

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
// If AllowedSchemas is non-empty, schema must be in the list.
// schema must not be in DeniedSchemas.
func (h *Handler) schemaAllowed(schema string) bool {
	if len(h.cfg.AllowedSchemas) > 0 {
		for _, s := range h.cfg.AllowedSchemas {
			if s == schema {
				goto checkDenied
			}
		}
		return false
	}
checkDenied:
	if slices.Contains(h.cfg.DeniedSchemas, schema) {
			return false
		}
	return true
}

// allowlists for each SQL class, used by query, transaction, and diagnostic handlers.
var (
	dqlAllowlist = []string{"SELECT", "SHOW", "TABLE", "WITH"}
	dmlAllowlist = []string{"INSERT", "UPDATE", "DELETE", "TRUNCATE"}
	ddlAllowlist = []string{"CREATE", "ALTER", "DROP"}
	dclAllowlist = []string{"GRANT", "REVOKE"}
)

// formatTable returns a plain-text tab-separated table from headers and rows.
// Each nil cell value is rendered as empty string.
func formatTable(headers []string, rows [][]string) string {
	var buf strings.Builder
	buf.WriteString(strings.Join(headers, "\t"))
	for _, row := range rows {
		buf.WriteByte('\n')
		buf.WriteString(strings.Join(row, "\t"))
	}
	return buf.String()
}
