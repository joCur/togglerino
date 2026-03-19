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
	"github.com/togglerino/togglerino/internal/stream"
)

func uniqueKey(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func testFlagLockSetup(t *testing.T) (*http.ServeMux, *store.FlagStore, *store.ProjectStore, *store.EnvironmentStore, *model.User) {
	t.Helper()
	pool := flagTestPool(t)

	flagStore := store.NewFlagStore(pool)
	projectStore := store.NewProjectStore(pool)
	envStore := store.NewEnvironmentStore(pool)
	auditStore := store.NewAuditStore(pool)

	user := createTestUser(t, pool, uniqueEmail("lockhandler"), model.RoleAdmin)

	hub := stream.NewHub()
	cache := evaluation.NewCache()
	pss := store.NewProjectSettingsStore(pool)

	flagHandler := handler.NewFlagHandler(flagStore, projectStore, envStore, auditStore, hub, cache, pool, nil, nil, pss, nil)

	mux := http.NewServeMux()
	withUser := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(auth.ContextWithUser(r.Context(), user))
			h(w, r)
		}
	}

	mux.HandleFunc("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/lock", withUser(flagHandler.LockEnvironmentConfig))
	mux.HandleFunc("DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/lock", withUser(flagHandler.UnlockEnvironmentConfig))
	mux.HandleFunc("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}", withUser(flagHandler.UpdateEnvironmentConfig))
	mux.HandleFunc("PUT /api/v1/projects/{key}/flags/{flag}/archive", withUser(flagHandler.Archive))
	mux.HandleFunc("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote", withUser(flagHandler.PromoteEnvironmentConfig))
	mux.HandleFunc("POST /api/v1/projects/{key}/flags/bulk-lock", withUser(flagHandler.BulkLockFlags))
	mux.HandleFunc("POST /api/v1/projects/{key}/flags/bulk-unlock", withUser(flagHandler.BulkUnlockFlags))

	return mux, flagStore, projectStore, envStore, user
}

// createTestFlag creates a project with environments and a flag for lock testing.
func createTestFlag(t *testing.T, ctx context.Context, ps *store.ProjectStore, es *store.EnvironmentStore, fs *store.FlagStore, prefix string) (*model.Project, *model.Environment, *model.Flag) {
	t.Helper()
	projKey := uniqueKey(prefix)
	project, err := ps.Create(ctx, projKey, prefix+" Project", "test")
	if err != nil {
		t.Fatal(err)
	}
	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatal(err)
	}
	flag, err := fs.Create(ctx, project.ID, uniqueKey(prefix+"flag"), prefix+" Flag", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return project, env, flag
}

func TestLockEnvironmentConfig(t *testing.T) {
	mux, _, ps, es, _ := testFlagLockSetup(t)
	ctx := context.Background()
	fs := store.NewFlagStore(flagTestPool(t))

	project, _, flag := createTestFlag(t, ctx, ps, es, fs, "lockh")

	body, _ := json.Marshal(map[string]string{"reason": "Holiday freeze"})
	req := httptest.NewRequest("POST", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/development/lock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.FlagEnvironmentConfig
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Locked {
		t.Error("expected locked=true in response")
	}
	if resp.LockReason == nil || *resp.LockReason != "Holiday freeze" {
		t.Error("expected lock_reason to match")
	}
}

func TestLockEnvironmentConfig_AlreadyLocked(t *testing.T) {
	mux, flagStore, ps, es, user := testFlagLockSetup(t)
	ctx := context.Background()

	project, env, flag := createTestFlag(t, ctx, ps, es, flagStore, "locktwice")

	// Lock first
	reason := "first"
	flagStore.LockEnvironmentConfig(ctx, flag.ID, env.ID, user.ID, &reason)

	body, _ := json.Marshal(map[string]string{"reason": "second"})
	req := httptest.NewRequest("POST", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/development/lock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLockEnvironmentConfig_ReasonTooLong(t *testing.T) {
	mux, _, ps, es, _ := testFlagLockSetup(t)
	ctx := context.Background()
	fs := store.NewFlagStore(flagTestPool(t))

	project, _, flag := createTestFlag(t, ctx, ps, es, fs, "longrsn")

	longReason := make([]byte, 256)
	for i := range longReason {
		longReason[i] = 'a'
	}
	body, _ := json.Marshal(map[string]string{"reason": string(longReason)})
	req := httptest.NewRequest("POST", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/development/lock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnlockEnvironmentConfig(t *testing.T) {
	mux, flagStore, ps, es, user := testFlagLockSetup(t)
	ctx := context.Background()

	project, env, flag := createTestFlag(t, ctx, ps, es, flagStore, "unlockh")

	// Lock first
	reason := "test"
	flagStore.LockEnvironmentConfig(ctx, flag.ID, env.ID, user.ID, &reason)

	req := httptest.NewRequest("DELETE", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/development/lock", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.FlagEnvironmentConfig
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Locked {
		t.Error("expected locked=false in response")
	}
}

func TestUnlockEnvironmentConfig_NotLocked(t *testing.T) {
	mux, _, ps, es, _ := testFlagLockSetup(t)
	ctx := context.Background()
	fs := store.NewFlagStore(flagTestPool(t))

	project, _, flag := createTestFlag(t, ctx, ps, es, fs, "notlockd")

	req := httptest.NewRequest("DELETE", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/development/lock", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEnvironmentConfig_Locked(t *testing.T) {
	mux, flagStore, ps, es, user := testFlagLockSetup(t)
	ctx := context.Background()

	project, env, flag := createTestFlag(t, ctx, ps, es, flagStore, "updlocked")

	// Lock it
	reason := "frozen"
	flagStore.LockEnvironmentConfig(ctx, flag.ID, env.ID, user.ID, &reason)

	body, _ := json.Marshal(map[string]any{
		"enabled":              true,
		"fallthrough_variant":  "true",
		"off_variant":          "false",
		"variants":             []map[string]any{{"name": "true", "value": true}, {"name": "false", "value": false}},
		"targeting_rules":      []any{},
	})
	req := httptest.NewRequest("PUT", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/development", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestArchiveFlag_Locked(t *testing.T) {
	mux, flagStore, ps, es, user := testFlagLockSetup(t)
	ctx := context.Background()

	project, env, flag := createTestFlag(t, ctx, ps, es, flagStore, "archlockd")

	// Lock in one env
	reason := "frozen"
	flagStore.LockEnvironmentConfig(ctx, flag.ID, env.ID, user.ID, &reason)

	body, _ := json.Marshal(map[string]bool{"archived": true})
	req := httptest.NewRequest("PUT", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/archive", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPromoteEnvironmentConfig_TargetLocked(t *testing.T) {
	mux, flagStore, ps, es, user := testFlagLockSetup(t)
	ctx := context.Background()

	projKey := uniqueKey("promlockd")
	project, err := ps.Create(ctx, projKey, "Promote Locked Test", "test")
	if err != nil {
		t.Fatal(err)
	}

	// Create two environments with proper sort order
	devEnv, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatal(err)
	}
	stagingEnv, err := es.Create(ctx, project.ID, "staging", "Staging")
	if err != nil {
		t.Fatal(err)
	}

	flag, err := flagStore.Create(ctx, project.ID, uniqueKey("plflag"), "Promote Locked Flag", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Lock the target env (staging)
	reason := "frozen"
	flagStore.LockEnvironmentConfig(ctx, flag.ID, stagingEnv.ID, user.ID, &reason)
	_ = devEnv // source env exists

	body, _ := json.Marshal(map[string]string{"source_environment_key": "development"})
	req := httptest.NewRequest("POST", "/api/v1/projects/"+project.Key+"/flags/"+flag.Key+"/environments/staging/promote", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestBulkLockFlags(t *testing.T) {
	mux, flagStore, ps, es, _ := testFlagLockSetup(t)
	ctx := context.Background()

	projKey := uniqueKey("bulklock")
	project, err := ps.Create(ctx, projKey, "Bulk Lock Test", "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatal(err)
	}

	flag1, _ := flagStore.Create(ctx, project.ID, uniqueKey("bf1"), "Bulk Flag 1", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	flag2, _ := flagStore.Create(ctx, project.ID, uniqueKey("bf2"), "Bulk Flag 2", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)

	body, _ := json.Marshal(map[string]any{
		"flag_keys":       []string{flag1.Key, flag2.Key},
		"environment_key": "development",
		"reason":          "Code freeze",
	})
	req := httptest.NewRequest("POST", "/api/v1/projects/"+project.Key+"/flags/bulk-lock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Locked        int      `json:"locked"`
		AlreadyLocked int      `json:"already_locked"`
		Errors        []string `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Locked != 2 {
		t.Errorf("expected 2 locked, got %d", resp.Locked)
	}
}

func TestBulkUnlockFlags(t *testing.T) {
	mux, flagStore, ps, es, user := testFlagLockSetup(t)
	ctx := context.Background()

	projKey := uniqueKey("bulkunlk")
	project, err := ps.Create(ctx, projKey, "Bulk Unlock Test", "test")
	if err != nil {
		t.Fatal(err)
	}

	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatal(err)
	}

	flag1, _ := flagStore.Create(ctx, project.ID, uniqueKey("bu1"), "Bulk Flag 1", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	flag2, _ := flagStore.Create(ctx, project.ID, uniqueKey("bu2"), "Bulk Flag 2", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)

	// Lock both
	reason := "freeze"
	flagStore.LockEnvironmentConfig(ctx, flag1.ID, env.ID, user.ID, &reason)
	flagStore.LockEnvironmentConfig(ctx, flag2.ID, env.ID, user.ID, &reason)

	body, _ := json.Marshal(map[string]any{
		"flag_keys":       []string{flag1.Key, flag2.Key},
		"environment_key": "development",
	})
	req := httptest.NewRequest("POST", "/api/v1/projects/"+project.Key+"/flags/bulk-unlock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Unlocked        int      `json:"unlocked"`
		AlreadyUnlocked int      `json:"already_unlocked"`
		Errors          []string `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Unlocked != 2 {
		t.Errorf("expected 2 unlocked, got %d", resp.Unlocked)
	}
}
