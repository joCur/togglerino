package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
)

// testRoleCache returns a RoleCache pre-loaded with the built-in roles.
func testRoleCache() *auth.RoleCache {
	rc := auth.NewRoleCache()
	rc.Load([]model.RoleDefinition{
		{Name: "admin", Permissions: []string{
			"flags:read", "flags:write", "environments:read", "environments:write",
			"sdk_keys:manage", "segments:write", "templates:manage", "project:settings",
		}},
		{Name: "editor", Permissions: []string{
			"flags:read", "flags:write", "environments:read", "environments:write",
			"sdk_keys:manage", "segments:write", "templates:manage",
		}},
		{Name: "viewer", Permissions: []string{
			"flags:read", "environments:read",
		}},
	})
	return rc
}

// ---------------------------------------------------------------------------
// Original RBAC tests from main
// ---------------------------------------------------------------------------

func TestRequireOrgPermission_AdminAllowed(t *testing.T) {
	user := &model.User{ID: "u1", Email: "admin@test.com", Role: model.RoleAdmin}

	handler := auth.RequireOrgPermission(model.PermOrgUsersManage)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireOrgPermission_MemberDenied(t *testing.T) {
	user := &model.User{ID: "u2", Email: "member@test.com", Role: model.RoleMember}

	handler := auth.RequireOrgPermission(model.PermOrgUsersManage)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireProjectPermission_AdminBypasses(t *testing.T) {
	user := &model.User{ID: "u1", Email: "admin@test.com", Role: model.RoleAdmin}

	resolverCalled := false
	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		resolverCalled = true
		return model.ProjectRoleViewer, nil
	}

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver, testRoleCache())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/projects/my-project/flags", nil)
	req.SetPathValue("key", "my-project")
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if resolverCalled {
		t.Fatal("resolver should NOT be called for admin users")
	}
}

func TestRequireProjectPermission_EditorAllowedWrite(t *testing.T) {
	user := &model.User{ID: "u2", Email: "member@test.com", Role: model.RoleMember}

	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return model.ProjectRoleEditor, nil
	}

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver, testRoleCache())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/projects/my-project/flags", nil)
	req.SetPathValue("key", "my-project")
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRequireProjectPermission_ViewerDeniedWrite(t *testing.T) {
	user := &model.User{ID: "u3", Email: "viewer@test.com", Role: model.RoleMember}

	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return model.ProjectRoleViewer, nil
	}

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver, testRoleCache())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/projects/my-project/flags", nil)
	req.SetPathValue("key", "my-project")
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRequireProjectPermission_NoAccess(t *testing.T) {
	user := &model.User{ID: "u4", Email: "noaccess@test.com", Role: model.RoleMember}

	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return "", fmt.Errorf("no access to project %q", projectKey)
	}

	handler := auth.RequireProjectPermission(model.PermFlagsRead, resolver, testRoleCache())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/projects/my-project/flags", nil)
	req.SetPathValue("key", "my-project")
	req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Environment-scoped permission tests (new on this branch)
// ---------------------------------------------------------------------------

func TestCheckEnvironmentAccess_Unrestricted(t *testing.T) {
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envKey string) (bool, error) {
			return true, nil
		},
	)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag/environments/production", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleMember})
	ctx = auth.ContextWithProject(ctx, &model.Project{ID: "p1"})
	ctx = auth.ContextWithResolvedRole(ctx, "editor")
	r = r.WithContext(ctx)
	r.SetPathValue("env", "production")

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestCheckEnvironmentAccess_Blocked(t *testing.T) {
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envKey string) (bool, error) {
			return false, nil
		},
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag/environments/production", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleMember})
	ctx = auth.ContextWithProject(ctx, &model.Project{ID: "p1"})
	ctx = auth.ContextWithResolvedRole(ctx, "editor")
	r = r.WithContext(ctx)
	r.SetPathValue("env", "production")

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCheckEnvironmentAccess_OrgAdminBypasses(t *testing.T) {
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envKey string) (bool, error) {
			t.Fatal("should not be called for org admin")
			return false, nil
		},
	)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag/environments/production", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleAdmin})
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if !called {
		t.Fatal("expected handler to be called for org admin")
	}
}

func TestCheckEnvironmentAccess_NoEnvInPath(t *testing.T) {
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envKey string) (bool, error) {
			t.Fatal("should not be called when no env in path")
			return false, nil
		},
	)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleMember})
	ctx = auth.ContextWithProject(ctx, &model.Project{ID: "p1"})
	ctx = auth.ContextWithResolvedRole(ctx, "editor")
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if !called {
		t.Fatal("expected handler to be called when no env in path")
	}
}

func TestResolvedRoleFromContext(t *testing.T) {
	ctx := context.Background()
	if got := auth.ResolvedRoleFromContext(ctx); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}

	ctx = auth.ContextWithResolvedRole(ctx, "editor")
	if got := auth.ResolvedRoleFromContext(ctx); got != "editor" {
		t.Fatalf("expected editor, got %q", got)
	}
}
