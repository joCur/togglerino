package override

import (
	"testing"
	"time"
)

func TestNewCleaner(t *testing.T) {
	c := NewCleaner(nil, 15*time.Minute)
	if c == nil {
		t.Fatal("expected non-nil cleaner")
	}
	if c.interval != 15*time.Minute {
		t.Fatalf("expected 15m interval, got %v", c.interval)
	}
}
