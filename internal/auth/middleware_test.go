package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
)

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
