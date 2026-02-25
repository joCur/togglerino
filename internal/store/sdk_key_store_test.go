package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

// createTestEnvironment is a helper that creates a project and environment for SDK key tests.
func createTestEnvironment(t *testing.T, ps *store.ProjectStore, es *store.EnvironmentStore) (projectID, environmentID string) {
	t.Helper()
	ctx := context.Background()

	key := uniqueKey("sdkproj")
	project, err := ps.Create(ctx, key, "SDK Key Test Project", "project for sdk key tests")
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating test environment: %v", err)
	}

	return project.ID, env.ID
}

func TestSDKKeyStore_Create(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, envID := createTestEnvironment(t, ps, es)

	sdkKey, err := ks.Create(ctx, envID, "My API Key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if sdkKey.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(sdkKey.Key, "sdk_") {
		t.Errorf("Key should start with 'sdk_', got %q", sdkKey.Key)
	}
	if len(sdkKey.Key) != 36 { // "sdk_" (4) + 32 hex chars
		t.Errorf("Key length: got %d, want 36", len(sdkKey.Key))
	}
	if sdkKey.EnvironmentID != envID {
		t.Errorf("EnvironmentID: got %q, want %q", sdkKey.EnvironmentID, envID)
	}
	if sdkKey.Name != "My API Key" {
		t.Errorf("Name: got %q, want %q", sdkKey.Name, "My API Key")
	}
	if sdkKey.Revoked {
		t.Error("expected Revoked to be false")
	}
	if sdkKey.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestSDKKeyStore_ListByEnvironment(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, envID := createTestEnvironment(t, ps, es)

	_, err := ks.Create(ctx, envID, "Key One")
	if err != nil {
		t.Fatalf("Create key 1: %v", err)
	}
	_, err = ks.Create(ctx, envID, "Key Two")
	if err != nil {
		t.Fatalf("Create key 2: %v", err)
	}

	keys, err := ks.ListByEnvironment(ctx, envID)
	if err != nil {
		t.Fatalf("ListByEnvironment: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 SDK keys, got %d", len(keys))
	}

	// Verify ordering by created_at DESC (most recent first)
	if keys[0].Name != "Key Two" {
		t.Errorf("first key name: got %q, want %q", keys[0].Name, "Key Two")
	}
	if keys[1].Name != "Key One" {
		t.Errorf("second key name: got %q, want %q", keys[1].Name, "Key One")
	}
}

func TestSDKKeyStore_ListByEnvironment_Empty(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, envID := createTestEnvironment(t, ps, es)

	keys, err := ks.ListByEnvironment(ctx, envID)
	if err != nil {
		t.Fatalf("ListByEnvironment: %v", err)
	}

	if len(keys) != 0 {
		t.Fatalf("expected 0 SDK keys, got %d", len(keys))
	}
}

func TestSDKKeyStore_FindByKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, envID := createTestEnvironment(t, ps, es)

	created, err := ks.Create(ctx, envID, "Findable Key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := ks.FindByKey(ctx, created.Key)
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.Key != created.Key {
		t.Errorf("Key: got %q, want %q", found.Key, created.Key)
	}
	if found.EnvironmentID != envID {
		t.Errorf("EnvironmentID: got %q, want %q", found.EnvironmentID, envID)
	}
	if found.Name != "Findable Key" {
		t.Errorf("Name: got %q, want %q", found.Name, "Findable Key")
	}
	if found.ProjectKey == "" {
		t.Error("ProjectKey: expected non-empty value")
	}
	if found.EnvironmentKey != "development" {
		t.Errorf("EnvironmentKey: got %q, want %q", found.EnvironmentKey, "development")
	}
}

func TestSDKKeyStore_FindByKey_NotFound(t *testing.T) {
	pool := testPool(t)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, err := ks.FindByKey(ctx, "sdk_nonexistent000000000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent key, got nil")
	}
}

func TestSDKKeyStore_FindByKey_Revoked(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, envID := createTestEnvironment(t, ps, es)

	created, err := ks.Create(ctx, envID, "Soon Revoked Key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ks.Revoke(ctx, created.ID)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Finding a revoked key should fail
	_, err = ks.FindByKey(ctx, created.Key)
	if err == nil {
		t.Fatal("expected error when finding revoked key, got nil")
	}
}

func TestSDKKeyStore_Revoke(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ks := store.NewSDKKeyStore(pool)
	ctx := context.Background()

	_, envID := createTestEnvironment(t, ps, es)

	created, err := ks.Create(ctx, envID, "To Revoke")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ks.Revoke(ctx, created.ID)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Verify the key shows as revoked in the list
	keys, err := ks.ListByEnvironment(ctx, envID)
	if err != nil {
		t.Fatalf("ListByEnvironment: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 SDK key, got %d", len(keys))
	}
	if !keys[0].Revoked {
		t.Error("expected key to be revoked")
	}
}
