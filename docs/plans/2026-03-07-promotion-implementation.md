# Environment Promotion Workflow Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a promotion workflow that copies flag environment configs forward through environments with diff preview, audit trail, and configurable environment ordering.

**Architecture:** Add `sort_order` column to environments table. New `POST .../promote` endpoint copies `default_variant`, `variants`, `targeting_rules` from source to target env (preserving target's `enabled` state). Frontend adds promote button with diff dialog on the flag detail page, and reorder UI on the environments settings page.

**Tech Stack:** Go (stdlib net/http, pgx/v5), React 19 + TypeScript + TanStack Query, PostgreSQL 16

---

### Task 1: Migration — add sort_order to environments

**Files:**
- Create: `migrations/020_environment_sort_order.up.sql`
- Create: `migrations/020_environment_sort_order.down.sql`

**Step 1: Write the migration files**

`020_environment_sort_order.up.sql`:
```sql
ALTER TABLE environments ADD COLUMN sort_order integer NOT NULL DEFAULT 0;

WITH ranked AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at) - 1 AS rn
  FROM environments
)
UPDATE environments SET sort_order = ranked.rn FROM ranked WHERE environments.id = ranked.id;
```

`020_environment_sort_order.down.sql`:
```sql
ALTER TABLE environments DROP COLUMN sort_order;
```

**Step 2: Verify migration applies**

Run: `./dev.sh` (restarts backend, runs migrations)
Expected: Backend starts without migration errors, `sort_order` column exists.

Verify: `psql postgres://togglerino:togglerino@localhost:5432/togglerino -c "SELECT key, sort_order FROM environments LIMIT 5;"`
Expected: Rows with sequential sort_order values per project.

**Step 3: Commit**

```bash
git add migrations/020_environment_sort_order.up.sql migrations/020_environment_sort_order.down.sql
git commit -m "feat: add sort_order column to environments table (#47)"
```

---

### Task 2: Update Environment model and store to include sort_order

**Files:**
- Modify: `internal/model/environment.go` — add `SortOrder int` field
- Modify: `internal/store/environment_store.go` — update all queries to include `sort_order`, add `UpdateOrder` method, update `ListByProject` to order by `sort_order`, update `Create` to auto-assign next sort_order
- Modify: `internal/store/environment_store_test.go` — add test for sort_order and UpdateOrder

**Step 1: Write the failing test for sort_order in ListByProject**

Add to `internal/store/environment_store_test.go`:
```go
func TestEnvironmentStore_SortOrder(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, ps)

	env1, err := es.Create(ctx, projectID, "development", "Development")
	if err != nil {
		t.Fatalf("Create env1: %v", err)
	}
	env2, err := es.Create(ctx, projectID, "staging", "Staging")
	if err != nil {
		t.Fatalf("Create env2: %v", err)
	}
	env3, err := es.Create(ctx, projectID, "production", "Production")
	if err != nil {
		t.Fatalf("Create env3: %v", err)
	}

	envs, err := es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(envs) != 3 {
		t.Fatalf("expected 3 environments, got %d", len(envs))
	}

	// Verify sort_order is sequential
	for i, env := range envs {
		if env.SortOrder != i {
			t.Errorf("env %q: sort_order got %d, want %d", env.Key, env.SortOrder, i)
		}
	}

	// Test UpdateOrder — reverse the order
	err = es.UpdateOrder(ctx, projectID, []string{env3.ID, env2.ID, env1.ID})
	if err != nil {
		t.Fatalf("UpdateOrder: %v", err)
	}

	envs, err = es.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject after reorder: %v", err)
	}
	if envs[0].Key != "production" || envs[1].Key != "staging" || envs[2].Key != "development" {
		t.Errorf("unexpected order after reorder: %v", []string{envs[0].Key, envs[1].Key, envs[2].Key})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestEnvironmentStore_SortOrder -v`
Expected: FAIL — `SortOrder` field doesn't exist on `model.Environment`.

**Step 3: Update model**

In `internal/model/environment.go`, add `SortOrder` to the `Environment` struct:
```go
type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}
```

**Step 4: Update store queries**

In `internal/store/environment_store.go`:

1. Update `Create` to auto-assign next sort_order:
```go
func (s *EnvironmentStore) Create(ctx context.Context, projectID, key, name string) (*model.Environment, error) {
	var e model.Environment
	err := s.pool.QueryRow(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order)
		 VALUES ($1, $2, $3, COALESCE((SELECT MAX(sort_order) + 1 FROM environments WHERE project_id = $1), 0))
		 RETURNING id, project_id, key, name, sort_order, created_at`,
		projectID, key, name,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.SortOrder, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating environment: %w", err)
	}
	return &e, nil
}
```

2. Update `ListByProject` to order by `sort_order` and scan it:
```go
func (s *EnvironmentStore) ListByProject(ctx context.Context, projectID string) ([]model.Environment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, sort_order, created_at FROM environments WHERE project_id = $1 ORDER BY sort_order`,
		projectID,
	)
	// ... scan including e.SortOrder
}
```

3. Update `FindByKey`, `FindByID` to include `sort_order` in SELECT and Scan.

4. Add `UpdateOrder`:
```go
func (s *EnvironmentStore) UpdateOrder(ctx context.Context, projectID string, environmentIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i, id := range environmentIDs {
		_, err := tx.Exec(ctx,
			`UPDATE environments SET sort_order = $1 WHERE id = $2 AND project_id = $3`,
			i, id, projectID,
		)
		if err != nil {
			return fmt.Errorf("updating sort_order for environment %s: %w", id, err)
		}
	}

	return tx.Commit(ctx)
}
```

5. Update `CreateDefaultEnvironments` to set sequential sort_order:
```go
for i, d := range defaults {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO environments (project_id, key, name, sort_order) VALUES ($1, $2, $3, $4)`,
		projectID, d.key, d.name, i,
	)
	// ...
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/store/... -run TestEnvironmentStore_SortOrder -v`
Expected: PASS

**Step 6: Run all store tests to check for regressions**

Run: `go test ./internal/store/... -v`
Expected: All existing tests pass (update any that break from the new Scan field).

**Step 7: Commit**

```bash
git add internal/model/environment.go internal/store/environment_store.go internal/store/environment_store_test.go
git commit -m "feat: add sort_order to environment model and store (#47)"
```

---

### Task 3: Environment reorder API endpoint

**Files:**
- Modify: `internal/handler/environment_handler.go` — add `UpdateOrder` handler
- Modify: `cmd/togglerino/main.go` — register route
- Test manually via curl or write a handler test

**Step 1: Add UpdateOrder handler**

In `internal/handler/environment_handler.go`:
```go
// UpdateOrder handles PUT /api/v1/projects/{key}/environments/order
func (h *EnvironmentHandler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
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
		EnvironmentIDs []string `json:"environment_ids"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.EnvironmentIDs) == 0 {
		writeError(w, http.StatusBadRequest, "environment_ids is required")
		return
	}

	if err := h.environments.UpdateOrder(r.Context(), project.ID, req.EnvironmentIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update environment order")
		return
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	writeJSON(w, http.StatusOK, envs)
}
```

**Step 2: Register route**

In `cmd/togglerino/main.go`, near line 221 (after existing environment routes):
```go
mux.Handle("PUT /api/v1/projects/{key}/environments/order", wrap(environmentHandler.UpdateOrder, sessionAuth, requireEnvsWrite))
```

**Step 3: Verify build compiles**

Run: `go build ./cmd/togglerino`
Expected: Compiles without errors.

**Step 4: Commit**

```bash
git add internal/handler/environment_handler.go cmd/togglerino/main.go
git commit -m "feat: add environment reorder API endpoint (#47)"
```

---

### Task 4: Promote endpoint — backend

**Files:**
- Modify: `internal/handler/flag_handler.go` — add `PromoteEnvironmentConfig` handler
- Modify: `cmd/togglerino/main.go` — register route

**Step 1: Add PromoteEnvironmentConfig handler**

In `internal/handler/flag_handler.go`:
```go
// PromoteEnvironmentConfig handles POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote
func (h *FlagHandler) PromoteEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	envKey := r.PathValue("env")
	if projectKey == "" || flagKey == "" || envKey == "" {
		writeError(w, http.StatusBadRequest, "project key, flag key, and environment key are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	sourceEnv, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "source environment not found")
		return
	}

	var req struct {
		TargetEnvironment string `json:"target_environment"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetEnvironment == "" {
		writeError(w, http.StatusBadRequest, "target_environment is required")
		return
	}

	targetEnv, err := h.environments.FindByKey(r.Context(), project.ID, req.TargetEnvironment)
	if err != nil {
		writeError(w, http.StatusNotFound, "target environment not found")
		return
	}

	// Enforce forward-only promotion
	if targetEnv.SortOrder <= sourceEnv.SortOrder {
		writeError(w, http.StatusBadRequest, "can only promote forward to a higher-order environment")
		return
	}

	// Fetch source config
	sourceConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, sourceEnv.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get source environment config")
		return
	}

	// Fetch target config (for audit old_value + preserving enabled state)
	targetConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, targetEnv.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get target environment config")
		return
	}

	// Preserve target's enabled state, copy everything else from source
	preservedEnabled := false
	if targetConfig != nil {
		preservedEnabled = targetConfig.Enabled
	}

	variantsJSON, _ := json.Marshal(sourceConfig.Variants)
	rulesJSON, _ := json.Marshal(sourceConfig.TargetingRules)

	var updatedBy *string
	if user := auth.UserFromContext(r.Context()); user != nil {
		updatedBy = &user.ID
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(
		r.Context(), flag.ID, targetEnv.ID,
		preservedEnabled, sourceConfig.DefaultVariant,
		variantsJSON, rulesJSON, updatedBy,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to promote environment config")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if targetConfig != nil {
			oldVal, _ = json.Marshal(targetConfig)
		}
		newValMap := map[string]any{
			"config":             cfg,
			"promoted_from_env":  sourceEnv.Key,
		}
		newVal, _ := json.Marshal(newValMap)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &targetEnv.ID,
			Action:        "promoted",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record promotion audit log", "error", err)
		}
	}

	// Refresh cache and broadcast SSE event for target env
	if err := h.cache.Refresh(r.Context(), h.pool, projectKey, req.TargetEnvironment); err != nil {
		slog.Warn("failed to refresh cache after promotion", "error", err)
	}
	h.hub.Broadcast(projectKey, req.TargetEnvironment, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
		Value:   cfg.Enabled,
		Variant: cfg.DefaultVariant,
	})

	writeJSON(w, http.StatusOK, cfg)
}
```

**Step 2: Register route**

In `cmd/togglerino/main.go`, after line 238 (after the UpdateEnvironmentConfig route):
```go
mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote", wrap(flagHandler.PromoteEnvironmentConfig, sessionAuth, requireFlagsWrite))
```

**Step 3: Verify build compiles**

Run: `go build ./cmd/togglerino`
Expected: Compiles without errors.

**Step 4: Commit**

```bash
git add internal/handler/flag_handler.go cmd/togglerino/main.go
git commit -m "feat: add flag environment config promote endpoint (#47)"
```

---

### Task 5: Frontend — update Environment type and add promote API call

**Files:**
- Modify: `web/src/api/types.ts` — add `sort_order` to `Environment` interface
- Modify: `web/src/api/client.ts` — add `promote` and `reorderEnvironments` API methods

**Step 1: Update Environment type**

In `web/src/api/types.ts`, update the `Environment` interface (line 27):
```ts
export interface Environment {
  id: string
  project_id: string
  key: string
  name: string
  sort_order: number
  created_at: string
}
```

**Step 2: Add API methods**

In `web/src/api/client.ts`, add to the `api` object:
```ts
environments: {
  reorder: (projectKey: string, environmentIds: string[]) =>
    request<Environment[]>(`/projects/${projectKey}/environments/order`, {
      method: 'PUT',
      body: JSON.stringify({ environment_ids: environmentIds }),
    }),
  promote: (projectKey: string, flagKey: string, sourceEnvKey: string, targetEnvKey: string) =>
    request<FlagEnvironmentConfig>(`/projects/${projectKey}/flags/${flagKey}/environments/${sourceEnvKey}/promote`, {
      method: 'POST',
      body: JSON.stringify({ target_environment: targetEnvKey }),
    }),
},
```

Add `FlagEnvironmentConfig` to the import in `client.ts`.

**Step 3: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat: add promote and reorder API calls to frontend client (#47)"
```

---

### Task 6: Frontend — promote button and diff dialog on FlagDetailPage

**Files:**
- Create: `web/src/components/PromoteDialog.tsx` — diff preview dialog
- Modify: `web/src/pages/FlagDetailPage.tsx` — add promote button per environment

**Step 1: Create PromoteDialog component**

Create `web/src/components/PromoteDialog.tsx`. This component:
- Takes `sourceConfig`, `targetConfig`, `sourceEnvName`, `targetEnvName`, `open`, `onOpenChange`, `onConfirm`, `isLoading` props
- Shows a dialog with side-by-side comparison of:
  - `default_variant`: old → new
  - `variants`: lists differences
  - `targeting_rules`: lists differences
  - `enabled`: shows "preserved (unchanged)"
- Has a "Confirm Promotion" button that calls `onConfirm`

Use existing `Dialog` components from `@/components/ui/dialog`. Keep the diff simple — show JSON for variants/rules since they can be complex. Use `JSON.stringify(value, null, 2)` for readable display.

**Step 2: Add promote button to FlagDetailPage**

In `web/src/pages/FlagDetailPage.tsx`, for each environment's config section:
- After the existing save button, add a "Promote to →" dropdown button
- The dropdown shows environments with higher `sort_order` than the current env
- Selecting a target opens the `PromoteDialog`
- Wire up a `useMutation` calling `api.environments.promote()`
- On success, invalidate the flag detail query

The environments are already fetched (line 63-66). Sort them by `sort_order` and filter for targets with higher sort_order than the current env.

**Step 3: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 4: Verify visually**

Run: `cd web && npm run dev`
Navigate to a flag detail page. Verify:
- Promote button appears on each environment except the last in sort order
- Clicking opens dropdown with valid targets
- Selecting a target shows diff dialog
- Confirming promotes and refreshes the page

**Step 5: Commit**

```bash
git add web/src/components/PromoteDialog.tsx web/src/pages/FlagDetailPage.tsx
git commit -m "feat: add promote button and diff dialog to flag detail page (#47)"
```

---

### Task 7: Frontend — environment reorder UI on EnvironmentsPage

**Files:**
- Modify: `web/src/pages/EnvironmentsPage.tsx` — add up/down reorder buttons

**Step 1: Add reorder controls**

In `web/src/pages/EnvironmentsPage.tsx`:
- Sort environments by `sort_order` in the display
- Add up/down arrow buttons to each row (if `canWrite`)
- Wire up a mutation calling `api.environments.reorder()` with the new order
- On success, invalidate the environments query

Use `ArrowUp` and `ArrowDown` icons from `lucide-react`. Disable up on first item, down on last item.

**Step 2: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 3: Verify visually**

Navigate to project environments page. Verify:
- Up/down arrows appear
- Clicking reorders and persists
- Sort order is reflected on the flag detail page

**Step 4: Commit**

```bash
git add web/src/pages/EnvironmentsPage.tsx
git commit -m "feat: add environment reorder controls (#47)"
```

---

### Task 8: Frontend — audit log display for promoted action

**Files:**
- Modify: `web/src/components/FlagHistory.tsx` (or wherever audit entries are rendered) — handle `"promoted"` action

**Step 1: Find audit rendering**

Check `web/src/components/FlagHistory.tsx` or the audit log page for where `action` is displayed. Add a case for `"promoted"` that renders:
- "Promoted config from {source_env} to {target_env}"
- Extract `promoted_from_env` from the `new_value` JSON

**Step 2: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add web/src/components/FlagHistory.tsx
git commit -m "feat: display promoted action in audit log (#47)"
```

---

### Task 9: Final verification and cleanup

**Step 1: Run Go tests**

Run: `go test ./...`
Expected: All tests pass.

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors.

**Step 3: Run frontend build**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 4: Full build**

Run: `go build -o togglerino ./cmd/togglerino`
Expected: Binary builds successfully.

**Step 5: Manual end-to-end test**

1. Start dev environment: `./dev.sh`
2. Start frontend: `cd web && npm run dev`
3. Create a project with default environments
4. Create a flag, configure it in development (add variants, targeting rules)
5. Promote from development → staging — verify diff dialog shows correct changes
6. Promote from staging → production — verify it works
7. Try promoting backward — verify it's blocked
8. Check audit log — verify "promoted" entries appear
9. Reorder environments — verify new order is reflected

**Step 6: Commit any fixes**

If any issues found, fix and commit.
