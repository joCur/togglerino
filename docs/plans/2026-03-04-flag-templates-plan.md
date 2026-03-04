# Flag Templates Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add flag templates — pre-configured templates that pre-fill flag type, value type, default value, variants, targeting rules, and environment defaults when creating a new flag. Both global and project-scoped templates supported.

**Architecture:** New `flag_templates` table with nullable `project_id` (NULL = global). Template CRUD via REST API. Extend flag creation to accept initial variants/rules per environment. Frontend template picker in CreateFlagModal + template management pages.

**Tech Stack:** Go (stdlib net/http, pgx/v5), React 19 + TypeScript + TanStack Query + shadcn/ui, PostgreSQL 16

---

### Task 1: Template Model

**Files:**
- Create: `internal/model/template.go`

**Step 1: Create the template model**

```go
package model

import (
	"encoding/json"
	"time"
)

// FlagTemplate represents a pre-configured template for creating flags.
type FlagTemplate struct {
	ID                  string          `json:"id"`
	ProjectID           *string         `json:"project_id,omitempty"`
	Key                 string          `json:"key"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	FlagType            FlagType        `json:"flag_type"`
	ValueType           ValueType       `json:"value_type"`
	DefaultValue        json.RawMessage `json:"default_value"`
	Tags                []string        `json:"tags"`
	EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
	VariantConfig       json.RawMessage `json:"variant_config"`
	IsSystem            bool            `json:"is_system"`
	SortOrder           int             `json:"sort_order"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// VariantConfig holds the pre-configured variant setup for a template.
type VariantConfig struct {
	Variants       []Variant       `json:"variants"`
	DefaultVariant string          `json:"default_variant"`
	TargetingRules []TargetingRule `json:"targeting_rules,omitempty"`
}
```

**Step 2: Commit**

```bash
git add internal/model/template.go
git commit -m "feat: add flag template model"
```

---

### Task 2: Database Migration

**Files:**
- Create: `migrations/015_flag_templates.up.sql`
- Create: `migrations/015_flag_templates.down.sql`

**Step 1: Write the up migration**

```sql
CREATE TABLE flag_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    flag_type TEXT NOT NULL DEFAULT 'release',
    value_type TEXT NOT NULL DEFAULT 'boolean',
    default_value JSONB NOT NULL DEFAULT 'false',
    tags TEXT[] NOT NULL DEFAULT '{}',
    environment_defaults JSONB NOT NULL DEFAULT '{}',
    variant_config JSONB NOT NULL DEFAULT '{}',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(COALESCE(project_id, '00000000-0000-0000-0000-000000000000'), key)
);

CREATE INDEX idx_flag_templates_project_id ON flag_templates(project_id);
```

**Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS flag_templates;
```

**Step 3: Run migration to verify**

```bash
go test ./internal/store/... -run TestFlagStore_Create -count=1
```

This will trigger migration execution. Expected: PASS (migration runs, existing tests unaffected).

**Step 4: Commit**

```bash
git add migrations/015_flag_templates.up.sql migrations/015_flag_templates.down.sql
git commit -m "feat: add flag_templates migration"
```

---

### Task 3: Template Store — Create and List (TDD)

**Files:**
- Create: `internal/store/template_store.go`
- Create: `internal/store/template_store_test.go`

**Step 1: Write the failing test for Create and ListGlobal**

```go
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

	tmpl, err := ts.Create(ctx, nil, "gradual-rollout", "Gradual Rollout", "Boolean flag with gradual rollout",
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
	if tmpl.Key != "gradual-rollout" {
		t.Errorf("Key: got %q, want %q", tmpl.Key, "gradual-rollout")
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/store/... -run TestTemplateStore -count=1
```

Expected: compilation error — `store.NewTemplateStore` doesn't exist.

**Step 3: Write minimal implementation**

```go
package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type TemplateStore struct {
	pool *pgxpool.Pool
}

func NewTemplateStore(pool *pgxpool.Pool) *TemplateStore {
	return &TemplateStore{pool: pool}
}

func (s *TemplateStore) Create(ctx context.Context, projectID *string, key, name, description string, flagType model.FlagType, valueType model.ValueType, defaultValue json.RawMessage, tags []string, environmentDefaults json.RawMessage, variantConfig json.RawMessage, isSystem bool, sortOrder int) (*model.FlagTemplate, error) {
	if tags == nil {
		tags = []string{}
	}
	var t model.FlagTemplate
	err := s.pool.QueryRow(ctx,
		`INSERT INTO flag_templates (project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at`,
		projectID, key, name, description, flagType, valueType, defaultValue, tags, environmentDefaults, variantConfig, isSystem, sortOrder,
	).Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating template: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

func (s *TemplateStore) ListGlobal(ctx context.Context) ([]model.FlagTemplate, error) {
	return s.list(ctx, `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id IS NULL ORDER BY sort_order, name`)
}

func (s *TemplateStore) ListByProject(ctx context.Context, projectID string) ([]model.FlagTemplate, error) {
	return s.list(ctx, `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id = $1 ORDER BY sort_order, name`, projectID)
}

func (s *TemplateStore) list(ctx context.Context, query string, args ...any) ([]model.FlagTemplate, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}
	defer rows.Close()

	var templates []model.FlagTemplate
	for rows.Next() {
		var t model.FlagTemplate
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning template: %w", err)
		}
		if t.Tags == nil {
			t.Tags = []string{}
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating templates: %w", err)
	}
	if templates == nil {
		templates = []model.FlagTemplate{}
	}
	return templates, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/store/... -run TestTemplateStore -count=1 -v
```

Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add internal/store/template_store.go internal/store/template_store_test.go
git commit -m "feat: add template store with Create and List methods"
```

---

### Task 4: Template Store — GetByKey, Update, Delete (TDD)

**Files:**
- Modify: `internal/store/template_store.go`
- Modify: `internal/store/template_store_test.go`

**Step 1: Write failing tests**

Add to `template_store_test.go`:

```go
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

	// Not found
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/store/... -run "TestTemplateStore_GetByKey|TestTemplateStore_Update|TestTemplateStore_Delete" -count=1
```

Expected: compilation error — methods don't exist.

**Step 3: Implement GetByKey, Update, Delete**

Add to `template_store.go`:

```go
func (s *TemplateStore) GetByKey(ctx context.Context, projectID *string, key string) (*model.FlagTemplate, error) {
	var t model.FlagTemplate
	var query string
	var args []any
	if projectID == nil {
		query = `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id IS NULL AND key = $1`
		args = []any{key}
	} else {
		query = `SELECT id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at FROM flag_templates WHERE project_id = $1 AND key = $2`
		args = []any{*projectID, key}
	}
	err := s.pool.QueryRow(ctx, query, args...).Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting template by key: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

func (s *TemplateStore) Update(ctx context.Context, id string, name, description string, flagType model.FlagType, valueType model.ValueType, defaultValue json.RawMessage, tags []string, environmentDefaults json.RawMessage, variantConfig json.RawMessage, sortOrder int) (*model.FlagTemplate, error) {
	if tags == nil {
		tags = []string{}
	}
	var t model.FlagTemplate
	err := s.pool.QueryRow(ctx,
		`UPDATE flag_templates SET name=$2, description=$3, flag_type=$4, value_type=$5, default_value=$6, tags=$7, environment_defaults=$8, variant_config=$9, sort_order=$10, updated_at=NOW()
		 WHERE id=$1
		 RETURNING id, project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order, created_at, updated_at`,
		id, name, description, flagType, valueType, defaultValue, tags, environmentDefaults, variantConfig, sortOrder,
	).Scan(&t.ID, &t.ProjectID, &t.Key, &t.Name, &t.Description, &t.FlagType, &t.ValueType, &t.DefaultValue, &t.Tags, &t.EnvironmentDefaults, &t.VariantConfig, &t.IsSystem, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating template: %w", err)
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

func (s *TemplateStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM flag_templates WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting template: %w", err)
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/... -run TestTemplateStore -count=1 -v
```

Expected: all 7 tests PASS.

**Step 5: Commit**

```bash
git add internal/store/template_store.go internal/store/template_store_test.go
git commit -m "feat: add template store GetByKey, Update, Delete"
```

---

### Task 5: Template Store — SeedSystemTemplates (TDD)

**Files:**
- Modify: `internal/store/template_store.go`
- Modify: `internal/store/template_store_test.go`

**Step 1: Write failing test**

Add to `template_store_test.go`:

```go
func TestTemplateStore_SeedSystemTemplates(t *testing.T) {
	pool := testPool(t)
	ts := store.NewTemplateStore(pool)
	ctx := context.Background()

	err := ts.SeedSystemTemplates(ctx)
	if err != nil {
		t.Fatalf("SeedSystemTemplates: %v", err)
	}

	templates, err := ts.ListGlobal(ctx)
	if err != nil {
		t.Fatalf("ListGlobal: %v", err)
	}

	systemKeys := map[string]bool{
		"gradual-rollout":  false,
		"kill-switch":      false,
		"ab-test":          false,
		"permission-gate":  false,
	}
	for _, tmpl := range templates {
		if _, ok := systemKeys[tmpl.Key]; ok {
			systemKeys[tmpl.Key] = true
			if !tmpl.IsSystem {
				t.Errorf("template %q should be system", tmpl.Key)
			}
		}
	}
	for key, found := range systemKeys {
		if !found {
			t.Errorf("system template %q not found", key)
		}
	}

	// Idempotent: run again, should not error or duplicate
	err = ts.SeedSystemTemplates(ctx)
	if err != nil {
		t.Fatalf("second SeedSystemTemplates: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/store/... -run TestTemplateStore_SeedSystemTemplates -count=1
```

Expected: compilation error.

**Step 3: Implement SeedSystemTemplates**

Add to `template_store.go`:

```go
func (s *TemplateStore) SeedSystemTemplates(ctx context.Context) error {
	templates := []struct {
		key, name, description string
		flagType               model.FlagType
		valueType              model.ValueType
		defaultValue           string
		tags                   []string
		envDefaults            string
		variantConfig          string
		sortOrder              int
	}{
		{
			key: "gradual-rollout", name: "Gradual Rollout",
			description: "Boolean flag with gradual percentage rollout",
			flagType: model.FlagTypeRelease, valueType: model.ValueTypeBoolean,
			defaultValue:  `false`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":true},"production":{"enabled":false}}`,
			variantConfig: `{"variants":[{"key":"on","value":true},{"key":"off","value":false}],"default_variant":"off","targeting_rules":[{"conditions":[],"variant":"on","percentage_rollout":0}]}`,
			sortOrder:     0,
		},
		{
			key: "kill-switch", name: "Kill Switch",
			description: "Emergency shutoff switch, enabled everywhere",
			flagType: model.FlagTypeKillSwitch, valueType: model.ValueTypeBoolean,
			defaultValue:  `true`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":true},"staging":{"enabled":true},"production":{"enabled":true}}`,
			variantConfig: `{"variants":[{"key":"on","value":true}],"default_variant":"on"}`,
			sortOrder:     1,
		},
		{
			key: "ab-test", name: "A/B Test",
			description: "Experiment with two variants split 50/50",
			flagType: model.FlagTypeExperiment, valueType: model.ValueTypeString,
			defaultValue:  `"control"`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":true},"production":{"enabled":false}}`,
			variantConfig: `{"variants":[{"key":"control","value":"control"},{"key":"treatment","value":"treatment"}],"default_variant":"control","targeting_rules":[{"conditions":[],"variant":"treatment","percentage_rollout":50}]}`,
			sortOrder:     2,
		},
		{
			key: "permission-gate", name: "Permission Gate",
			description: "Permission flag, disabled by default with targeting rules",
			flagType: model.FlagTypePermission, valueType: model.ValueTypeBoolean,
			defaultValue:  `false`,
			tags:          []string{},
			envDefaults:   `{"development":{"enabled":false},"staging":{"enabled":false},"production":{"enabled":false}}`,
			variantConfig: `{"variants":[{"key":"on","value":true},{"key":"off","value":false}],"default_variant":"off"}`,
			sortOrder:     3,
		},
	}

	for _, tmpl := range templates {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO flag_templates (project_id, key, name, description, flag_type, value_type, default_value, tags, environment_defaults, variant_config, is_system, sort_order)
			 VALUES (NULL, $1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
			 ON CONFLICT (COALESCE(project_id, '00000000-0000-0000-0000-000000000000'), key) DO UPDATE SET
			   name=EXCLUDED.name, description=EXCLUDED.description, flag_type=EXCLUDED.flag_type,
			   value_type=EXCLUDED.value_type, default_value=EXCLUDED.default_value, tags=EXCLUDED.tags,
			   environment_defaults=EXCLUDED.environment_defaults, variant_config=EXCLUDED.variant_config,
			   sort_order=EXCLUDED.sort_order, updated_at=NOW()`,
			tmpl.key, tmpl.name, tmpl.description, tmpl.flagType, tmpl.valueType,
			json.RawMessage(tmpl.defaultValue), tmpl.tags,
			json.RawMessage(tmpl.envDefaults), json.RawMessage(tmpl.variantConfig), tmpl.sortOrder,
		)
		if err != nil {
			return fmt.Errorf("seeding template %q: %w", tmpl.key, err)
		}
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/store/... -run TestTemplateStore_SeedSystemTemplates -count=1 -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/store/template_store.go internal/store/template_store_test.go
git commit -m "feat: add system template seeding"
```

---

### Task 6: Template Handler — Global CRUD (TDD)

**Files:**
- Create: `internal/handler/template_handler.go`

The handler follows the same pattern as `segment_handler.go`: struct with store dependencies, methods for each endpoint.

**Step 1: Write the handler**

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type TemplateHandler struct {
	templates *store.TemplateStore
	projects  *store.ProjectStore
}

func NewTemplateHandler(templates *store.TemplateStore, projects *store.ProjectStore) *TemplateHandler {
	return &TemplateHandler{templates: templates, projects: projects}
}

// ListGlobal handles GET /api/v1/templates
func (h *TemplateHandler) ListGlobal(w http.ResponseWriter, r *http.Request) {
	templates, err := h.templates.ListGlobal(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

// CreateGlobal handles POST /api/v1/templates
func (h *TemplateHandler) CreateGlobal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key                 string          `json:"key"`
		Name                string          `json:"name"`
		Description         string          `json:"description"`
		FlagType            model.FlagType  `json:"flag_type"`
		ValueType           model.ValueType `json:"value_type"`
		DefaultValue        json.RawMessage `json:"default_value"`
		Tags                []string        `json:"tags"`
		EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
		VariantConfig       json.RawMessage `json:"variant_config"`
		SortOrder           int             `json:"sort_order"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}
	if req.FlagType == "" {
		req.FlagType = model.FlagTypeRelease
	}
	if req.ValueType == "" {
		req.ValueType = model.ValueTypeBoolean
	}
	if !model.ValidFlagTypes[req.FlagType] {
		writeError(w, http.StatusBadRequest, "invalid flag_type")
		return
	}
	if !model.ValidValueTypes[req.ValueType] {
		writeError(w, http.StatusBadRequest, "invalid value_type")
		return
	}
	if req.DefaultValue == nil {
		req.DefaultValue = json.RawMessage(`false`)
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	if req.EnvironmentDefaults == nil {
		req.EnvironmentDefaults = json.RawMessage(`{}`)
	}
	if req.VariantConfig == nil {
		req.VariantConfig = json.RawMessage(`{}`)
	}

	tmpl, err := h.templates.Create(r.Context(), nil, req.Key, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, false, req.SortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "template key already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}
	writeJSON(w, http.StatusCreated, tmpl)
}

// UpdateGlobal handles PUT /api/v1/templates/{key}
func (h *TemplateHandler) UpdateGlobal(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "template key is required")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), nil, key)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	var req struct {
		Name                string          `json:"name"`
		Description         string          `json:"description"`
		FlagType            model.FlagType  `json:"flag_type"`
		ValueType           model.ValueType `json:"value_type"`
		DefaultValue        json.RawMessage `json:"default_value"`
		Tags                []string        `json:"tags"`
		EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
		VariantConfig       json.RawMessage `json:"variant_config"`
		SortOrder           int             `json:"sort_order"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !model.ValidFlagTypes[req.FlagType] {
		writeError(w, http.StatusBadRequest, "invalid flag_type")
		return
	}
	if !model.ValidValueTypes[req.ValueType] {
		writeError(w, http.StatusBadRequest, "invalid value_type")
		return
	}

	updated, err := h.templates.Update(r.Context(), existing.ID, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, req.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteGlobal handles DELETE /api/v1/templates/{key}
func (h *TemplateHandler) DeleteGlobal(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "template key is required")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), nil, key)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	if existing.IsSystem {
		writeError(w, http.StatusForbidden, "system templates cannot be deleted")
		return
	}

	if err := h.templates.Delete(r.Context(), existing.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListForProject handles GET /api/v1/projects/{key}/templates
// Returns both global and project-scoped templates in separate sections.
func (h *TemplateHandler) ListForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	global, err := h.templates.ListGlobal(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list global templates")
		return
	}

	projectTemplates, err := h.templates.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list project templates")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"global":  global,
		"project": projectTemplates,
	})
}

// CreateForProject handles POST /api/v1/projects/{key}/templates
func (h *TemplateHandler) CreateForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Key                 string          `json:"key"`
		Name                string          `json:"name"`
		Description         string          `json:"description"`
		FlagType            model.FlagType  `json:"flag_type"`
		ValueType           model.ValueType `json:"value_type"`
		DefaultValue        json.RawMessage `json:"default_value"`
		Tags                []string        `json:"tags"`
		EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
		VariantConfig       json.RawMessage `json:"variant_config"`
		SortOrder           int             `json:"sort_order"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}
	if req.FlagType == "" {
		req.FlagType = model.FlagTypeRelease
	}
	if req.ValueType == "" {
		req.ValueType = model.ValueTypeBoolean
	}
	if !model.ValidFlagTypes[req.FlagType] {
		writeError(w, http.StatusBadRequest, "invalid flag_type")
		return
	}
	if !model.ValidValueTypes[req.ValueType] {
		writeError(w, http.StatusBadRequest, "invalid value_type")
		return
	}
	if req.DefaultValue == nil {
		req.DefaultValue = json.RawMessage(`false`)
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}
	if req.EnvironmentDefaults == nil {
		req.EnvironmentDefaults = json.RawMessage(`{}`)
	}
	if req.VariantConfig == nil {
		req.VariantConfig = json.RawMessage(`{}`)
	}

	tmpl, err := h.templates.Create(r.Context(), &project.ID, req.Key, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, false, req.SortOrder)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "template key already exists for this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create template")
		return
	}
	writeJSON(w, http.StatusCreated, tmpl)
}

// UpdateForProject handles PUT /api/v1/projects/{key}/templates/{templateKey}
func (h *TemplateHandler) UpdateForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	templateKey := r.PathValue("templateKey")
	if projectKey == "" || templateKey == "" {
		writeError(w, http.StatusBadRequest, "project key and template key are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), &project.ID, templateKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	var req struct {
		Name                string          `json:"name"`
		Description         string          `json:"description"`
		FlagType            model.FlagType  `json:"flag_type"`
		ValueType           model.ValueType `json:"value_type"`
		DefaultValue        json.RawMessage `json:"default_value"`
		Tags                []string        `json:"tags"`
		EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
		VariantConfig       json.RawMessage `json:"variant_config"`
		SortOrder           int             `json:"sort_order"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !model.ValidFlagTypes[req.FlagType] {
		writeError(w, http.StatusBadRequest, "invalid flag_type")
		return
	}
	if !model.ValidValueTypes[req.ValueType] {
		writeError(w, http.StatusBadRequest, "invalid value_type")
		return
	}

	updated, err := h.templates.Update(r.Context(), existing.ID, req.Name, req.Description, req.FlagType, req.ValueType, req.DefaultValue, req.Tags, req.EnvironmentDefaults, req.VariantConfig, req.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update template")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// DeleteForProject handles DELETE /api/v1/projects/{key}/templates/{templateKey}
func (h *TemplateHandler) DeleteForProject(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	templateKey := r.PathValue("templateKey")
	if projectKey == "" || templateKey == "" {
		writeError(w, http.StatusBadRequest, "project key and template key are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	existing, err := h.templates.GetByKey(r.Context(), &project.ID, templateKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}

	if err := h.templates.Delete(r.Context(), existing.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete template")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Verify compilation**

```bash
go build ./internal/handler/...
```

Expected: PASS.

**Step 3: Commit**

```bash
git add internal/handler/template_handler.go
git commit -m "feat: add template handler with global and project CRUD"
```

---

### Task 7: Wire Template Store and Handler in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add template store, handler, seed call, and routes**

In the store initialization section (after `oidcStore`):
```go
templateStore := store.NewTemplateStore(pool)
```

After `cache.LoadAll(ctx, pool)` and before handler initialization:
```go
if err := templateStore.SeedSystemTemplates(ctx); err != nil {
    log.Fatalf("failed to seed system templates: %v", err)
}
```

In the handler initialization section:
```go
templateHandler := handler.NewTemplateHandler(templateStore, projectStore)
```

In the route registration section, add after the Segments block:
```go
// Templates (global)
mux.Handle("GET /api/v1/templates", wrap(templateHandler.ListGlobal, sessionAuth))
mux.Handle("POST /api/v1/templates", wrap(templateHandler.CreateGlobal, sessionAuth, requireAdmin))
mux.Handle("PUT /api/v1/templates/{key}", wrap(templateHandler.UpdateGlobal, sessionAuth, requireAdmin))
mux.Handle("DELETE /api/v1/templates/{key}", wrap(templateHandler.DeleteGlobal, sessionAuth, requireAdmin))

// Templates (project-scoped)
mux.Handle("GET /api/v1/projects/{key}/templates", wrap(templateHandler.ListForProject, sessionAuth))
mux.Handle("POST /api/v1/projects/{key}/templates", wrap(templateHandler.CreateForProject, sessionAuth))
mux.Handle("PUT /api/v1/projects/{key}/templates/{templateKey}", wrap(templateHandler.UpdateForProject, sessionAuth))
mux.Handle("DELETE /api/v1/projects/{key}/templates/{templateKey}", wrap(templateHandler.DeleteForProject, sessionAuth))
```

**Step 2: Verify compilation**

```bash
go build ./cmd/togglerino/...
```

Expected: PASS.

**Step 3: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: wire template store and handler in main.go"
```

---

### Task 8: Extend Flag Creation to Accept Variant Config (TDD)

**Files:**
- Modify: `internal/model/project_settings.go`
- Modify: `internal/store/flag_store.go`
- Modify: `internal/handler/flag_handler.go`
- Modify: `internal/store/flag_store_test.go`

**Step 1: Extend EnvironmentDefault model**

In `internal/model/project_settings.go`, change `EnvironmentDefault` to:

```go
type EnvironmentDefault struct {
	Enabled        bool            `json:"enabled"`
	Variants       json.RawMessage `json:"variants,omitempty"`
	DefaultVariant string          `json:"default_variant,omitempty"`
	TargetingRules json.RawMessage `json:"targeting_rules,omitempty"`
}
```

Add `"encoding/json"` to imports.

**Step 2: Write failing test for flag creation with variant config**

Add to `flag_store_test.go`:

```go
func TestFlagStore_CreateWithVariantConfig(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagvariant")
	project, err := ps.Create(ctx, projKey, "Variant Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	defaultValue := json.RawMessage(`false`)
	envOverrides := map[string]model.EnvironmentDefault{
		"development": {
			Enabled:        true,
			Variants:       json.RawMessage(`[{"key":"on","value":true},{"key":"off","value":false}]`),
			DefaultVariant: "off",
			TargetingRules: json.RawMessage(`[{"conditions":[],"variant":"on","percentage_rollout":10}]`),
		},
	}

	flag, err := fs.Create(ctx, project.ID, "test-flag", "Test Flag", "desc",
		model.ValueTypeBoolean, model.FlagTypeRelease, defaultValue, []string{}, nil, nil, envOverrides)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	configs, err := fs.GetAllEnvironmentConfigs(ctx, flag.ID)
	if err != nil {
		t.Fatalf("GetAllEnvironmentConfigs: %v", err)
	}

	var devConfig *model.FlagEnvironmentConfig
	for i := range configs {
		if configs[i].EnvironmentID == env.ID {
			devConfig = &configs[i]
			break
		}
	}
	if devConfig == nil {
		t.Fatal("development config not found")
	}
	if !devConfig.Enabled {
		t.Error("expected development to be enabled")
	}
	if devConfig.DefaultVariant != "off" {
		t.Errorf("DefaultVariant: got %q, want %q", devConfig.DefaultVariant, "off")
	}
	if len(devConfig.Variants) != 2 {
		t.Errorf("Variants length: got %d, want 2", len(devConfig.Variants))
	}
	if len(devConfig.TargetingRules) != 1 {
		t.Errorf("TargetingRules length: got %d, want 1", len(devConfig.TargetingRules))
	}
}
```

**Step 3: Run test to verify it fails**

```bash
go test ./internal/store/... -run TestFlagStore_CreateWithVariantConfig -count=1
```

Expected: compilation error — `Create` signature doesn't accept `envOverrides`.

**Step 4: Extend FlagStore.Create to accept environment overrides**

Change `FlagStore.Create` signature to add `envOverrides map[string]model.EnvironmentDefault` as the last parameter. Update the environment config insertion loop:

```go
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string, envEnabled map[string]bool, ownerID *string, envOverrides map[string]model.EnvironmentDefault) (*model.Flag, error) {
```

Replace the env config insertion loop (the `for _, env := range envs` block) with:

```go
for _, env := range envs {
    enabled := false
    if envEnabled != nil {
        if v, ok := envEnabled[env.Key]; ok {
            enabled = v
        }
    }

    defaultVariant := ""
    variants := json.RawMessage(`[]`)
    targetingRules := json.RawMessage(`[]`)

    if envOverrides != nil {
        if override, ok := envOverrides[env.Key]; ok {
            if override.DefaultVariant != "" {
                defaultVariant = override.DefaultVariant
            }
            if override.Variants != nil {
                variants = override.Variants
            }
            if override.TargetingRules != nil {
                targetingRules = override.TargetingRules
            }
        }
    }

    _, err := tx.Exec(ctx,
        `INSERT INTO flag_environment_configs (flag_id, environment_id, enabled, default_variant, variants, targeting_rules) VALUES ($1, $2, $3, $4, $5, $6)`,
        f.ID, env.ID, enabled, defaultVariant, variants, targetingRules,
    )
    if err != nil {
        return nil, fmt.Errorf("creating flag environment config for env %s: %w", env.ID, err)
    }
}
```

**Step 5: Update all existing callers of FlagStore.Create**

In `flag_handler.go` line ~125, update the Create call to pass `nil` for the new parameter:

```go
flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.ValueType, req.FlagType, req.DefaultValue, req.Tags, envEnabled, req.OwnerID, nil)
```

This will be updated in the next step to pass the actual overrides.

**Step 6: Update flag handler to pass environment overrides**

In `flag_handler.go`, change the `EnvironmentOverrides` field type and update the Create call:

Change the request struct field:
```go
EnvironmentOverrides map[string]model.EnvironmentDefault `json:"environment_overrides"`
```

This is already the correct type. Now pass overrides to Create:

```go
flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.ValueType, req.FlagType, req.DefaultValue, req.Tags, envEnabled, req.OwnerID, req.EnvironmentOverrides)
```

**Step 7: Run all tests**

```bash
go test ./internal/store/... -count=1 -v
```

Expected: ALL tests PASS including new `TestFlagStore_CreateWithVariantConfig`.

**Step 8: Commit**

```bash
git add internal/model/project_settings.go internal/store/flag_store.go internal/handler/flag_handler.go internal/store/flag_store_test.go
git commit -m "feat: extend flag creation to accept initial variant config per environment"
```

---

### Task 9: Frontend — TypeScript Types and API Client

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

**Step 1: Add FlagTemplate type**

Add to `web/src/api/types.ts`:

```typescript
export interface FlagTemplate {
    id: string
    project_id: string | null
    key: string
    name: string
    description: string
    flag_type: FlagPurpose
    value_type: ValueType
    default_value: unknown
    tags: string[]
    environment_defaults: Record<string, { enabled: boolean }>
    variant_config: {
        variants?: Variant[]
        default_variant?: string
        targeting_rules?: TargetingRule[]
    }
    is_system: boolean
    sort_order: number
    created_at: string
    updated_at: string
}

export interface TemplatesForProject {
    global: FlagTemplate[]
    project: FlagTemplate[]
}
```

**Step 2: Add template API methods to client**

Add to `web/src/api/client.ts` in the `api` object:

```typescript
templates: {
    listGlobal: () => request<FlagTemplate[]>('/templates'),
    createGlobal: (body: Partial<FlagTemplate>) =>
        request<FlagTemplate>('/templates', { method: 'POST', body: JSON.stringify(body) }),
    updateGlobal: (key: string, body: Partial<FlagTemplate>) =>
        request<FlagTemplate>(`/templates/${key}`, { method: 'PUT', body: JSON.stringify(body) }),
    deleteGlobal: (key: string) =>
        request<void>(`/templates/${key}`, { method: 'DELETE' }),
    listForProject: (projectKey: string) =>
        request<TemplatesForProject>(`/projects/${projectKey}/templates`),
    createForProject: (projectKey: string, body: Partial<FlagTemplate>) =>
        request<FlagTemplate>(`/projects/${projectKey}/templates`, { method: 'POST', body: JSON.stringify(body) }),
    updateForProject: (projectKey: string, templateKey: string, body: Partial<FlagTemplate>) =>
        request<FlagTemplate>(`/projects/${projectKey}/templates/${templateKey}`, { method: 'PUT', body: JSON.stringify(body) }),
    deleteForProject: (projectKey: string, templateKey: string) =>
        request<void>(`/projects/${projectKey}/templates/${templateKey}`, { method: 'DELETE' }),
},
```

Add the imports at the top of `client.ts`:

```typescript
import type { Condition, Segment, BulkActionRequest, BulkActionResponse, FlagTemplate, TemplatesForProject } from './types'
```

**Step 3: Verify build**

```bash
cd web && npm run build
```

Expected: PASS.

**Step 4: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat: add flag template TypeScript types and API client methods"
```

---

### Task 10: Frontend — Template Picker in CreateFlagModal

**Files:**
- Modify: `web/src/components/CreateFlagModal.tsx`

**Step 1: Add template fetching and selection state**

At the top of the component, add a query for templates and state for selection:

```typescript
const [selectedTemplate, setSelectedTemplate] = useState<FlagTemplate | null>(null)
const [showTemplates, setShowTemplates] = useState(true)

const { data: templatesData } = useQuery({
    queryKey: ['projects', projectKey, 'templates'],
    queryFn: () => api.templates.listForProject(projectKey),
    enabled: open,
})
```

**Step 2: Add template picker view**

When `showTemplates` is true, render a template selection grid instead of the form. Each template is a clickable card. Include a "Blank" option. When a template is selected, pre-fill all form fields and switch to `showTemplates = false`.

The template selection function:

```typescript
const selectTemplate = (tmpl: FlagTemplate | null) => {
    if (tmpl) {
        setFlagPurpose(tmpl.flag_type)
        setFlagType(tmpl.value_type)
        setDescription(tmpl.description.startsWith('Emergency') ? 'Emergency shutoff for...' : '')
        setDefaultValue(typeof tmpl.default_value === 'string' ? tmpl.default_value : JSON.stringify(tmpl.default_value))
        setTags(tmpl.tags.join(', '))
        // Apply environment defaults
        const overrides: Record<string, boolean> = {}
        for (const [envKey, config] of Object.entries(tmpl.environment_defaults)) {
            overrides[envKey] = config.enabled
        }
        setEnvOverrides(overrides)
    }
    setSelectedTemplate(tmpl)
    setShowTemplates(false)
}
```

The template picker grid JSX (rendered when `showTemplates` is true):

```tsx
<div className="grid grid-cols-2 gap-3">
    <button onClick={() => selectTemplate(null)} className="p-4 rounded-lg border border-border hover:border-amber-500/50 text-left transition-colors">
        <div className="font-medium text-sm">Blank</div>
        <div className="text-xs text-muted-foreground mt-1">Start from scratch</div>
    </button>
    {templatesData?.global.map(tmpl => (
        <button key={tmpl.id} onClick={() => selectTemplate(tmpl)} className="p-4 rounded-lg border border-border hover:border-amber-500/50 text-left transition-colors">
            <div className="font-medium text-sm">{tmpl.name}</div>
            <div className="text-xs text-muted-foreground mt-1">{tmpl.description}</div>
            <Badge variant="outline" className="mt-2 text-[10px]">{tmpl.flag_type}</Badge>
        </button>
    ))}
</div>
```

If `templatesData?.project` has entries, render a "Project Templates" section below.

**Step 3: Pass variant config in mutation**

When submitting the form, if a template was selected and has `variant_config`, include it in `environment_overrides`:

```typescript
const envOverridesPayload: Record<string, any> = {}
for (const [envKey, enabled] of Object.entries(environmentOverrides)) {
    envOverridesPayload[envKey] = { enabled }
}
// Apply variant config from template to all overridden environments
if (selectedTemplate?.variant_config?.variants) {
    for (const envKey of Object.keys(envOverridesPayload)) {
        envOverridesPayload[envKey] = {
            ...envOverridesPayload[envKey],
            variants: selectedTemplate.variant_config.variants,
            default_variant: selectedTemplate.variant_config.default_variant,
            targeting_rules: selectedTemplate.variant_config.targeting_rules,
        }
    }
}
```

**Step 4: Reset template state in resetAndClose**

```typescript
setSelectedTemplate(null)
setShowTemplates(true)
```

**Step 5: Verify build**

```bash
cd web && npm run build
```

Expected: PASS.

**Step 6: Run lint**

```bash
cd web && npm run lint
```

Expected: PASS.

**Step 7: Commit**

```bash
git add web/src/components/CreateFlagModal.tsx
git commit -m "feat: add template picker to flag creation modal"
```

---

### Task 11: Frontend — Global Template Management Page

**Files:**
- Create: `web/src/pages/settings/TemplatesSettingsTab.tsx`
- Modify: `web/src/pages/settings/SettingsPage.tsx` (add tab)
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create TemplatesSettingsTab**

This page follows the pattern of `OIDCSettingsTab.tsx` — list templates in a table, with create/edit/delete actions. Use `useQuery` for the list, `useMutation` for CRUD, dialog for create/edit form.

Create a full CRUD page at `web/src/pages/settings/TemplatesSettingsTab.tsx` with:
- Table listing all global templates (name, key, flag type, value type, system badge)
- "Create Template" button opening a dialog with the full template form
- Edit/Delete actions per row (system templates: edit only, no delete)
- Form fields: name, key, description, flag type, value type, default value, tags, environment defaults (JSON), variant config (JSON)

**Step 2: Add route and tab**

In `SettingsPage.tsx`, add a "Templates" tab that renders `<TemplatesSettingsTab />`.

In `App.tsx`, add `<Route path="templates" element={<TemplatesSettingsTab />} />` under the settings route if settings uses nested routes, or adjust the tabs approach used in SettingsPage.

**Step 3: Verify build and lint**

```bash
cd web && npm run build && npm run lint
```

Expected: PASS.

**Step 4: Commit**

```bash
git add web/src/pages/settings/TemplatesSettingsTab.tsx web/src/pages/settings/SettingsPage.tsx web/src/App.tsx
git commit -m "feat: add global template management page in admin settings"
```

---

### Task 12: Frontend — Project Template Management

**Files:**
- Create: `web/src/pages/project/TemplatesTab.tsx`
- Modify: `web/src/pages/project/ProjectSettingsPage.tsx` (add tab)
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create TemplatesTab**

Similar to the global templates page but scoped to a project. Uses `api.templates.listForProject`, `api.templates.createForProject`, etc. Shows only project-scoped templates (not global ones — those are managed in admin settings).

**Step 2: Add tab and route**

In `ProjectSettingsPage.tsx`, add a "Templates" tab.

In `App.tsx`, under the `projects/:key/settings` route group, add:
```tsx
<Route path="templates" element={<TemplatesTab />} />
```

**Step 3: Verify build and lint**

```bash
cd web && npm run build && npm run lint
```

Expected: PASS.

**Step 4: Commit**

```bash
git add web/src/pages/project/TemplatesTab.tsx web/src/pages/project/ProjectSettingsPage.tsx web/src/App.tsx
git commit -m "feat: add project template management in project settings"
```

---

### Task 13: Run Full Test Suite

**Step 1: Run all Go tests**

```bash
go test ./... -count=1
```

Expected: ALL tests PASS.

**Step 2: Run frontend build and lint**

```bash
cd web && npm run build && npm run lint
```

Expected: PASS.

**Step 3: Commit any fixes if needed**

---

### Task 14: Final Review and Cleanup

**Step 1: Review all changes**

```bash
git log --oneline main..HEAD
git diff main..HEAD --stat
```

**Step 2: Verify the full application builds**

```bash
cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino
```

Expected: binary builds successfully.

**Step 3: Clean up any TODO comments or debug code**
