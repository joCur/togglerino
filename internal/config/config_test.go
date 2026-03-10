package config

import (
	"testing"
)

func TestLoad_OIDCSkipEmailVerification(t *testing.T) {
	// Default: false
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDCSkipEmailVerification {
		t.Error("expected OIDCSkipEmailVerification=false by default")
	}

	// Explicit true
	t.Setenv("OIDC_SKIP_EMAIL_VERIFICATION", "true")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OIDCSkipEmailVerification {
		t.Error("expected OIDCSkipEmailVerification=true when env var is 'true'")
	}
}

func TestLoad_MetricsDefaults(t *testing.T) {
	t.Setenv("METRICS_ENABLED", "")
	t.Setenv("METRICS_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled to default to true")
	}
	if cfg.MetricsPort != "" {
		t.Errorf("expected MetricsPort to default to empty, got %q", cfg.MetricsPort)
	}
}

func TestLoad_MetricsDisabled(t *testing.T) {
	t.Setenv("METRICS_ENABLED", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsEnabled {
		t.Error("expected MetricsEnabled to be false")
	}
}

func TestLoad_MetricsPort(t *testing.T) {
	t.Setenv("METRICS_PORT", "9090")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsPort != "9090" {
		t.Errorf("expected MetricsPort 9090, got %q", cfg.MetricsPort)
	}
}
