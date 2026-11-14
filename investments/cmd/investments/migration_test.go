package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/rthomazel/mcp/investments/internal/storage/postgres"
)

func TestMigrateAppliesSchemaAndIsRepeatable(t *testing.T) {
	adminDSN := os.Getenv("INVESTMENTS_MCP_TEST_DSN")
	if adminDSN == "" {
		t.Skip("INVESTMENTS_MCP_TEST_DSN not set")
	}

	ctx := context.Background()
	name := fmt.Sprintf("investments_migrate_%d_%d", os.Getpid(), rand.Uint32())

	admin, err := pgx.Connect(ctx, adminDSN)
	require.NoError(t, err)

	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)

	u, err := url.Parse(adminDSN)
	require.NoError(t, err)

	u.Path = "/" + name

	store, cleanup, err := postgres.Open(u.String(), 5)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = cleanup()
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})

	require.NoError(t, migrate(store))
	require.NoError(t, migrate(store), "a second run is a no-op")

	var tables int
	require.NoError(t, store.DB().QueryRow(
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'public'
		   AND table_name IN ('instruments', 'transactions', 'prices', 'snapshots', 'updates')`).Scan(&tables))
	require.Equal(t, 5, tables)
}
