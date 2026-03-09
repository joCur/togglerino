package oidc

import (
	"encoding/json"
	"testing"
)

func TestClaimsEmailVerified(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantVer  bool
		wantEmail string
	}{
		{
			name:      "email_verified true",
			json:      `{"sub":"123","email":"user@example.com","name":"User","email_verified":true}`,
			wantVer:   true,
			wantEmail: "user@example.com",
		},
		{
			name:      "email_verified false",
			json:      `{"sub":"123","email":"user@example.com","name":"User","email_verified":false}`,
			wantVer:   false,
			wantEmail: "user@example.com",
		},
		{
			name:      "email_verified missing defaults to false",
			json:      `{"sub":"123","email":"user@example.com","name":"User"}`,
			wantVer:   false,
			wantEmail: "user@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Claims
			if err := json.Unmarshal([]byte(tt.json), &c); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if c.EmailVerified != tt.wantVer {
				t.Errorf("EmailVerified = %v, want %v", c.EmailVerified, tt.wantVer)
			}
			if c.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", c.Email, tt.wantEmail)
			}
		})
	}
}
