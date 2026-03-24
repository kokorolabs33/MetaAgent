package db

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files := []string{"migrations/001_foundation.sql", "migrations/004_a2a_migration.sql"}
	for _, file := range files {
		sql, err := migrationsFS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("run migration %s: %w", file, err)
		}
	}
	return nil
}
