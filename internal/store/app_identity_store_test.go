package store_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func createTestUserForOverrides(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	us := store.NewUserStore(pool)
	email := uniqueEmail("override")
	user, err := us.Create(context.Background(), email, "hashedpw", model.RoleMember)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return user.ID
}

func createTestUserWithEmailForOverrides(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	us := store.NewUserStore(pool)
	user, err := us.Create(context.Background(), email, "hashedpw", model.RoleMember)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return user.ID
}

func createTestProjectForOverrides(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ps := store.NewProjectStore(pool)
	key := uniqueKey("overrideproj")
	project, err := ps.Create(context.Background(), key, "Override Test Project", "project for override tests")
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	return project.ID
}

func TestAppIdentityStore_SetAndGet(t *testing.T) {
	pool := testPool(t)
	s := store.NewAppIdentityStore(pool)
	ctx := context.Background()

	userID := createTestUserForOverrides(t, pool)
	projectID := createTestProjectForOverrides(t, pool)

	// Set identity
	identity, err := s.Set(ctx, userID, projectID, "app-user-42")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if identity.AppUserID != "app-user-42" {
		t.Fatalf("expected app-user-42, got %s", identity.AppUserID)
	}

	// Get identity
	got, err := s.Get(ctx, userID, projectID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AppUserID != "app-user-42" {
		t.Fatalf("expected app-user-42, got %s", got.AppUserID)
	}

	// Update (upsert)
	updated, err := s.Set(ctx, userID, projectID, "app-user-99")
	if err != nil {
		t.Fatalf("Set update: %v", err)
	}
	if updated.AppUserID != "app-user-99" {
		t.Fatalf("expected app-user-99, got %s", updated.AppUserID)
	}
}

func TestAppIdentityStore_Delete(t *testing.T) {
	pool := testPool(t)
	s := store.NewAppIdentityStore(pool)
	ctx := context.Background()

	userID := createTestUserForOverrides(t, pool)
	projectID := createTestProjectForOverrides(t, pool)

	_, err := s.Set(ctx, userID, projectID, "app-user-42")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	err = s.Delete(ctx, userID, projectID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(ctx, userID, projectID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestAppIdentityStore_UniqueAppUserPerProject(t *testing.T) {
	pool := testPool(t)
	s := store.NewAppIdentityStore(pool)
	ctx := context.Background()

	projectID := createTestProjectForOverrides(t, pool)
	user1 := createTestUserForOverrides(t, pool)
	user2 := createTestUserWithEmailForOverrides(t, pool, uniqueEmail("user2"))

	_, err := s.Set(ctx, user1, projectID, "same-app-user")
	if err != nil {
		t.Fatalf("Set user1: %v", err)
	}

	_, err = s.Set(ctx, user2, projectID, "same-app-user")
	if err == nil {
		t.Fatal("expected uniqueness error, got nil")
	}
}
