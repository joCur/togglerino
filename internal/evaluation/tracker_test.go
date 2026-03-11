package evaluation

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockFlagUpdater struct {
	mu      sync.Mutex
	batches [][]string
}

func (m *mockFlagUpdater) UpdateLastEvaluatedAt(_ context.Context, flagIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(flagIDs))
	copy(cp, flagIDs)
	m.batches = append(m.batches, cp)
	return nil
}

func TestTracker_Track_And_Flush(t *testing.T) {
	store := &mockFlagUpdater{}
	tracker := NewTracker(store, 24*time.Hour) // long interval — we flush manually

	tracker.Track("flag-1")
	tracker.Track("flag-2")
	tracker.Track("flag-1") // duplicate should be deduplicated

	tracker.flush()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.batches) != 1 {
		t.Fatalf("expected 1 flush batch, got %d", len(store.batches))
	}
	if len(store.batches[0]) != 2 {
		t.Errorf("expected 2 unique flag IDs, got %d", len(store.batches[0]))
	}
}

func TestTracker_Flush_Empty_NoDBCall(t *testing.T) {
	store := &mockFlagUpdater{}
	tracker := NewTracker(store, 24*time.Hour)

	tracker.flush()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.batches) != 0 {
		t.Errorf("expected no flush for empty tracker, got %d batches", len(store.batches))
	}
}

func TestTracker_Stop_FinalFlush(t *testing.T) {
	store := &mockFlagUpdater{}
	tracker := NewTracker(store, 24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	go tracker.Start(ctx)

	tracker.Track("flag-final")
	cancel()
	time.Sleep(50 * time.Millisecond) // allow goroutine to finish

	tracker.Stop()

	store.mu.Lock()
	defer store.mu.Unlock()

	// Should have at least 1 batch from Stop's final flush
	total := 0
	for _, b := range store.batches {
		total += len(b)
	}
	if total == 0 {
		t.Error("expected final flush to write flag-final, got 0 flags")
	}
}
