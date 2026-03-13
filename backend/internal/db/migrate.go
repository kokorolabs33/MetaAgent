package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func RunMigrations(db *sql.DB) error {
	_, filename, _, _ := runtime.Caller(0)
	// migrations dir is 2 levels up from internal/db/migrate.go
	migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	sql, err := os.ReadFile(filepath.Join(migrationsDir, "001_init.sql"))
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := db.Exec(string(sql)); err != nil {
		return fmt.Errorf("run migration: %w", err)
	}
	return nil
}
