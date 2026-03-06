package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

// LifecycleCounter returns flag counts grouped by project and lifecycle status.
type LifecycleCounter interface {
	LifecycleCountsByProject(ctx context.Context) ([]model.LifecycleCountRow, error)
}

// SnapshotRecorder records a lifecycle snapshot for a project.
type SnapshotRecorder interface {
	Record(ctx context.Context, projectID string, active, potentiallyStale, stale, archived int) error
}

// Recorder periodically records lifecycle snapshots for all projects.
type Recorder struct {
	counter   LifecycleCounter
	snapshots SnapshotRecorder
	interval  time.Duration
}

// NewRecorder creates a new lifecycle snapshot recorder.
func NewRecorder(counter LifecycleCounter, snapshots SnapshotRecorder, interval time.Duration) *Recorder {
	return &Recorder{counter: counter, snapshots: snapshots, interval: interval}
}

// Run starts the recorder loop. Blocks until ctx is cancelled.
func (r *Recorder) Run(ctx context.Context) {
	slog.Info("lifecycle snapshot recorder started", "interval", r.interval)
	r.tick(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("lifecycle snapshot recorder stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

type projectCounts struct {
	active, potentiallyStale, stale, archived int
}

func (r *Recorder) tick(ctx context.Context) {
	rows, err := r.counter.LifecycleCountsByProject(ctx)
	if err != nil {
		slog.Error("lifecycle recorder: failed to query counts", "error", err)
		return
	}

	counts := map[string]*projectCounts{}
	for _, row := range rows {
		c, ok := counts[row.ProjectID]
		if !ok {
			c = &projectCounts{}
			counts[row.ProjectID] = c
		}
		switch row.Status {
		case "active":
			c.active += row.Count
		case "potentially_stale":
			c.potentiallyStale += row.Count
		case "stale":
			c.stale += row.Count
		case "archived":
			c.archived += row.Count
		}
	}

	for projectID, c := range counts {
		if err := r.snapshots.Record(ctx, projectID, c.active, c.potentiallyStale, c.stale, c.archived); err != nil {
			slog.Error("lifecycle recorder: failed to record snapshot", "project_id", projectID, "error", err)
		}
	}

	slog.Info("lifecycle recorder: recorded snapshots", "projects", len(counts))
}
