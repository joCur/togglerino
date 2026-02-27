package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/store"
)

func TestUnknownFlagStore_Upsert(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	// Create project and environment
	projKey := uniqueKey("ufupsert")
	project, err := ps.Create(ctx, projKey, "UF Upsert Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	// First upsert creates the row
	err = ufs.Upsert(ctx, project.ID, env.ID, "non-existent-flag")
	if err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	// Verify the row was created with request_count = 1
	flags, err := ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 unknown flag, got %d", len(flags))
	}
	if flags[0].FlagKey != "non-existent-flag" {
		t.Errorf("FlagKey: got %q, want %q", flags[0].FlagKey, "non-existent-flag")
	}
	if flags[0].RequestCount != 1 {
		t.Errorf("RequestCount after first upsert: got %d, want 1", flags[0].RequestCount)
	}

	// Second upsert increments count
	err = ufs.Upsert(ctx, project.ID, env.ID, "non-existent-flag")
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}

	flags, err = ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject after second upsert: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 unknown flag after second upsert, got %d", len(flags))
	}
	if flags[0].RequestCount != 2 {
		t.Errorf("RequestCount after second upsert: got %d, want 2", flags[0].RequestCount)
	}
}

func TestUnknownFlagStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("uflist")
	project, err := ps.Create(ctx, projKey, "UF List Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env1, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}

	env2, err := es.Create(ctx, project.ID, "production", "Production")
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	// Empty list initially
	flags, err := ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject empty: %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("expected 0 unknown flags initially, got %d", len(flags))
	}

	// Insert first flag (older)
	err = ufs.Upsert(ctx, project.ID, env1.ID, "flag-alpha")
	if err != nil {
		t.Fatalf("Upsert flag-alpha: %v", err)
	}

	// Small sleep to ensure last_seen_at ordering is deterministic
	time.Sleep(10 * time.Millisecond)

	// Insert second flag (newer)
	err = ufs.Upsert(ctx, project.ID, env2.ID, "flag-beta")
	if err != nil {
		t.Fatalf("Upsert flag-beta: %v", err)
	}

	flags, err = ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2 unknown flags, got %d", len(flags))
	}

	// Verify ordering: last_seen_at DESC means flag-beta should be first
	if flags[0].FlagKey != "flag-beta" {
		t.Errorf("first flag should be flag-beta (newest), got %q", flags[0].FlagKey)
	}
	if flags[1].FlagKey != "flag-alpha" {
		t.Errorf("second flag should be flag-alpha (oldest), got %q", flags[1].FlagKey)
	}

	// Verify environment key is populated
	if flags[0].EnvironmentKey != "production" {
		t.Errorf("EnvironmentKey: got %q, want %q", flags[0].EnvironmentKey, "production")
	}
	if flags[0].EnvironmentName != "Production" {
		t.Errorf("EnvironmentName: got %q, want %q", flags[0].EnvironmentName, "Production")
	}
	if flags[1].EnvironmentKey != "development" {
		t.Errorf("EnvironmentKey: got %q, want %q", flags[1].EnvironmentKey, "development")
	}

	// Verify fields are non-zero
	if flags[0].ID == "" {
		t.Error("expected non-empty ID")
	}
	if flags[0].ProjectID != project.ID {
		t.Errorf("ProjectID: got %q, want %q", flags[0].ProjectID, project.ID)
	}
	if flags[0].RequestCount != 1 {
		t.Errorf("RequestCount: got %d, want 1", flags[0].RequestCount)
	}
	if flags[0].FirstSeenAt.IsZero() {
		t.Error("expected non-zero FirstSeenAt")
	}
	if flags[0].LastSeenAt.IsZero() {
		t.Error("expected non-zero LastSeenAt")
	}
}

func TestUnknownFlagStore_Dismiss(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("ufdismiss")
	project, err := ps.Create(ctx, projKey, "UF Dismiss Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	// Create an unknown flag
	err = ufs.Upsert(ctx, project.ID, env.ID, "dismiss-me")
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	flags, err := ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}

	flagID := flags[0].ID

	// Dismiss it
	err = ufs.Dismiss(ctx, flagID, project.ID)
	if err != nil {
		t.Fatalf("Dismiss: %v", err)
	}

	// Verify it disappears from the list
	flags, err = ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject after dismiss: %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("expected 0 flags after dismiss, got %d", len(flags))
	}

	// Upsert again — should clear dismissed_at and resurface
	err = ufs.Upsert(ctx, project.ID, env.ID, "dismiss-me")
	if err != nil {
		t.Fatalf("Upsert after dismiss: %v", err)
	}

	flags, err = ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject after re-upsert: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag after re-upsert, got %d", len(flags))
	}
	if flags[0].FlagKey != "dismiss-me" {
		t.Errorf("FlagKey: got %q, want %q", flags[0].FlagKey, "dismiss-me")
	}
	// After dismiss + re-upsert, count should be incremented (original 1 + 1 = 2)
	if flags[0].RequestCount != 2 {
		t.Errorf("RequestCount after re-upsert: got %d, want 2", flags[0].RequestCount)
	}
}

func TestUnknownFlagStore_DeleteByProjectAndKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("ufdelete")
	project, err := ps.Create(ctx, projKey, "UF Delete Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env1, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}

	env2, err := es.Create(ctx, project.ID, "production", "Production")
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	// Create unknown flags across environments
	err = ufs.Upsert(ctx, project.ID, env1.ID, "delete-this")
	if err != nil {
		t.Fatalf("Upsert env1 delete-this: %v", err)
	}
	err = ufs.Upsert(ctx, project.ID, env2.ID, "delete-this")
	if err != nil {
		t.Fatalf("Upsert env2 delete-this: %v", err)
	}
	err = ufs.Upsert(ctx, project.ID, env1.ID, "keep-this")
	if err != nil {
		t.Fatalf("Upsert env1 keep-this: %v", err)
	}

	// Verify we have 3 flags
	flags, err := ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(flags) != 3 {
		t.Fatalf("expected 3 unknown flags, got %d", len(flags))
	}

	// Delete "delete-this" across all environments
	err = ufs.DeleteByProjectAndKey(ctx, project.ID, "delete-this")
	if err != nil {
		t.Fatalf("DeleteByProjectAndKey: %v", err)
	}

	// Verify only "keep-this" remains
	flags, err = ufs.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject after delete: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag after delete, got %d", len(flags))
	}
	if flags[0].FlagKey != "keep-this" {
		t.Errorf("remaining flag key: got %q, want %q", flags[0].FlagKey, "keep-this")
	}
}
