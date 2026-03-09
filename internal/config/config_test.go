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
