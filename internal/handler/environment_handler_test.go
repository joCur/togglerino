package handler_test

import (
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

func TestEnvironmentHandler_Delete(t *testing.T) {
	pool := testPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "env-del-"+suffix+"@test.dev", model.RoleAdmin)
	projectKey := "env-del-" + suffix
	projectID := createTestProject(t, pool, projectKey, "Env Delete Test")

	envStore := store.NewEnvironmentStore(pool)
	projectStore := store.NewProjectStore(pool)
	auditStore := store.NewAuditStore(pool)
	cache := evaluation.NewCache()

	h := handler.NewEnvironmentHandler(envStore, projectStore, nil, auditStore, cache)

	// Create default environments (development, staging, production)
	if err := envStore.CreateDefaultEnvironments(t.Context(), projectID); err != nil {
		t.Fatalf("creating default environments: %v", err)
	}

	// Helper: list environments via handler
	listEnvs := func() []model.Environment {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectKey+"/environments", nil)
		req.SetPathValue("key", projectKey)
		req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
		rr := httptest.NewRecorder()
		h.List(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("list environments: expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
		var envs []model.Environment
		if err := json.NewDecoder(rr.Body).Decode(&envs); err != nil {
			t.Fatalf("decoding environments list: %v", err)
		}
		return envs
	}

	// Helper: delete an environment via handler
	deleteEnv := func(envKey string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectKey+"/environments/"+envKey, nil)
		req.SetPathValue("key", projectKey)
		req.SetPathValue("envKey", envKey)
		req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
		rr := httptest.NewRecorder()
		h.Delete(rr, req)
		return rr.Code
	}

	// Step 1: Verify initial state — 3 default environments
	envs := listEnvs()
	if len(envs) != 3 {
		t.Fatalf("expected 3 default environments, got %d", len(envs))
	}

	// Step 2: Delete one environment — expects 204
	firstEnvKey := envs[0].Key
	status := deleteEnv(firstEnvKey)
	if status != http.StatusNoContent {
		t.Fatalf("delete first env: expected 204, got %d", status)
	}

	// Step 3: Verify the environment is gone — list returns 2
	envs = listEnvs()
	if len(envs) != 2 {
		t.Fatalf("after first delete: expected 2 environments, got %d", len(envs))
	}
	for _, e := range envs {
		if e.Key == firstEnvKey {
			t.Errorf("deleted environment %q should not appear in list", firstEnvKey)
		}
	}

	// Step 4: Delete another environment — expects 204
	secondEnvKey := envs[0].Key
	status = deleteEnv(secondEnvKey)
	if status != http.StatusNoContent {
		t.Fatalf("delete second env: expected 204, got %d", status)
	}

	// Verify 1 environment remains
	envs = listEnvs()
	if len(envs) != 1 {
		t.Fatalf("after second delete: expected 1 environment, got %d", len(envs))
	}

	// Step 5: Try to delete the last environment — expects 409 Conflict
	lastEnvKey := envs[0].Key
	status = deleteEnv(lastEnvKey)
	if status != http.StatusConflict {
		t.Fatalf("delete last env: expected 409 Conflict, got %d", status)
	}

	// Confirm the last environment is still present after the failed delete
	envs = listEnvs()
	if len(envs) != 1 {
		t.Fatalf("after rejected delete: expected 1 environment, got %d", len(envs))
	}
	if envs[0].Key != lastEnvKey {
		t.Errorf("expected last env key %q, got %q", lastEnvKey, envs[0].Key)
	}
}

func TestEnvironmentHandler_Delete_NotFound(t *testing.T) {
	pool := testPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "env-del-nf-"+suffix+"@test.dev", model.RoleAdmin)
	projectKey := "env-del-nf-" + suffix
	projectID := createTestProject(t, pool, projectKey, "Env Delete NotFound Test")

	envStore := store.NewEnvironmentStore(pool)
	projectStore := store.NewProjectStore(pool)
	auditStore := store.NewAuditStore(pool)
	cache := evaluation.NewCache()

	h := handler.NewEnvironmentHandler(envStore, projectStore, nil, auditStore, cache)

	if err := envStore.CreateDefaultEnvironments(t.Context(), projectID); err != nil {
		t.Fatalf("creating default environments: %v", err)
	}

	// Attempt to delete a non-existent environment key
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectKey+"/environments/nonexistent", nil)
	req.SetPathValue("key", projectKey)
	req.SetPathValue("envKey", "nonexistent")
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("delete nonexistent env: expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
