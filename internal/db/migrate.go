package db

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations
var migrationsFS embed.FS

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Create migration tracking table
	_, err := pool.Exec(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	files := []string{
		"migrations/001_foundation.sql",
		"migrations/004_a2a_migration.sql",
		"migrations/005_remove_org_add_templates.sql",
		"migrations/006_webhooks.sql",
		"migrations/007_conversations.sql",
	}

	for _, file := range files {
		// Skip if already applied
		var applied bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, file).
			Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", file, err)
		}
		if applied {
			continue
		}

		// Read and execute migration
		sql, err := migrationsFS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("run migration %s: %w", file, err)
		}

		// Record as applied
		_, err = pool.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, file)
		if err != nil {
			return fmt.Errorf("record migration %s: %w", file, err)
		}

		log.Printf("migration applied: %s", file)
	}

	return nil
}
