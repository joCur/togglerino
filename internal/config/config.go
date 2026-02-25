// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
	LogFormat   string
	CORSOrigins []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"),
		LogFormat:   envOr("LOG_FORMAT", "json"),
		CORSOrigins: parseOrigins(envOr("CORS_ORIGINS", "*")),
	}
	return cfg, nil
}

// parseOrigins splits a comma-separated string into a slice of trimmed, non-empty origins.
func parseOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
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
