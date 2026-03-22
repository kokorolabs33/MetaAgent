package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/db"
)

type TestEnv struct {
	Pool *pgxpool.Pool
}

func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.RunMigrations(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return &TestEnv{Pool: pool}
}
