package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestOrgSettingsStore_GetBaseProjectRole_Default(t *testing.T) {
	pool := testPool(t)
	s := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	role, err := s.GetBaseProjectRole(ctx)
	if err != nil {
		t.Fatalf("GetBaseProjectRole: %v", err)
	}
	if role != "editor" {
		t.Errorf("default base project role: got %q, want %q", role, "editor")
	}
}

func TestOrgSettingsStore_SetAndGet(t *testing.T) {
	pool := testPool(t)
	s := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	// Set to viewer
	if err := s.SetBaseProjectRole(ctx, "viewer"); err != nil {
		t.Fatalf("SetBaseProjectRole(viewer): %v", err)
	}
	role, err := s.GetBaseProjectRole(ctx)
	if err != nil {
		t.Fatalf("GetBaseProjectRole after set viewer: %v", err)
	}
	if role != "viewer" {
		t.Errorf("base project role: got %q, want %q", role, "viewer")
	}

	// Set to none
	if err := s.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("SetBaseProjectRole(none): %v", err)
	}
	role, err = s.GetBaseProjectRole(ctx)
	if err != nil {
		t.Fatalf("GetBaseProjectRole after set none: %v", err)
	}
	if role != "none" {
		t.Errorf("base project role: got %q, want %q", role, "none")
	}

	// Reset to editor to avoid polluting other tests
	if err := s.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("SetBaseProjectRole(editor): %v", err)
	}
}
