package auth_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestBuildRoleResolver_ExplicitMembership(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	members := store.NewProjectMemberStore(pool)
	projects := store.NewProjectStore(pool)
	orgSettings := store.NewOrgSettingsStore(pool)
	users := store.NewUserStore(pool)

	// Create a user and project.
	email := uniqueEmail("explicit")
	user, err := users.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	projectKey := uniqueProjectKey("explicit")
	project, err := projects.Create(ctx, projectKey, "Explicit Project", "")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// Add explicit membership as viewer.
	_, err = members.Add(ctx, project.ID, user.ID, model.ProjectRoleViewer)
	if err != nil {
		t.Fatalf("adding member: %v", err)
	}

	resolver := auth.BuildRoleResolver(members, projects, orgSettings)
	role, err := resolver(ctx, projectKey, user.ID)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	if role != model.ProjectRoleViewer {
		t.Fatalf("expected viewer, got %s", role)
	}
}

func TestBuildRoleResolver_FallbackToBaseRole(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	members := store.NewProjectMemberStore(pool)
	projects := store.NewProjectStore(pool)
	orgSettings := store.NewOrgSettingsStore(pool)
	users := store.NewUserStore(pool)

	// Save and restore base role to avoid cross-test interference.
	origRole, _ := orgSettings.GetBaseProjectRole(ctx)
	if err := orgSettings.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("setting base role: %v", err)
	}
	t.Cleanup(func() {
		if origRole != "" {
			_ = orgSettings.SetBaseProjectRole(ctx, origRole)
		}
	})

	email := uniqueEmail("fallback")
	user, err := users.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	projectKey := uniqueProjectKey("fallback")
	_, err = projects.Create(ctx, projectKey, "Fallback Project", "")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// No explicit membership — should fall back to base role.
	resolver := auth.BuildRoleResolver(members, projects, orgSettings)
	role, err := resolver(ctx, projectKey, user.ID)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	if role != model.ProjectRoleEditor {
		t.Fatalf("expected editor, got %s", role)
	}
}

func TestBuildRoleResolver_BaseRoleNone_NoAccess(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	members := store.NewProjectMemberStore(pool)
	projects := store.NewProjectStore(pool)
	orgSettings := store.NewOrgSettingsStore(pool)
	users := store.NewUserStore(pool)

	// Save and restore base role to avoid cross-test interference.
	origRole, _ := orgSettings.GetBaseProjectRole(ctx)
	if err := orgSettings.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("setting base role: %v", err)
	}
	t.Cleanup(func() {
		if origRole != "" {
			_ = orgSettings.SetBaseProjectRole(ctx, origRole)
		}
	})

	email := uniqueEmail("noaccess")
	user, err := users.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	projectKey := uniqueProjectKey("noaccess")
	_, err = projects.Create(ctx, projectKey, "NoAccess Project", "")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// No explicit membership, base role is "none" — should return error.
	resolver := auth.BuildRoleResolver(members, projects, orgSettings)
	_, err = resolver(ctx, projectKey, user.ID)
	if err == nil {
		t.Fatal("expected error for no access, got nil")
	}
}
