// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds all runtime configuration for the server.
type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	CORSOrigins  []string
	SeedDemo     bool
	LogLevel     string
	ShutdownWait int
}

// Load reads configuration from environment variables with sane defaults.
func Load() (Config, error) {
	cfg := Config{
		Port:         getenv("PORT", "8080"),
		DatabaseURL:  getenv("DATABASE_URL", "postgres://lifelink:lifelink_dev_password@localhost:5432/lifelink?sslmode=disable"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		SeedDemo:     getenvBool("SEED_DEMO", true),
		LogLevel:     getenv("LOG_LEVEL", "info"),
		ShutdownWait: 10,
	}

	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	origins := getenv("CORS_ORIGINS", "http://localhost:5173,http://localhost:8080")
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1" || strings.EqualFold(v, "yes")
}
