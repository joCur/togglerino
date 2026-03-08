package model

import "testing"

func TestValidPermission(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"flags:read", true},
		{"flags:write", true},
		{"environments:read", true},
		{"environments:write", true},
		{"sdk_keys:manage", true},
		{"segments:write", true},
		{"templates:manage", true},
		{"project:settings", true},
		{"", false},
		{"unknown", false},
		{"org:users:manage", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ValidPermission(tt.input); got != tt.want {
				t.Errorf("ValidPermission(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
