package handlers

import (
	"strings"
	"testing"

	"github.com/rthomazel/mcp/postgres/internal"
)

// newTestHandler constructs a Handler with nil pool for unit tests.
// Only use for methods that do not touch the database (schemaAllowed, sqlClassify, formatTable).
func newTestHandler(cfg *internal.Config) *Handler {
	return &Handler{cfg: cfg, pool: nil}
}

// — schemaAllowed —

func TestSchemaAllowed(t *testing.T) {
	tests := []struct {
		name           string
		schema         string
		allowedSchemas []string
		deniedSchemas  []string
		want           bool
	}{
		{
			name: "no allowlist no denylist - public passes",
			schema: "public",
			want:   true,
		},
		{
			name:   "no allowlist no denylist - any schema passes",
			schema: "arbitrary",
			want:   true,
		},
		{
			name:           "allowlist set - matching schema passes",
			schema:         "public",
			allowedSchemas: []string{"public"},
			want:           true,
		},
		{
			name:           "allowlist set - non-matching schema fails",
			schema:         "secret",
			allowedSchemas: []string{"public"},
			want:           false,
		},
		{
			name:          "denylist set - non-denied passes",
			schema:        "public",
			deniedSchemas: []string{"pg_catalog"},
			want:          true,
		},
		{
			name:          "denylist set - denied schema fails",
			schema:        "pg_catalog",
			deniedSchemas: []string{"pg_catalog"},
			want:          false,
		},
		{
			name:           "both set - in allowlist and not in denylist passes",
			schema:         "public",
			allowedSchemas: []string{"public", "app"},
			deniedSchemas:  []string{"app"},
			want:           true,
		},
		{
			name:           "both set - in both allowlist and denylist fails",
			schema:         "app",
			allowedSchemas: []string{"public", "app"},
			deniedSchemas:  []string{"app"},
			want:           false,
		},
		{
			name:           "both set - not in allowlist fails",
			schema:         "other",
			allowedSchemas: []string{"public", "app"},
			deniedSchemas:  []string{"app"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&internal.Config{
				AllowedSchemas: tt.allowedSchemas,
				DeniedSchemas:  tt.deniedSchemas,
			})
			if got := h.schemaAllowed(tt.schema); got != tt.want {
				t.Errorf("schemaAllowed(%q) = %v, want %v", tt.schema, got, tt.want)
			}
		})
	}
}

// — sqlClassify —

func TestSqlClassify(t *testing.T) {
	tests := []struct {
		name            string
		sql             string
		allowDML        bool
		allowDDL        bool
		allowDCL        bool
		wantAllowlist   []string
		wantFlagAllowed bool
		wantFlagName    string
		wantErr         bool
		wantErrContains string
	}{
		{name: "INSERT", sql: "INSERT INTO foo VALUES (1)", allowDML: true, wantAllowlist: dmlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DML"},
		{name: "UPDATE", sql: "UPDATE foo SET x=1", allowDML: true, wantAllowlist: dmlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DML"},
		{name: "DELETE", sql: "DELETE FROM foo", allowDML: true, wantAllowlist: dmlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DML"},
		{name: "TRUNCATE", sql: "TRUNCATE foo", allowDML: true, wantAllowlist: dmlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DML"},
		{name: "CREATE", sql: "CREATE TABLE foo ()", allowDDL: true, wantAllowlist: ddlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DDL"},
		{name: "ALTER", sql: "ALTER TABLE foo ADD COLUMN x int", allowDDL: true, wantAllowlist: ddlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DDL"},
		{name: "DROP", sql: "DROP TABLE foo", allowDDL: true, wantAllowlist: ddlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DDL"},
		{name: "GRANT", sql: "GRANT SELECT ON foo TO user1", allowDCL: true, wantAllowlist: dclAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DCL"},
		{name: "REVOKE", sql: "REVOKE SELECT ON foo FROM user1", allowDCL: true, wantAllowlist: dclAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DCL"},
		{name: "SELECT", sql: "SELECT * FROM foo", wantAllowlist: dqlAllowlist, wantFlagAllowed: true, wantFlagName: ""},
		{name: "SHOW", sql: "SHOW search_path", wantAllowlist: dqlAllowlist, wantFlagAllowed: true, wantFlagName: ""},
		{name: "TABLE", sql: "TABLE foo", wantAllowlist: dqlAllowlist, wantFlagAllowed: true, wantFlagName: ""},
		{name: "WITH", sql: "WITH cte AS (SELECT 1) SELECT * FROM cte", wantAllowlist: dqlAllowlist, wantFlagAllowed: true, wantFlagName: ""},
		{name: "lowercase insert", sql: "insert into foo values (1)", allowDML: true, wantAllowlist: dmlAllowlist, wantFlagAllowed: true, wantFlagName: "POSTGRES_MCP_ALLOW_DML"},
		{name: "DML flag off", sql: "INSERT INTO foo VALUES (1)", allowDML: false, wantAllowlist: dmlAllowlist, wantFlagAllowed: false, wantFlagName: "POSTGRES_MCP_ALLOW_DML"},
		{name: "whitespace only", sql: "  ", wantErr: true, wantErrContains: "empty"},
		{name: "unknown token", sql: "UNKNOWN blah", wantErr: true, wantErrContains: "not recognized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(&internal.Config{
				AllowDML: tt.allowDML,
				AllowDDL: tt.allowDDL,
				AllowDCL: tt.allowDCL,
			})
			gotAllowlist, gotFlagAllowed, gotFlagName, err := h.sqlClassify(tt.sql)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("sqlClassify(%q) error = nil, want error containing %q", tt.sql, tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("sqlClassify(%q) error = %q, want to contain %q", tt.sql, err.Error(), tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("sqlClassify(%q) unexpected error: %v", tt.sql, err)
			}
			if gotFlagAllowed != tt.wantFlagAllowed {
				t.Errorf("sqlClassify(%q) flagAllowed = %v, want %v", tt.sql, gotFlagAllowed, tt.wantFlagAllowed)
			}
			if gotFlagName != tt.wantFlagName {
				t.Errorf("sqlClassify(%q) flagName = %q, want %q", tt.sql, gotFlagName, tt.wantFlagName)
			}
			if len(gotAllowlist) != len(tt.wantAllowlist) {
				t.Fatalf("sqlClassify(%q) allowlist length = %d, want %d", tt.sql, len(gotAllowlist), len(tt.wantAllowlist))
			}
			for i := range gotAllowlist {
				if gotAllowlist[i] != tt.wantAllowlist[i] {
					t.Errorf("sqlClassify(%q) allowlist[%d] = %q, want %q", tt.sql, i, gotAllowlist[i], tt.wantAllowlist[i])
				}
			}
		})
	}
}

// — formatTable —

func TestFormatTable(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		want    string
	}{
		{
			name:    "headers only no rows",
			headers: []string{"id", "name"},
			rows:    [][]string{},
			want:    "id\tname",
		},
		{
			name:    "single row",
			headers: []string{"id", "name"},
			rows:    [][]string{{"1", "alice"}},
			want:    "id\tname\n1\talice",
		},
		{
			name:    "multiple rows",
			headers: []string{"id", "name"},
			rows:    [][]string{{"1", "alice"}, {"2", "bob"}, {"3", "charlie"}},
			want:    "id\tname\n1\talice\n2\tbob\n3\tcharlie",
		},
		{
			name:    "empty string cells render empty not NULL",
			headers: []string{"id", "value"},
			rows:    [][]string{{"1", ""}, {"2", "text"}},
			want:    "id\tvalue\n1\t\n2\ttext",
		},
		{
			name:    "tab in cell value passed through",
			headers: []string{"id", "data"},
			rows:    [][]string{{"1", "a\tb"}},
			want:    "id\tdata\n1\ta\tb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTable(tt.headers, tt.rows)
			if got != tt.want {
				t.Errorf("formatTable() = %q, want %q", got, tt.want)
			}
			wantLines := 1 + len(tt.rows)
			gotLines := strings.Count(got, "\n") + 1
			if gotLines != wantLines {
				t.Errorf("formatTable() line count = %d, want %d", gotLines, wantLines)
			}
		})
	}
}
