package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"

	"taskhub/internal/config"
	"taskhub/internal/db"
	"taskhub/internal/seed"
)

func main() {
	_ = godotenv.Load()
	ctx := context.Background()
	cfg := config.Load()

	log.Printf("seed-demo: connecting to database...")

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	// Ensure base seed data exists (user, templates, policies)
	if err := seed.LocalSeed(ctx, pool); err != nil {
		log.Fatalf("local seed: %v", err)
	}

	if err := seed.SeedDemo(ctx, pool); err != nil {
		log.Fatalf("seed-demo: %v", err)
	}

	log.Println("Demo data seeded successfully")
}
