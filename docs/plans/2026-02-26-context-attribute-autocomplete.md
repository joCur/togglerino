# Context Attribute Autocomplete Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track context attributes from SDK evaluation requests and surface them as autocomplete suggestions in the rule builder UI.

**Architecture:** New `context_attributes` DB table tracks attribute names per project. The evaluate handler fires an async goroutine to upsert attributes after each evaluation. A new management API endpoint lists known attributes. The frontend replaces the attribute `<Input>` with a combobox.

**Tech Stack:** Go (pgx/v5, net/http), React 19, TanStack Query, shadcn/ui (Popover + Command), Tailwind CSS v4

---

### Task 1: Database migration

**Files:**
- Create: `migrations/003_context_attributes.up.sql`
- Create: `migrations/003_context_attributes.down.sql`

**Step 1: Create the up migration**

```sql
-- migrations/003_context_attributes.up.sql
CREATE TABLE context_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE INDEX idx_context_attributes_project ON context_attributes(project_id);
```

**Step 2: Create the down migration**

```sql
-- migrations/003_context_attributes.down.sql
DROP TABLE IF EXISTS context_attributes;
```

**Step 3: Commit**

```bash
git add migrations/003_context_attributes.up.sql migrations/003_context_attributes.down.sql
git commit -m "feat: add context_attributes migration"
```

---

### Task 2: Context attribute model

**Files:**
- Modify: `internal/model/flag.go`

**Step 1: Add the ContextAttribute struct**

Add to the end of `internal/model/flag.go`:

```go
type ContextAttribute struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Name       string    `json:"name"`
	LastSeenAt time.Time `json:"last_seen_at"`
}
```

**Step 2: Commit**

```bash
git add internal/model/flag.go
git commit -m "feat: add ContextAttribute model"
```

---

### Task 3: Context attribute store with tests

**Files:**
- Create: `internal/store/context_attribute_store.go`
- Create: `internal/store/context_attribute_store_test.go`

**Step 1: Write the failing tests**

Create `internal/store/context_attribute_store_test.go`:

```go
package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestContextAttributeStore_UpsertAndList(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	// Create a project
	key := uniqueKey("ctx-attr")
	project, err := ps.Create(ctx, key, "Context Attr Project", "for context attr tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Upsert some attributes
	err = cas.UpsertByProjectKey(ctx, key, []string{"country", "plan", "email"})
	if err != nil {
		t.Fatalf("UpsertByProjectKey: %v", err)
	}

	// List by project
	attrs, err := cas.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}

	if len(attrs) != 3 {
		t.Fatalf("expected 3 attributes, got %d", len(attrs))
	}

	// Verify alphabetical order
	expected := []string{"country", "email", "plan"}
	for i, a := range attrs {
		if a.Name != expected[i] {
			t.Errorf("attr[%d]: expected %q, got %q", i, expected[i], a.Name)
		}
		if a.ID == "" {
			t.Error("expected non-empty ID")
		}
		if a.LastSeenAt.IsZero() {
			t.Error("expected non-zero LastSeenAt")
		}
	}
}

func TestContextAttributeStore_UpsertUpdatesLastSeen(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	key := uniqueKey("ctx-upd")
	_, err := ps.Create(ctx, key, "Update Project", "for update tests")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// First upsert
	err = cas.UpsertByProjectKey(ctx, key, []string{"country"})
	if err != nil {
		t.Fatalf("First UpsertByProjectKey: %v", err)
	}

	// Second upsert — should not error (ON CONFLICT updates last_seen_at)
	err = cas.UpsertByProjectKey(ctx, key, []string{"country", "plan"})
	if err != nil {
		t.Fatalf("Second UpsertByProjectKey: %v", err)
	}
}

func TestContextAttributeStore_ListByProject_Empty(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	key := uniqueKey("ctx-empty")
	project, err := ps.Create(ctx, key, "Empty Attr Project", "no attributes")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	attrs, err := cas.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if attrs != nil {
		t.Errorf("expected nil for empty result, got %d attributes", len(attrs))
	}
}

func TestContextAttributeStore_UpsertEmptySlice(t *testing.T) {
	pool := testPool(t)
	cas := store.NewContextAttributeStore(pool)
	ctx := context.Background()

	// Upsert with empty slice should not error
	err := cas.UpsertByProjectKey(ctx, "nonexistent-key", []string{})
	if err != nil {
		t.Fatalf("UpsertByProjectKey with empty slice: %v", err)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestContextAttributeStore -v`
Expected: compilation error — `store.NewContextAttributeStore` does not exist

**Step 3: Implement the store**

Create `internal/store/context_attribute_store.go`:

```go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type ContextAttributeStore struct {
	pool *pgxpool.Pool
}

func NewContextAttributeStore(pool *pgxpool.Pool) *ContextAttributeStore {
	return &ContextAttributeStore{pool: pool}
}

// UpsertByProjectKey inserts or updates context attributes for a project identified by key.
// Uses a single query that resolves the project key to ID and unnests the attribute names.
func (s *ContextAttributeStore) UpsertByProjectKey(ctx context.Context, projectKey string, names []string) error {
	if len(names) == 0 {
		return nil
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO context_attributes (project_id, name)
		 SELECT p.id, unnest($2::text[])
		 FROM projects p WHERE p.key = $1
		 ON CONFLICT (project_id, name) DO UPDATE SET last_seen_at = NOW()`,
		projectKey, names,
	)
	if err != nil {
		return fmt.Errorf("upserting context attributes: %w", err)
	}
	return nil
}

// ListByProject returns all context attributes for a project, ordered alphabetically by name.
func (s *ContextAttributeStore) ListByProject(ctx context.Context, projectID string) ([]model.ContextAttribute, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, name, last_seen_at
		 FROM context_attributes WHERE project_id = $1 ORDER BY name`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing context attributes: %w", err)
	}
	defer rows.Close()

	var attrs []model.ContextAttribute
	for rows.Next() {
		var a model.ContextAttribute
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scanning context attribute: %w", err)
		}
		attrs = append(attrs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating context attributes: %w", err)
	}
	return attrs, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestContextAttributeStore -v`
Expected: all 4 tests PASS

**Step 5: Commit**

```bash
git add internal/store/context_attribute_store.go internal/store/context_attribute_store_test.go
git commit -m "feat: add ContextAttributeStore with tests"
```

---

### Task 4: Async attribute tracking in evaluate handler

**Files:**
- Modify: `internal/handler/evaluate_handler.go`

**Step 1: Add ContextAttributeStore dependency and tracking**

Modify `internal/handler/evaluate_handler.go`:

1. Add `contextAttrs *store.ContextAttributeStore` field to `EvaluateHandler` struct
2. Update `NewEvaluateHandler` to accept and store the new dependency
3. Add a `trackAttributes` method
4. Call `trackAttributes` from both `EvaluateAll` and `EvaluateSingle`

The updated file should look like:

```go
package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type EvaluateHandler struct {
	cache        *evaluation.Cache
	engine       *evaluation.Engine
	contextAttrs *store.ContextAttributeStore
}

func NewEvaluateHandler(cache *evaluation.Cache, engine *evaluation.Engine, contextAttrs *store.ContextAttributeStore) *EvaluateHandler {
	return &EvaluateHandler{cache: cache, engine: engine, contextAttrs: contextAttrs}
}

// ... (evaluateRequest, evaluateAllResponse structs unchanged)

// EvaluateAll evaluates all flags for the SDK key's project/environment.
// POST /api/v1/evaluate
func (h *EvaluateHandler) EvaluateAll(w http.ResponseWriter, r *http.Request) {
	sdkKey := auth.SDKKeyFromContext(r.Context())
	evalCtx := h.parseContext(r)

	h.trackAttributes(sdkKey.ProjectKey, evalCtx)

	flags := h.cache.GetFlags(sdkKey.ProjectKey, sdkKey.EnvironmentKey)
	results := make(map[string]*model.EvaluationResult, len(flags))
	for flagKey, fd := range flags {
		results[flagKey] = h.engine.Evaluate(&fd.Flag, &fd.Config, evalCtx)
	}

	writeJSON(w, http.StatusOK, evaluateAllResponse{Flags: results})
}

// EvaluateSingle evaluates a single flag for the SDK key's project/environment.
// POST /api/v1/evaluate/{flag}
func (h *EvaluateHandler) EvaluateSingle(w http.ResponseWriter, r *http.Request) {
	flagKey := r.PathValue("flag")
	sdkKey := auth.SDKKeyFromContext(r.Context())
	evalCtx := h.parseContext(r)

	h.trackAttributes(sdkKey.ProjectKey, evalCtx)

	fd, ok := h.cache.GetFlag(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey)
	if !ok {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	result := h.engine.Evaluate(&fd.Flag, &fd.Config, evalCtx)
	writeJSON(w, http.StatusOK, result)
}

// trackAttributes asynchronously records attribute names from the evaluation context.
func (h *EvaluateHandler) trackAttributes(projectKey string, evalCtx *model.EvaluationContext) {
	if len(evalCtx.Attributes) == 0 {
		return
	}

	names := make([]string, 0, len(evalCtx.Attributes))
	for k := range evalCtx.Attributes {
		names = append(names, k)
	}

	go func() {
		if err := h.contextAttrs.UpsertByProjectKey(context.Background(), projectKey, names); err != nil {
			slog.Error("tracking context attributes", "error", err, "project", projectKey)
		}
	}()
}

// parseContext reads the evaluation context from the request body. (unchanged)
```

**Step 2: Update main.go to wire up the new dependency**

In `cmd/togglerino/main.go`:

1. Create the store after other stores: `contextAttributeStore := store.NewContextAttributeStore(pool)`
2. Update `NewEvaluateHandler` call: `handler.NewEvaluateHandler(cache, engine, contextAttributeStore)`

Specifically change:
```go
// Before:
evaluateHandler := handler.NewEvaluateHandler(cache, engine)

// After:
contextAttributeStore := store.NewContextAttributeStore(pool)
evaluateHandler := handler.NewEvaluateHandler(cache, engine, contextAttributeStore)
```

**Step 3: Verify compilation**

Run: `go build ./cmd/togglerino`
Expected: builds successfully

**Step 4: Commit**

```bash
git add internal/handler/evaluate_handler.go cmd/togglerino/main.go
git commit -m "feat: track context attributes from SDK evaluation requests"
```

---

### Task 5: Context attribute API handler with tests

**Files:**
- Create: `internal/handler/context_attribute_handler.go`
- Modify: `cmd/togglerino/main.go` (add route)

**Step 1: Create the handler**

Create `internal/handler/context_attribute_handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ContextAttributeHandler struct {
	contextAttrs *store.ContextAttributeStore
	projects     *store.ProjectStore
}

func NewContextAttributeHandler(contextAttrs *store.ContextAttributeStore, projects *store.ProjectStore) *ContextAttributeHandler {
	return &ContextAttributeHandler{contextAttrs: contextAttrs, projects: projects}
}

// List handles GET /api/v1/projects/{key}/context-attributes
func (h *ContextAttributeHandler) List(w http.ResponseWriter, r *http.Request) {
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

	attrs, err := h.contextAttrs.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list context attributes")
		return
	}
	if attrs == nil {
		attrs = []model.ContextAttribute{}
	}

	writeJSON(w, http.StatusOK, attrs)
}
```

**Step 2: Wire up the route in main.go**

In `cmd/togglerino/main.go`, add after `auditHandler` initialization:

```go
contextAttributeHandler := handler.NewContextAttributeHandler(contextAttributeStore, projectStore)
```

Add the route after the audit log route:

```go
// Context attributes
mux.Handle("GET /api/v1/projects/{key}/context-attributes", wrap(contextAttributeHandler.List, sessionAuth))
```

**Step 3: Verify compilation**

Run: `go build ./cmd/togglerino`
Expected: builds successfully

**Step 4: Commit**

```bash
git add internal/handler/context_attribute_handler.go cmd/togglerino/main.go
git commit -m "feat: add context attributes API endpoint"
```

---

### Task 6: Frontend — add shadcn/ui Command and Popover components

**Files:**
- Create: `web/src/components/ui/popover.tsx` (via shadcn CLI)
- Create: `web/src/components/ui/command.tsx` (via shadcn CLI)

**Step 1: Install the shadcn components**

```bash
cd web && npx shadcn@latest add popover command
```

This will install the necessary Radix UI dependencies and create the component files.

**Step 2: Verify the components were added**

Check that `web/src/components/ui/popover.tsx` and `web/src/components/ui/command.tsx` exist.

**Step 3: Commit**

```bash
cd web && git add -A && git commit -m "feat: add shadcn popover and command components"
```

---

### Task 7: Frontend — API client and types

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

**Step 1: Add ContextAttribute type**

Add to `web/src/api/types.ts`:

```typescript
export interface ContextAttribute {
  id: string
  project_id: string
  name: string
  last_seen_at: string
}
```

**Step 2: Add API function**

The API client uses a generic `api.get<T>(path)` pattern. No changes needed to `client.ts` — the caller will use `api.get<ContextAttribute[]>` directly, same pattern as all other endpoints in the codebase.

**Step 3: Commit**

```bash
git add web/src/api/types.ts
git commit -m "feat: add ContextAttribute frontend type"
```

---

### Task 8: Frontend — AttributeCombobox component

**Files:**
- Create: `web/src/components/AttributeCombobox.tsx`

**Step 1: Create the combobox component**

Create `web/src/components/AttributeCombobox.tsx`:

```tsx
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { ContextAttribute } from '../api/types.ts'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'

interface Props {
  value: string
  onChange: (value: string) => void
}

export default function AttributeCombobox({ value, onChange }: Props) {
  const { key: projectKey } = useParams<{ key: string }>()
  const [open, setOpen] = useState(false)
  const [inputValue, setInputValue] = useState('')

  const { data: attributes } = useQuery({
    queryKey: ['projects', projectKey, 'context-attributes'],
    queryFn: () => api.get<ContextAttribute[]>(`/projects/${projectKey}/context-attributes`),
    enabled: !!projectKey,
    staleTime: 30_000,
  })

  const suggestions = attributes?.map((a) => a.name) ?? []

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          role="combobox"
          aria-expanded={open}
          className="flex-1 flex items-center px-3 py-1.5 text-xs border rounded-md bg-input text-foreground text-left outline-none cursor-pointer hover:border-foreground/30 transition-colors min-w-0 h-9"
        >
          <span className={value ? 'text-foreground' : 'text-muted-foreground/60'}>
            {value || 'e.g. user_id, email, plan'}
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-[240px] p-0" align="start">
        <Command>
          <CommandInput
            placeholder="Search or type attribute..."
            value={inputValue}
            onValueChange={setInputValue}
          />
          <CommandList>
            <CommandEmpty>
              {inputValue ? (
                <button
                  type="button"
                  className="w-full px-2 py-1.5 text-xs text-left hover:bg-accent rounded cursor-pointer"
                  onClick={() => {
                    onChange(inputValue)
                    setOpen(false)
                    setInputValue('')
                  }}
                >
                  Use "<span className="font-medium">{inputValue}</span>"
                </button>
              ) : (
                <span className="text-xs text-muted-foreground">No known attributes yet.</span>
              )}
            </CommandEmpty>
            {suggestions.length > 0 && (
              <CommandGroup heading="Known attributes">
                {suggestions.map((name) => (
                  <CommandItem
                    key={name}
                    value={name}
                    onSelect={(val) => {
                      onChange(val)
                      setOpen(false)
                      setInputValue('')
                    }}
                  >
                    {name}
                  </CommandItem>
                ))}
              </CommandGroup>
            )}
            {inputValue && !suggestions.includes(inputValue) && suggestions.length > 0 && (
              <CommandGroup heading="Custom">
                <CommandItem
                  value={`custom-${inputValue}`}
                  onSelect={() => {
                    onChange(inputValue)
                    setOpen(false)
                    setInputValue('')
                  }}
                >
                  Use "{inputValue}"
                </CommandItem>
              </CommandGroup>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
```

**Step 2: Verify no lint errors**

Run: `cd web && npm run lint`
Expected: no errors

**Step 3: Commit**

```bash
git add web/src/components/AttributeCombobox.tsx
git commit -m "feat: add AttributeCombobox component"
```

---

### Task 9: Frontend — integrate combobox into RuleBuilder

**Files:**
- Modify: `web/src/components/RuleBuilder.tsx`

**Step 1: Replace the attribute Input with AttributeCombobox**

In `web/src/components/RuleBuilder.tsx`:

1. Add import: `import AttributeCombobox from './AttributeCombobox.tsx'`
2. Replace the attribute `<Input>` (lines 150-155) with:

```tsx
<AttributeCombobox
  value={cond.attribute}
  onChange={(val) => updateCondition(ruleIdx, condIdx, { attribute: val })}
/>
```

The `<Input>` block to replace:
```tsx
<Input
  className="flex-1 text-xs"
  placeholder="e.g. user_id, email, plan"
  value={cond.attribute}
  onChange={(e) => updateCondition(ruleIdx, condIdx, { attribute: e.target.value })}
/>
```

**Step 2: Verify no lint errors**

Run: `cd web && npm run lint`
Expected: no errors

**Step 3: Verify frontend builds**

Run: `cd web && npm run build`
Expected: builds successfully

**Step 4: Commit**

```bash
git add web/src/components/RuleBuilder.tsx
git commit -m "feat: replace attribute input with combobox in RuleBuilder"
```

---

### Task 10: Full build verification

**Files:** None (verification only)

**Step 1: Build frontend**

Run: `cd web && npm run build`
Expected: builds successfully

**Step 2: Build Go binary**

Run: `go build -o togglerino ./cmd/togglerino`
Expected: builds successfully

**Step 3: Run all Go tests**

Run: `go test ./...`
Expected: all tests pass (requires running PostgreSQL via `docker compose up`)

**Step 4: Run frontend lint**

Run: `cd web && npm run lint`
Expected: no errors
