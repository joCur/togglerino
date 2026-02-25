package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestSessionStore_CreateAndFind(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ss := store.NewSessionStore(pool)
	ctx := context.Background()

	// Create a user first (sessions require a valid user_id)
	email := uniqueEmail("session-create")
	user, err := us.Create(ctx, email, "hashsess", model.RoleMember)
	if err != nil {
		t.Fatalf("Create user: %v", err)
	}

	// Create a session with 1 hour duration
	session, err := ss.Create(ctx, user.ID, 1*time.Hour)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if len(session.ID) != 64 {
		t.Errorf("session ID length: got %d, want 64 (32 bytes hex)", len(session.ID))
	}
	if session.UserID != user.ID {
		t.Errorf("UserID: got %q, want %q", session.UserID, user.ID)
	}
	if session.ExpiresAt.Before(time.Now()) {
		t.Error("expected ExpiresAt to be in the future")
	}
	if session.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// Find the session by ID
	found, err := ss.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID != session.ID {
		t.Errorf("ID: got %q, want %q", found.ID, session.ID)
	}
	if found.UserID != user.ID {
		t.Errorf("UserID: got %q, want %q", found.UserID, user.ID)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ss := store.NewSessionStore(pool)
	ctx := context.Background()

	// Create a user and session
	email := uniqueEmail("session-delete")
	user, err := us.Create(ctx, email, "hashdel", model.RoleMember)
	if err != nil {
		t.Fatalf("Create user: %v", err)
	}

	session, err := ss.Create(ctx, user.ID, 1*time.Hour)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	// Delete the session
	if err := ss.Delete(ctx, session.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify FindByID fails after deletion
	_, err = ss.FindByID(ctx, session.ID)
	if err == nil {
		t.Fatal("expected error finding deleted session, got nil")
	}
	if !strings.Contains(err.Error(), "finding session") {
		t.Errorf("unexpected error message: %v", err)
	}
}
