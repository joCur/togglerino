package webhook

import "testing"

func TestMatchesEventType(t *testing.T) {
	tests := []struct {
		name      string
		hookTypes []string
		eventType string
		want      bool
	}{
		{"exact match", []string{"flag.created", "flag.updated"}, "flag.created", true},
		{"no match", []string{"flag.created"}, "flag.updated", false},
		{"empty filter", []string{}, "flag.created", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesEventType(tt.hookTypes, tt.eventType); got != tt.want {
				t.Errorf("matchesEventType(%v, %q) = %v, want %v", tt.hookTypes, tt.eventType, got, tt.want)
			}
		})
	}
}
