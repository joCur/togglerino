# Environment Deletion Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to delete any environment (including defaults) with a last-environment guard, cascade cleanup, audit logging, and webhook events.

**Architecture:** New DELETE endpoint on `EnvironmentHandler` with `project:settings` permission. Database `ON DELETE CASCADE` handles dependent record cleanup. Frontend adds delete button with confirmation dialog. Cache eviction removes stale entries.

**Tech Stack:** Go (stdlib net/http, pgx/v5), React 19 (TanStack Query, shadcn/ui), PostgreSQL

**Spec:** `docs/superpowers/specs/2026-03-12-environment-deletion-design.md`

---

## Chunk 1: Backend

### Task 1: Add `Evict` method to evaluation cache

**Files:**
- Modify: `internal/evaluation/cache.go:336` (after existing override methods)
- Test: `internal/evaluation/cache_test.go`

- [ ] **Step 1: Write the failing test**

In `internal/evaluation/cache_test.go`, add:

```go
func TestCache_Evict(t *testing.T) {
	c := NewCache()

	// Populate data and overrides for two environments
	c.Set("proj", "dev", map[string]FlagData{
		"flag1": {Flag: model.Flag{Key: "flag1"}},
	})
	c.Set("proj", "staging", map[string]FlagData{
		"flag2": {Flag: model.Flag{Key: "flag2"}},
	})
	c.SetOverride("proj", "dev", "user1", "flag1", []byte(`true`), nil)
	c.SetOverride("proj", "staging", "user1", "flag2", []byte(`false`), nil)

	// Evict dev environment
	c.Evict("proj", "dev")

	// dev data and overrides should be gone
	if flags := c.GetFlags("proj", "dev"); flags != nil {
		t.Errorf("expected nil flags for evicted env, got %v", flags)
	}
	if val, ok := c.GetOverride("proj", "dev", "user1", "flag1"); ok {
		t.Errorf("expected no override for evicted env, got %v", val)
	}

	// staging should be untouched
	if flags := c.GetFlags("proj", "staging"); flags == nil {
		t.Error("expected staging flags to still exist")
	}
	if _, ok := c.GetOverride("proj", "staging", "user1", "flag2"); !ok {
		t.Error("expected staging override to still exist")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/evaluation/... -run TestCache_Evict -v`
Expected: FAIL — `c.Evict undefined`

- [ ] **Step 3: Implement `Evict` method**

Add to `internal/evaluation/cache.go` after the `DeleteOverridesForUser` method (~line 387):

```go
// Evict removes all cached data and overrides for a specific project/environment.
// Called when an environment is deleted.
func (c *Cache) Evict(projectKey, envKey string) {
	key := cacheKey(projectKey, envKey)
	c.mu.Lock()
	delete(c.data, key)
	delete(c.overrides, key)
	c.mu.Unlock()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/evaluation/... -run TestCache_Evict -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/evaluation/cache.go internal/evaluation/cache_test.go
git commit -m "feat(evaluation): add Evict method to cache for environment deletion"
```

---

### Task 2: Add `DeleteIfNotLast` transactional method to environment store

**Files:**
- Modify: `internal/store/environment_store.go:91` (after `Delete` method)
- Test: `internal/store/environment_store_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/store/environment_store_test.go`, add (follow existing test patterns — use `testPool()` helper):

```go
func TestEnvironmentStore_DeleteIfNotLast(t *testing.T) {
	pool := testPool(t)
	envStore := NewEnvironmentStore(pool)
	projectStore := NewProjectStore(pool)
	ctx := context.Background()

	project, err := projectStore.Create(ctx, "delete-guard-test", "Delete Guard Test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	// Create default environments (development, staging, production)
	if err := envStore.CreateDefaultEnvironments(ctx, project.ID); err != nil {
		t.Fatalf("creating defaults: %v", err)
	}

	envs, _ := envStore.ListByProject(ctx, project.ID)
	if len(envs) != 3 {
		t.Fatalf("expected 3 envs, got %d", len(envs))
	}

	// Delete first env — should succeed
	if err := envStore.DeleteIfNotLast(ctx, envs[0].ID, project.ID); err != nil {
		t.Fatalf("expected success deleting first env: %v", err)
	}

	// Delete second env — should succeed
	if err := envStore.DeleteIfNotLast(ctx, envs[1].ID, project.ID); err != nil {
		t.Fatalf("expected success deleting second env: %v", err)
	}

	// Delete last env — should fail with ErrLastEnvironment
	err = envStore.DeleteIfNotLast(ctx, envs[2].ID, project.ID)
	if err != store.ErrLastEnvironment {
		t.Fatalf("expected ErrLastEnvironment, got: %v", err)
	}

	// Verify the last env still exists
	remaining, _ := envStore.ListByProject(ctx, project.ID)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining env, got %d", len(remaining))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestEnvironmentStore_DeleteIfNotLast -v`
Expected: FAIL — `envStore.DeleteIfNotLast undefined`

- [ ] **Step 3: Implement `DeleteIfNotLast`**

Add to `internal/store/environment_store.go` after the `Delete` method (~line 91):

```go
// ErrLastEnvironment is returned when attempting to delete the last environment in a project.
var ErrLastEnvironment = fmt.Errorf("cannot delete the last environment")

// DeleteIfNotLast deletes an environment in a transaction, guarding against deleting the last one.
func (s *EnvironmentStore) DeleteIfNotLast(ctx context.Context, envID, projectID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM environments WHERE project_id = $1`, projectID,
	).Scan(&count); err != nil {
		return fmt.Errorf("counting environments: %w", err)
	}
	if count <= 1 {
		return ErrLastEnvironment
	}

	tag, err := tx.Exec(ctx, `DELETE FROM environments WHERE id = $1 AND project_id = $2`, envID, projectID)
	if err != nil {
		return fmt.Errorf("deleting environment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("environment not found")
	}

	return tx.Commit(ctx)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -run TestEnvironmentStore_DeleteIfNotLast -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store/environment_store.go internal/store/environment_store_test.go
git commit -m "feat(store): add DeleteIfNotLast with transactional guard"
```

---

### Task 3: Add `environment.deleted` webhook event

**Files:**
- Modify: `internal/webhook/event.go:33` (add constant and register in map)

- [ ] **Step 1: Add the event constant and register it**

In `internal/webhook/event.go`, add after line 33 (`EventEnvironmentCreated`):

```go
EventEnvironmentDeleted = "environment.deleted"
```

And add to the `ValidEventTypes` map:

```go
EventEnvironmentDeleted: true,
```

- [ ] **Step 2: Run existing tests to verify nothing breaks**

Run: `go test ./internal/webhook/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/webhook/event.go
git commit -m "feat(webhook): add environment.deleted event type"
```

---

### Task 4: Add `Delete` handler to `EnvironmentHandler`

**Files:**
- Modify: `internal/handler/environment_handler.go` (extend struct, constructor, add Delete method)

- [ ] **Step 1: Extend the handler struct and constructor**

In `internal/handler/environment_handler.go`, update the struct and constructor:

```go
type EnvironmentHandler struct {
	environments *store.EnvironmentStore
	projects     *store.ProjectStore
	webhooks     *webhook.Dispatcher
	audit        *store.AuditStore
	cache        *evaluation.Cache
}

func NewEnvironmentHandler(environments *store.EnvironmentStore, projects *store.ProjectStore, webhooks *webhook.Dispatcher, audit *store.AuditStore, cache *evaluation.Cache) *EnvironmentHandler {
	return &EnvironmentHandler{environments: environments, projects: projects, webhooks: webhooks, audit: audit, cache: cache}
}
```

Add the `evaluation` import:

```go
"github.com/togglerino/togglerino/internal/evaluation"
```

- [ ] **Step 2: Update `main.go` constructor call**

In `cmd/togglerino/main.go`, update the `NewEnvironmentHandler` call (line ~163):

```go
environmentHandler := handler.NewEnvironmentHandler(environmentStore, projectStore, webhookDispatcher, auditStore, cache)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./cmd/togglerino`
Expected: SUCCESS (no errors)

- [ ] **Step 4: Implement the Delete handler**

Add to `internal/handler/environment_handler.go`:

```go
// Delete handles DELETE /api/v1/projects/{key}/environments/{envKey}
func (h *EnvironmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	envKey := r.PathValue("envKey")
	if projectKey == "" || envKey == "" {
		writeError(w, http.StatusBadRequest, "project key and environment key are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	// Transactional delete with last-environment guard
	if err := h.environments.DeleteIfNotLast(r.Context(), env.ID, project.ID); err != nil {
		if errors.Is(err, store.ErrLastEnvironment) {
			writeError(w, http.StatusConflict, "cannot delete the last environment")
			return
		}
		slog.Error("failed to delete environment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete environment")
		return
	}

	// Evict cache
	h.cache.Evict(projectKey, envKey)

	// Audit log (best-effort)
	user, _ := auth.UserFromContext(r.Context())
	if user != nil {
		oldVal, _ := json.Marshal(env)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "delete",
			EntityType: "environment",
			EntityID:   env.Key,
			OldValue:   oldVal,
		}); err != nil {
			slog.Error("failed to record audit entry", "error", err)
		}
	}

	// Webhook (best-effort)
	if h.webhooks != nil {
		envJSON, _ := json.Marshal(env)
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventEnvironmentDeleted,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    envJSON,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
```

Add to imports: `"errors"`, `"github.com/togglerino/togglerino/internal/auth"`, and `"github.com/togglerino/togglerino/internal/model"` (if not already present).

- [ ] **Step 5: Register the route in `main.go`**

In `cmd/togglerino/main.go`, add after the existing environment routes (~line 271):

```go
mux.Handle("DELETE /api/v1/projects/{key}/environments/{envKey}", wrap(environmentHandler.Delete, sessionAuth, requireProjectSettings))
```

- [ ] **Step 6: Verify compilation**

Run: `go build ./cmd/togglerino`
Expected: SUCCESS

- [ ] **Step 7: Commit**

```bash
git add internal/handler/environment_handler.go cmd/togglerino/main.go
git commit -m "feat(handler): add DELETE endpoint for environment deletion"
```

---

### Task 5: Backend integration test

**Files:**
- Test: `internal/handler/environment_handler_test.go`

- [ ] **Step 1: Write integration test**

Check existing handler test patterns first (look at `internal/handler/` for test files). Write a test that:

1. Creates a project with default environments
2. Deletes one environment → expects 204
3. Verifies the environment is gone (list returns 2)
4. Tries to delete remaining environments until one is left
5. Tries to delete the last one → expects 409

Follow the test setup patterns from existing handler tests in the same package (test helpers, auth context setup, etc.).

- [ ] **Step 2: Run the test**

Run: `go test ./internal/handler/... -run TestEnvironmentHandler_Delete -v`
Expected: PASS

- [ ] **Step 3: Run all tests to check for regressions**

Run: `go test ./...`
Expected: PASS (the constructor change in main.go should not break existing tests since handler tests construct their own handlers)

- [ ] **Step 4: Commit**

```bash
git add internal/handler/environment_handler_test.go
git commit -m "test: add integration tests for environment deletion endpoint"
```

---

## Chunk 2: Frontend

### Task 6: Add environment delete API function

**Files:**
- Modify: `web/src/api/client.ts:84` (in the `environments` section)

- [ ] **Step 1: Add delete function to api client**

In `web/src/api/client.ts`, in the `environments` object (after the `reorder` function at ~line 88), add:

```typescript
delete: (projectKey: string, envKey: string) =>
  request<void>(`/projects/${projectKey}/environments/${envKey}`, { method: 'DELETE' }),
```

- [ ] **Step 2: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(web): add environment delete API function"
```

---

### Task 7: Add delete UI to EnvironmentsPage

**Files:**
- Modify: `web/src/pages/EnvironmentsPage.tsx`

- [ ] **Step 1: Add delete functionality**

Update `web/src/pages/EnvironmentsPage.tsx` with these changes:

1. Add imports:

```typescript
import { ArrowUp, ArrowDown, Trash2 } from 'lucide-react'
import { useIsProjectAdmin } from '@/hooks/usePermissions'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
```

2. Add permission hook and state inside the component (after existing state declarations):

```typescript
const canDelete = useIsProjectAdmin(key)
const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null)
const [sdkKeyCount, setSdkKeyCount] = useState<number | null>(null)
```

3. Add delete mutation (after `reorderMutation`):

```typescript
const deleteMutation = useMutation({
  mutationFn: (envKey: string) => api.environments.delete(key!, envKey),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments'] })
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
    setDeleteTarget(null)
    setSdkKeyCount(null)
  },
})
```

4. Add delete click handler (after `handleCreate`):

```typescript
const handleDeleteClick = async (env: Environment) => {
  setDeleteTarget(env)
  deleteMutation.reset()
  try {
    const keys = await api.get<SDKKey[]>(`/projects/${key}/environments/${env.key}/sdk-keys`)
    setSdkKeyCount(keys.length)
  } catch {
    setSdkKeyCount(0)
  }
}
```

Also add `SDKKey` to the type imports:

```typescript
import type { Environment, SDKKey } from '../api/types.ts'
```

5. Add an "Actions" column header in the table (after the SDK Keys column):

```typescript
{canDelete && <TableHead className="font-mono text-[11px] uppercase tracking-wider w-[60px]" />}
```

6. Add delete button cell in each row (after the SDK Keys cell):

```typescript
{canDelete && (
  <TableCell>
    <Button
      variant="ghost"
      size="sm"
      className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
      onClick={() => handleDeleteClick(env)}
    >
      <Trash2 className="w-3.5 h-3.5" />
    </Button>
  </TableCell>
)}
```

7. Add confirmation dialog (before the closing `</div>` of the component):

```typescript
<Dialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) { setDeleteTarget(null); setSdkKeyCount(null); deleteMutation.reset() } }}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Delete environment</DialogTitle>
      <DialogDescription>
        {sdkKeyCount && sdkKeyCount > 0
          ? `This environment has ${sdkKeyCount} active SDK key${sdkKeyCount === 1 ? '' : 's'}. Deleting it will revoke all keys and remove all flag configurations for this environment. This cannot be undone.`
          : `All flag configurations for the "${deleteTarget?.name}" environment will be removed. This cannot be undone.`}
      </DialogDescription>
    </DialogHeader>
    {deleteMutation.error && (
      <Alert variant="destructive">
        <AlertDescription>
          {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete environment'}
        </AlertDescription>
      </Alert>
    )}
    <DialogFooter>
      <Button variant="outline" onClick={() => { setDeleteTarget(null); setSdkKeyCount(null); deleteMutation.reset() }}>
        Cancel
      </Button>
      <Button
        variant="destructive"
        disabled={deleteMutation.isPending}
        onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.key)}
      >
        {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

- [ ] **Step 2: Verify frontend builds**

Run: `cd web && npm run build`
Expected: SUCCESS

- [ ] **Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS (no new lint errors)

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/EnvironmentsPage.tsx
git commit -m "feat(web): add environment deletion with confirmation dialog"
```

---

## Chunk 3: Documentation

### Task 8: Update documentation

**Files:**
- Modify: `docs-site/docs/api-reference/` (find the environments API doc)
- Modify: `docs-site/docs/dashboard/` (find the environments dashboard doc)

- [ ] **Step 1: Find and update relevant doc pages**

Search `docs-site/docs/` for environment-related documentation. Add:

- The new `DELETE /api/v1/projects/{key}/environments/{envKey}` endpoint to the API reference
- Environment deletion instructions to the dashboard guide
- Note the last-environment guard and cascade behavior

- [ ] **Step 2: Verify docs build**

Run: `cd docs-site && npm run build`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add docs-site/
git commit -m "docs: add environment deletion to API reference and dashboard guide"
```

---

### Task 9: Final verification

- [ ] **Step 1: Run all backend tests**

Run: `go test ./...`
Expected: ALL PASS

- [ ] **Step 2: Run frontend build and lint**

Run: `cd web && npm run build && npm run lint`
Expected: SUCCESS

- [ ] **Step 3: Run docs build**

Run: `cd docs-site && npm run build`
Expected: SUCCESS

- [ ] **Step 4: Manual smoke test (optional)**

Start the dev environment and verify:
1. Navigate to a project's Environments page
2. Delete button visible for project admins
3. Clicking delete shows confirmation dialog
4. Deleting an environment works and updates the list
5. Cannot delete the last environment (409 error shown)
