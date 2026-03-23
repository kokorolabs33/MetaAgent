package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Mode            string // "local" (no auth, single user) or "cloud" (full auth + multi-org)
	DatabaseURL     string
	Port            string
	GoogleClientID  string
	GoogleSecret    string
	SessionSecret   string
	SecretKey       string
	AnthropicAPIKey string
	FrontendURL     string
}

// IsLocal returns true if running in local/community mode (no auth required).
func (c *Config) IsLocal() bool {
	return c.Mode != "cloud"
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Mode:            getEnv("TASKHUB_MODE", "local"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://localhost:5432/taskhub?sslmode=disable"),
		Port:            getEnv("PORT", "8080"),
		GoogleClientID:  getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleSecret:    getEnv("GOOGLE_CLIENT_SECRET", ""),
		SessionSecret:   getEnv("SESSION_SECRET", "change-me-in-production"),
		SecretKey:       getEnv("TASKHUB_SECRET_KEY", ""),
		AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),
		FrontendURL:     getEnv("FRONTEND_URL", "http://localhost:3000"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
