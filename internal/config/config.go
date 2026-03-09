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
	// OIDC (optional, overrides DB config when set)
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCDefaultRole           string
	OIDCSkipEmailVerification bool
	SessionSecret string
	BaseURL       string
}

func Load() (*Config, error) {
	oidcRole := envOr("OIDC_DEFAULT_ROLE", "member")
	if oidcRole != "admin" && oidcRole != "member" {
		return nil, fmt.Errorf("invalid OIDC_DEFAULT_ROLE %q: must be \"admin\" or \"member\"", oidcRole)
	}

	cfg := &Config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"),
		LogFormat:   envOr("LOG_FORMAT", "json"),
		CORSOrigins: parseOrigins(envOr("CORS_ORIGINS", "*")),
		OIDCIssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCDefaultRole:           oidcRole,
		OIDCSkipEmailVerification: os.Getenv("OIDC_SKIP_EMAIL_VERIFICATION") == "true",
		SessionSecret: os.Getenv("SESSION_SECRET"),
		BaseURL:       os.Getenv("BASE_URL"),
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

func (c *Config) OIDCConfigured() bool {
	return c.OIDCIssuerURL != "" && c.OIDCClientID != "" && c.OIDCClientSecret != ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
