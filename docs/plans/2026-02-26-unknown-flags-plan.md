# Unknown Flags Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track SDK evaluation requests for non-existent flags and surface them in the dashboard so users can spot typos, misconfigurations, or stale references.

**Architecture:** New `unknown_flags` DB table with upsert-on-miss tracking in the evaluate handler. New store, management API endpoints, and a frontend tab on the project detail page. Auto-cleanup when matching flags are created.

**Tech Stack:** Go (stdlib net/http, pgx/v5, slog), React 19, TanStack Query, Tailwind CSS + shadcn/ui

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/003_unknown_flags.up.sql`
- Create: `migrations/003_unknown_flags.down.sql`

**Step 1: Write the up migration**

Create `migrations/003_unknown_flags.up.sql`:

```sql
CREATE TABLE unknown_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    flag_key        TEXT NOT NULL,
    request_count   BIGINT NOT NULL DEFAULT 1,
    first_seen_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dismissed_at    TIMESTAMPTZ,
    UNIQUE (project_id, environment_id, flag_key)
);

CREATE INDEX idx_unknown_flags_project ON unknown_flags(project_id) WHERE dismissed_at IS NULL;
```

**Step 2: Write the down migration**

Create `migrations/003_unknown_flags.down.sql`:

```sql
DROP TABLE IF EXISTS unknown_flags;
```

**Step 3: Commit**

```bash
git add migrations/003_unknown_flags.up.sql migrations/003_unknown_flags.down.sql
git commit -m "feat: add unknown_flags migration"
```

---

### Task 2: Add ProjectID to SDKKey Model

The `SDKKey` model has `EnvironmentID` but not `ProjectID`. We need `ProjectID` for the unknown flags upsert.

**Files:**
- Modify: `internal/model/environment.go` (SDKKey struct)
- Modify: `internal/store/sdk_key_store.go` (FindByKey query)

**Step 1: Add ProjectID field to SDKKey**

In `internal/model/environment.go`, add `ProjectID` to the SDKKey struct:

```go
type SDKKey struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	EnvironmentID  string    `json:"environment_id"`
	Name           string    `json:"name"`
	Revoked        bool      `json:"revoked"`
	CreatedAt      time.Time `json:"created_at"`
	ProjectID      string    `json:"project_id"`
	ProjectKey     string    `json:"project_key"`
	EnvironmentKey string    `json:"environment_key"`
}
```

**Step 2: Update FindByKey to select p.id**

In `internal/store/sdk_key_store.go`, modify the `FindByKey` method to also select `p.id` and scan it into `k.ProjectID`:

```go
func (s *SDKKeyStore) FindByKey(ctx context.Context, key string) (*model.SDKKey, error) {
	var k model.SDKKey
	err := s.pool.QueryRow(ctx,
		`SELECT sk.id, sk.key, sk.environment_id, sk.name, sk.revoked, sk.created_at, p.id, p.key, e.key
		 FROM sdk_keys sk
		 JOIN environments e ON e.id = sk.environment_id
		 JOIN projects p ON p.id = e.project_id
		 WHERE sk.key = $1 AND sk.revoked = FALSE`,
		key,
	).Scan(&k.ID, &k.Key, &k.EnvironmentID, &k.Name, &k.Revoked, &k.CreatedAt, &k.ProjectID, &k.ProjectKey, &k.EnvironmentKey)
	if err != nil {
		return nil, fmt.Errorf("finding SDK key: %w", err)
	}
	return &k, nil
}
```

**Step 3: Run tests to verify nothing broke**

```bash
go test ./internal/store/... ./internal/auth/... ./internal/handler/...
```

Expected: all existing tests pass.

**Step 4: Commit**

```bash
git add internal/model/environment.go internal/store/sdk_key_store.go
git commit -m "feat: add ProjectID to SDKKey model"
```

---

### Task 3: UnknownFlag Model

**Files:**
- Create: `internal/model/unknown_flag.go`

**Step 1: Create the model type**

Create `internal/model/unknown_flag.go`:

```go
package model

import "time"

type UnknownFlag struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"project_id"`
	EnvironmentID  string    `json:"environment_id"`
	FlagKey        string    `json:"flag_key"`
	RequestCount   int64     `json:"request_count"`
	FirstSeenAt    time.Time `json:"first_seen_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
	EnvironmentKey string    `json:"environment_key"`
	EnvironmentName string   `json:"environment_name"`
}
```

Note: `EnvironmentKey` and `EnvironmentName` are populated by the JOIN in `ListByProject` — they aren't stored in the `unknown_flags` table itself.

**Step 2: Commit**

```bash
git add internal/model/unknown_flag.go
git commit -m "feat: add UnknownFlag model type"
```

---

### Task 4: UnknownFlagStore — Upsert

**Files:**
- Create: `internal/store/unknown_flag_store.go`
- Create: `internal/store/unknown_flag_store_test.go`

**Step 1: Write the failing test for Upsert**

Create `internal/store/unknown_flag_store_test.go`:

```go
package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestUnknownFlagStore_Upsert(t *testing.T) {
	pool := testPool(t)
	s := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, pool)
	envID := createTestEnvironment(t, pool, projectID)

	// First upsert creates the row
	err := s.Upsert(ctx, projectID, envID, "nonexistent-flag")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second upsert increments count
	err = s.Upsert(ctx, projectID, envID, "nonexistent-flag")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	// Verify count is 2
	flags, err := s.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 unknown flag, got %d", len(flags))
	}
	if flags[0].RequestCount != 2 {
		t.Fatalf("expected request_count=2, got %d", flags[0].RequestCount)
	}
	if flags[0].FlagKey != "nonexistent-flag" {
		t.Fatalf("expected flag_key=nonexistent-flag, got %s", flags[0].FlagKey)
	}
}
```

Note: the `testPool`, `createTestProject`, and `createTestEnvironment` helpers may already exist in the store test files. Check existing `*_test.go` files in `internal/store/` for the pattern. If they don't exist, create them following the existing test patterns (read `DATABASE_URL` env var, use `pgxpool.New`). Use direct SQL inserts for test data setup.

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/store/... -run TestUnknownFlagStore_Upsert -v
```

Expected: FAIL — `NewUnknownFlagStore` doesn't exist yet.

**Step 3: Write the store with Upsert method**

Create `internal/store/unknown_flag_store.go`:

```go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type UnknownFlagStore struct {
	pool *pgxpool.Pool
}

func NewUnknownFlagStore(pool *pgxpool.Pool) *UnknownFlagStore {
	return &UnknownFlagStore{pool: pool}
}

func (s *UnknownFlagStore) Upsert(ctx context.Context, projectID, environmentID, flagKey string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO unknown_flags (project_id, environment_id, flag_key)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (project_id, environment_id, flag_key) DO UPDATE
		 SET request_count = unknown_flags.request_count + 1,
		     last_seen_at = now(),
		     dismissed_at = NULL`,
		projectID, environmentID, flagKey,
	)
	if err != nil {
		return fmt.Errorf("upserting unknown flag: %w", err)
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/store/... -run TestUnknownFlagStore_Upsert -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/unknown_flag_store.go internal/store/unknown_flag_store_test.go
git commit -m "feat: add UnknownFlagStore with Upsert method"
```

---

### Task 5: UnknownFlagStore — ListByProject

**Files:**
- Modify: `internal/store/unknown_flag_store.go`
- Modify: `internal/store/unknown_flag_store_test.go`

**Step 1: Write the failing test**

Add to `internal/store/unknown_flag_store_test.go`:

```go
func TestUnknownFlagStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	s := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, pool)
	envID := createTestEnvironment(t, pool, projectID)

	// Empty list when no unknown flags
	flags, err := s.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(flags) != 0 {
		t.Fatalf("expected 0, got %d", len(flags))
	}

	// Insert some unknown flags
	s.Upsert(ctx, projectID, envID, "flag-a")
	s.Upsert(ctx, projectID, envID, "flag-b")
	s.Upsert(ctx, projectID, envID, "flag-a") // increment

	flags, err = s.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2, got %d", len(flags))
	}

	// Should be ordered by last_seen_at DESC — flag-a was upserted most recently
	if flags[0].FlagKey != "flag-a" {
		t.Fatalf("expected flag-a first (most recent), got %s", flags[0].FlagKey)
	}
	if flags[0].RequestCount != 2 {
		t.Fatalf("expected count=2 for flag-a, got %d", flags[0].RequestCount)
	}
	// Should include environment key and name from JOIN
	if flags[0].EnvironmentKey == "" {
		t.Fatal("expected environment_key to be populated")
	}
}
```

**Step 2: Run to verify it fails**

```bash
go test ./internal/store/... -run TestUnknownFlagStore_ListByProject -v
```

Expected: FAIL — `ListByProject` not implemented yet (if it was stubbed as part of Task 4, it may compile but return wrong results).

**Step 3: Implement ListByProject**

Add to `internal/store/unknown_flag_store.go`:

```go
func (s *UnknownFlagStore) ListByProject(ctx context.Context, projectID string) ([]model.UnknownFlag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT uf.id, uf.project_id, uf.environment_id, uf.flag_key,
		        uf.request_count, uf.first_seen_at, uf.last_seen_at,
		        e.key, e.name
		 FROM unknown_flags uf
		 JOIN environments e ON e.id = uf.environment_id
		 WHERE uf.project_id = $1 AND uf.dismissed_at IS NULL
		 ORDER BY uf.last_seen_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing unknown flags: %w", err)
	}
	defer rows.Close()

	var flags []model.UnknownFlag
	for rows.Next() {
		var f model.UnknownFlag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.EnvironmentID, &f.FlagKey,
			&f.RequestCount, &f.FirstSeenAt, &f.LastSeenAt,
			&f.EnvironmentKey, &f.EnvironmentName); err != nil {
			return nil, fmt.Errorf("scanning unknown flag: %w", err)
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating unknown flags: %w", err)
	}
	if flags == nil {
		flags = []model.UnknownFlag{}
	}
	return flags, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/store/... -run TestUnknownFlagStore_ListByProject -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/unknown_flag_store.go internal/store/unknown_flag_store_test.go
git commit -m "feat: add ListByProject to UnknownFlagStore"
```

---

### Task 6: UnknownFlagStore — Dismiss and DeleteByProjectAndKey

**Files:**
- Modify: `internal/store/unknown_flag_store.go`
- Modify: `internal/store/unknown_flag_store_test.go`

**Step 1: Write the failing tests**

Add to `internal/store/unknown_flag_store_test.go`:

```go
func TestUnknownFlagStore_Dismiss(t *testing.T) {
	pool := testPool(t)
	s := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, pool)
	envID := createTestEnvironment(t, pool, projectID)

	s.Upsert(ctx, projectID, envID, "dismiss-me")

	flags, _ := s.ListByProject(ctx, projectID)
	if len(flags) != 1 {
		t.Fatalf("expected 1, got %d", len(flags))
	}

	err := s.Dismiss(ctx, flags[0].ID)
	if err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	// Dismissed flag should not appear in list
	flags, _ = s.ListByProject(ctx, projectID)
	if len(flags) != 0 {
		t.Fatalf("expected 0 after dismiss, got %d", len(flags))
	}

	// Upserting again should resurface it (clears dismissed_at)
	s.Upsert(ctx, projectID, envID, "dismiss-me")
	flags, _ = s.ListByProject(ctx, projectID)
	if len(flags) != 1 {
		t.Fatalf("expected 1 after resurface, got %d", len(flags))
	}
}

func TestUnknownFlagStore_DeleteByProjectAndKey(t *testing.T) {
	pool := testPool(t)
	s := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	projectID := createTestProject(t, pool)
	envID1 := createTestEnvironment(t, pool, projectID)
	envID2 := createTestEnvironment(t, pool, projectID)

	// Same flag key in two environments
	s.Upsert(ctx, projectID, envID1, "cleanup-me")
	s.Upsert(ctx, projectID, envID2, "cleanup-me")
	s.Upsert(ctx, projectID, envID1, "keep-me")

	err := s.DeleteByProjectAndKey(ctx, projectID, "cleanup-me")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	flags, _ := s.ListByProject(ctx, projectID)
	if len(flags) != 1 {
		t.Fatalf("expected 1, got %d", len(flags))
	}
	if flags[0].FlagKey != "keep-me" {
		t.Fatalf("expected keep-me, got %s", flags[0].FlagKey)
	}
}
```

**Step 2: Run to verify they fail**

```bash
go test ./internal/store/... -run "TestUnknownFlagStore_Dismiss|TestUnknownFlagStore_Delete" -v
```

Expected: FAIL — methods not implemented.

**Step 3: Implement Dismiss and DeleteByProjectAndKey**

Add to `internal/store/unknown_flag_store.go`:

```go
func (s *UnknownFlagStore) Dismiss(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE unknown_flags SET dismissed_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("dismissing unknown flag: %w", err)
	}
	return nil
}

func (s *UnknownFlagStore) DeleteByProjectAndKey(ctx context.Context, projectID, flagKey string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM unknown_flags WHERE project_id = $1 AND flag_key = $2`,
		projectID, flagKey,
	)
	if err != nil {
		return fmt.Errorf("deleting unknown flags: %w", err)
	}
	return nil
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/store/... -run "TestUnknownFlagStore_Dismiss|TestUnknownFlagStore_Delete" -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/unknown_flag_store.go internal/store/unknown_flag_store_test.go
git commit -m "feat: add Dismiss and DeleteByProjectAndKey to UnknownFlagStore"
```

---

### Task 7: Hook Unknown Flag Tracking into EvaluateSingle

**Files:**
- Modify: `internal/handler/evaluate_handler.go`

**Step 1: Add UnknownFlagStore to EvaluateHandler**

Update the struct and constructor in `internal/handler/evaluate_handler.go`:

```go
type EvaluateHandler struct {
	cache        *evaluation.Cache
	engine       *evaluation.Engine
	unknownFlags *store.UnknownFlagStore
}

func NewEvaluateHandler(cache *evaluation.Cache, engine *evaluation.Engine, unknownFlags *store.UnknownFlagStore) *EvaluateHandler {
	return &EvaluateHandler{cache: cache, engine: engine, unknownFlags: unknownFlags}
}
```

**Step 2: Add best-effort tracking to EvaluateSingle**

In the `EvaluateSingle` method, after the `!ok` check, add the tracking call before returning 404. The full updated method:

```go
func (h *EvaluateHandler) EvaluateSingle(w http.ResponseWriter, r *http.Request) {
	flagKey := r.PathValue("flag")
	sdkKey := auth.SDKKeyFromContext(r.Context())
	evalCtx := h.parseContext(r)

	fd, ok := h.cache.GetFlag(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey)
	if !ok {
		// Best-effort unknown flag tracking
		go func() {
			if err := h.unknownFlags.Upsert(context.Background(), sdkKey.ProjectID, sdkKey.EnvironmentID, flagKey); err != nil {
				slog.Warn("failed to track unknown flag", "flag_key", flagKey, "error", err)
			}
		}()
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	result := h.engine.Evaluate(&fd.Flag, &fd.Config, evalCtx)
	writeJSON(w, http.StatusOK, result)
}
```

Note: Uses `context.Background()` for the goroutine since the request context will be cancelled after the response is sent. Imports needed: `"context"`, `"log/slog"`.

**Step 3: Run all tests**

```bash
go test ./internal/handler/... -v
```

Expected: PASS (or compilation may fail if tests construct `NewEvaluateHandler` — update those calls to pass `nil` for `unknownFlags` if needed).

**Step 4: Commit**

```bash
git add internal/handler/evaluate_handler.go
git commit -m "feat: track unknown flags in EvaluateSingle handler"
```

---

### Task 8: Auto-Cleanup on Flag Creation

**Files:**
- Modify: `internal/handler/flag_handler.go`

**Step 1: Add UnknownFlagStore to FlagHandler**

Add `unknownFlags *store.UnknownFlagStore` to the `FlagHandler` struct and update `NewFlagHandler`:

```go
type FlagHandler struct {
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
	unknownFlags *store.UnknownFlagStore
}

func NewFlagHandler(flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool, unknownFlags *store.UnknownFlagStore) *FlagHandler {
	return &FlagHandler{flags: flags, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool, unknownFlags: unknownFlags}
}
```

**Step 2: Add cleanup call in Create handler**

In the `Create` method of `FlagHandler`, after the successful flag creation and before the audit log block, add:

```go
	// Best-effort cleanup of unknown flags with this key
	if err := h.unknownFlags.DeleteByProjectAndKey(r.Context(), project.ID, req.Key); err != nil {
		slog.Warn("failed to cleanup unknown flags", "flag_key", req.Key, "error", err)
	}
```

**Step 3: Run all tests**

```bash
go test ./internal/handler/... -v
```

Expected: PASS (update any test calls to `NewFlagHandler` to pass the extra parameter — use `nil` if not testing unknown flags specifically).

**Step 4: Commit**

```bash
git add internal/handler/flag_handler.go
git commit -m "feat: auto-cleanup unknown flags on flag creation"
```

---

### Task 9: Unknown Flags Management Handler

**Files:**
- Create: `internal/handler/unknown_flag_handler.go`

**Step 1: Create the handler**

Create `internal/handler/unknown_flag_handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/store"
)

type UnknownFlagHandler struct {
	unknownFlags *store.UnknownFlagStore
	projects     *store.ProjectStore
}

func NewUnknownFlagHandler(unknownFlags *store.UnknownFlagStore, projects *store.ProjectStore) *UnknownFlagHandler {
	return &UnknownFlagHandler{unknownFlags: unknownFlags, projects: projects}
}

func (h *UnknownFlagHandler) List(w http.ResponseWriter, r *http.Request) {
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

	flags, err := h.unknownFlags.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unknown flags")
		return
	}

	writeJSON(w, http.StatusOK, flags)
}

func (h *UnknownFlagHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "unknown flag id is required")
		return
	}

	if err := h.unknownFlags.Dismiss(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to dismiss unknown flag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Commit**

```bash
git add internal/handler/unknown_flag_handler.go
git commit -m "feat: add unknown flags management handler"
```

---

### Task 10: Wire Everything in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add store initialization**

After the existing store initializations (around line 58), add:

```go
unknownFlagStore := store.NewUnknownFlagStore(pool)
```

**Step 2: Update handler constructors**

Update `evaluateHandler` to pass the unknown flag store:

```go
evaluateHandler := handler.NewEvaluateHandler(cache, engine, unknownFlagStore)
```

Update `flagHandler` to pass the unknown flag store:

```go
flagHandler := handler.NewFlagHandler(flagStore, projectStore, environmentStore, auditStore, hub, cache, pool, unknownFlagStore)
```

Add the new handler:

```go
unknownFlagHandler := handler.NewUnknownFlagHandler(unknownFlagStore, projectStore)
```

**Step 3: Register routes**

Add the new routes alongside the existing flag routes (after the flag route block):

```go
// Unknown flags
mux.Handle("GET /api/v1/projects/{key}/unknown-flags", wrap(unknownFlagHandler.List, sessionAuth))
mux.Handle("DELETE /api/v1/projects/{key}/unknown-flags/{id}", wrap(unknownFlagHandler.Dismiss, sessionAuth))
```

**Step 4: Verify compilation**

```bash
go build ./cmd/togglerino
```

Expected: compiles successfully.

**Step 5: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: wire unknown flags store, handler, and routes"
```

---

### Task 11: Frontend — API Client and Types

**Files:**
- Modify: `web/src/api/client.ts` (or wherever types are defined — check existing pattern)

**Step 1: Add UnknownFlag type**

Check where frontend types are defined (likely in `web/src/types.ts` or inline in components). Add the type where it fits the existing pattern:

```typescript
export interface UnknownFlag {
  id: string
  project_id: string
  environment_id: string
  flag_key: string
  request_count: number
  first_seen_at: string
  last_seen_at: string
  environment_key: string
  environment_name: string
}
```

**Step 2: Commit**

```bash
git add web/src/
git commit -m "feat(web): add UnknownFlag type"
```

---

### Task 12: Frontend — Unknown Flags Tab on ProjectDetailPage

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Add query for unknown flags**

Add alongside the existing queries in `ProjectDetailPage`:

```typescript
const { data: unknownFlags } = useQuery({
  queryKey: ['projects', key, 'unknown-flags'],
  queryFn: () => api.get<UnknownFlag[]>(`/projects/${key}/unknown-flags`),
  enabled: !!key,
})
```

**Step 2: Add dismiss mutation**

```typescript
const queryClient = useQueryClient()

const dismissMutation = useMutation({
  mutationFn: (id: string) => api.delete(`/projects/${key}/unknown-flags/${id}`),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'unknown-flags'] })
  },
})
```

**Step 3: Add tab state and tab UI**

Add a tab state variable:

```typescript
const [activeTab, setActiveTab] = useState<'flags' | 'unknown'>('flags')
```

Add a tab bar above the existing flags table. Use the existing Tabs component from shadcn/ui if available (`web/src/components/ui/tabs.tsx`), otherwise use simple buttons styled with the project's design system. The "Unknown Flags" tab should show a count badge when `unknownFlags && unknownFlags.length > 0`.

**Step 4: Add Unknown Flags table**

When `activeTab === 'unknown'`, render a table with columns: Flag Key (monospace `font-mono`), Environment (Badge component), Requests, First Seen, Last Seen, Actions.

For relative timestamps, use a simple helper or `Intl.RelativeTimeFormat`. Don't add a dependency — format inline.

Actions column: "Create Flag" button navigates to the flag creation form. Check how flag creation works in the UI — likely `navigate(`/projects/${key}/flags/new?key=${flag.flag_key}`)` or a dialog. Match the existing pattern. "Dismiss" button calls `dismissMutation.mutate(flag.id)`.

Empty state when `unknownFlags?.length === 0`: display centered text "No unknown flags detected. Unknown flags appear here when your SDKs try to evaluate flags that don't exist in this project."

**Step 5: Verify the build**

```bash
cd web && npm run build
```

Expected: builds successfully.

**Step 6: Commit**

```bash
git add web/src/
git commit -m "feat(web): add Unknown Flags tab to project detail page"
```

---

### Task 13: End-to-End Verification

**Step 1: Build the full binary**

```bash
cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino
```

**Step 2: Run all Go tests**

```bash
go test ./...
```

Expected: all pass.

**Step 3: Run frontend lint**

```bash
cd web && npm run lint
```

Expected: no errors.

**Step 4: Commit any remaining fixes if needed**

---

### Task 14: Manual Smoke Test (optional, if local Docker is available)

**Step 1: Start the stack**

```bash
docker compose up --build
```

**Step 2: Verify the migration ran**

Check logs for migration 003 applying successfully.

**Step 3: Create a project and SDK key in the UI**

Navigate to `http://localhost:8090`, set up, create a project with an environment and SDK key.

**Step 4: Evaluate a non-existent flag via curl**

```bash
curl -X POST http://localhost:8090/api/v1/evaluate/nonexistent-flag \
  -H "Authorization: Bearer <sdk-key>" \
  -H "Content-Type: application/json" \
  -d '{"context": {"user_id": "test"}}'
```

Expected: 404 response.

**Step 5: Check the Unknown Flags tab**

Navigate to the project detail page and click the "Unknown Flags" tab. Should show `nonexistent-flag` with count 1.

**Step 6: Dismiss and verify**

Click "Dismiss" and verify it disappears.

**Step 7: Create a flag with the unknown key**

Create a real flag with key `nonexistent-flag`. Re-evaluate via curl (should get a real result). Unknown flags tab should remain clean (auto-cleanup).
