package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

type mockLifecycleCounter struct {
	rows []model.LifecycleCountRow
}

func (m *mockLifecycleCounter) LifecycleCountsByProject(_ context.Context) ([]model.LifecycleCountRow, error) {
	return m.rows, nil
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
	counter := &mockLifecycleCounter{
		rows: []model.LifecycleCountRow{
			{ProjectID: "proj-1", Status: "active", Count: 2},
			{ProjectID: "proj-1", Status: "stale", Count: 1},
			{ProjectID: "proj-2", Status: "active", Count: 1},
			{ProjectID: "proj-2", Status: "archived", Count: 1},
		},
	}
	ss := &mockSnapshotStore{}
	r := NewRecorder(counter, ss, 24*time.Hour)

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

func TestRecorder_Tick_ZeroFlagProject(t *testing.T) {
	counter := &mockLifecycleCounter{
		rows: []model.LifecycleCountRow{
			{ProjectID: "proj-empty", Status: "active", Count: 0},
		},
	}
	ss := &mockSnapshotStore{}
	r := NewRecorder(counter, ss, 24*time.Hour)

	r.tick(context.Background())

	if len(ss.recorded) != 1 {
		t.Fatalf("expected 1 snapshot for zero-flag project, got %d", len(ss.recorded))
	}
	if ss.recorded[0].active != 0 {
		t.Errorf("expected active=0, got %d", ss.recorded[0].active)
	}
}
