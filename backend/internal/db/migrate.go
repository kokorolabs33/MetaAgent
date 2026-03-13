package db

import (
	"database/sql"
	"embed"
	"fmt"
)

//go:embed migrations
var migrationsFS embed.FS

func RunMigrations(db *sql.DB) error {
	sql, err := migrationsFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.Exec(string(sql)); err != nil {
		return fmt.Errorf("run migration: %w", err)
	}
	return nil
}
