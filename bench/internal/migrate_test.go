package internal

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rthomazel/mcp/bench/db"
)

func openMigrateTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "migrate.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestMigrateDB_CreatesSchema(t *testing.T) {
	conn := openMigrateTestDB(t)
	if err := MigrateDB(conn, db.Migrations); err != nil {
		t.Fatalf("MigrateDB: %v", err)
	}

	// A minimal insert should succeed after migration.
	_, err := conn.Exec(`INSERT INTO tool_calls (id, tool, called_at) VALUES ('test-id', 'shell', datetime('now'))`)
	if err != nil {
		t.Fatalf("insert after migration: %v", err)
	}
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM tool_calls`).Scan(&count); err != nil {
		t.Fatalf("query after migration: %v", err)
	}
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

func TestMigrateDB_Idempotent(t *testing.T) {
	conn := openMigrateTestDB(t)
	for i := 0; i < 3; i++ {
		if err := MigrateDB(conn, db.Migrations); err != nil {
			t.Fatalf("MigrateDB run %d: %v", i+1, err)
		}
	}
}

func TestMigrateDB_IndexesExist(t *testing.T) {
	conn := openMigrateTestDB(t)
	if err := MigrateDB(conn, db.Migrations); err != nil {
		t.Fatalf("MigrateDB: %v", err)
	}

	wantIndexes := []string{
		"idx_tc_called_at",
		"idx_tc_tool_date",
		"idx_tc_tool_hash_date",
		"idx_tc_file_path",
	}
	for _, idx := range wantIndexes {
		var name string
		err := conn.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %q not found: %v", idx, err)
		}
	}
}
