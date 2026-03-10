package stream

import "testing"

func TestHub_AllSubscriberCounts(t *testing.T) {
	h := NewHub()

	counts := h.AllSubscriberCounts()
	if len(counts) != 0 {
		t.Errorf("empty hub: expected 0 entries, got %d", len(counts))
	}

	ch1 := h.Subscribe("proj1", "dev")
	ch2 := h.Subscribe("proj1", "dev")
	ch3 := h.Subscribe("proj2", "prod")

	counts = h.AllSubscriberCounts()
	if counts["proj1:dev"] != 2 {
		t.Errorf("expected proj1:dev=2, got %d", counts["proj1:dev"])
	}
	if counts["proj2:prod"] != 1 {
		t.Errorf("expected proj2:prod=1, got %d", counts["proj2:prod"])
	}

	h.Unsubscribe("proj1", "dev", ch1)
	h.Unsubscribe("proj1", "dev", ch2)
	h.Unsubscribe("proj2", "prod", ch3)
}
