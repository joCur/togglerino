package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestLifecycleSnapshotStore_RecordAndGet(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ls := store.NewLifecycleSnapshotStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("lcsnap")
	project, err := ps.Create(ctx, projKey, "Lifecycle Snapshot Test", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	err = ls.Record(ctx, project.ID, 5, 2, 1, 3)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	trends, err := ls.GetTrends(ctx, project.ID, 30)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}

	if len(trends) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(trends))
	}

	snap := trends[0]
	if snap.ActiveCount != 5 {
		t.Errorf("ActiveCount: got %d, want 5", snap.ActiveCount)
	}
	if snap.PotentiallyStaleCount != 2 {
		t.Errorf("PotentiallyStaleCount: got %d, want 2", snap.PotentiallyStaleCount)
	}
	if snap.StaleCount != 1 {
		t.Errorf("StaleCount: got %d, want 1", snap.StaleCount)
	}
	if snap.ArchivedCount != 3 {
		t.Errorf("ArchivedCount: got %d, want 3", snap.ArchivedCount)
	}
	if snap.Date == "" {
		t.Error("expected non-empty date")
	}
}

func TestLifecycleSnapshotStore_RecordUpsert(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ls := store.NewLifecycleSnapshotStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("lcsnapupsert")
	project, err := ps.Create(ctx, projKey, "Lifecycle Upsert Test", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// First record
	err = ls.Record(ctx, project.ID, 10, 3, 2, 1)
	if err != nil {
		t.Fatalf("Record (first): %v", err)
	}

	// Second record on same day — should upsert
	err = ls.Record(ctx, project.ID, 20, 6, 4, 2)
	if err != nil {
		t.Fatalf("Record (second): %v", err)
	}

	trends, err := ls.GetTrends(ctx, project.ID, 30)
	if err != nil {
		t.Fatalf("GetTrends: %v", err)
	}

	if len(trends) != 1 {
		t.Fatalf("expected 1 snapshot after upsert, got %d", len(trends))
	}

	snap := trends[0]
	if snap.ActiveCount != 20 {
		t.Errorf("ActiveCount after upsert: got %d, want 20", snap.ActiveCount)
	}
	if snap.PotentiallyStaleCount != 6 {
		t.Errorf("PotentiallyStaleCount after upsert: got %d, want 6", snap.PotentiallyStaleCount)
	}
	if snap.StaleCount != 4 {
		t.Errorf("StaleCount after upsert: got %d, want 4", snap.StaleCount)
	}
	if snap.ArchivedCount != 2 {
		t.Errorf("ArchivedCount after upsert: got %d, want 2", snap.ArchivedCount)
	}
}
