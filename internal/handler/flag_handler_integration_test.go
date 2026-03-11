package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

func flagTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestFlagHandler_List_IncludeEnvironmentConfigs(t *testing.T) {
	pool := flagTestPool(t)
	ctx := context.Background()

	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	as := store.NewAuditStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	hub := stream.NewHub()
	cache := evaluation.NewCache()
	pss := store.NewProjectSettingsStore(pool)

	h := handler.NewFlagHandler(fs, ps, es, as, hub, cache, pool, ufs, nil, pss, nil)

	// Create project with environments
	projKey := fmt.Sprintf("incltest-%d", time.Now().UnixNano())
	project, err := ps.Create(ctx, projKey, "Include Test", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	// Create a flag
	_, err = fs.Create(ctx, project.ID, "my-flag", "My Flag", "test", model.ValueTypeBoolean, model.FlagTypeKillSwitch, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating flag: %v", err)
	}

	// Request WITHOUT include — should NOT have environment_configs
	req := httptest.NewRequest("GET", "/api/v1/projects/"+projKey+"/flags", nil)
	req.SetPathValue("key", projKey)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var respWithout struct {
		Data []json.RawMessage `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&respWithout)
	if len(respWithout.Data) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(respWithout.Data))
	}
	// Check that environment_configs key is absent
	var flagMap map[string]any
	json.Unmarshal(respWithout.Data[0], &flagMap)
	if _, exists := flagMap["environment_configs"]; exists {
		t.Error("environment_configs should NOT be present without include param")
	}

	// Request WITH include=environment_configs — should have environment_configs
	req2 := httptest.NewRequest("GET", "/api/v1/projects/"+projKey+"/flags?include=environment_configs", nil)
	req2.SetPathValue("key", projKey)
	w2 := httptest.NewRecorder()
	h.List(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w2.Code)
	}

	var respWith struct {
		Data []json.RawMessage `json:"data"`
	}
	json.NewDecoder(w2.Body).Decode(&respWith)
	if len(respWith.Data) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(respWith.Data))
	}
	var flagMap2 map[string]any
	json.Unmarshal(respWith.Data[0], &flagMap2)
	configs, exists := flagMap2["environment_configs"]
	if !exists {
		t.Fatal("environment_configs should be present with include param")
	}
	configSlice, ok := configs.([]any)
	if !ok {
		t.Fatalf("environment_configs should be an array, got %T", configs)
	}
	if len(configSlice) != 1 {
		t.Errorf("expected 1 environment config, got %d", len(configSlice))
	}
}
