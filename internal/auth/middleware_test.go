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

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := auth.RequireProjectPermission(model.PermFlagsRead, resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
