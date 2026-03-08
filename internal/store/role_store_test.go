package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestRoleStore_List(t *testing.T) {
	pool := testPool(t)
	s := store.NewRoleStore(pool)
	ctx := context.Background()

	roles, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(roles) < 3 {
		t.Fatalf("expected at least 3 built-in roles, got %d", len(roles))
	}

	// Built-in roles should come first (is_built_in DESC).
	for i, r := range roles[:3] {
		if !r.IsBuiltIn {
			t.Errorf("roles[%d] %q: expected built-in", i, r.Name)
		}
	}

	// Verify the 3 built-in names exist.
	names := map[string]bool{}
	for _, r := range roles {
		names[r.Name] = true
	}
	for _, want := range []string{"admin", "editor", "viewer"} {
		if !names[want] {
			t.Errorf("missing built-in role %q", want)
		}
	}
}

func TestRoleStore_CreateAndGetByName(t *testing.T) {
	pool := testPool(t)
	s := store.NewRoleStore(pool)
	ctx := context.Background()

	name := "test-create-role"
	// Cleanup in case a previous run left data.
	defer pool.Exec(ctx, `DELETE FROM roles WHERE name = $1`, name)

	created, err := s.Create(ctx, name, "A test role", []string{"flags:read", "flags:write"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != name {
		t.Errorf("name = %q, want %q", created.Name, name)
	}
	if created.IsBuiltIn {
		t.Error("expected is_built_in = false")
	}
	if len(created.Permissions) != 2 {
		t.Errorf("permissions len = %d, want 2", len(created.Permissions))
	}

	got, err := s.GetByName(ctx, name)
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: got %q, want %q", got.ID, created.ID)
	}
	if got.Description != "A test role" {
		t.Errorf("description = %q, want %q", got.Description, "A test role")
	}
}

func TestRoleStore_Update(t *testing.T) {
	pool := testPool(t)
	s := store.NewRoleStore(pool)
	ctx := context.Background()

	name := "test-update-role"
	defer pool.Exec(ctx, `DELETE FROM roles WHERE name = $1`, name)

	_, err := s.Create(ctx, name, "Original", []string{"flags:read"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := s.Update(ctx, name, "Updated description", []string{"flags:read", "flags:write", "segments:write"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "Updated description" {
		t.Errorf("description = %q, want %q", updated.Description, "Updated description")
	}
	if len(updated.Permissions) != 3 {
		t.Errorf("permissions len = %d, want 3", len(updated.Permissions))
	}

	// Updating a built-in role should fail.
	_, err = s.Update(ctx, "admin", "Hacked", []string{"flags:read"})
	if err == nil {
		t.Fatal("expected error updating built-in role")
	}
	if !errors.Is(err, store.ErrBuiltInRole) {
		t.Errorf("expected ErrBuiltInRole, got: %v", err)
	}
}

func TestRoleStore_Delete(t *testing.T) {
	pool := testPool(t)
	s := store.NewRoleStore(pool)
	ctx := context.Background()

	name := "test-delete-role"
	defer pool.Exec(ctx, `DELETE FROM roles WHERE name = $1`, name)

	_, err := s.Create(ctx, name, "To be deleted", []string{"flags:read"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Deleting a built-in role should fail.
	err = s.Delete(ctx, "admin")
	if err == nil {
		t.Fatal("expected error deleting built-in role")
	}

	// Delete the custom role.
	err = s.Delete(ctx, name)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone.
	exists, err := s.Exists(ctx, name)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("role still exists after delete")
	}

	// Deleting a non-existent role should return ErrNotFound.
	err = s.Delete(ctx, "nonexistent-role-xyz")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestRoleStore_Exists(t *testing.T) {
	pool := testPool(t)
	s := store.NewRoleStore(pool)
	ctx := context.Background()

	// Built-in role should exist.
	exists, err := s.Exists(ctx, "admin")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("expected admin role to exist")
	}

	// Non-existent role.
	exists, err = s.Exists(ctx, "nonexistent-role-xyz")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected nonexistent role to not exist")
	}
}
