package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestContextAttributeStore_UpsertAndList(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	// Create a project
	key := uniqueKey("ctx-attr")
	project, err := ps.Create(ctx, key, "Context Attr Project", "for context attr tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Upsert some attributes
	err = cas.UpsertByProjectKey(ctx, key, []string{"country", "plan", "email"})
	if err != nil {
		t.Fatalf("UpsertByProjectKey: %v", err)
	}

	// List by project
	attrs, err := cas.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if len(attrs) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(attrs))
	}

	// Verify alphabetical order
	expected := []string{"country", "email", "plan"}
	for i, a := range attrs {
		if a.Name != expected[i] {
			t.Errorf("attr[%d]: expected %q, got %q", i, expected[i], a.Name)
		}
		if a.ID == "" {
			t.Error("expected non-empty ID")
		}
		if a.LastSeenAt.IsZero() {
			t.Error("expected non-zero LastSeenAt")
		}
	}
}

func TestContextAttributeStore_UpsertUpdatesLastSeen(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	key := uniqueKey("ctx-upd")
	project, err := ps.Create(ctx, key, "Update Project", "for update tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// First upsert
	err = cas.UpsertByProjectKey(ctx, key, []string{"country"})
	if err != nil {
		t.Fatalf("First UpsertByProjectKey: %v", err)
	}

	// Second upsert — "country" should be deduplicated, "plan" added
	err = cas.UpsertByProjectKey(ctx, key, []string{"country", "plan"})
	if err != nil {
		t.Fatalf("Second UpsertByProjectKey: %v", err)
	}

	// Verify conflict resolution: should have exactly 2 attributes
	attrs, err := cas.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attributes after upsert, got %d", len(attrs))
	}
}

func TestContextAttributeStore_ListByProject_Empty(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	key := uniqueKey("ctx-empty")
	project, err := ps.Create(ctx, key, "Empty Attr Project", "no attributes")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	attrs, err := cas.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if attrs != nil {
		t.Errorf("expected nil for empty result, got %d attributes", len(attrs))
	}
}

func TestContextAttributeStore_UpsertEmptySlice(t *testing.T) {
	pool := testPool(t)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	// Upsert with empty slice should not error
	err := cas.UpsertByProjectKey(ctx, "nonexistent-key", []string{})
	if err != nil {
		t.Fatalf("UpsertByProjectKey with empty slice: %v", err)
	}
}
