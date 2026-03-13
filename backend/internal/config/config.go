package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL  string
	OpenAIAPIKey string
	Port         string
}

func Load() *Config {
	_ = godotenv.Load()
	cfg := &Config{
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://localhost:5432/taskhub?sslmode=disable"),
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
		Port:         getEnv("PORT", "8080"),
	}
	if cfg.OpenAIAPIKey == "" {
		log.Println("WARNING: OPENAI_API_KEY not set")
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
