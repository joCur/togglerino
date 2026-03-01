# Reusable Segments Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add reusable, project-scoped segments (named groups of targeting conditions) that can be referenced by targeting rules across multiple flags.

**Architecture:** New `segments` table, `SegmentStore`, `SegmentHandler`, cache extension with segment data, evaluation engine change to resolve `segment_match` conditions, and frontend segment management + rule builder extension.

**Tech Stack:** Go 1.25 (stdlib net/http, pgx/v5), React 19 + TypeScript + TanStack Query, PostgreSQL 16

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/009_segments.up.sql`
- Create: `migrations/009_segments.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/009_segments.up.sql
CREATE TABLE segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    conditions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key)
);

CREATE INDEX idx_segments_project_id ON segments(project_id);
```

**Step 2: Write the down migration**

```sql
-- migrations/009_segments.down.sql
DROP TABLE IF EXISTS segments;
```

**Step 3: Verify migration compiles**

Run: `go build ./...`
Expected: Build succeeds (migrations are embedded via `embed.FS`).

**Step 4: Commit**

```bash
git add migrations/009_segments.up.sql migrations/009_segments.down.sql
git commit -m "feat: add segments table migration (#31)"
```

---

### Task 2: Model — Segment Type and Operator Constant

**Files:**
- Modify: `internal/model/flag.go:83-101` (add operator constant)
- Create: `internal/model/segment.go` (new file for Segment type)

**Step 1: Create the Segment model**

Create `internal/model/segment.go`:

```go
package model

import "time"

type Segment struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"project_id"`
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Conditions  []Condition `json:"conditions"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
```

**Step 2: Add OpSegmentMatch constant**

In `internal/model/flag.go`, after line 100 (`OpMatches`), add:

```go
	OpSegmentMatch Operator = "segment_match"
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/model/segment.go internal/model/flag.go
git commit -m "feat: add Segment model and segment_match operator (#31)"
```

---

### Task 3: Segment Store — CRUD Operations

**Files:**
- Create: `internal/store/segment_store.go`
- Test: `internal/store/segment_store_test.go`

**Step 1: Write failing tests for segment CRUD**

Create `internal/store/segment_store_test.go`. Tests require a running PostgreSQL. Use the same `testPool()` helper pattern as other store tests.

```go
package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func TestSegmentStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	setupTestDB(t, pool)
	ss := NewSegmentStore(pool)
	ps := NewProjectStore(pool)
	ctx := context.Background()

	project, err := ps.Create(ctx, "seg-test", "Seg Test", "")
	if err != nil {
		t.Fatal(err)
	}

	conditions := []model.Condition{
		{Attribute: "plan", Operator: "equals", Value: "enterprise"},
	}
	condJSON, _ := json.Marshal(conditions)

	seg, err := ss.Create(ctx, project.ID, "beta-users", "Beta Users", "Beta testing group", condJSON)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if seg.Key != "beta-users" {
		t.Errorf("expected key 'beta-users', got %q", seg.Key)
	}
	if seg.Name != "Beta Users" {
		t.Errorf("expected name 'Beta Users', got %q", seg.Name)
	}
	if len(seg.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(seg.Conditions))
	}

	got, err := ss.GetByKey(ctx, project.ID, "beta-users")
	if err != nil {
		t.Fatalf("GetByKey: %v", err)
	}
	if got.ID != seg.ID {
		t.Errorf("IDs don't match")
	}
}

func TestSegmentStore_List(t *testing.T) {
	pool := testPool(t)
	setupTestDB(t, pool)
	ss := NewSegmentStore(pool)
	ps := NewProjectStore(pool)
	ctx := context.Background()

	project, _ := ps.Create(ctx, "seg-list", "Seg List", "")
	cond, _ := json.Marshal([]model.Condition{{Attribute: "a", Operator: "equals", Value: "b"}})

	ss.Create(ctx, project.ID, "seg-a", "Seg A", "", cond)
	ss.Create(ctx, project.ID, "seg-b", "Seg B", "", cond)

	list, err := ss.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 segments, got %d", len(list))
	}
}

func TestSegmentStore_Update(t *testing.T) {
	pool := testPool(t)
	setupTestDB(t, pool)
	ss := NewSegmentStore(pool)
	ps := NewProjectStore(pool)
	ctx := context.Background()

	project, _ := ps.Create(ctx, "seg-update", "Seg Update", "")
	cond, _ := json.Marshal([]model.Condition{{Attribute: "a", Operator: "equals", Value: "b"}})
	seg, _ := ss.Create(ctx, project.ID, "seg-upd", "Old Name", "", cond)

	newCond, _ := json.Marshal([]model.Condition{{Attribute: "x", Operator: "contains", Value: "y"}})
	updated, err := ss.Update(ctx, seg.ID, "New Name", "Updated desc", newCond)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", updated.Name)
	}
	if updated.Description != "Updated desc" {
		t.Errorf("expected description 'Updated desc', got %q", updated.Description)
	}
}

func TestSegmentStore_Delete(t *testing.T) {
	pool := testPool(t)
	setupTestDB(t, pool)
	ss := NewSegmentStore(pool)
	ps := NewProjectStore(pool)
	ctx := context.Background()

	project, _ := ps.Create(ctx, "seg-del", "Seg Del", "")
	cond, _ := json.Marshal([]model.Condition{{Attribute: "a", Operator: "equals", Value: "b"}})
	seg, _ := ss.Create(ctx, project.ID, "seg-del-1", "To Delete", "", cond)

	err := ss.Delete(ctx, seg.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = ss.GetByKey(ctx, project.ID, "seg-del-1")
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestSegmentStore -v`
Expected: FAIL — `NewSegmentStore` not defined.

**Step 3: Write the SegmentStore implementation**

Create `internal/store/segment_store.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type SegmentStore struct {
	pool *pgxpool.Pool
}

func NewSegmentStore(pool *pgxpool.Pool) *SegmentStore {
	return &SegmentStore{pool: pool}
}

func (s *SegmentStore) Create(ctx context.Context, projectID, key, name, description string, conditions json.RawMessage) (*model.Segment, error) {
	var seg model.Segment
	var condJSON []byte
	err := s.pool.QueryRow(ctx,
		`INSERT INTO segments (project_id, key, name, description, conditions)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, project_id, key, name, description, conditions, created_at, updated_at`,
		projectID, key, name, description, conditions,
	).Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating segment: %w", err)
	}
	json.Unmarshal(condJSON, &seg.Conditions)
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	return &seg, nil
}

func (s *SegmentStore) GetByKey(ctx context.Context, projectID, key string) (*model.Segment, error) {
	var seg model.Segment
	var condJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key, name, description, conditions, created_at, updated_at
		 FROM segments WHERE project_id = $1 AND key = $2`,
		projectID, key,
	).Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding segment by key: %w", err)
	}
	json.Unmarshal(condJSON, &seg.Conditions)
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	return &seg, nil
}

func (s *SegmentStore) ListByProject(ctx context.Context, projectID string) ([]model.Segment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, description, conditions, created_at, updated_at
		 FROM segments WHERE project_id = $1 ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing segments: %w", err)
	}
	defer rows.Close()

	var segments []model.Segment
	for rows.Next() {
		var seg model.Segment
		var condJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning segment: %w", err)
		}
		json.Unmarshal(condJSON, &seg.Conditions)
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating segments: %w", err)
	}
	return segments, nil
}

func (s *SegmentStore) Update(ctx context.Context, segmentID, name, description string, conditions json.RawMessage) (*model.Segment, error) {
	var seg model.Segment
	var condJSON []byte
	err := s.pool.QueryRow(ctx,
		`UPDATE segments SET name=$2, description=$3, conditions=$4, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, conditions, created_at, updated_at`,
		segmentID, name, description, conditions,
	).Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating segment: %w", err)
	}
	json.Unmarshal(condJSON, &seg.Conditions)
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	return &seg, nil
}

func (s *SegmentStore) Delete(ctx context.Context, segmentID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM segments WHERE id = $1`, segmentID)
	if err != nil {
		return fmt.Errorf("deleting segment: %w", err)
	}
	return nil
}

// ListByProjectKey returns all segments for a project identified by project key.
// Used by the evaluation cache to load segments.
func (s *SegmentStore) ListByProjectKey(ctx context.Context, projectKey string) ([]model.Segment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT s.id, s.project_id, s.key, s.name, s.description, s.conditions, s.created_at, s.updated_at
		 FROM segments s
		 JOIN projects p ON p.id = s.project_id
		 WHERE p.key = $1
		 ORDER BY s.created_at DESC`,
		projectKey,
	)
	if err != nil {
		return nil, fmt.Errorf("listing segments by project key: %w", err)
	}
	defer rows.Close()

	var segments []model.Segment
	for rows.Next() {
		var seg model.Segment
		var condJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning segment: %w", err)
		}
		json.Unmarshal(condJSON, &seg.Conditions)
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating segments: %w", err)
	}
	return segments, nil
}

// FindReferencingFlags scans targeting_rules JSONB across all flag_environment_configs
// in a project to find flags that contain a segment_match condition referencing the given segment key.
// Returns a list of flag keys.
func (s *SegmentStore) FindReferencingFlags(ctx context.Context, projectID, segmentKey string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT f.key
		 FROM flags f
		 JOIN flag_environment_configs fec ON fec.flag_id = f.id
		 WHERE f.project_id = $1
		   AND fec.targeting_rules @> $2::jsonb`,
		projectID, fmt.Sprintf(`[{"conditions":[{"operator":"segment_match","value":"%s"}]}]`, segmentKey),
	)
	if err != nil {
		return nil, fmt.Errorf("finding referencing flags: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scanning flag key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating flag keys: %w", err)
	}
	return keys, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestSegmentStore -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/segment_store.go internal/store/segment_store_test.go
git commit -m "feat: add SegmentStore with CRUD and reference lookup (#31)"
```

---

### Task 4: Evaluation Engine — Segment-Aware Condition Matching

**Files:**
- Modify: `internal/evaluation/engine.go:19,40,68-77` (add segments param to Evaluate + matchesAllConditions)
- Modify: `internal/evaluation/cache.go:14-25,28-32,39-83,87-115,119-124,127-137,140-145` (add segment data to cache)
- Test: `internal/evaluation/engine_test.go` (add segment tests)

**Step 1: Write failing test for segment_match evaluation**

Add to `internal/evaluation/engine_test.go`:

```go
func TestEngine_SegmentMatchCondition(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "", Operator: "segment_match", Value: "beta-users"},
			},
			Variant: "on",
		},
	})

	segments := map[string]model.Segment{
		"beta-users": {
			Key: "beta-users",
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
				{Attribute: "beta_opted_in", Operator: "equals", Value: "true"},
			},
		},
	}

	t.Run("matches segment conditions", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-1",
			Attributes: map[string]any{
				"plan":          "enterprise",
				"beta_opted_in": "true",
			},
		}
		result := engine.EvaluateWithSegments(flag, config, ctx, segments)
		if result.Reason != "rule_match" {
			t.Errorf("expected reason 'rule_match', got %q", result.Reason)
		}
		if result.Variant != "on" {
			t.Errorf("expected variant 'on', got %q", result.Variant)
		}
	})

	t.Run("does not match segment conditions", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-2",
			Attributes: map[string]any{
				"plan":          "free",
				"beta_opted_in": "true",
			},
		}
		result := engine.EvaluateWithSegments(flag, config, ctx, segments)
		if result.Reason != "default" {
			t.Errorf("expected reason 'default', got %q", result.Reason)
		}
	})

	t.Run("segment not found - fails silently", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-3",
			Attributes: map[string]any{
				"plan": "enterprise",
			},
		}
		result := engine.EvaluateWithSegments(flag, config, ctx, map[string]model.Segment{})
		if result.Reason != "default" {
			t.Errorf("expected reason 'default' for missing segment, got %q", result.Reason)
		}
	})

	t.Run("segment_match mixed with inline conditions", func(t *testing.T) {
		mixedConfig := makeConfig(true, "off", []model.Variant{
			{Key: "off", Value: rawJSON(false)},
			{Key: "on", Value: rawJSON(true)},
		}, []model.TargetingRule{
			{
				Conditions: []model.Condition{
					{Attribute: "", Operator: "segment_match", Value: "beta-users"},
					{Attribute: "country", Operator: "equals", Value: "US"},
				},
				Variant: "on",
			},
		})

		ctx := &model.EvaluationContext{
			UserID: "user-4",
			Attributes: map[string]any{
				"plan":          "enterprise",
				"beta_opted_in": "true",
				"country":       "US",
			},
		}
		result := engine.EvaluateWithSegments(flag, mixedConfig, ctx, segments)
		if result.Reason != "rule_match" {
			t.Errorf("expected 'rule_match', got %q", result.Reason)
		}

		// Same user but wrong country
		ctx.Attributes["country"] = "UK"
		result = engine.EvaluateWithSegments(flag, mixedConfig, ctx, segments)
		if result.Reason != "default" {
			t.Errorf("expected 'default' when inline condition fails, got %q", result.Reason)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/evaluation/ -run TestEngine_SegmentMatch -v`
Expected: FAIL — `EvaluateWithSegments` not defined.

**Step 3: Implement segment-aware evaluation**

Modify `internal/evaluation/engine.go`:

1. Add `EvaluateWithSegments` method that accepts a segments map.
2. Make `Evaluate` call `EvaluateWithSegments` with nil segments for backward compatibility.
3. Update `matchesAllConditions` to accept segments and handle `segment_match` operator.

The updated `engine.go` should have:

```go
// Evaluate evaluates a flag without segment support (backward compatible).
func (e *Engine) Evaluate(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext) *model.EvaluationResult {
	return e.EvaluateWithSegments(flag, config, ctx, nil)
}

// EvaluateWithSegments evaluates a flag with segment resolution support.
func (e *Engine) EvaluateWithSegments(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext, segments map[string]model.Segment) *model.EvaluationResult {
	// ... same logic as Evaluate but passes segments to matchesAllConditions
}
```

Update `matchesAllConditions` signature:

```go
func matchesAllConditions(conditions []model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) bool {
	for _, cond := range conditions {
		if cond.Operator == string(model.OpSegmentMatch) {
			segKey, ok := cond.Value.(string)
			if !ok {
				return false
			}
			if segments == nil {
				return false
			}
			seg, exists := segments[segKey]
			if !exists {
				return false
			}
			// Evaluate segment conditions (no segments param to prevent nesting)
			if !matchesAllConditions(seg.Conditions, ctx, nil) {
				return false
			}
			continue
		}
		attrValue := ctx.Attributes[cond.Attribute]
		if !EvaluateCondition(attrValue, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}
```

**Step 4: Run all evaluation tests**

Run: `go test ./internal/evaluation/ -v`
Expected: ALL PASS (both old tests and new segment tests).

**Step 5: Commit**

```bash
git add internal/evaluation/engine.go internal/evaluation/engine_test.go
git commit -m "feat: add segment_match condition evaluation (#31)"
```

---

### Task 5: Cache — Load Segments Into Memory

**Files:**
- Modify: `internal/evaluation/cache.go` (add segment storage, loading, and accessors)

**Step 1: Write failing test for segment cache**

Add to a new file `internal/evaluation/cache_test.go` or append to engine_test.go:

```go
func TestCache_SegmentStorage(t *testing.T) {
	cache := NewCache()

	segments := map[string]model.Segment{
		"beta": {Key: "beta", Conditions: []model.Condition{{Attribute: "plan", Operator: "equals", Value: "pro"}}},
	}
	cache.SetSegments("myproject", segments)

	got := cache.GetSegments("myproject")
	if got == nil {
		t.Fatal("expected segments, got nil")
	}
	if _, ok := got["beta"]; !ok {
		t.Error("expected 'beta' segment")
	}

	// Non-existent project
	got = cache.GetSegments("other")
	if got != nil {
		t.Error("expected nil for non-existent project")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/evaluation/ -run TestCache_Segment -v`
Expected: FAIL — `SetSegments`/`GetSegments` not defined.

**Step 3: Implement segment cache**

Add to `Cache` struct in `internal/evaluation/cache.go`:

```go
type Cache struct {
	mu   sync.RWMutex
	data map[string]map[string]FlagData
	// segments: projectKey -> segmentKey -> Segment
	segments map[string]map[string]model.Segment
}
```

Update `NewCache`:

```go
func NewCache() *Cache {
	return &Cache{
		data:     make(map[string]map[string]FlagData),
		segments: make(map[string]map[string]model.Segment),
	}
}
```

Add methods:

```go
func (c *Cache) GetSegments(projectKey string) map[string]model.Segment {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.segments[projectKey]
}

func (c *Cache) SetSegments(projectKey string, segments map[string]model.Segment) {
	c.mu.Lock()
	c.segments[projectKey] = segments
	c.mu.Unlock()
}
```

Note: `LoadAll` and `Refresh` will be updated in Task 6 to actually load segments from DB. For now the `SetSegments` method is sufficient for testing and for the evaluate handler to use.

**Step 4: Run tests**

Run: `go test ./internal/evaluation/ -v`
Expected: ALL PASS.

**Step 5: Commit**

```bash
git add internal/evaluation/cache.go internal/evaluation/cache_test.go
git commit -m "feat: add segment storage to evaluation cache (#31)"
```

---

### Task 6: Cache — Load Segments from Database

**Files:**
- Modify: `internal/evaluation/cache.go:51-83` (LoadAll), `internal/evaluation/cache.go:85-115` (Refresh)

The cache currently loads only flags. We need `LoadAll` and `Refresh` to also load segments. Since segments are project-scoped (not per-environment), loading is slightly different.

**Step 1: Update LoadAll to also load segments**

After the existing flag-loading logic in `LoadAll`, add segment loading:

```go
// Load segments (project-scoped)
segRows, err := pool.Query(ctx,
	`SELECT p.key AS project_key, s.id, s.project_id, s.key, s.name, s.description, s.conditions, s.created_at, s.updated_at
	 FROM segments s
	 JOIN projects p ON p.id = s.project_id`)
if err != nil {
	return fmt.Errorf("cache LoadAll segments query: %w", err)
}
defer segRows.Close()

newSegments := make(map[string]map[string]model.Segment)
for segRows.Next() {
	var projectKey string
	var seg model.Segment
	var condJSON []byte
	if err := segRows.Scan(&projectKey, &seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
		return fmt.Errorf("cache LoadAll segment scan: %w", err)
	}
	json.Unmarshal(condJSON, &seg.Conditions)
	if seg.Conditions == nil {
		seg.Conditions = []model.Condition{}
	}
	if newSegments[projectKey] == nil {
		newSegments[projectKey] = make(map[string]model.Segment)
	}
	newSegments[projectKey][seg.Key] = seg
}
if err := segRows.Err(); err != nil {
	return fmt.Errorf("cache LoadAll segments rows: %w", err)
}
```

Then in the lock section, also set segments:

```go
c.mu.Lock()
c.data = newData
c.segments = newSegments
c.mu.Unlock()
```

**Step 2: Add RefreshSegments method**

Add a method to refresh segments for a single project:

```go
func (c *Cache) RefreshSegments(ctx context.Context, pool *pgxpool.Pool, projectKey string) error {
	rows, err := pool.Query(ctx,
		`SELECT s.id, s.project_id, s.key, s.name, s.description, s.conditions, s.created_at, s.updated_at
		 FROM segments s
		 JOIN projects p ON p.id = s.project_id
		 WHERE p.key = $1`, projectKey)
	if err != nil {
		return fmt.Errorf("cache RefreshSegments query: %w", err)
	}
	defer rows.Close()

	segs := make(map[string]model.Segment)
	for rows.Next() {
		var seg model.Segment
		var condJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &condJSON, &seg.CreatedAt, &seg.UpdatedAt); err != nil {
			return fmt.Errorf("cache RefreshSegments scan: %w", err)
		}
		json.Unmarshal(condJSON, &seg.Conditions)
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segs[seg.Key] = seg
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("cache RefreshSegments rows: %w", err)
	}

	c.mu.Lock()
	c.segments[projectKey] = segs
	c.mu.Unlock()
	return nil
}
```

**Step 3: Verify build and existing tests pass**

Run: `go test ./internal/evaluation/ -v && go build ./...`
Expected: ALL PASS.

**Step 4: Commit**

```bash
git add internal/evaluation/cache.go
git commit -m "feat: load segments into evaluation cache from database (#31)"
```

---

### Task 7: Evaluate Handler — Use Segments from Cache

**Files:**
- Modify: `internal/handler/evaluate_handler.go:56-68` (EvaluateAll), `internal/handler/evaluate_handler.go:73-94` (EvaluateSingle)

**Step 1: Update EvaluateAll to pass segments**

In `EvaluateAll`, after getting flags from cache, also get segments:

```go
segments := h.cache.GetSegments(sdkKey.ProjectKey)
```

Then change the evaluate call:

```go
results[flagKey] = h.engine.EvaluateWithSegments(&fd.Flag, &fd.Config, evalCtx, segments)
```

**Step 2: Update EvaluateSingle similarly**

```go
segments := h.cache.GetSegments(sdkKey.ProjectKey)
result := h.engine.EvaluateWithSegments(&fd.Flag, &fd.Config, evalCtx, segments)
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add internal/handler/evaluate_handler.go
git commit -m "feat: pass segments to evaluation engine from SDK handlers (#31)"
```

---

### Task 8: Segment Handler — HTTP API

**Files:**
- Create: `internal/handler/segment_handler.go`

**Step 1: Write the SegmentHandler**

Create `internal/handler/segment_handler.go`:

```go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

type SegmentHandler struct {
	segments     *store.SegmentStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
}

func NewSegmentHandler(segments *store.SegmentStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool) *SegmentHandler {
	return &SegmentHandler{segments: segments, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool}
}

var segmentKeyRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

func validateSegmentConditions(conditions []model.Condition) string {
	if len(conditions) == 0 {
		return "at least one condition is required"
	}
	for _, c := range conditions {
		if c.Operator == string(model.OpSegmentMatch) {
			return "segment conditions cannot reference other segments"
		}
	}
	return ""
}

// List handles GET /api/v1/projects/{key}/segments
func (h *SegmentHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	segments, err := h.segments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list segments")
		return
	}
	if segments == nil {
		segments = []model.Segment{}
	}
	writeJSON(w, http.StatusOK, segments)
}

// Create handles POST /api/v1/projects/{key}/segments
func (h *SegmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Key         string          `json:"key"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Conditions  json.RawMessage `json:"conditions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}
	if !segmentKeyRegex.MatchString(req.Key) {
		writeError(w, http.StatusBadRequest, "key must be 3-64 lowercase alphanumeric characters and hyphens")
		return
	}

	// Parse and validate conditions
	var conditions []model.Condition
	if err := json.Unmarshal(req.Conditions, &conditions); err != nil {
		writeError(w, http.StatusBadRequest, "invalid conditions format")
		return
	}
	if msg := validateSegmentConditions(conditions); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	seg, err := h.segments.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.Conditions)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "segment key already exists for this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create segment")
		return
	}

	// Audit log
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(seg)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "create",
			EntityType: "segment",
			EntityID:   seg.Key,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh segment cache
	h.refreshSegmentCache(r.Context(), projectKey)

	writeJSON(w, http.StatusCreated, seg)
}

// Get handles GET /api/v1/projects/{key}/segments/{segmentKey}
func (h *SegmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	segmentKey := r.PathValue("segmentKey")

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	seg, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	writeJSON(w, http.StatusOK, seg)
}

// Update handles PUT /api/v1/projects/{key}/segments/{segmentKey}
func (h *SegmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	segmentKey := r.PathValue("segmentKey")

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	seg, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	var req struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Conditions  json.RawMessage `json:"conditions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var conditions []model.Condition
	if err := json.Unmarshal(req.Conditions, &conditions); err != nil {
		writeError(w, http.StatusBadRequest, "invalid conditions format")
		return
	}
	if msg := validateSegmentConditions(conditions); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	updated, err := h.segments.Update(r.Context(), seg.ID, req.Name, req.Description, req.Conditions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update segment")
		return
	}

	// Audit log
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(seg)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "segment",
			EntityID:   seg.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh segment cache and broadcast to all environments
	h.refreshSegmentCacheAndBroadcast(r.Context(), projectKey, project.ID)

	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/projects/{key}/segments/{segmentKey}
func (h *SegmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	segmentKey := r.PathValue("segmentKey")

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	seg, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	// Check for referencing flags
	refs, err := h.segments.FindReferencingFlags(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check segment usage")
		return
	}
	if len(refs) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":            "segment is referenced by active flags",
			"referencing_flags": refs,
		})
		return
	}

	if err := h.segments.Delete(r.Context(), seg.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete segment")
		return
	}

	// Audit log
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(seg)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "delete",
			EntityType: "segment",
			EntityID:   seg.Key,
			OldValue:   oldVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refreshSegmentCache(r.Context(), projectKey)

	w.WriteHeader(http.StatusNoContent)
}

// Usage handles GET /api/v1/projects/{key}/segments/{segmentKey}/usage
func (h *SegmentHandler) Usage(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	segmentKey := r.PathValue("segmentKey")

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	refs, err := h.segments.FindReferencingFlags(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find segment usage")
		return
	}
	if refs == nil {
		refs = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"referencing_flags": refs,
	})
}

func (h *SegmentHandler) refreshSegmentCache(ctx context.Context, projectKey string) {
	if err := h.cache.RefreshSegments(ctx, h.pool, projectKey); err != nil {
		slog.Warn("failed to refresh segment cache", "project", projectKey, "error", err)
	}
}

func (h *SegmentHandler) refreshSegmentCacheAndBroadcast(ctx context.Context, projectKey, projectID string) {
	h.refreshSegmentCache(ctx, projectKey)

	envs, err := h.environments.ListByProject(ctx, projectID)
	if err != nil {
		slog.Warn("failed to list environments for segment broadcast", "error", err)
		return
	}
	for _, env := range envs {
		h.hub.Broadcast(projectKey, env.Key, stream.Event{
			Type: "flag_update",
		})
	}
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add internal/handler/segment_handler.go
git commit -m "feat: add SegmentHandler with CRUD, usage, and delete safety (#31)"
```

---

### Task 9: Wire Up — Register Routes in main.go

**Files:**
- Modify: `cmd/togglerino/main.go:53-93` (add segment store + handler init), `cmd/togglerino/main.go:145-167` (add segment routes)

**Step 1: Add segment store initialization**

In `main.go`, after `unknownFlagStore` (line 63), add:

```go
segmentStore := store.NewSegmentStore(pool)
```

**Step 2: Add segment handler initialization**

After `unknownFlagHandler` (line 92), add:

```go
segmentHandler := handler.NewSegmentHandler(segmentStore, projectStore, environmentStore, auditStore, hub, cache, pool)
```

**Step 3: Add segment routes**

After the context attributes route (line 167), add:

```go
// Segments
mux.Handle("GET /api/v1/projects/{key}/segments", wrap(segmentHandler.List, sessionAuth))
mux.Handle("POST /api/v1/projects/{key}/segments", wrap(segmentHandler.Create, sessionAuth))
mux.Handle("GET /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Get, sessionAuth))
mux.Handle("PUT /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Update, sessionAuth))
mux.Handle("DELETE /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Delete, sessionAuth))
mux.Handle("GET /api/v1/projects/{key}/segments/{segmentKey}/usage", wrap(segmentHandler.Usage, sessionAuth))
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: Build succeeds.

**Step 5: Run all Go tests**

Run: `go test ./...`
Expected: ALL PASS.

**Step 6: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: wire up segment routes in main.go (#31)"
```

---

### Task 10: Frontend — Types and API Client

**Files:**
- Modify: `web/src/api/types.ts:117` (add Segment interface)
- Modify: `web/src/api/client.ts:24-31` (add segment API functions)

**Step 1: Add Segment interface to types.ts**

After `ContextAttribute` interface (line 117), add:

```typescript
export interface Segment {
  id: string
  project_id: string
  key: string
  name: string
  description: string
  conditions: Condition[]
  created_at: string
  updated_at: string
}
```

**Step 2: Add segment API functions to client.ts**

Add to the `api` object:

```typescript
// Segments
segments: {
  list: (projectKey: string) => api.get<Segment[]>(`/projects/${projectKey}/segments`),
  get: (projectKey: string, segmentKey: string) => api.get<Segment>(`/projects/${projectKey}/segments/${segmentKey}`),
  create: (projectKey: string, body: { key: string; name: string; description: string; conditions: Condition[] }) =>
    api.post<Segment>(`/projects/${projectKey}/segments`, body),
  update: (projectKey: string, segmentKey: string, body: { name: string; description: string; conditions: Condition[] }) =>
    api.put<Segment>(`/projects/${projectKey}/segments/${segmentKey}`, body),
  delete: (projectKey: string, segmentKey: string) =>
    api.delete<void>(`/projects/${projectKey}/segments/${segmentKey}`),
  usage: (projectKey: string, segmentKey: string) =>
    api.get<{ referencing_flags: string[] }>(`/projects/${projectKey}/segments/${segmentKey}/usage`),
},
```

Note: You'll need to import the `Segment` and `Condition` types at the top of `client.ts`.

**Step 3: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat: add Segment types and API client functions (#31)"
```

---

### Task 11: Frontend — Segments List Page

**Files:**
- Create: `web/src/pages/SegmentsPage.tsx`
- Modify: `web/src/App.tsx:72-80` (add route)
- Modify: `web/src/components/ProjectLayout.tsx:27-31` (add nav link)

**Step 1: Create SegmentsPage**

Create `web/src/pages/SegmentsPage.tsx` — a page that lists all segments for the current project with a "Create Segment" button. Each segment row shows key, name, condition count, and last updated. Clicking a segment opens an edit dialog. The create button opens a create dialog.

Use the same patterns as other pages in the codebase:
- `useParams` to get `key`
- `useQuery` from TanStack Query to fetch segments
- `useMutation` for create/update/delete with `queryClient.invalidateQueries`
- shadcn/ui components (Button, Input, Dialog, Table)
- Reuse the condition builder from `RuleBuilder` for editing segment conditions (extract into a shared `ConditionBuilder` component or inline)

The page should include:
- Segment list table
- Create dialog with: key (input), name (input), description (textarea), conditions (same UI as RuleBuilder conditions but without `segment_match` in operator dropdown)
- Edit dialog with: name, description, conditions, and a "Used by" section showing referencing flags
- Delete button (shows error if segment is referenced)

**Step 2: Add route in App.tsx**

In `App.tsx`, add the import and route inside the `ProjectLayout` routes (after line 79, `settings` route):

```tsx
import SegmentsPage from './pages/SegmentsPage.tsx'
// ...
<Route path="segments" element={<SegmentsPage />} />
```

**Step 3: Add nav link in ProjectLayout.tsx**

In `ProjectLayout.tsx`, inside the `navLinks` function (after the "Environments" NavLink on line 29), add:

```tsx
<NavLink to={`/projects/${key}/segments`} className={navLinkClass} onClick={onNavigate}>Segments</NavLink>
```

**Step 4: Verify frontend builds and lint passes**

Run: `cd web && npm run build && npm run lint`
Expected: Build and lint succeed.

**Step 5: Commit**

```bash
git add web/src/pages/SegmentsPage.tsx web/src/App.tsx web/src/components/ProjectLayout.tsx
git commit -m "feat: add Segments management page with CRUD (#31)"
```

---

### Task 12: Frontend — RuleBuilder Segment Support

**Files:**
- Modify: `web/src/components/RuleBuilder.tsx:14-60` (add Segment operator group), `web/src/components/RuleBuilder.tsx:147-204` (condition row rendering)

**Step 1: Add segment_match to operator groups**

In `RuleBuilder.tsx`, add a new operator group after the existing groups (after line 59, "Pattern" group):

```typescript
{
  label: 'Segment',
  operators: [
    { value: 'segment_match', label: 'matches segment' },
  ],
},
```

**Step 2: Update Props to accept projectKey**

Add `projectKey` to the Props interface:

```typescript
interface Props {
  rules: TargetingRule[]
  variants: Variant[]
  onChange: (rules: TargetingRule[]) => void
  projectKey?: string  // needed for segment picker
}
```

**Step 3: Update condition row rendering**

When `cond.operator === 'segment_match'`, replace the attribute input and value input with a segment picker. Use a `<select>` that fetches segments via `useQuery`:

```tsx
{cond.operator === 'segment_match' ? (
  <SegmentPicker
    projectKey={projectKey}
    value={String(cond.value ?? '')}
    onChange={(segKey) => updateCondition(ruleIdx, condIdx, { attribute: '', value: segKey })}
  />
) : (
  // existing attribute + value inputs
)}
```

Create a small `SegmentPicker` component (inline or separate file) that fetches segments and renders a `<select>`:

```tsx
function SegmentPicker({ projectKey, value, onChange }: { projectKey?: string; value: string; onChange: (v: string) => void }) {
  const { data: segments } = useQuery({
    queryKey: ['segments', projectKey],
    queryFn: () => api.segments.list(projectKey!),
    enabled: !!projectKey,
  })

  return (
    <select
      className="flex-1 px-2.5 py-1.5 text-xs border rounded-md bg-input text-foreground outline-none cursor-pointer"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      <option value="">Select segment...</option>
      {segments?.map((s) => (
        <option key={s.key} value={s.key}>{s.name} ({s.key})</option>
      ))}
    </select>
  )
}
```

**Step 4: Update all call sites of RuleBuilder to pass projectKey**

Find `ConfigEditor.tsx` where `RuleBuilder` is used and pass `projectKey` from the route params.

**Step 5: Verify frontend builds and lint passes**

Run: `cd web && npm run build && npm run lint`
Expected: Build and lint succeed.

**Step 6: Commit**

```bash
git add web/src/components/RuleBuilder.tsx web/src/components/ConfigEditor.tsx
git commit -m "feat: add segment_match support to rule builder (#31)"
```

---

### Task 13: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update documentation**

Update the following sections in `CLAUDE.md`:
- Add `segment` to the `store` package description
- Add segment routes to the API Routes table
- Add segments to the Key Patterns section
- Update the Database section to mention the `segments` table
- Update migration count

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with segments feature (#31)"
```

---

### Task 14: Full Integration Verification

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: ALL PASS.

**Step 2: Build the full binary**

Run: `cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: Build succeeds.

**Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors.

**Step 4: Manual smoke test (if DB available)**

Start the app with `docker compose up` and verify:
1. Navigate to a project → "Segments" in sidebar
2. Create a segment with conditions
3. Create a flag with a `segment_match` targeting rule
4. Verify evaluation works via SDK endpoint
