package webhook

import "testing"

func TestSign(t *testing.T) {
	payload := []byte(`{"type":"flag.created","project_id":"abc"}`)
	secret := "whsec_deadbeef"
	sig := Sign(payload, secret)
	if sig == "" {
		t.Fatal("Sign() returned empty string")
	}
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}
}

func TestSign_Deterministic(t *testing.T) {
	payload := []byte(`{"test":true}`)
	secret := "whsec_test123"
	s1 := Sign(payload, secret)
	s2 := Sign(payload, secret)
	if s1 != s2 {
		t.Error("same payload+secret should produce same signature")
	}
}

func TestSign_DifferentSecrets(t *testing.T) {
	payload := []byte(`{"test":true}`)
	s1 := Sign(payload, "whsec_secret1")
	s2 := Sign(payload, "whsec_secret2")
	if s1 == s2 {
		t.Error("different secrets should produce different signatures")
	}
}
