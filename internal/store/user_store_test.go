package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@test.togglerino.dev", prefix, time.Now().UnixNano())
}

func TestUserStore_Create(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	email := uniqueEmail("create")
	user, err := us.Create(ctx, email, "hashedpw123", model.RoleMember)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
	if user.Email != email {
		t.Errorf("email: got %q, want %q", user.Email, email)
	}
	if user.PasswordHash != "hashedpw123" {
		t.Errorf("password_hash: got %q, want %q", user.PasswordHash, "hashedpw123")
	}
	if user.Role != model.RoleMember {
		t.Errorf("role: got %q, want %q", user.Role, model.RoleMember)
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if user.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
}

func TestUserStore_FindByEmail(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	email := uniqueEmail("findbyemail")
	created, err := us.Create(ctx, email, "hash456", model.RoleAdmin)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := us.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.Email != email {
		t.Errorf("Email: got %q, want %q", found.Email, email)
	}
	if found.Role != model.RoleAdmin {
		t.Errorf("Role: got %q, want %q", found.Role, model.RoleAdmin)
	}
}

func TestUserStore_FindByID(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	email := uniqueEmail("findbyid")
	created, err := us.Create(ctx, email, "hash789", model.RoleMember)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := us.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.Email != email {
		t.Errorf("Email: got %q, want %q", found.Email, email)
	}
}

func TestUserStore_Count(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	count, err := us.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count < 0 {
		t.Errorf("count: got %d, want >= 0", count)
	}

	// Create a user and verify count increases
	email := uniqueEmail("count")
	_, err = us.Create(ctx, email, "hashcount", model.RoleMember)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newCount, err := us.Count(ctx)
	if err != nil {
		t.Fatalf("Count after insert: %v", err)
	}
	if newCount <= count {
		t.Errorf("count did not increase: before=%d, after=%d", count, newCount)
	}
}

func TestUserStore_UpdateProfile(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	email := uniqueEmail("updateprofile")
	user, err := us.Create(ctx, email, "hash123", model.RoleMember)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update display name only
	displayName := "Test User"
	updated, err := us.UpdateProfile(ctx, user.ID, nil, &displayName)
	if err != nil {
		t.Fatalf("UpdateProfile (display_name): %v", err)
	}
	if updated.DisplayName == nil || *updated.DisplayName != "Test User" {
		t.Errorf("display_name: got %v, want %q", updated.DisplayName, "Test User")
	}
	if updated.Email != email {
		t.Errorf("email should not change: got %q, want %q", updated.Email, email)
	}

	// Update email only
	newEmail := uniqueEmail("updateprofile-new")
	updated, err = us.UpdateProfile(ctx, user.ID, &newEmail, nil)
	if err != nil {
		t.Fatalf("UpdateProfile (email): %v", err)
	}
	if updated.Email != newEmail {
		t.Errorf("email: got %q, want %q", updated.Email, newEmail)
	}
	if updated.DisplayName == nil || *updated.DisplayName != "Test User" {
		t.Errorf("display_name should not change: got %v, want %q", updated.DisplayName, "Test User")
	}

	// Update both
	finalEmail := uniqueEmail("updateprofile-final")
	finalName := "Final Name"
	updated, err = us.UpdateProfile(ctx, user.ID, &finalEmail, &finalName)
	if err != nil {
		t.Fatalf("UpdateProfile (both): %v", err)
	}
	if updated.Email != finalEmail {
		t.Errorf("email: got %q, want %q", updated.Email, finalEmail)
	}
	if updated.DisplayName == nil || *updated.DisplayName != "Final Name" {
		t.Errorf("display_name: got %v, want %q", updated.DisplayName, "Final Name")
	}
}
