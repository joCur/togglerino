package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

// createTestProject is a helper that creates a project with a unique key for use in environment tests.
func createTestProject(t *testing.T, ps *store.ProjectStore) string {
	t.Helper()
	key := uniqueKey("envproj")
	project, err := ps.Create(context.Background(), key, "Env Test Project", "project for env tests")
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	return project.ID
}

func TestEnvironmentStore_Create(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	env, err := es.Create(ctx, projectID, "staging", "Staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if env.ID == "" {
		t.Error("expected non-empty ID")
	}
	if env.ProjectID != projectID {
		t.Errorf("ProjectID: got %q, want %q", env.ProjectID, projectID)
	}
	if env.Key != "staging" {
		t.Errorf("Key: got %q, want %q", env.Key, "staging")
	}
	if env.Name != "Staging" {
		t.Errorf("Name: got %q, want %q", env.Name, "Staging")
	}
	if env.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestEnvironmentStore_Create_DuplicateKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	_, err := es.Create(ctx, projectID, "production", "Production")
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}

	_, err = es.Create(ctx, projectID, "production", "Production Again")
	if err == nil {
		t.Fatal("expected error for duplicate environment key within same project, got nil")
	}
}

func TestEnvironmentStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	_, err := es.Create(ctx, projectID, "dev", "Development")
	if err != nil {
		t.Fatalf("Create dev: %v", err)
	}
	_, err = es.Create(ctx, projectID, "staging", "Staging")
	if err != nil {
		t.Fatalf("Create staging: %v", err)
	}

	envs, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(envs))
	}

	// Verify ordering by created_at (dev first, staging second)
	if envs[0].Key != "dev" {
		t.Errorf("first env key: got %q, want %q", envs[0].Key, "dev")
	}
	if envs[1].Key != "staging" {
		t.Errorf("second env key: got %q, want %q", envs[1].Key, "staging")
	}
}

func TestEnvironmentStore_ListByProject_Empty(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	envs, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if len(envs) != 0 {
		t.Fatalf("expected 0 environments, got %d", len(envs))
	}
}

func TestEnvironmentStore_FindByKey(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	created, err := es.Create(ctx, projectID, "production", "Production")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := es.FindByKey(ctx, projectID, "production")
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.ProjectID != projectID {
		t.Errorf("ProjectID: got %q, want %q", found.ProjectID, projectID)
	}
	if found.Key != "production" {
		t.Errorf("Key: got %q, want %q", found.Key, "production")
	}
	if found.Name != "Production" {
		t.Errorf("Name: got %q, want %q", found.Name, "Production")
	}
}

func TestEnvironmentStore_FindByKey_NotFound(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	_, err := es.FindByKey(ctx, projectID, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent environment key, got nil")
	}
}

func TestEnvironmentStore_Delete(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	env, err := es.Create(ctx, projectID, "to-delete", "To Delete")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = es.Delete(ctx, env.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	_, err = es.FindByKey(ctx, projectID, "to-delete")
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}
}

func TestEnvironmentStore_CreateDefaultEnvironments(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	err := es.CreateDefaultEnvironments(ctx, projectID)
	if err != nil {
		t.Fatalf("CreateDefaultEnvironments: %v", err)
	}

	envs, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if len(envs) != 3 {
		t.Fatalf("expected 3 default environments, got %d", len(envs))
	}

	// Verify the three environments exist (ordered by created_at)
	expectedKeys := map[string]string{
		"development": "Development",
		"staging":     "Staging",
		"production":  "Production",
	}

	for _, env := range envs {
		expectedName, ok := expectedKeys[env.Key]
		if !ok {
			t.Errorf("unexpected environment key: %q", env.Key)
			continue
		}
		if env.Name != expectedName {
			t.Errorf("environment %q: name got %q, want %q", env.Key, env.Name, expectedName)
		}
		if env.ProjectID != projectID {
			t.Errorf("environment %q: project_id got %q, want %q", env.Key, env.ProjectID, projectID)
		}
		delete(expectedKeys, env.Key)
	}

	if len(expectedKeys) > 0 {
		t.Errorf("missing environments: %v", expectedKeys)
	}
}
