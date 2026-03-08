package store_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestEnvironmentAccessStore_NoRestrictions(t *testing.T) {
	pool := testPool(t)
	s := store.NewEnvironmentAccessStore(pool)
	ctx := context.Background()

	// Create a project
	email := uniqueEmail("ea-none")
	var projectID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"ea-none-"+strings.ReplaceAll(email, "@", "-"), "EA None Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	// No rows inserted — should return empty slice
	envIDs, err := s.ListByProjectAndRole(ctx, projectID, "editor")
	if err != nil {
		t.Fatalf("ListByProjectAndRole: %v", err)
	}
	if len(envIDs) != 0 {
		t.Errorf("expected empty slice, got %v", envIDs)
	}
}

func TestEnvironmentAccessStore_ReplaceForProject(t *testing.T) {
	pool := testPool(t)
	s := store.NewEnvironmentAccessStore(pool)
	ctx := context.Background()

	// Create a project and environments
	email := uniqueEmail("ea-replace")
	var projectID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"ea-rep-"+strings.ReplaceAll(email, "@", "-"), "EA Replace Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	var envID1, envID2, envID3 string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'dev', 'Development', 0) RETURNING id`,
		projectID,
	).Scan(&envID1)
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'staging', 'Staging', 1) RETURNING id`,
		projectID,
	).Scan(&envID2)
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'prod', 'Production', 2) RETURNING id`,
		projectID,
	).Scan(&envID3)
	if err != nil {
		t.Fatalf("creating env3: %v", err)
	}

	// Replace: editor can only access dev and staging
	restrictions := []store.EnvironmentAccessRestriction{
		{RoleName: "editor", EnvironmentIDs: []string{envID1, envID2}},
	}
	err = s.ReplaceForProject(ctx, projectID, restrictions)
	if err != nil {
		t.Fatalf("ReplaceForProject: %v", err)
	}

	// Verify editor restrictions
	envIDs, err := s.ListByProjectAndRole(ctx, projectID, "editor")
	if err != nil {
		t.Fatalf("ListByProjectAndRole(editor): %v", err)
	}
	sort.Strings(envIDs)
	expected := []string{envID1, envID2}
	sort.Strings(expected)
	if len(envIDs) != len(expected) {
		t.Fatalf("editor envIDs: got %d, want %d", len(envIDs), len(expected))
	}
	for i := range expected {
		if envIDs[i] != expected[i] {
			t.Errorf("editor envIDs[%d]: got %q, want %q", i, envIDs[i], expected[i])
		}
	}

	// Verify viewer is unaffected (unrestricted)
	viewerIDs, err := s.ListByProjectAndRole(ctx, projectID, "viewer")
	if err != nil {
		t.Fatalf("ListByProjectAndRole(viewer): %v", err)
	}
	if len(viewerIDs) != 0 {
		t.Errorf("viewer should be unrestricted (empty), got %v", viewerIDs)
	}

	// Replace again: editor now only has prod
	restrictions2 := []store.EnvironmentAccessRestriction{
		{RoleName: "editor", EnvironmentIDs: []string{envID3}},
	}
	err = s.ReplaceForProject(ctx, projectID, restrictions2)
	if err != nil {
		t.Fatalf("ReplaceForProject (update): %v", err)
	}

	envIDs2, err := s.ListByProjectAndRole(ctx, projectID, "editor")
	if err != nil {
		t.Fatalf("ListByProjectAndRole after update: %v", err)
	}
	if len(envIDs2) != 1 || envIDs2[0] != envID3 {
		t.Errorf("after update: got %v, want [%s]", envIDs2, envID3)
	}

	// Replace with empty restrictions — clears all
	err = s.ReplaceForProject(ctx, projectID, nil)
	if err != nil {
		t.Fatalf("ReplaceForProject (clear): %v", err)
	}
	envIDs3, err := s.ListByProjectAndRole(ctx, projectID, "editor")
	if err != nil {
		t.Fatalf("ListByProjectAndRole after clear: %v", err)
	}
	if len(envIDs3) != 0 {
		t.Errorf("after clear: got %v, want empty", envIDs3)
	}
}

func TestEnvironmentAccessStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	s := store.NewEnvironmentAccessStore(pool)
	ctx := context.Background()

	email := uniqueEmail("ea-list")
	var projectID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"ea-list-"+strings.ReplaceAll(email, "@", "-"), "EA List Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	var envID1, envID2 string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'dev', 'Development', 0) RETURNING id`,
		projectID,
	).Scan(&envID1)
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'prod', 'Production', 1) RETURNING id`,
		projectID,
	).Scan(&envID2)
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	// Set restrictions for two roles
	restrictions := []store.EnvironmentAccessRestriction{
		{RoleName: "editor", EnvironmentIDs: []string{envID1}},
		{RoleName: "viewer", EnvironmentIDs: []string{envID1, envID2}},
	}
	err = s.ReplaceForProject(ctx, projectID, restrictions)
	if err != nil {
		t.Fatalf("ReplaceForProject: %v", err)
	}

	all, err := s.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListByProject: got %d restrictions, want 2", len(all))
	}

	// Build a map for easier assertion
	byRole := make(map[string][]string)
	for _, r := range all {
		sort.Strings(r.EnvironmentIDs)
		byRole[r.RoleName] = r.EnvironmentIDs
	}

	editorEnvs := byRole["editor"]
	if len(editorEnvs) != 1 || editorEnvs[0] != envID1 {
		t.Errorf("editor envs: got %v, want [%s]", editorEnvs, envID1)
	}

	viewerEnvs := byRole["viewer"]
	expectedViewer := []string{envID1, envID2}
	sort.Strings(expectedViewer)
	if len(viewerEnvs) != 2 {
		t.Fatalf("viewer envs count: got %d, want 2", len(viewerEnvs))
	}
	for i := range expectedViewer {
		if viewerEnvs[i] != expectedViewer[i] {
			t.Errorf("viewer envs[%d]: got %q, want %q", i, viewerEnvs[i], expectedViewer[i])
		}
	}
}

func TestEnvironmentAccessStore_HasAccessByEnvKey(t *testing.T) {
	pool := testPool(t)
	s := store.NewEnvironmentAccessStore(pool)
	ctx := context.Background()

	email := uniqueEmail("ea-access")
	var projectID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"ea-acc-"+strings.ReplaceAll(email, "@", "-"), "EA Access Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}

	var envID1, envID2 string
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'dev', 'Development', 0) RETURNING id`,
		projectID,
	).Scan(&envID1)
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, 'prod', 'Production', 1) RETURNING id`,
		projectID,
	).Scan(&envID2)
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	// No restrictions — unrestricted access
	hasAccess, err := s.HasAccessByEnvKey(ctx, projectID, "editor", "dev")
	if err != nil {
		t.Fatalf("HasAccessByEnvKey (unrestricted): %v", err)
	}
	if !hasAccess {
		t.Error("expected true for unrestricted role")
	}

	// Also test HasAccess by ID when unrestricted
	hasAccessID, err := s.HasAccess(ctx, projectID, "editor", envID1)
	if err != nil {
		t.Fatalf("HasAccess (unrestricted): %v", err)
	}
	if !hasAccessID {
		t.Error("expected true for unrestricted role (by ID)")
	}

	// Add restriction: editor can only access dev
	restrictions := []store.EnvironmentAccessRestriction{
		{RoleName: "editor", EnvironmentIDs: []string{envID1}},
	}
	err = s.ReplaceForProject(ctx, projectID, restrictions)
	if err != nil {
		t.Fatalf("ReplaceForProject: %v", err)
	}

	// editor + dev → true (in allow-list)
	hasAccess, err = s.HasAccessByEnvKey(ctx, projectID, "editor", "dev")
	if err != nil {
		t.Fatalf("HasAccessByEnvKey (allowed): %v", err)
	}
	if !hasAccess {
		t.Error("expected true for allowed env")
	}

	// editor + prod → false (not in allow-list)
	hasAccess, err = s.HasAccessByEnvKey(ctx, projectID, "editor", "prod")
	if err != nil {
		t.Fatalf("HasAccessByEnvKey (denied): %v", err)
	}
	if hasAccess {
		t.Error("expected false for denied env")
	}

	// HasAccess by ID: editor + envID1 → true
	hasAccessID, err = s.HasAccess(ctx, projectID, "editor", envID1)
	if err != nil {
		t.Fatalf("HasAccess (allowed by ID): %v", err)
	}
	if !hasAccessID {
		t.Error("expected true for allowed env by ID")
	}

	// HasAccess by ID: editor + envID2 → false
	hasAccessID, err = s.HasAccess(ctx, projectID, "editor", envID2)
	if err != nil {
		t.Fatalf("HasAccess (denied by ID): %v", err)
	}
	if hasAccessID {
		t.Error("expected false for denied env by ID")
	}

	// viewer is unrestricted (no rows for viewer)
	hasAccess, err = s.HasAccessByEnvKey(ctx, projectID, "viewer", "prod")
	if err != nil {
		t.Fatalf("HasAccessByEnvKey (viewer unrestricted): %v", err)
	}
	if !hasAccess {
		t.Error("expected true for unrestricted viewer")
	}
}
