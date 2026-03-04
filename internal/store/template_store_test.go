package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestTemplateStore_Create(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	key := uniqueKey("gradual")
	tmpl, err := ts.Create(ctx, nil, key, "Gradual Rollout", "Boolean flag with gradual rollout",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), []string{"release"},
		json.RawMessage(`{"development":{"enabled":true},"production":{"enabled":false}}`),
		json.RawMessage(`{"variants":[{"key":"on","value":true},{"key":"off","value":false}],"default_variant":"off","targeting_rules":[{"conditions":[],"variant":"on","percentage_rollout":0}]}`),
		true, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tmpl.ID == "" {
		t.Error("expected non-empty ID")
	}
	if tmpl.Key != key {
		t.Errorf("Key: got %q, want %q", tmpl.Key, key)
	}
	if tmpl.Name != "Gradual Rollout" {
		t.Errorf("Name: got %q, want %q", tmpl.Name, "Gradual Rollout")
	}
	if tmpl.FlagType != model.FlagTypeRelease {
		t.Errorf("FlagType: got %q, want %q", tmpl.FlagType, model.FlagTypeRelease)
	}
	if !tmpl.IsSystem {
		t.Error("expected IsSystem to be true")
	}
	if tmpl.ProjectID != nil {
		t.Error("expected nil ProjectID for global template")
	}
}

func TestTemplateStore_ListGlobal(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	key := uniqueKey("tmpl")
	_, err := ts.Create(ctx, nil, key, "Test Template", "desc",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	templates, err := ts.ListGlobal(ctx)
	if err != nil {
		t.Fatalf("ListGlobal: %v", err)
	}
	found := false
	for _, tmpl := range templates {
		if tmpl.Key == key {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find template with key %q in global list", key)
	}
}

func TestTemplateStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("tmplproj")
	project, err := ps.Create(ctx, projKey, "Template Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	key := uniqueKey("projtmpl")
	_, err = ts.Create(ctx, &project.ID, key, "Project Template", "desc",
		model.FlagTypeExperiment, model.ValueTypeString,
		json.RawMessage(`"control"`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err != nil {
		t.Fatalf("Create project template: %v", err)
	}

	templates, err := ts.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	found := false
	for _, tmpl := range templates {
		if tmpl.Key == key {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find template with key %q in project list", key)
	}
}

func TestTemplateStore_DuplicateKey(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	key := uniqueKey("dup")
	_, err := ts.Create(ctx, nil, key, "Template 1", "",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = ts.Create(ctx, nil, key, "Template 2", "",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err == nil {
		t.Fatal("expected error for duplicate key, got nil")
	}
}

func TestTemplateStore_GetByKey(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	key := uniqueKey("getbykey")
	created, err := ts.Create(ctx, nil, key, "Get Template", "desc",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ts.GetByKey(ctx, nil, key)
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID: got %q, want %q", got.ID, created.ID)
	}

	_, err = ts.GetByKey(ctx, nil, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestTemplateStore_Update(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	key := uniqueKey("update")
	created, err := ts.Create(ctx, nil, key, "Original", "desc",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := ts.Update(ctx, created.ID, "Updated Name", "new desc",
		model.FlagTypeExperiment, model.ValueTypeString,
		json.RawMessage(`"control"`), []string{"test"},
		json.RawMessage(`{"production":{"enabled":true}}`),
		json.RawMessage(`{"variants":[{"key":"a","value":"a"}]}`),
		1)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("Name: got %q, want %q", updated.Name, "Updated Name")
	}
	if updated.FlagType != model.FlagTypeExperiment {
		t.Errorf("FlagType: got %q, want %q", updated.FlagType, model.FlagTypeExperiment)
	}
}

func TestTemplateStore_Delete(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	key := uniqueKey("delete")
	created, err := ts.Create(ctx, nil, key, "Delete Me", "desc",
		model.FlagTypeRelease, model.ValueTypeBoolean,
		json.RawMessage(`false`), nil,
		json.RawMessage(`{}`), json.RawMessage(`{}`),
		false, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ts.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = ts.GetByKey(ctx, nil, key)
	if err == nil {
		t.Error("expected error after delete")
	}
}
