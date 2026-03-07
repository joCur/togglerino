package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func setupOverrideTest(t *testing.T) (
	h *handler.OverrideHandler,
	user *model.User,
	projectKey, flagKey string,
	cache *evaluation.Cache,
) {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create user
	user = createTestUser(t, pool, "override-"+suffix+"@test.dev", model.RoleMember)

	// Create project
	projectKey = "ovr-proj-" + suffix
	ps := store.NewProjectStore(pool)
	project, err := ps.Create(ctx, projectKey, "Override Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// Create environment
	es := store.NewEnvironmentStore(pool)
	_, err = es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating environment: %v", err)
	}

	// Create flag
	flagKey = "test-flag-" + suffix
	fs := store.NewFlagStore(pool)
	_, err = fs.Create(ctx, project.ID, flagKey, "Test Flag", "test", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating flag: %v", err)
	}

	cache = evaluation.NewCache()
	h = handler.NewOverrideHandler(
		store.NewOverrideStore(pool),
		store.NewAppIdentityStore(pool),
		ps,
		fs,
		es,
		cache,
	)

	return h, user, projectKey, flagKey, cache
}

func overrideRequest(t *testing.T, method, path string, user *model.User, body any) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != nil {
		req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	}
	return req
}

func TestOverrideHandler_SetAndGetAppIdentity(t *testing.T) {
	h, user, projectKey, _, _ := setupOverrideTest(t)

	// Set app identity
	req := overrideRequest(t, http.MethodPut, "/api/v1/projects/"+projectKey+"/app-identity", user, map[string]string{"app_user_id": "my-app-id"})
	req.SetPathValue("key", projectKey)
	rr := httptest.NewRecorder()
	h.SetAppIdentity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SetAppIdentity: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var identity model.AppIdentity
	if err := json.NewDecoder(rr.Body).Decode(&identity); err != nil {
		t.Fatalf("decoding identity: %v", err)
	}
	if identity.AppUserID != "my-app-id" {
		t.Fatalf("expected my-app-id, got %s", identity.AppUserID)
	}

	// Get app identity
	req = overrideRequest(t, http.MethodGet, "/api/v1/projects/"+projectKey+"/app-identity", user, nil)
	req.SetPathValue("key", projectKey)
	rr = httptest.NewRecorder()
	h.GetAppIdentity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GetAppIdentity: expected 200, got %d", rr.Code)
	}
}

func TestOverrideHandler_SetOverride(t *testing.T) {
	h, user, projectKey, flagKey, cache := setupOverrideTest(t)

	// First set app identity
	req := overrideRequest(t, http.MethodPut, "/api/v1/projects/"+projectKey+"/app-identity", user, map[string]string{"app_user_id": "sdk-user-1"})
	req.SetPathValue("key", projectKey)
	rr := httptest.NewRecorder()
	h.SetAppIdentity(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("SetAppIdentity: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Set override
	req = overrideRequest(t, http.MethodPut,
		"/api/v1/projects/"+projectKey+"/flags/"+flagKey+"/environments/development/override",
		user,
		map[string]any{"value": true, "duration": "24h"},
	)
	req.SetPathValue("key", projectKey)
	req.SetPathValue("flag", flagKey)
	req.SetPathValue("env", "development")
	rr = httptest.NewRecorder()
	h.SetOverride(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SetOverride: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify cache was updated
	val, ok := cache.GetOverride(projectKey, "development", "sdk-user-1", flagKey)
	if !ok {
		t.Fatal("expected override in cache")
	}
	if string(val) != "true" {
		t.Fatalf("expected true in cache, got %s", string(val))
	}
}

func TestOverrideHandler_SetOverride_NoIdentity(t *testing.T) {
	h, user, projectKey, flagKey, _ := setupOverrideTest(t)

	// Try to set override without app identity
	req := overrideRequest(t, http.MethodPut,
		"/api/v1/projects/"+projectKey+"/flags/"+flagKey+"/environments/development/override",
		user,
		map[string]any{"value": true},
	)
	req.SetPathValue("key", projectKey)
	req.SetPathValue("flag", flagKey)
	req.SetPathValue("env", "development")
	rr := httptest.NewRecorder()
	h.SetOverride(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestOverrideHandler_DeleteOverride(t *testing.T) {
	h, user, projectKey, flagKey, _ := setupOverrideTest(t)

	// Set app identity
	req := overrideRequest(t, http.MethodPut, "/api/v1/projects/"+projectKey+"/app-identity", user, map[string]string{"app_user_id": "del-user"})
	req.SetPathValue("key", projectKey)
	rr := httptest.NewRecorder()
	h.SetAppIdentity(rr, req)

	// Set override
	req = overrideRequest(t, http.MethodPut,
		"/api/v1/projects/"+projectKey+"/flags/"+flagKey+"/environments/development/override",
		user,
		map[string]any{"value": true},
	)
	req.SetPathValue("key", projectKey)
	req.SetPathValue("flag", flagKey)
	req.SetPathValue("env", "development")
	rr = httptest.NewRecorder()
	h.SetOverride(rr, req)

	// Delete override
	req = overrideRequest(t, http.MethodDelete,
		"/api/v1/projects/"+projectKey+"/flags/"+flagKey+"/environments/development/override",
		user, nil,
	)
	req.SetPathValue("key", projectKey)
	req.SetPathValue("flag", flagKey)
	req.SetPathValue("env", "development")
	rr = httptest.NewRecorder()
	h.DeleteOverride(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("DeleteOverride: expected 204, got %d", rr.Code)
	}
}

func TestOverrideHandler_ListMyOverrides(t *testing.T) {
	h, user, projectKey, flagKey, _ := setupOverrideTest(t)

	// Set app identity
	req := overrideRequest(t, http.MethodPut, "/api/v1/projects/"+projectKey+"/app-identity", user, map[string]string{"app_user_id": "list-user"})
	req.SetPathValue("key", projectKey)
	rr := httptest.NewRecorder()
	h.SetAppIdentity(rr, req)

	// Set override
	req = overrideRequest(t, http.MethodPut,
		"/api/v1/projects/"+projectKey+"/flags/"+flagKey+"/environments/development/override",
		user,
		map[string]any{"value": true},
	)
	req.SetPathValue("key", projectKey)
	req.SetPathValue("flag", flagKey)
	req.SetPathValue("env", "development")
	rr = httptest.NewRecorder()
	h.SetOverride(rr, req)

	// List overrides
	req = overrideRequest(t, http.MethodGet, "/api/v1/overrides/me", user, nil)
	rr = httptest.NewRecorder()
	h.ListMyOverrides(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ListMyOverrides: expected 200, got %d", rr.Code)
	}

	var overrides []model.FlagOverride
	if err := json.NewDecoder(rr.Body).Decode(&overrides); err != nil {
		t.Fatalf("decoding overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("expected 1 override, got %d", len(overrides))
	}
}
