package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestProjectSettingsStore_GetNonExistent(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ss := store.NewProjectSettingsStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("settingsnone")
	project, err := ps.Create(ctx, projKey, "No Settings", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	settings, err := ss.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if settings != nil {
		t.Error("expected nil settings for project with no settings")
	}
}

func TestProjectSettingsStore_Upsert(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ss := store.NewProjectSettingsStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("settingsupsert")
	project, err := ps.Create(ctx, projKey, "Upsert Settings", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	days30 := 30
	lifetimes := map[model.FlagType]*int{
		model.FlagTypeRelease: &days30,
	}

	settings, err := ss.Upsert(ctx, project.ID, lifetimes)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if settings.ProjectID != project.ID {
		t.Errorf("ProjectID: got %q, want %q", settings.ProjectID, project.ID)
	}
	if settings.FlagLifetimes == nil {
		t.Fatal("expected non-nil FlagLifetimes")
	}
	if *settings.FlagLifetimes[model.FlagTypeRelease] != 30 {
		t.Errorf("release lifetime: got %d, want 30", *settings.FlagLifetimes[model.FlagTypeRelease])
	}

	// Upsert again to update
	days20 := 20
	lifetimes[model.FlagTypeRelease] = &days20

	updated, err := ss.Upsert(ctx, project.ID, lifetimes)
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	if *updated.FlagLifetimes[model.FlagTypeRelease] != 20 {
		t.Errorf("release lifetime after update: got %d, want 20", *updated.FlagLifetimes[model.FlagTypeRelease])
	}

	// Read back
	readBack, err := ss.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("Get after upsert: %v", err)
	}
	if *readBack.FlagLifetimes[model.FlagTypeRelease] != 20 {
		t.Errorf("release lifetime after read: got %d, want 20", *readBack.FlagLifetimes[model.FlagTypeRelease])
	}
}

func TestProjectSettingsStore_GetAll(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ss := store.NewProjectSettingsStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("settingsall")
	project, err := ps.Create(ctx, projKey, "All Settings", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	days10 := 10
	_, err = ss.Upsert(ctx, project.ID, map[model.FlagType]*int{
		model.FlagTypeOperational: &days10,
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	all, err := ss.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}

	if all[project.ID] == nil {
		t.Fatal("expected settings for project")
	}
	if *all[project.ID].FlagLifetimes[model.FlagTypeOperational] != 10 {
		t.Errorf("operational lifetime: got %d, want 10", *all[project.ID].FlagLifetimes[model.FlagTypeOperational])
	}
}
