package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestProjectMemberStore_AddAndGetRole(t *testing.T) {
	pool := testPool(t)
	s := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	var projectID, userID string
	email := uniqueEmail("pm-add")
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"pm-add-"+strings.ReplaceAll(email, "@", "-"), "PM Add Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'member') RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}

	member, err := s.Add(ctx, projectID, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if member.ProjectID != projectID {
		t.Errorf("ProjectID: got %q, want %q", member.ProjectID, projectID)
	}
	if member.UserID != userID {
		t.Errorf("UserID: got %q, want %q", member.UserID, userID)
	}
	if member.Role != model.ProjectRoleEditor {
		t.Errorf("Role: got %q, want %q", member.Role, model.ProjectRoleEditor)
	}
	if member.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// GetRole
	role, err := s.GetRole(ctx, projectID, userID)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	if role != model.ProjectRoleEditor {
		t.Errorf("GetRole: got %q, want %q", role, model.ProjectRoleEditor)
	}
}

func TestProjectMemberStore_GetRole_NotFound(t *testing.T) {
	pool := testPool(t)
	s := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	_, err := s.GetRole(ctx, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected error for non-existent membership, got nil")
	}
}

func TestProjectMemberStore_Update(t *testing.T) {
	pool := testPool(t)
	s := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	email := uniqueEmail("pm-update")
	var projectID, userID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"pm-upd-"+strings.ReplaceAll(email, "@", "-"), "PM Update Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'member') RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}

	_, err = s.Add(ctx, projectID, userID, model.ProjectRoleViewer)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := s.Update(ctx, projectID, userID, model.ProjectRoleAdmin)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Role != model.ProjectRoleAdmin {
		t.Errorf("Role after update: got %q, want %q", updated.Role, model.ProjectRoleAdmin)
	}

	// Verify via GetRole
	role, err := s.GetRole(ctx, projectID, userID)
	if err != nil {
		t.Fatalf("GetRole after update: %v", err)
	}
	if role != model.ProjectRoleAdmin {
		t.Errorf("GetRole after update: got %q, want %q", role, model.ProjectRoleAdmin)
	}
}

func TestProjectMemberStore_Remove(t *testing.T) {
	pool := testPool(t)
	s := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	email := uniqueEmail("pm-remove")
	var projectID, userID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"pm-rm-"+strings.ReplaceAll(email, "@", "-"), "PM Remove Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'member') RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}

	_, err = s.Add(ctx, projectID, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	err = s.Remove(ctx, projectID, userID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify membership is gone
	_, err = s.GetRole(ctx, projectID, userID)
	if err == nil {
		t.Fatal("expected error after remove, got nil")
	}

	// Removing again should return not found
	err = s.Remove(ctx, projectID, userID)
	if err == nil {
		t.Fatal("expected error on second remove, got nil")
	}
}

func TestProjectMemberStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	s := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	email1 := uniqueEmail("pm-list1")
	email2 := uniqueEmail("pm-list2")
	var projectID, userID1, userID2 string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"pm-list-"+strings.ReplaceAll(email1, "@", "-"), "PM List Test",
	).Scan(&projectID)
	if err != nil {
		t.Fatalf("creating test project: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'member') RETURNING id`,
		email1,
	).Scan(&userID1)
	if err != nil {
		t.Fatalf("creating test user 1: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'member') RETURNING id`,
		email2,
	).Scan(&userID2)
	if err != nil {
		t.Fatalf("creating test user 2: %v", err)
	}

	_, err = s.Add(ctx, projectID, userID1, model.ProjectRoleAdmin)
	if err != nil {
		t.Fatalf("Add user1: %v", err)
	}
	_, err = s.Add(ctx, projectID, userID2, model.ProjectRoleViewer)
	if err != nil {
		t.Fatalf("Add user2: %v", err)
	}

	members, err := s.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("ListByProject: got %d members, want 2", len(members))
	}

	// Verify first member has correct email and role
	found := false
	for _, m := range members {
		if m.UserID == userID1 {
			found = true
			if m.Email != email1 {
				t.Errorf("member1 email: got %q, want %q", m.Email, email1)
			}
			if m.Role != model.ProjectRoleAdmin {
				t.Errorf("member1 role: got %q, want %q", m.Role, model.ProjectRoleAdmin)
			}
		}
	}
	if !found {
		t.Error("user1 not found in ListByProject results")
	}
}

func TestProjectMemberStore_ListAccessibleProjectIDs(t *testing.T) {
	pool := testPool(t)
	s := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	email := uniqueEmail("pm-accessible")
	var projectID1, projectID2, userID string
	err := pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"pm-acc1-"+strings.ReplaceAll(email, "@", "-"), "PM Accessible Test 1",
	).Scan(&projectID1)
	if err != nil {
		t.Fatalf("creating test project 1: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		"pm-acc2-"+strings.ReplaceAll(email, "@", "-"), "PM Accessible Test 2",
	).Scan(&projectID2)
	if err != nil {
		t.Fatalf("creating test project 2: %v", err)
	}
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', 'member') RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}

	_, err = s.Add(ctx, projectID1, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add project1: %v", err)
	}
	_, err = s.Add(ctx, projectID2, userID, model.ProjectRoleViewer)
	if err != nil {
		t.Fatalf("Add project2: %v", err)
	}

	ids, err := s.ListAccessibleProjectIDs(ctx, userID)
	if err != nil {
		t.Fatalf("ListAccessibleProjectIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ListAccessibleProjectIDs: got %d IDs, want 2", len(ids))
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet[projectID1] {
		t.Errorf("project1 ID %q not in accessible IDs", projectID1)
	}
	if !idSet[projectID2] {
		t.Errorf("project2 ID %q not in accessible IDs", projectID2)
	}
}
