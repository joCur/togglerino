package store_test

import (
	"context"
	"errors"
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

	// Create two environments so we can delete one (guard requires >1)
	env, err := es.Create(ctx, projectID, "to-delete", "To Delete")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := es.Create(ctx, projectID, "keeper", "Keeper"); err != nil {
		t.Fatalf("Create keeper: %v", err)
	}

	err = es.DeleteIfNotLast(ctx, env.ID, projectID)
	if err != nil {
		t.Fatalf("DeleteIfNotLast: %v", err)
	}

	// Verify it's gone
	_, err = es.FindByKey(ctx, projectID, "to-delete")
	if err == nil {
		t.Fatal("expected error after deletion, got nil")
	}
}

func TestEnvironmentStore_SortOrder(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	// CreateDefaultEnvironments should assign sequential sort_order 0, 1, 2
	err := es.CreateDefaultEnvironments(ctx, projectID)
	if err != nil {
		t.Fatalf("CreateDefaultEnvironments: %v", err)
	}

	envs, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(envs) != 3 {
		t.Fatalf("expected 3 environments, got %d", len(envs))
	}

	// Verify sort_order is sequential
	for i, env := range envs {
		if env.SortOrder != i {
			t.Errorf("envs[%d].SortOrder: got %d, want %d", i, env.SortOrder, i)
		}
	}

	// Verify ordering: development(0), staging(1), production(2)
	expectedOrder := []string{"development", "staging", "production"}
	for i, env := range envs {
		if env.Key != expectedOrder[i] {
			t.Errorf("envs[%d].Key: got %q, want %q", i, env.Key, expectedOrder[i])
		}
	}

	// Create a new environment — should get sort_order 3
	newEnv, err := es.Create(ctx, projectID, "qa", "QA")
	if err != nil {
		t.Fatalf("Create qa: %v", err)
	}
	if newEnv.SortOrder != 3 {
		t.Errorf("new env SortOrder: got %d, want 3", newEnv.SortOrder)
	}

	// Reorder: production, staging, development, qa
	err = es.UpdateOrder(ctx, projectID, []string{envs[2].ID, envs[1].ID, envs[0].ID, newEnv.ID})
	if err != nil {
		t.Fatalf("UpdateOrder: %v", err)
	}

	reordered, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject after reorder: %v", err)
	}

	expectedReorder := []string{"production", "staging", "development", "qa"}
	for i, env := range reordered {
		if env.Key != expectedReorder[i] {
			t.Errorf("reordered[%d].Key: got %q, want %q", i, env.Key, expectedReorder[i])
		}
		if env.SortOrder != i {
			t.Errorf("reordered[%d].SortOrder: got %d, want %d", i, env.SortOrder, i)
		}
	}

	// Verify FindByKey includes sort_order
	found, err := es.FindByKey(ctx, projectID, "staging")
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}
	if found.SortOrder != 1 {
		t.Errorf("FindByKey staging SortOrder: got %d, want 1", found.SortOrder)
	}

	// Verify FindByID includes sort_order
	foundByID, err := es.FindByID(ctx, found.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if foundByID.SortOrder != 1 {
		t.Errorf("FindByID staging SortOrder: got %d, want 1", foundByID.SortOrder)
	}

	// Verify UpdateOrder rejects unknown environment ID
	err = es.UpdateOrder(ctx, projectID, []string{"nonexistent-id"})
	if err == nil {
		t.Fatal("expected error for unknown environment ID, got nil")
	}
}

func TestEnvironmentStore_DeleteIfNotLast(t *testing.T) {
	pool := testPool(t)
	envStore := store.NewEnvironmentStore(pool)
	projectStore := store.NewProjectStore(pool)
	ctx := context.Background()

	project, err := projectStore.Create(ctx, uniqueKey("delete-guard"), "Delete Guard Test", "")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// Create default environments (development, staging, production)
	if err := envStore.CreateDefaultEnvironments(ctx, project.ID); err != nil {
		t.Fatalf("creating defaults: %v", err)
	}

	envs, _ := envStore.ListByProject(ctx, project.ID)
	if len(envs) != 3 {
		t.Fatalf("expected 3 envs, got %d", len(envs))
	}

	// Delete first env — should succeed
	if err := envStore.DeleteIfNotLast(ctx, envs[0].ID, project.ID); err != nil {
		t.Fatalf("expected success deleting first env: %v", err)
	}

	// Delete second env — should succeed
	if err := envStore.DeleteIfNotLast(ctx, envs[1].ID, project.ID); err != nil {
		t.Fatalf("expected success deleting second env: %v", err)
	}

	// Delete last env — should fail with ErrLastEnvironment
	err = envStore.DeleteIfNotLast(ctx, envs[2].ID, project.ID)
	if !errors.Is(err, store.ErrLastEnvironment) {
		t.Fatalf("expected ErrLastEnvironment, got: %v", err)
	}

	// Verify the last env still exists
	remaining, _ := envStore.ListByProject(ctx, project.ID)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining env, got %d", len(remaining))
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
