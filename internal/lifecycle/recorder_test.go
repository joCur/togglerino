package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

type mockAllFlagStore struct {
	flags []model.Flag
}

func (m *mockAllFlagStore) ListAll(_ context.Context) ([]model.Flag, error) {
	return m.flags, nil
}

type snapshot struct {
	projectID                                string
	active, potentiallyStale, stale, archived int
}

type mockSnapshotStore struct {
	recorded []snapshot
}

func (m *mockSnapshotStore) Record(_ context.Context, projectID string, active, potentiallyStale, stale, archived int) error {
	m.recorded = append(m.recorded, snapshot{projectID, active, potentiallyStale, stale, archived})
	return nil
}

func TestRecorder_Tick(t *testing.T) {
	flags := &mockAllFlagStore{
		flags: []model.Flag{
			{ProjectID: "proj-1", LifecycleStatus: model.LifecycleActive},
			{ProjectID: "proj-1", LifecycleStatus: model.LifecycleActive},
			{ProjectID: "proj-1", LifecycleStatus: model.LifecycleStale},
			{ProjectID: "proj-2", LifecycleStatus: model.LifecycleActive},
			{ProjectID: "proj-2", LifecycleStatus: model.LifecycleArchived},
		},
	}
	ss := &mockSnapshotStore{}
	r := NewRecorder(flags, ss, 24*time.Hour)

	r.tick(context.Background())

	if len(ss.recorded) != 2 {
		t.Fatalf("expected 2 snapshots (one per project), got %d", len(ss.recorded))
	}

	var proj1, proj2 *snapshot
	for i := range ss.recorded {
		switch ss.recorded[i].projectID {
		case "proj-1":
			proj1 = &ss.recorded[i]
		case "proj-2":
			proj2 = &ss.recorded[i]
		}
	}

	if proj1 == nil || proj2 == nil {
		t.Fatal("missing snapshot for one of the projects")
	}
	if proj1.active != 2 || proj1.stale != 1 {
		t.Errorf("proj-1: expected active=2 stale=1, got active=%d stale=%d", proj1.active, proj1.stale)
	}
	if proj2.active != 1 || proj2.archived != 1 {
		t.Errorf("proj-2: expected active=1 archived=1, got active=%d archived=%d", proj2.active, proj2.archived)
	}
}
