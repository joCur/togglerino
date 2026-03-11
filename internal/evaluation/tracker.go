package evaluation

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// FlagEvaluationUpdater is the interface for batch-updating last_evaluated_at timestamps.
type FlagEvaluationUpdater interface {
	UpdateLastEvaluatedAt(ctx context.Context, flagIDs []string) error
}

// Tracker batches flag evaluation events and periodically flushes them to the database.
type Tracker struct {
	store    FlagEvaluationUpdater
	interval time.Duration

	mu      sync.Mutex
	pending map[string]struct{}
}

// NewTracker creates a new evaluation tracker.
func NewTracker(store FlagEvaluationUpdater, interval time.Duration) *Tracker {
	return &Tracker{
		store:    store,
		interval: interval,
		pending:  make(map[string]struct{}),
	}
}

// Track records that a flag was evaluated. Safe for concurrent use.
func (t *Tracker) Track(flagID string) {
	t.mu.Lock()
	t.pending[flagID] = struct{}{}
	t.mu.Unlock()
}

// Start runs the periodic flush loop. Blocks until ctx is cancelled.
func (t *Tracker) Start(ctx context.Context) {
	slog.Info("evaluation tracker started", "interval", t.interval)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("evaluation tracker stopped")
			return
		case <-ticker.C:
			t.flush()
		}
	}
}

// Stop performs a final flush to ensure no tracked evaluations are lost.
func (t *Tracker) Stop() {
	t.flush()
}

func (t *Tracker) flush() {
	t.mu.Lock()
	if len(t.pending) == 0 {
		t.mu.Unlock()
		return
	}
	batch := t.pending
	t.pending = make(map[string]struct{})
	t.mu.Unlock()

	ids := make([]string, 0, len(batch))
	for id := range batch {
		ids = append(ids, id)
	}

	// On failure, the batch is intentionally dropped. The tracker operates on
	// day-scale thresholds, so a missed flush is corrected on the next cycle.
	if err := t.store.UpdateLastEvaluatedAt(context.Background(), ids); err != nil {
		slog.Error("evaluation tracker: failed to flush", "count", len(ids), "error", err)
	} else {
		slog.Debug("evaluation tracker: flushed", "count", len(ids))
	}
}
