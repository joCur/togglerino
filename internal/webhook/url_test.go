package webhook

import "testing"

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com/webhook", false},
		{"valid https with path", "https://hooks.slack.com/services/T00/B00/xxx", false},
		{"http localhost allowed", "http://localhost:8080/hook", false},
		{"http 127.0.0.1 allowed", "http://127.0.0.1:3000/hook", false},
		{"http non-local rejected", "http://example.com/webhook", true},
		{"private 10.x rejected", "https://10.0.0.1/webhook", true},
		{"private 172.16.x rejected", "https://172.16.0.1/webhook", true},
		{"private 192.168.x rejected", "https://192.168.1.1/webhook", true},
		{"link-local rejected", "https://169.254.1.1/webhook", true},
		{"empty url", "", true},
		{"no scheme", "example.com/webhook", true},
		{"ftp rejected", "ftp://example.com/file", true},
		{"too long", "https://example.com/" + string(make([]byte, 2048)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}
