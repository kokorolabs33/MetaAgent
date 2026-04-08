package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear any env vars that might interfere
	envVars := []string{
		"TASKHUB_MODE", "DATABASE_URL", "PORT",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"SESSION_SECRET", "TASKHUB_SECRET_KEY",
		"ANTHROPIC_API_KEY", "FRONTEND_URL",
	}
	for _, v := range envVars {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}

	cfg := Load()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"Mode", cfg.Mode, "local"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://localhost:5432/taskhub?sslmode=disable"},
		{"Port", cfg.Port, "8080"},
		{"GoogleClientID", cfg.GoogleClientID, ""},
		{"GoogleSecret", cfg.GoogleSecret, ""},
		{"SessionSecret", cfg.SessionSecret, "change-me-in-production"},
		{"SecretKey", cfg.SecretKey, ""},
		{"AnthropicAPIKey", cfg.AnthropicAPIKey, ""},
		{"FrontendURL", cfg.FrontendURL, "http://localhost:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("TASKHUB_MODE", "cloud")
	t.Setenv("DATABASE_URL", "postgres://prod:5432/taskhub")
	t.Setenv("PORT", "9090")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-key")
	t.Setenv("FRONTEND_URL", "https://app.taskhub.io")

	cfg := Load()

	if cfg.Mode != "cloud" {
		t.Errorf("Mode = %q, want cloud", cfg.Mode)
	}
	if cfg.DatabaseURL != "postgres://prod:5432/taskhub" {
		t.Errorf("DatabaseURL = %q, want postgres://prod:5432/taskhub", cfg.DatabaseURL)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.AnthropicAPIKey != "sk-test-key" { // pragma: allowlist secret
		t.Errorf("AnthropicAPIKey = %q, want sk-test-key", cfg.AnthropicAPIKey) // pragma: allowlist secret
	}
	if cfg.FrontendURL != "https://app.taskhub.io" {
		t.Errorf("FrontendURL = %q, want https://app.taskhub.io", cfg.FrontendURL)
	}
}

func TestIsLocal_DefaultIsLocal(t *testing.T) {
	t.Setenv("TASKHUB_MODE", "")
	os.Unsetenv("TASKHUB_MODE")

	cfg := Load()

	if !cfg.IsLocal() {
		t.Error("expected IsLocal() = true for default mode")
	}
}

func TestIsLocal_CloudMode(t *testing.T) {
	t.Setenv("TASKHUB_MODE", "cloud")

	cfg := Load()

	if cfg.IsLocal() {
		t.Error("expected IsLocal() = false for cloud mode")
	}
}

func TestIsLocal_ExplicitLocal(t *testing.T) {
	t.Setenv("TASKHUB_MODE", "local")

	cfg := Load()

	if !cfg.IsLocal() {
		t.Error("expected IsLocal() = true for explicit local mode")
	}
}

func TestIsLocal_AnyNonCloudIsLocal(t *testing.T) {
	tests := []struct {
		mode     string
		expected bool
	}{
		{"local", true},
		{"", true},
		{"development", true},
		{"cloud", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := &Config{Mode: tt.mode}
			if cfg.IsLocal() != tt.expected {
				t.Errorf("IsLocal() for mode %q = %v, want %v", tt.mode, cfg.IsLocal(), tt.expected)
			}
		})
	}
}
