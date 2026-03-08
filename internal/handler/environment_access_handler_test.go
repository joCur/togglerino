package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestEnvironmentAccess_GetNoRestrictions(t *testing.T) {
	pool := testPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create test data
	admin := createTestUser(t, pool, "ea-get-"+suffix+"@test.dev", model.RoleAdmin)
	projectID := createTestProject(t, pool, "ea-get-"+suffix, "EA Get")

	envStore := store.NewEnvironmentStore(pool)
	envStore.CreateDefaultEnvironments(t.Context(), projectID)

	h := handler.NewEnvironmentAccessHandler(
		store.NewEnvironmentAccessStore(pool),
		envStore,
		store.NewProjectStore(pool),
		store.NewRoleStore(pool),
		store.NewAuditStore(pool),
	)

	// Look up the project model (needed for context)
	projectStore := store.NewProjectStore(pool)
	project, err := projectStore.FindByKey(t.Context(), "ea-get-"+suffix)
	if err != nil {
		t.Fatalf("finding project: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.Key+"/environment-access", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
		Environments []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"environments"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if len(resp.Restrictions) != 0 {
		t.Errorf("expected 0 restrictions, got %d", len(resp.Restrictions))
	}
	if len(resp.Environments) != 3 {
		t.Errorf("expected 3 environments, got %d", len(resp.Environments))
	}
}

func TestEnvironmentAccess_PutValidRestrictions(t *testing.T) {
	pool := testPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "ea-put-"+suffix+"@test.dev", model.RoleAdmin)
	projectID := createTestProject(t, pool, "ea-put-"+suffix, "EA Put")

	envStore := store.NewEnvironmentStore(pool)
	envStore.CreateDefaultEnvironments(t.Context(), projectID)

	envs, err := envStore.ListByProject(t.Context(), projectID)
	if err != nil {
		t.Fatalf("listing environments: %v", err)
	}

	projectStore := store.NewProjectStore(pool)
	project, err := projectStore.FindByKey(t.Context(), "ea-put-"+suffix)
	if err != nil {
		t.Fatalf("finding project: %v", err)
	}

	h := handler.NewEnvironmentAccessHandler(
		store.NewEnvironmentAccessStore(pool),
		envStore,
		projectStore,
		store.NewRoleStore(pool),
		store.NewAuditStore(pool),
	)

	// PUT with editor restricted to first env only
	body, _ := json.Marshal(map[string]any{
		"restrictions": []map[string]any{
			{"role_name": "editor", "environment_ids": []string{envs[0].ID}},
		},
	})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+project.Key+"/environment-access", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %q", resp["status"])
	}
}

func TestEnvironmentAccess_GetAfterPut(t *testing.T) {
	pool := testPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "ea-gap-"+suffix+"@test.dev", model.RoleAdmin)
	projectID := createTestProject(t, pool, "ea-gap-"+suffix, "EA GAP")

	envStore := store.NewEnvironmentStore(pool)
	envStore.CreateDefaultEnvironments(t.Context(), projectID)

	envs, err := envStore.ListByProject(t.Context(), projectID)
	if err != nil {
		t.Fatalf("listing environments: %v", err)
	}

	projectStore := store.NewProjectStore(pool)
	project, err := projectStore.FindByKey(t.Context(), "ea-gap-"+suffix)
	if err != nil {
		t.Fatalf("finding project: %v", err)
	}

	h := handler.NewEnvironmentAccessHandler(
		store.NewEnvironmentAccessStore(pool),
		envStore,
		projectStore,
		store.NewRoleStore(pool),
		store.NewAuditStore(pool),
	)

	// PUT restrictions: editor -> first two envs
	body, _ := json.Marshal(map[string]any{
		"restrictions": []map[string]any{
			{"role_name": "editor", "environment_ids": []string{envs[0].ID, envs[1].ID}},
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+project.Key+"/environment-access", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// GET and verify
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.Key+"/environment-access", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr = httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
		Environments []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"environments"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding GET response: %v", err)
	}

	if len(resp.Restrictions) != 1 {
		t.Fatalf("expected 1 restriction, got %d", len(resp.Restrictions))
	}
	if resp.Restrictions[0].RoleName != "editor" {
		t.Errorf("expected role_name editor, got %q", resp.Restrictions[0].RoleName)
	}
	if len(resp.Restrictions[0].EnvironmentIDs) != 2 {
		t.Errorf("expected 2 environment_ids, got %d", len(resp.Restrictions[0].EnvironmentIDs))
	}
}

func TestEnvironmentAccess_PutEmptyClearsRestrictions(t *testing.T) {
	pool := testPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "ea-clr-"+suffix+"@test.dev", model.RoleAdmin)
	projectID := createTestProject(t, pool, "ea-clr-"+suffix, "EA Clear")

	envStore := store.NewEnvironmentStore(pool)
	envStore.CreateDefaultEnvironments(t.Context(), projectID)

	envs, err := envStore.ListByProject(t.Context(), projectID)
	if err != nil {
		t.Fatalf("listing environments: %v", err)
	}

	projectStore := store.NewProjectStore(pool)
	project, err := projectStore.FindByKey(t.Context(), "ea-clr-"+suffix)
	if err != nil {
		t.Fatalf("finding project: %v", err)
	}

	h := handler.NewEnvironmentAccessHandler(
		store.NewEnvironmentAccessStore(pool),
		envStore,
		projectStore,
		store.NewRoleStore(pool),
		store.NewAuditStore(pool),
	)

	// First set a restriction
	body, _ := json.Marshal(map[string]any{
		"restrictions": []map[string]any{
			{"role_name": "viewer", "environment_ids": []string{envs[0].ID}},
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+project.Key+"/environment-access", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr := httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first PUT expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Now clear all restrictions
	body, _ = json.Marshal(map[string]any{
		"restrictions": []map[string]any{},
	})
	req = httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+project.Key+"/environment-access", bytes.NewReader(body))
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr = httptest.NewRecorder()
	h.Update(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("clear PUT expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// GET should show no restrictions
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.Key+"/environment-access", nil)
	req = req.WithContext(auth.ContextWithUser(req.Context(), admin))
	req = req.WithContext(auth.ContextWithProject(req.Context(), project))
	rr = httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(resp.Restrictions) != 0 {
		t.Errorf("expected 0 restrictions after clear, got %d", len(resp.Restrictions))
	}
}
