package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func createTestFlagForOverrides(t *testing.T, pool *pgxpool.Pool, projectID string) string {
	t.Helper()
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	// Ensure at least one environment exists
	envs, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("listing environments: %v", err)
	}
	if len(envs) == 0 {
		_, err = es.Create(ctx, projectID, "development", "Development")
		if err != nil {
			t.Fatalf("creating environment: %v", err)
		}
	}

	key := uniqueKey("testflag")
	flag, err := fs.Create(ctx, projectID, key, "Test Flag", "test", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating test flag: %v", err)
	}
	return flag.ID
}

func getTestEnvironmentID(t *testing.T, pool *pgxpool.Pool, projectID string) string {
	t.Helper()
	es := store.NewEnvironmentStore(pool)
	envs, err := es.ListByProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("listing environments: %v", err)
	}
	if len(envs) == 0 {
		t.Fatal("no environments found for project")
	}
	return envs[0].ID
}

func TestOverrideStore_SetAndGet(t *testing.T) {
	pool := testPool(t)
	s := store.NewOverrideStore(pool)
	ctx := context.Background()

	userID := createTestUserForOverrides(t, pool)
	projectID := createTestProjectForOverrides(t, pool)
	flagID := createTestFlagForOverrides(t, pool, projectID)
	envID := getTestEnvironmentID(t, pool, projectID)

	value := json.RawMessage(`true`)
	expiresAt := time.Now().Add(24 * time.Hour)

	override, err := s.Set(ctx, userID, flagID, envID, value, &expiresAt)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if override.FlagID != flagID {
		t.Fatalf("expected flag ID %s, got %s", flagID, override.FlagID)
	}

	got, err := s.Get(ctx, userID, flagID, envID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got.Value) != `true` {
		t.Fatalf("expected true, got %s", got.Value)
	}
}

func TestOverrideStore_Delete(t *testing.T) {
	pool := testPool(t)
	s := store.NewOverrideStore(pool)
	ctx := context.Background()

	userID := createTestUserForOverrides(t, pool)
	projectID := createTestProjectForOverrides(t, pool)
	flagID := createTestFlagForOverrides(t, pool, projectID)
	envID := getTestEnvironmentID(t, pool, projectID)

	value := json.RawMessage(`true`)
	_, err := s.Set(ctx, userID, flagID, envID, value, nil)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	err = s.Delete(ctx, userID, flagID, envID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = s.Get(ctx, userID, flagID, envID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
}

func TestOverrideStore_ListByUser(t *testing.T) {
	pool := testPool(t)
	s := store.NewOverrideStore(pool)
	ctx := context.Background()

	userID := createTestUserForOverrides(t, pool)
	projectID := createTestProjectForOverrides(t, pool)
	flagID := createTestFlagForOverrides(t, pool, projectID)
	envID := getTestEnvironmentID(t, pool, projectID)

	value := json.RawMessage(`"on"`)
	_, err := s.Set(ctx, userID, flagID, envID, value, nil)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	overrides, err := s.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(overrides))
	}
}

func TestOverrideStore_DeleteExpired(t *testing.T) {
	pool := testPool(t)
	s := store.NewOverrideStore(pool)
	ctx := context.Background()

	userID := createTestUserForOverrides(t, pool)
	projectID := createTestProjectForOverrides(t, pool)
	flagID := createTestFlagForOverrides(t, pool, projectID)
	envID := getTestEnvironmentID(t, pool, projectID)

	// Create an already-expired override
	pastTime := time.Now().Add(-1 * time.Hour)
	value := json.RawMessage(`true`)
	_, err := s.Set(ctx, userID, flagID, envID, value, &pastTime)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	count, err := s.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 deleted, got %d", count)
	}
}
