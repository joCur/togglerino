package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func createTestUser(t *testing.T, us *store.UserStore) *model.User {
	t.Helper()
	ctx := context.Background()
	email := uniqueKey("pat") + "@test.com"
	user, err := us.Create(ctx, email, "hashedpw", model.RoleAdmin)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return user
}

func TestPATStore_Create(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	pat, err := ps.Create(ctx, user.ID, "My Token", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if pat.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(pat.Token, "pat_") {
		t.Errorf("Token should start with 'pat_', got %q", pat.Token)
	}
	if len(pat.Token) != 44 {
		t.Errorf("Token length: got %d, want 44", len(pat.Token))
	}
	if pat.Name != "My Token" {
		t.Errorf("Name: got %q, want %q", pat.Name, "My Token")
	}
	if pat.TokenPrefix != pat.Token[:12] {
		t.Errorf("TokenPrefix: got %q, want %q", pat.TokenPrefix, pat.Token[:12])
	}
	if pat.ExpiresAt != nil {
		t.Error("expected nil ExpiresAt")
	}
}

func TestPATStore_FindByHash(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	created, err := ps.Create(ctx, user.ID, "Findable", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	h := sha256.Sum256([]byte(created.Token))
	hash := hex.EncodeToString(h[:])

	found, err := ps.FindByHash(ctx, hash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.UserID != user.ID {
		t.Errorf("UserID: got %q, want %q", found.UserID, user.ID)
	}
}

func TestPATStore_FindByHash_NotFound(t *testing.T) {
	pool := testPool(t)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	h := sha256.Sum256([]byte("pat_nonexistent0000000000000000000000000000"))
	hash := hex.EncodeToString(h[:])

	_, err := ps.FindByHash(ctx, hash)
	if err == nil {
		t.Fatal("expected error for non-existent token, got nil")
	}
}

func TestPATStore_ListByUser(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	_, err := ps.Create(ctx, user.ID, "Token 1", nil)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	_, err = ps.Create(ctx, user.ID, "Token 2", nil)
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	tokens, err := ps.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Name != "Token 2" {
		t.Errorf("first token: got %q, want %q", tokens[0].Name, "Token 2")
	}
}

func TestPATStore_Delete(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	created, err := ps.Create(ctx, user.ID, "To Delete", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.Delete(ctx, created.ID, user.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	tokens, err := ps.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after delete, got %d", len(tokens))
	}
}

func TestPATStore_Delete_WrongUser(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user1 := createTestUser(t, us)
	user2 := createTestUser(t, us)

	created, err := ps.Create(ctx, user1.ID, "User1 Token", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.Delete(ctx, created.ID, user2.ID)
	if err == nil {
		t.Fatal("expected error when deleting another user's token")
	}
}

func TestPATStore_UpdateLastUsed(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	created, err := ps.Create(ctx, user.ID, "Track Usage", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.UpdateLastUsed(ctx, created.ID)
	if err != nil {
		t.Fatalf("UpdateLastUsed: %v", err)
	}

	tokens, err := ps.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].LastUsedAt == nil {
		t.Error("expected LastUsedAt to be set after UpdateLastUsed")
	}
}

func TestPATStore_Create_WithExpiry(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	expiry := time.Now().Add(24 * time.Hour)
	pat, err := ps.Create(ctx, user.ID, "Expiring Token", &expiry)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if pat.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
}
