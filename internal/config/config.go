// internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	LogFormat   string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"),
		LogFormat:   envOr("LOG_FORMAT", "json"),
	}
	return cfg, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
