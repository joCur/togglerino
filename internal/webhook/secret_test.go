package webhook

import (
	"strings"
	"testing"
)

func TestGenerateSecret(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error: %v", err)
	}
	if !strings.HasPrefix(secret, "whsec_") {
		t.Errorf("secret %q missing whsec_ prefix", secret)
	}
	// whsec_ (6) + 64 hex chars (32 bytes) = 70
	if len(secret) != 70 {
		t.Errorf("secret length = %d, want 70", len(secret))
	}
}

func TestGenerateSecret_Unique(t *testing.T) {
	s1, _ := GenerateSecret()
	s2, _ := GenerateSecret()
	if s1 == s2 {
		t.Error("two generated secrets should not be equal")
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"whsec_abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678", "whsec_abcd****"},
		{"short", "****"},
	}
	for _, tt := range tests {
		got := MaskSecret(tt.input)
		if got != tt.want {
			t.Errorf("MaskSecret(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
