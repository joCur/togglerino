# Flag Lifecycle Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the per-project lifecycle kanban board with a full lifecycle dashboard featuring stat cards, health score, staleness trends chart, filterable action queue with bulk operations.

**Architecture:** New DB migration for `lifecycle_snapshots` table, new background snapshot recorder (same pattern as staleness checker), new lifecycle handler with summary/trends endpoints, recharts-based frontend dashboard replacing the existing `LifecycleBoardPage.tsx`.

**Tech Stack:** Go (stdlib net/http, pgx/v5), React 19, TypeScript, TanStack Query, recharts, shadcn/ui, Tailwind CSS v4.

---

### Task 1: Database Migration — lifecycle_snapshots table

**Files:**
- Create: `migrations/018_lifecycle_snapshots.up.sql`
- Create: `migrations/018_lifecycle_snapshots.down.sql`

**Step 1: Create up migration**

```sql
CREATE TABLE lifecycle_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    active_count INTEGER NOT NULL DEFAULT 0,
    potentially_stale_count INTEGER NOT NULL DEFAULT 0,
    stale_count INTEGER NOT NULL DEFAULT 0,
    archived_count INTEGER NOT NULL DEFAULT 0,
    recorded_at DATE NOT NULL DEFAULT CURRENT_DATE,
    UNIQUE (project_id, recorded_at)
);

CREATE INDEX idx_lifecycle_snapshots_project_date ON lifecycle_snapshots (project_id, recorded_at);
```

**Step 2: Create down migration**

```sql
DROP TABLE IF EXISTS lifecycle_snapshots;
```

**Step 3: Verify migration applies**

Run: `docker compose up -d postgres && LOG_FORMAT=text go run ./cmd/togglerino 2>&1 | head -20`
Expected: Server starts without migration errors.

**Step 4: Commit**

```
feat: add lifecycle_snapshots migration (#37)
```

---

### Task 2: Lifecycle Snapshot Store

**Files:**
- Create: `internal/store/lifecycle_snapshot_store.go`
- Test: `internal/store/lifecycle_snapshot_store_test.go`

**Step 1: Write failing test for RecordSnapshots**

```go
package store

import (
	"context"
	"testing"
	"time"
)

func TestLifecycleSnapshotStore_RecordAndGet(t *testing.T) {
	pool := testPool(t)
	ss := NewLifecycleSnapshotStore(pool)

	// Need a project to reference
	ps := NewProjectStore(pool)
	project, err := ps.Create(context.Background(), "snap-test-"+time.Now().Format("150405"), "Snap Test", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Record a snapshot
	err = ss.Record(ctx, project.ID, 10, 3, 2, 1)
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	// Get trends for last 30 days
	trends, err := ss.GetTrends(ctx, project.ID, 30)
	if err != nil {
		t.Fatalf("GetTrends failed: %v", err)
	}

	if len(trends) != 1 {
		t.Fatalf("expected 1 trend row, got %d", len(trends))
	}
	if trends[0].ActiveCount != 10 {
		t.Errorf("expected active_count=10, got %d", trends[0].ActiveCount)
	}
	if trends[0].PotentiallyStaleCount != 3 {
		t.Errorf("expected potentially_stale_count=3, got %d", trends[0].PotentiallyStaleCount)
	}
	if trends[0].StaleCount != 2 {
		t.Errorf("expected stale_count=2, got %d", trends[0].StaleCount)
	}
	if trends[0].ArchivedCount != 1 {
		t.Errorf("expected archived_count=1, got %d", trends[0].ArchivedCount)
	}
}

func TestLifecycleSnapshotStore_RecordUpsert(t *testing.T) {
	pool := testPool(t)
	ss := NewLifecycleSnapshotStore(pool)

	ps := NewProjectStore(pool)
	project, err := ps.Create(context.Background(), "snap-upsert-"+time.Now().Format("150405"), "Snap Upsert", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Record twice on same day — should upsert
	if err := ss.Record(ctx, project.ID, 5, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := ss.Record(ctx, project.ID, 8, 2, 1, 0); err != nil {
		t.Fatal(err)
	}

	trends, err := ss.GetTrends(ctx, project.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(trends) != 1 {
		t.Fatalf("expected 1 trend row after upsert, got %d", len(trends))
	}
	if trends[0].ActiveCount != 8 {
		t.Errorf("expected upserted active_count=8, got %d", trends[0].ActiveCount)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestLifecycleSnapshotStore -v`
Expected: FAIL — `NewLifecycleSnapshotStore` not defined.

**Step 3: Write the store implementation**

```go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type LifecycleSnapshot struct {
	Date                  string `json:"date"`
	ActiveCount           int    `json:"active"`
	PotentiallyStaleCount int    `json:"potentially_stale"`
	StaleCount            int    `json:"stale"`
	ArchivedCount         int    `json:"archived"`
}

type LifecycleSnapshotStore struct {
	pool *pgxpool.Pool
}

func NewLifecycleSnapshotStore(pool *pgxpool.Pool) *LifecycleSnapshotStore {
	return &LifecycleSnapshotStore{pool: pool}
}

// Record inserts or updates the daily snapshot for a project (upsert on project_id + recorded_at).
func (s *LifecycleSnapshotStore) Record(ctx context.Context, projectID string, active, potentiallyStale, stale, archived int) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO lifecycle_snapshots (project_id, active_count, potentially_stale_count, stale_count, archived_count, recorded_at)
		 VALUES ($1, $2, $3, $4, $5, CURRENT_DATE)
		 ON CONFLICT (project_id, recorded_at)
		 DO UPDATE SET active_count=$2, potentially_stale_count=$3, stale_count=$4, archived_count=$5`,
		projectID, active, potentiallyStale, stale, archived,
	)
	if err != nil {
		return fmt.Errorf("recording lifecycle snapshot: %w", err)
	}
	return nil
}

// GetTrends returns snapshots for a project over the last N days, ordered by date.
func (s *LifecycleSnapshotStore) GetTrends(ctx context.Context, projectID string, days int) ([]LifecycleSnapshot, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT recorded_at, active_count, potentially_stale_count, stale_count, archived_count
		 FROM lifecycle_snapshots
		 WHERE project_id = $1 AND recorded_at >= CURRENT_DATE - $2::int
		 ORDER BY recorded_at ASC`,
		projectID, days,
	)
	if err != nil {
		return nil, fmt.Errorf("querying lifecycle trends: %w", err)
	}
	defer rows.Close()

	var snapshots []LifecycleSnapshot
	for rows.Next() {
		var s LifecycleSnapshot
		var recordedAt time.Time
		if err := rows.Scan(&recordedAt, &s.ActiveCount, &s.PotentiallyStaleCount, &s.StaleCount, &s.ArchivedCount); err != nil {
			return nil, fmt.Errorf("scanning lifecycle snapshot: %w", err)
		}
		s.Date = recordedAt.Format("2006-01-02")
		snapshots = append(snapshots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lifecycle snapshots: %w", err)
	}
	return snapshots, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestLifecycleSnapshotStore -v`
Expected: PASS

**Step 5: Commit**

```
feat: add lifecycle snapshot store with upsert and trends query (#37)
```

---

### Task 3: Lifecycle Summary Store Method

Add a method to `FlagStore` that returns flag counts grouped by lifecycle status for a project.

**Files:**
- Modify: `internal/store/flag_store.go`
- Test: `internal/store/flag_store_test.go` (if exists, or inline test)

**Step 1: Write failing test**

Create `internal/store/lifecycle_summary_test.go`:

```go
package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestFlagStore_LifecycleSummary(t *testing.T) {
	pool := testPool(t)
	fs := NewFlagStore(pool)
	ps := NewProjectStore(pool)
	es := NewEnvironmentStore(pool)

	project, err := ps.Create(context.Background(), "summary-test-"+time.Now().Format("150405"), "Summary Test", "")
	if err != nil {
		t.Fatal(err)
	}

	// Create a default environment (required for flag creation)
	_, err = es.Create(context.Background(), project.ID, "dev", "Development")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Create some flags
	_, err = fs.Create(ctx, project.ID, "flag-active", "Active Flag", "", "boolean", "release", json.RawMessage(`false`), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := fs.Create(ctx, project.ID, "flag-stale", "Stale Flag", "", "boolean", "release", json.RawMessage(`false`), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fs.SetLifecycleStatus(ctx, f2.ID, "stale")
	if err != nil {
		t.Fatal(err)
	}

	summary, err := fs.LifecycleSummary(ctx, project.ID)
	if err != nil {
		t.Fatalf("LifecycleSummary failed: %v", err)
	}

	if summary.Active != 1 {
		t.Errorf("expected 1 active, got %d", summary.Active)
	}
	if summary.Stale != 1 {
		t.Errorf("expected 1 stale, got %d", summary.Stale)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestFlagStore_LifecycleSummary -v`
Expected: FAIL — `LifecycleSummary` method not defined.

**Step 3: Implement LifecycleSummary method**

Add to `internal/store/flag_store.go`:

```go
type LifecycleSummary struct {
	Active           int     `json:"active"`
	PotentiallyStale int     `json:"potentially_stale"`
	Stale            int     `json:"stale"`
	Archived         int     `json:"archived"`
	HealthScore      float64 `json:"health_score"`
}

func (s *FlagStore) LifecycleSummary(ctx context.Context, projectID string) (*LifecycleSummary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT lifecycle_status, COUNT(*)
		 FROM flags WHERE project_id = $1
		 GROUP BY lifecycle_status`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying lifecycle summary: %w", err)
	}
	defer rows.Close()

	summary := &LifecycleSummary{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scanning lifecycle count: %w", err)
		}
		switch status {
		case "active":
			summary.Active = count
		case "potentially_stale":
			summary.PotentiallyStale = count
		case "stale":
			summary.Stale = count
		case "archived":
			summary.Archived = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating lifecycle counts: %w", err)
	}

	nonArchived := summary.Active + summary.PotentiallyStale + summary.Stale
	if nonArchived > 0 {
		summary.HealthScore = float64(summary.Active) / float64(nonArchived) * 100
	} else {
		summary.HealthScore = 100
	}

	return summary, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestFlagStore_LifecycleSummary -v`
Expected: PASS

**Step 5: Commit**

```
feat: add LifecycleSummary method to FlagStore (#37)
```

---

### Task 4: Snapshot Recorder Background Job

**Files:**
- Create: `internal/lifecycle/recorder.go`
- Create: `internal/lifecycle/recorder_test.go`

**Step 1: Write failing test**

```go
package lifecycle

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

type mockFlagStore struct {
	flags []model.Flag
}

func (m *mockFlagStore) ListNonArchived(_ context.Context) ([]model.Flag, error) {
	return m.flags, nil
}

type mockAllFlagStore struct {
	flags []model.Flag
}

func (m *mockAllFlagStore) ListAll(_ context.Context) ([]model.Flag, error) {
	return m.flags, nil
}

type snapshot struct {
	projectID                                    string
	active, potentiallyStale, stale, archived int
}

type mockSnapshotStore struct {
	recorded []snapshot
}

func (m *mockSnapshotStore) Record(_ context.Context, projectID string, active, potentiallyStale, stale, archived int) error {
	m.recorded = append(m.recorded, snapshot{projectID, active, potentiallyStale, stale, archived})
	return nil
}

func TestRecorder_Tick(t *testing.T) {
	flags := &mockAllFlagStore{
		flags: []model.Flag{
			{ProjectID: "proj-1", LifecycleStatus: model.LifecycleActive},
			{ProjectID: "proj-1", LifecycleStatus: model.LifecycleActive},
			{ProjectID: "proj-1", LifecycleStatus: model.LifecycleStale},
			{ProjectID: "proj-2", LifecycleStatus: model.LifecycleActive},
			{ProjectID: "proj-2", LifecycleStatus: model.LifecycleArchived},
		},
	}
	ss := &mockSnapshotStore{}
	r := NewRecorder(flags, ss, 24*60*60)

	r.tick(context.Background())

	if len(ss.recorded) != 2 {
		t.Fatalf("expected 2 snapshots (one per project), got %d", len(ss.recorded))
	}

	// Find proj-1 snapshot
	var proj1, proj2 *snapshot
	for i := range ss.recorded {
		switch ss.recorded[i].projectID {
		case "proj-1":
			proj1 = &ss.recorded[i]
		case "proj-2":
			proj2 = &ss.recorded[i]
		}
	}

	if proj1 == nil || proj2 == nil {
		t.Fatal("missing snapshot for one of the projects")
	}

	if proj1.active != 2 || proj1.stale != 1 {
		t.Errorf("proj-1: expected active=2 stale=1, got active=%d stale=%d", proj1.active, proj1.stale)
	}
	if proj2.active != 1 || proj2.archived != 1 {
		t.Errorf("proj-2: expected active=1 archived=1, got active=%d archived=%d", proj2.active, proj2.archived)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/lifecycle/ -run TestRecorder -v`
Expected: FAIL — package/types not defined.

**Step 3: Implement the recorder**

```go
package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/togglerino/togglerino/internal/model"
)

type FlagLister interface {
	ListAll(ctx context.Context) ([]model.Flag, error)
}

type SnapshotRecorder interface {
	Record(ctx context.Context, projectID string, active, potentiallyStale, stale, archived int) error
}

type Recorder struct {
	flags     FlagLister
	snapshots SnapshotRecorder
	interval  time.Duration
}

func NewRecorder(flags FlagLister, snapshots SnapshotRecorder, interval time.Duration) *Recorder {
	return &Recorder{flags: flags, snapshots: snapshots, interval: interval}
}

func (r *Recorder) Run(ctx context.Context) {
	slog.Info("lifecycle snapshot recorder started", "interval", r.interval)

	r.tick(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("lifecycle snapshot recorder stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

type projectCounts struct {
	active, potentiallyStale, stale, archived int
}

func (r *Recorder) tick(ctx context.Context) {
	flags, err := r.flags.ListAll(ctx)
	if err != nil {
		slog.Error("lifecycle recorder: failed to list flags", "error", err)
		return
	}

	counts := map[string]*projectCounts{}
	for _, f := range flags {
		c, ok := counts[f.ProjectID]
		if !ok {
			c = &projectCounts{}
			counts[f.ProjectID] = c
		}
		switch f.LifecycleStatus {
		case model.LifecycleActive:
			c.active++
		case model.LifecyclePotentiallyStale:
			c.potentiallyStale++
		case model.LifecycleStale:
			c.stale++
		case model.LifecycleArchived:
			c.archived++
		}
	}

	for projectID, c := range counts {
		if err := r.snapshots.Record(ctx, projectID, c.active, c.potentiallyStale, c.stale, c.archived); err != nil {
			slog.Error("lifecycle recorder: failed to record snapshot",
				"project_id", projectID, "error", err)
		}
	}

	slog.Info("lifecycle recorder: recorded snapshots", "projects", len(counts))
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/lifecycle/ -run TestRecorder -v`
Expected: PASS

**Step 5: Add FlagStore.ListAll method**

Add to `internal/store/flag_store.go`:

```go
// ListAll returns all flags across all projects (for snapshot recording).
func (s *FlagStore) ListAll(ctx context.Context) ([]model.Flag, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id
		 FROM flags`)
	if err != nil {
		return nil, fmt.Errorf("listing all flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	for rows.Next() {
		var f model.Flag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID); err != nil {
			return nil, fmt.Errorf("scanning flag: %w", err)
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating flags: %w", err)
	}
	return flags, nil
}
```

**Step 6: Run all tests**

Run: `go test ./internal/lifecycle/ ./internal/store/ -v`
Expected: PASS

**Step 7: Commit**

```
feat: add lifecycle snapshot recorder background job (#37)
```

---

### Task 5: Lifecycle Handler (Summary + Trends endpoints)

**Files:**
- Create: `internal/handler/lifecycle_handler.go`
- Create: `internal/handler/lifecycle_handler_test.go`

**Step 1: Write failing test**

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

type mockLifecycleFlagStore struct {
	summary *store.LifecycleSummary
}

func (m *mockLifecycleFlagStore) LifecycleSummary(_ context.Context, _ string) (*store.LifecycleSummary, error) {
	return m.summary, nil
}

type mockLifecycleSnapshotStore struct {
	trends []store.LifecycleSnapshot
}

func (m *mockLifecycleSnapshotStore) GetTrends(_ context.Context, _ string, _ int) ([]store.LifecycleSnapshot, error) {
	return m.trends, nil
}

type mockLifecycleProjectStore struct {
	projectID string
}

func (m *mockLifecycleProjectStore) FindByKey(_ context.Context, _ string) (*struct{ ID string }, error) {
	return &struct{ ID string }{ID: m.projectID}, nil
}

func TestLifecycleHandler_Summary(t *testing.T) {
	h := NewLifecycleHandler(
		&mockLifecycleFlagStore{summary: &store.LifecycleSummary{Active: 10, PotentiallyStale: 3, Stale: 2, Archived: 5, HealthScore: 66.67}},
		nil,
		nil,
	)

	req := httptest.NewRequest("GET", "/api/v1/projects/my-proj/lifecycle/summary", nil)
	req.SetPathValue("key", "my-proj")
	w := httptest.NewRecorder()

	h.Summary(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result store.LifecycleSummary
	json.NewDecoder(w.Body).Decode(&result)
	if result.Active != 10 {
		t.Errorf("expected active=10, got %d", result.Active)
	}
	if result.HealthScore < 66 || result.HealthScore > 67 {
		t.Errorf("expected health_score ~66.67, got %f", result.HealthScore)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestLifecycleHandler -v`
Expected: FAIL — `NewLifecycleHandler` not defined.

**Step 3: Implement the handler**

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/togglerino/togglerino/internal/store"
)

type LifecycleHandler struct {
	flags     *store.FlagStore
	snapshots *store.LifecycleSnapshotStore
	projects  *store.ProjectStore
}

func NewLifecycleHandler(flags *store.FlagStore, snapshots *store.LifecycleSnapshotStore, projects *store.ProjectStore) *LifecycleHandler {
	return &LifecycleHandler{flags: flags, snapshots: snapshots, projects: projects}
}

// Summary handles GET /api/v1/projects/{key}/lifecycle/summary
func (h *LifecycleHandler) Summary(w http.ResponseWriter, r *http.Request) {
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

	summary, err := h.flags.LifecycleSummary(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get lifecycle summary")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// Trends handles GET /api/v1/projects/{key}/lifecycle/trends?days=30
func (h *LifecycleHandler) Trends(w http.ResponseWriter, r *http.Request) {
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

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	trends, err := h.snapshots.GetTrends(r.Context(), project.ID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get lifecycle trends")
		return
	}

	if trends == nil {
		trends = []store.LifecycleSnapshot{}
	}

	writeJSON(w, http.StatusOK, trends)
}
```

Note: The test above uses mock interfaces. Since the handler uses concrete store types, the actual test should use `testPool` and real stores (matching the pattern in other handler tests), OR the handler should accept interfaces. Check existing handler test patterns first — if they use real DB (integration tests), follow that pattern. If no handler tests exist, skip the unit test for the handler and rely on integration testing via the API.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/ -run TestLifecycleHandler -v`
Expected: PASS (adjust based on actual test pattern used)

**Step 5: Commit**

```
feat: add lifecycle handler with summary and trends endpoints (#37)
```

---

### Task 6: Wire Up Backend — Routes + Recorder in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add store, handler, recorder initialization**

In the "Initialize all stores" section (~line 68), add:

```go
lifecycleSnapshotStore := store.NewLifecycleSnapshotStore(pool)
```

In the "Initialize cache, engine, hub" section (~line 92), add:

```go
snapshotRecorder := lifecycle.NewRecorder(flagStore, lifecycleSnapshotStore, 24*time.Hour)
```

In the goroutine section (~line 109), add:

```go
go snapshotRecorder.Run(ctx)
```

In the "Initialize all handlers" section (~line 118), add:

```go
lifecycleHandler := handler.NewLifecycleHandler(flagStore, lifecycleSnapshotStore, projectStore)
```

**Step 2: Add route registrations**

After the project settings routes (~line 254), add:

```go
// Lifecycle dashboard
mux.Handle("GET /api/v1/projects/{key}/lifecycle/summary", wrap(lifecycleHandler.Summary, sessionAuth, requireFlagsRead))
mux.Handle("GET /api/v1/projects/{key}/lifecycle/trends", wrap(lifecycleHandler.Trends, sessionAuth, requireFlagsRead))
```

**Step 3: Add import**

Add `"github.com/togglerino/togglerino/internal/lifecycle"` to imports.

**Step 4: Verify it compiles and starts**

Run: `go build ./cmd/togglerino && echo "Build OK"`
Expected: Build OK

**Step 5: Commit**

```
feat: wire lifecycle handler and snapshot recorder in main.go (#37)
```

---

### Task 7: Install recharts in frontend

**Files:**
- Modify: `web/package.json`

**Step 1: Install recharts**

Run: `cd web && npm install recharts`

**Step 2: Verify it installs cleanly**

Run: `cd web && npm ls recharts`
Expected: Shows recharts version.

**Step 3: Commit**

```
feat: add recharts dependency for lifecycle trends chart (#37)
```

---

### Task 8: Frontend Types + API Methods

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

**Step 1: Add types to `web/src/api/types.ts`**

```typescript
export interface LifecycleSummary {
  active: number
  potentially_stale: number
  stale: number
  archived: number
  health_score: number
}

export interface LifecycleSnapshot {
  date: string
  active: number
  potentially_stale: number
  stale: number
  archived: number
}
```

**Step 2: Add API methods to `web/src/api/client.ts`**

Add to the `api` object:

```typescript
lifecycle: {
  summary: (projectKey: string) =>
    request<LifecycleSummary>(`/projects/${projectKey}/lifecycle/summary`),
  trends: (projectKey: string, days = 30) =>
    request<LifecycleSnapshot[]>(`/projects/${projectKey}/lifecycle/trends?days=${days}`),
},
```

Add `LifecycleSummary` and `LifecycleSnapshot` to the imports from `./types`.

**Step 3: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors.

**Step 4: Commit**

```
feat: add lifecycle dashboard API types and client methods (#37)
```

---

### Task 9: Lifecycle Dashboard Page — Overview + Health Score

Replace `LifecycleBoardPage.tsx` with the new dashboard. Build incrementally: stat cards + health score first.

**Files:**
- Rewrite: `web/src/pages/LifecycleBoardPage.tsx`

**Step 1: Rewrite the page with stat cards and health score**

```tsx
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import type { LifecycleSummary } from '../api/types'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

function HealthBadge({ score }: { score: number }) {
  const rounded = Math.round(score)
  let color = 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
  if (rounded < 50) color = 'bg-red-500/10 text-red-400 border-red-500/20'
  else if (rounded < 80) color = 'bg-amber-500/10 text-amber-400 border-amber-500/20'
  return <Badge variant="outline" className={`text-sm font-semibold ${color}`}>{rounded}%</Badge>
}

const STATUS_CARDS = [
  { key: 'active' as const, label: 'Active', color: 'text-emerald-400', border: 'border-emerald-500/30' },
  { key: 'potentially_stale' as const, label: 'Potentially Stale', color: 'text-amber-400', border: 'border-amber-500/30' },
  { key: 'stale' as const, label: 'Stale', color: 'text-red-400', border: 'border-red-500/30' },
  { key: 'archived' as const, label: 'Archived', color: 'text-muted-foreground', border: 'border-muted-foreground/30' },
]

export default function LifecycleBoardPage() {
  const { key } = useParams<{ key: string }>()

  const { data: summary, isLoading } = useQuery({
    queryKey: ['projects', key, 'lifecycle', 'summary'],
    queryFn: () => api.lifecycle.summary(key!),
    enabled: !!key,
  })

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading lifecycle dashboard...
      </div>
    )
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">Projects</Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">{key}</Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Lifecycle</span>
      </div>

      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Flag Lifecycle</h1>
        {summary && <HealthBadge score={summary.health_score} />}
      </div>
      <p className="text-[13px] text-muted-foreground/60 mb-6">Track flag health and manage cleanup across lifecycle stages.</p>

      {summary && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          {STATUS_CARDS.map(card => (
            <Card key={card.key} className={`border-l-2 ${card.border}`}>
              <CardContent className="p-4">
                <div className={`text-[11px] uppercase tracking-wider font-medium ${card.color} mb-1`}>{card.label}</div>
                <div className="text-2xl font-bold text-foreground">{summary[card.key]}</div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Trends chart and action queue will be added in subsequent tasks */}
    </div>
  )
}
```

**Step 2: Verify frontend builds**

Run: `cd web && npx tsc --noEmit && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```
feat: lifecycle dashboard stat cards and health score (#37)
```

---

### Task 10: Trends Chart Component

**Files:**
- Modify: `web/src/pages/LifecycleBoardPage.tsx`

**Step 1: Add the trends chart section**

Add imports at the top of `LifecycleBoardPage.tsx`:

```tsx
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
import type { LifecycleSnapshot } from '../api/types'
```

Add a second query alongside the summary query:

```tsx
const { data: trends } = useQuery({
  queryKey: ['projects', key, 'lifecycle', 'trends'],
  queryFn: () => api.lifecycle.trends(key!),
  enabled: !!key,
})
```

Add the chart component after the stat cards grid, replacing the comment placeholder:

```tsx
{trends && trends.length > 0 && (
  <Card className="mb-8">
    <CardContent className="p-4">
      <div className="text-[13px] font-medium text-foreground mb-4">Staleness Trends</div>
      <ResponsiveContainer width="100%" height={240}>
        <AreaChart data={trends}>
          <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
          <XAxis
            dataKey="date"
            tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
            tickFormatter={(v: string) => new Date(v + 'T00:00:00').toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
          />
          <YAxis tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }} allowDecimals={false} />
          <Tooltip
            contentStyle={{
              backgroundColor: 'hsl(var(--card))',
              border: '1px solid hsl(var(--border))',
              borderRadius: '8px',
              fontSize: '12px',
            }}
          />
          <Area type="monotone" dataKey="active" stackId="1" stroke="#34d399" fill="#34d399" fillOpacity={0.3} name="Active" />
          <Area type="monotone" dataKey="potentially_stale" stackId="1" stroke="#fbbf24" fill="#fbbf24" fillOpacity={0.3} name="Potentially Stale" />
          <Area type="monotone" dataKey="stale" stackId="1" stroke="#f87171" fill="#f87171" fillOpacity={0.3} name="Stale" />
          <Area type="monotone" dataKey="archived" stackId="1" stroke="#6b7280" fill="#6b7280" fillOpacity={0.3} name="Archived" />
        </AreaChart>
      </ResponsiveContainer>
    </CardContent>
  </Card>
)}
```

**Step 2: Verify frontend builds**

Run: `cd web && npx tsc --noEmit && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```
feat: lifecycle dashboard trends area chart (#37)
```

---

### Task 11: Action Queue with Filters and Bulk Actions

**Files:**
- Modify: `web/src/pages/LifecycleBoardPage.tsx`

**Step 1: Add action queue with filters and bulk actions**

Add these imports:

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { Flag, FlagPurpose, LifecycleStatus } from '../api/types'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCanWrite } from '@/hooks/usePermissions'
```

Add filter state and queries:

```tsx
const canWrite = useCanWrite(key)
const queryClient = useQueryClient()
const navigate = useNavigate()

const [statusFilter, setStatusFilter] = useState<string>('potentially_stale,stale')
const [typeFilter, setTypeFilter] = useState<string>('all')
const [selected, setSelected] = useState<Set<string>>(new Set())

const flagsQueryKey = ['projects', key, 'lifecycle-flags', statusFilter, typeFilter]
const { data: flags } = useQuery({
  queryKey: flagsQueryKey,
  queryFn: () => {
    let path = `/projects/${key}/flags?lifecycle_status=${statusFilter}`
    if (typeFilter !== 'all') path += `&flag_type=${typeFilter}`
    return api.get<Flag[]>(path)
  },
  enabled: !!key,
})

const sortedFlags = [...(flags || [])].sort((a, b) =>
  new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
)

const archiveMutation = useMutation({
  mutationFn: (flagKey: string) => api.put(`/projects/${key}/flags/${flagKey}/archive`, { archived: true }),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key] })
  },
})

const stalenessMutation = useMutation({
  mutationFn: (flagKey: string) => api.put(`/projects/${key}/flags/${flagKey}/staleness`, { status: 'stale' }),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key] })
  },
})

const bulkArchiveMutation = useMutation({
  mutationFn: () => api.flags.bulk(key!, { action: 'archive', flag_keys: [...selected] }),
  onSuccess: () => {
    setSelected(new Set())
    queryClient.invalidateQueries({ queryKey: ['projects', key] })
  },
})

function toggleSelect(flagKey: string) {
  setSelected(prev => {
    const next = new Set(prev)
    if (next.has(flagKey)) next.delete(flagKey)
    else next.add(flagKey)
    return next
  })
}

function toggleAll() {
  if (selected.size === sortedFlags.length) setSelected(new Set())
  else setSelected(new Set(sortedFlags.map(f => f.key)))
}
```

Add the action queue JSX after the trends chart:

```tsx
<Card>
  <CardContent className="p-4">
    <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
      <div className="text-[13px] font-medium text-foreground">Action Queue</div>
      <div className="flex items-center gap-2">
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="h-8 text-[12px] w-[180px]">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="potentially_stale,stale">Needs Attention</SelectItem>
            <SelectItem value="potentially_stale">Potentially Stale</SelectItem>
            <SelectItem value="stale">Stale</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="archived">Archived</SelectItem>
            <SelectItem value="active,potentially_stale,stale,archived">All</SelectItem>
          </SelectContent>
        </Select>
        <Select value={typeFilter} onValueChange={setTypeFilter}>
          <SelectTrigger className="h-8 text-[12px] w-[140px]">
            <SelectValue placeholder="Flag type" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All types</SelectItem>
            <SelectItem value="release">Release</SelectItem>
            <SelectItem value="experiment">Experiment</SelectItem>
            <SelectItem value="operational">Operational</SelectItem>
            <SelectItem value="kill-switch">Kill Switch</SelectItem>
            <SelectItem value="permission">Permission</SelectItem>
          </SelectContent>
        </Select>
        {canWrite && selected.size > 0 && (
          <Button
            size="sm"
            variant="destructive"
            className="h-8 text-[12px]"
            onClick={() => bulkArchiveMutation.mutate()}
            disabled={bulkArchiveMutation.isPending}
          >
            {bulkArchiveMutation.isPending ? 'Archiving...' : `Archive ${selected.size} selected`}
          </Button>
        )}
      </div>
    </div>

    {sortedFlags.length === 0 ? (
      <div className="text-center py-8 text-muted-foreground/40 text-[13px]">
        No flags match the current filters.
      </div>
    ) : (
      <div className="border rounded-lg overflow-hidden">
        <table className="w-full text-[13px]">
          <thead>
            <tr className="border-b bg-muted/30">
              {canWrite && (
                <th className="p-2 w-8">
                  <Checkbox
                    checked={selected.size === sortedFlags.length && sortedFlags.length > 0}
                    onCheckedChange={toggleAll}
                  />
                </th>
              )}
              <th className="p-2 text-left text-muted-foreground font-medium">Flag</th>
              <th className="p-2 text-left text-muted-foreground font-medium hidden sm:table-cell">Type</th>
              <th className="p-2 text-left text-muted-foreground font-medium hidden md:table-cell">Status</th>
              <th className="p-2 text-left text-muted-foreground font-medium hidden md:table-cell">Age</th>
              {canWrite && <th className="p-2 text-right text-muted-foreground font-medium">Action</th>}
            </tr>
          </thead>
          <tbody>
            {sortedFlags.map(flag => (
              <tr
                key={flag.id}
                className="border-b last:border-0 hover:bg-muted/20 cursor-pointer transition-colors"
                onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
              >
                {canWrite && (
                  <td className="p-2" onClick={e => e.stopPropagation()}>
                    <Checkbox
                      checked={selected.has(flag.key)}
                      onCheckedChange={() => toggleSelect(flag.key)}
                    />
                  </td>
                )}
                <td className="p-2">
                  <div className="font-medium text-foreground">{flag.name}</div>
                  <div className="font-mono text-[11px] text-[#d4956a]">{flag.key}</div>
                </td>
                <td className="p-2 hidden sm:table-cell">
                  <Badge variant="secondary" className={`text-[10px] ${PURPOSE_COLORS[flag.flag_type] || ''}`}>
                    {flag.flag_type}
                  </Badge>
                </td>
                <td className="p-2 hidden md:table-cell">
                  <Badge variant="outline" className={`text-[10px] ${STATUS_COLORS[flag.lifecycle_status] || ''}`}>
                    {flag.lifecycle_status.replace('_', ' ')}
                  </Badge>
                </td>
                <td className="p-2 hidden md:table-cell text-muted-foreground text-[12px]">
                  {daysAgo(flag.created_at)}d
                  {flag.lifecycle_status_changed_at && (
                    <span className="text-muted-foreground/60"> · {daysAgo(flag.lifecycle_status_changed_at)}d in status</span>
                  )}
                </td>
                {canWrite && (
                  <td className="p-2 text-right" onClick={e => e.stopPropagation()}>
                    {flag.lifecycle_status === 'stale' && (
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-[11px] border-destructive/50 text-destructive hover:bg-destructive/10"
                        onClick={() => archiveMutation.mutate(flag.key)}
                        disabled={archiveMutation.isPending}
                      >
                        Archive
                      </Button>
                    )}
                    {flag.lifecycle_status === 'potentially_stale' && (
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 text-[11px] border-amber-500/50 text-amber-400 hover:bg-amber-500/10"
                        onClick={() => stalenessMutation.mutate(flag.key)}
                        disabled={stalenessMutation.isPending}
                      >
                        Mark Stale
                      </Button>
                    )}
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )}
  </CardContent>
</Card>
```

Add the helper constants (at top of file, outside component):

```tsx
const PURPOSE_COLORS: Record<string, string> = {
  'release': 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  'experiment': 'bg-purple-500/10 text-purple-400 border-purple-500/20',
  'operational': 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  'kill-switch': 'bg-red-500/10 text-red-400 border-red-500/20',
  'permission': 'bg-green-500/10 text-green-400 border-green-500/20',
}

const STATUS_COLORS: Record<string, string> = {
  'active': 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  'potentially_stale': 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  'stale': 'bg-red-500/10 text-red-400 border-red-500/20',
  'archived': 'bg-muted text-muted-foreground border-muted',
}

function daysAgo(dateStr: string): number {
  return Math.floor((Date.now() - new Date(dateStr).getTime()) / (1000 * 60 * 60 * 24))
}
```

**Step 2: Check that `Select` component exists**

Run: `ls web/src/components/ui/select.tsx`
If missing: `cd web && npx shadcn@latest add select`

**Step 3: Verify frontend builds**

Run: `cd web && npx tsc --noEmit && npm run build`
Expected: Build succeeds.

**Step 4: Verify lint passes**

Run: `cd web && npm run lint`
Expected: No errors.

**Step 5: Commit**

```
feat: lifecycle dashboard action queue with filters and bulk actions (#37)
```

---

### Task 12: Run Full Test Suite + Final Verification

**Step 1: Run Go tests**

Run: `go test ./...`
Expected: All tests pass.

**Step 2: Run frontend build + lint**

Run: `cd web && npm run build && npm run lint`
Expected: Build succeeds, no lint errors.

**Step 3: Run full binary build (tests embed)**

Run: `cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino && echo "Full build OK"`
Expected: Full build OK.

**Step 4: Manual smoke test (optional)**

Run:
```bash
docker compose up -d postgres
cd web && npm run build && cd ..
LOG_FORMAT=text go run ./cmd/togglerino
```
Then open browser to `http://localhost:8080`, navigate to a project's Lifecycle page, and verify:
- Stat cards show with correct counts
- Health score badge displays
- Action queue table loads with filters
- Bulk selection works

**Step 5: Final commit (if any fixups needed)**

```
fix: address review feedback for lifecycle dashboard (#37)
```
