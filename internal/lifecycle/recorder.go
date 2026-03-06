package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

// FlagLister lists all flags across all projects.
type FlagLister interface {
	ListAll(ctx context.Context) ([]model.Flag, error)
}

// SnapshotRecorder records a lifecycle snapshot for a project.
type SnapshotRecorder interface {
	Record(ctx context.Context, projectID string, active, potentiallyStale, stale, archived int) error
}

// Recorder periodically records lifecycle snapshots for all projects.
type Recorder struct {
	flags     FlagLister
	snapshots SnapshotRecorder
	interval  time.Duration
}

// NewRecorder creates a new lifecycle snapshot recorder.
func NewRecorder(flags FlagLister, snapshots SnapshotRecorder, interval time.Duration) *Recorder {
	return &Recorder{flags: flags, snapshots: snapshots, interval: interval}
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
	flags, err := r.flags.ListAll(ctx)
	if err != nil {
		slog.Error("lifecycle recorder: failed to list flags", "error", err)
		return
	}

	counts := map[string]*projectCounts{}
	for _, f := range flags {
		c, ok := counts[f.ProjectID]
		if !ok {
			c = &projectCounts{}
			counts[f.ProjectID] = c
		}
		switch f.LifecycleStatus {
		case model.LifecycleActive:
			c.active++
		case model.LifecyclePotentiallyStale:
			c.potentiallyStale++
		case model.LifecycleStale:
			c.stale++
		case model.LifecycleArchived:
			c.archived++
		}
	}

	for projectID, c := range counts {
		if err := r.snapshots.Record(ctx, projectID, c.active, c.potentiallyStale, c.stale, c.archived); err != nil {
			slog.Error("lifecycle recorder: failed to record snapshot", "project_id", projectID, "error", err)
		}
	}

	slog.Info("lifecycle recorder: recorded snapshots", "projects", len(counts))
}
