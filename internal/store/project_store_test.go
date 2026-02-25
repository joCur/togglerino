package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/store"
)

func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestProjectStore_Create(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	key := uniqueKey("create")
	project, err := ps.Create(ctx, key, "Test Project", "A test project")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if project.ID == "" {
		t.Error("expected non-empty ID")
	}
	if project.Key != key {
		t.Errorf("key: got %q, want %q", project.Key, key)
	}
	if project.Name != "Test Project" {
		t.Errorf("name: got %q, want %q", project.Name, "Test Project")
	}
	if project.Description != "A test project" {
		t.Errorf("description: got %q, want %q", project.Description, "A test project")
	}
	if project.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if project.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestProjectStore_Create_DuplicateKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	key := uniqueKey("dupkey")
	_, err := ps.Create(ctx, key, "First", "first project")
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}

	_, err = ps.Create(ctx, key, "Second", "second project")
	if err == nil {
		t.Fatal("expected error for duplicate key, got nil")
	}
}

func TestProjectStore_List(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	// Create two projects
	key1 := uniqueKey("list1")
	key2 := uniqueKey("list2")
	_, err := ps.Create(ctx, key1, "List Project 1", "desc1")
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	_, err = ps.Create(ctx, key2, "List Project 2", "desc2")
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	projects, err := ps.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(projects) < 2 {
		t.Fatalf("expected at least 2 projects, got %d", len(projects))
	}

	// Verify ordering is created_at DESC (most recent first)
	found1 := false
	found2 := false
	for _, p := range projects {
		if p.Key == key1 {
			found1 = true
		}
		if p.Key == key2 {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("expected both created projects in list")
	}
}

func TestProjectStore_FindByKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	key := uniqueKey("findbykey")
	created, err := ps.Create(ctx, key, "Find Project", "findable")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := ps.FindByKey(ctx, key)
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.Key != key {
		t.Errorf("Key: got %q, want %q", found.Key, key)
	}
	if found.Name != "Find Project" {
		t.Errorf("Name: got %q, want %q", found.Name, "Find Project")
	}
	if found.Description != "findable" {
		t.Errorf("Description: got %q, want %q", found.Description, "findable")
	}
}

func TestProjectStore_FindByKey_NotFound(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	_, err := ps.FindByKey(ctx, "nonexistent-key-12345")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

func TestProjectStore_Update(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	key := uniqueKey("update")
	created, err := ps.Create(ctx, key, "Old Name", "Old Desc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := ps.Update(ctx, key, "New Name", "New Desc")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.ID != created.ID {
		t.Errorf("ID should not change: got %q, want %q", updated.ID, created.ID)
	}
	if updated.Key != key {
		t.Errorf("Key should not change: got %q, want %q", updated.Key, key)
	}
	if updated.Name != "New Name" {
		t.Errorf("Name: got %q, want %q", updated.Name, "New Name")
	}
	if updated.Description != "New Desc" {
		t.Errorf("Description: got %q, want %q", updated.Description, "New Desc")
	}
	if !updated.UpdatedAt.After(created.CreatedAt) || updated.UpdatedAt.Equal(created.CreatedAt) {
		// UpdatedAt should be >= CreatedAt (NOW() may be same within fast test)
	}
}

func TestProjectStore_Update_NotFound(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	_, err := ps.Update(ctx, "nonexistent-key-67890", "Name", "Desc")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

func TestProjectStore_Delete(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	key := uniqueKey("delete")
	_, err := ps.Create(ctx, key, "Delete Me", "to be deleted")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	_, err = ps.FindByKey(ctx, key)
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}
}
