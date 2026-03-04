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
