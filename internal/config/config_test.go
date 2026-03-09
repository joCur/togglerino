package config

import (
	"os"
	"testing"
)

func TestLoad_OIDCSkipEmailVerification(t *testing.T) {
	// Default: false
	os.Unsetenv("OIDC_SKIP_EMAIL_VERIFICATION")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDCSkipEmailVerification {
		t.Error("expected OIDCSkipEmailVerification=false by default")
	}

	// Explicit true
	os.Setenv("OIDC_SKIP_EMAIL_VERIFICATION", "true")
	defer os.Unsetenv("OIDC_SKIP_EMAIL_VERIFICATION")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OIDCSkipEmailVerification {
		t.Error("expected OIDCSkipEmailVerification=true when env var is 'true'")
	}
}
