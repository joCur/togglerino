# Flag Change History Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add per-flag change history with structured diffs and rollback to previous configurations.

**Architecture:** Query-over-audit-log — history is a filtered view of the existing `audit_log` table. New columns (`environment_id`, `user_email`) and a composite index enable per-flag queries. Rollback reuses the existing `UpdateEnvironmentConfig` code path. Frontend adds tabs to the flag detail page with a structured diff component.

**Tech Stack:** Go 1.25 (stdlib net/http, pgx/v5), React 19, TypeScript, TanStack Query, shadcn/ui (Tabs), Tailwind CSS v4.

**Design doc:** `docs/plans/2026-03-03-flag-history-design.md`

**Worktree:** `.worktrees/flag-history` on branch `feature/flag-change-history`

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/011_flag_history.up.sql`
- Create: `migrations/011_flag_history.down.sql`

**Step 1: Write up migration**

```sql
-- migrations/011_flag_history.up.sql
ALTER TABLE audit_log ADD COLUMN environment_id UUID REFERENCES environments(id) ON DELETE SET NULL;
ALTER TABLE audit_log ADD COLUMN user_email TEXT;
CREATE INDEX idx_audit_log_flag_history ON audit_log(project_id, entity_id, entity_type, created_at DESC);
```

**Step 2: Write down migration**

```sql
-- migrations/011_flag_history.down.sql
DROP INDEX IF EXISTS idx_audit_log_flag_history;
ALTER TABLE audit_log DROP COLUMN IF EXISTS user_email;
ALTER TABLE audit_log DROP COLUMN IF EXISTS environment_id;
```

**Step 3: Verify migration embeds**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go build ./migrations/...`
Expected: builds successfully (the `//go:embed *.sql` in `migrations/migrations.go` picks up new files automatically)

**Step 4: Commit**

```bash
git add migrations/011_flag_history.up.sql migrations/011_flag_history.down.sql
git commit -m "feat: add migration for flag history columns and index (#43)"
```

---

### Task 2: Update AuditEntry Model

**Files:**
- Modify: `internal/model/audit.go`

**Step 1: Add EnvironmentID and UserEmail fields**

Update the `AuditEntry` struct in `internal/model/audit.go` to:

```go
package model

import (
	"encoding/json"
	"time"
)

type AuditEntry struct {
	ID            string          `json:"id"`
	ProjectID     *string         `json:"project_id,omitempty"`
	UserID        *string         `json:"user_id,omitempty"`
	UserEmail     *string         `json:"user_email,omitempty"`
	EnvironmentID *string         `json:"environment_id,omitempty"`
	Action        string          `json:"action"`
	EntityType    string          `json:"entity_type"`
	EntityID      string          `json:"entity_id"`
	OldValue      json.RawMessage `json:"old_value,omitempty"`
	NewValue      json.RawMessage `json:"new_value,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}
```

**Step 2: Verify compilation**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go build ./internal/model/...`
Expected: builds successfully

**Step 3: Commit**

```bash
git add internal/model/audit.go
git commit -m "feat: add environment_id and user_email to AuditEntry model (#43)"
```

---

### Task 3: Update AuditStore — Record, ListByProject, and New Methods

**Files:**
- Modify: `internal/store/audit_store.go`
- Modify: `internal/store/audit_store_test.go`

**Step 1: Write failing tests for new methods**

Add these tests to `internal/store/audit_store_test.go`:

```go
func TestAuditStore_Record_WithNewFields(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-new")
	project, err := ps.Create(ctx, key, "New Fields Project", "testing new fields")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	envs, err := es.ListByProject(ctx, project.ID)
	if err != nil {
		t.Fatalf("ListByProject envs: %v", err)
	}
	if len(envs) == 0 {
		t.Fatal("expected at least one environment")
	}
	envID := envs[0].ID
	email := "test@example.com"

	entry := model.AuditEntry{
		ProjectID:     &project.ID,
		UserEmail:     &email,
		EnvironmentID: &envID,
		Action:        "update",
		EntityType:    "flag_config",
		EntityID:      "test-flag",
		NewValue:      json.RawMessage(`{"enabled":true}`),
	}

	err = as.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record with new fields: %v", err)
	}
}

func TestAuditStore_GetByID(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-get")
	project, err := ps.Create(ctx, key, "GetByID Project", "for GetByID test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	entry := model.AuditEntry{
		ProjectID:  &project.ID,
		Action:     "create",
		EntityType: "flag",
		EntityID:   "my-flag",
		NewValue:   json.RawMessage(`{"key":"my-flag"}`),
	}
	if err := as.Record(ctx, entry); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// List to find the ID
	entries, err := as.ListByProject(ctx, project.ID, 1, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}

	got, err := as.GetByID(ctx, entries[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != entries[0].ID {
		t.Errorf("expected ID %q, got %q", entries[0].ID, got.ID)
	}
	if got.EntityID != "my-flag" {
		t.Errorf("expected EntityID 'my-flag', got %q", got.EntityID)
	}
}

func TestAuditStore_ListByFlag(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-flag")
	project, err := ps.Create(ctx, key, "ListByFlag Project", "for ListByFlag test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	// Record entries for two different flags
	for _, fk := range []string{"flag-a", "flag-b"} {
		for i := 0; i < 3; i++ {
			entry := model.AuditEntry{
				ProjectID:  &project.ID,
				Action:     "update",
				EntityType: "flag_config",
				EntityID:   fk,
				NewValue:   json.RawMessage(`{"enabled":true}`),
			}
			if err := as.Record(ctx, entry); err != nil {
				t.Fatalf("Record %s-%d: %v", fk, i, err)
			}
		}
	}

	// Also record a flag-level entry for flag-a
	if err := as.Record(ctx, model.AuditEntry{
		ProjectID:  &project.ID,
		Action:     "create",
		EntityType: "flag",
		EntityID:   "flag-a",
		NewValue:   json.RawMessage(`{"key":"flag-a"}`),
	}); err != nil {
		t.Fatalf("Record flag create: %v", err)
	}

	// ListByFlag for flag-a should return 4 entries (3 config + 1 flag)
	entries, err := as.ListByFlag(ctx, project.ID, "flag-a", nil, 50, 0)
	if err != nil {
		t.Fatalf("ListByFlag: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries for flag-a, got %d", len(entries))
	}

	// All should be for flag-a
	for _, e := range entries {
		if e.EntityID != "flag-a" {
			t.Errorf("expected EntityID 'flag-a', got %q", e.EntityID)
		}
	}

	// Verify ordering is created_at DESC
	for i := 1; i < len(entries); i++ {
		if entries[i].CreatedAt.After(entries[i-1].CreatedAt) {
			t.Error("entries should be ordered by created_at DESC")
			break
		}
	}
}

func TestAuditStore_ListByFlag_EnvFilter(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-env")
	project, err := ps.Create(ctx, key, "EnvFilter Project", "for env filter test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	envs, err := es.ListByProject(ctx, project.ID)
	if err != nil || len(envs) < 2 {
		t.Fatalf("expected at least 2 environments, got %d (err: %v)", len(envs), err)
	}
	env1ID := envs[0].ID
	env2ID := envs[1].ID

	// Record 2 entries for env1, 1 for env2
	for i := 0; i < 2; i++ {
		if err := as.Record(ctx, model.AuditEntry{
			ProjectID:     &project.ID,
			EnvironmentID: &env1ID,
			Action:        "update",
			EntityType:    "flag_config",
			EntityID:      "my-flag",
			NewValue:      json.RawMessage(`{"enabled":true}`),
		}); err != nil {
			t.Fatalf("Record env1-%d: %v", i, err)
		}
	}
	if err := as.Record(ctx, model.AuditEntry{
		ProjectID:     &project.ID,
		EnvironmentID: &env2ID,
		Action:        "update",
		EntityType:    "flag_config",
		EntityID:      "my-flag",
		NewValue:      json.RawMessage(`{"enabled":false}`),
	}); err != nil {
		t.Fatalf("Record env2: %v", err)
	}

	// Filter by env1 should return 2
	entries, err := as.ListByFlag(ctx, project.ID, "my-flag", &env1ID, 50, 0)
	if err != nil {
		t.Fatalf("ListByFlag with env filter: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for env1, got %d", len(entries))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go test ./internal/store/... -run "TestAuditStore_(Record_WithNewFields|GetByID|ListByFlag)" -v`
Expected: FAIL — `GetByID` and `ListByFlag` methods don't exist yet

**Step 3: Update Record method to include new columns**

In `internal/store/audit_store.go`, update the `Record` method's INSERT to include `environment_id` and `user_email`:

```go
func (s *AuditStore) Record(ctx context.Context, entry model.AuditEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (project_id, user_id, user_email, environment_id, action, entity_type, entity_id, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.ProjectID, entry.UserID, entry.UserEmail, entry.EnvironmentID, entry.Action, entry.EntityType, entry.EntityID, entry.OldValue, entry.NewValue,
	)
	if err != nil {
		return fmt.Errorf("recording audit entry: %w", err)
	}
	return nil
}
```

**Step 4: Update ListByProject to scan new columns**

Update the `ListByProject` method to select and scan the new columns:

```go
func (s *AuditStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.AuditEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, user_id, user_email, environment_id, action, entity_type, entity_id, old_value, new_value, created_at
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit entries: %w", err)
	}
	return entries, nil
}
```

**Step 5: Add GetByID method**

Add to `internal/store/audit_store.go`:

```go
// GetByID returns a single audit entry by ID.
func (s *AuditStore) GetByID(ctx context.Context, id string) (*model.AuditEntry, error) {
	var e model.AuditEntry
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, user_id, user_email, environment_id, action, entity_type, entity_id, old_value, new_value, created_at
		 FROM audit_log WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting audit entry by id: %w", err)
	}
	return &e, nil
}
```

**Step 6: Add ListByFlag method**

Add to `internal/store/audit_store.go`:

```go
// ListByFlag returns audit entries for a specific flag within a project.
// It includes both 'flag' and 'flag_config' entity types.
// Optionally filters by environment_id if envID is non-nil.
func (s *AuditStore) ListByFlag(ctx context.Context, projectID, flagKey string, envID *string, limit, offset int) ([]model.AuditEntry, error) {
	query := `SELECT id, project_id, user_id, user_email, environment_id, action, entity_type, entity_id, old_value, new_value, created_at
		 FROM audit_log
		 WHERE project_id = $1 AND entity_id = $2 AND entity_type IN ('flag', 'flag_config')`
	args := []any{projectID, flagKey}
	argIdx := 3

	if envID != nil {
		query += fmt.Sprintf(" AND (environment_id = $%d OR environment_id IS NULL)", argIdx)
		args = append(args, *envID)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing audit entries by flag: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit entries: %w", err)
	}
	return entries, nil
}
```

**Step 7: Run tests to verify they pass**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go test ./internal/store/... -run "TestAuditStore" -v`
Expected: ALL PASS (requires PostgreSQL running via `docker compose up`)

**Step 8: Commit**

```bash
git add internal/store/audit_store.go internal/store/audit_store_test.go
git commit -m "feat: add GetByID and ListByFlag to AuditStore, update Record with new columns (#43)"
```

---

### Task 4: Fix Audit Recording in UpdateEnvironmentConfig Handler

**Files:**
- Modify: `internal/handler/flag_handler.go:449-538` (the `UpdateEnvironmentConfig` method)

**Step 1: Add old config fetch before update**

In `internal/handler/flag_handler.go`, in the `UpdateEnvironmentConfig` method, fetch the old config before calling `UpdateEnvironmentConfig` on the store. The old config fetch should go after the env lookup (line ~485) and before the store update (line ~505).

Add this block after the `env` lookup (after line 485):

```go
	// Fetch old config for audit logging
	oldConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, env.ID)
	if err != nil {
		slog.Warn("failed to fetch old config for audit", "error", err)
		// Continue — audit old_value will be nil, but the update should still proceed
	}
```

**Step 2: Update the audit recording block to include old_value, environment_id, and user_email**

Replace the audit logging block (currently lines 511-524) with:

```go
	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if oldConfig != nil {
			oldVal, _ = json.Marshal(oldConfig)
		}
		newVal, _ := json.Marshal(cfg)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			Action:        "update",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}
```

**Step 3: Verify compilation**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go build ./internal/handler/...`
Expected: builds successfully

**Step 4: Also update other flag handler audit calls to include user_email**

Update audit calls in `Create` (line ~141), `Update` (line ~280), `Delete` (line ~342), `Archive` (line ~423), and `SetStaleness` (line ~579) to also set `UserEmail: &user.Email` on the `AuditEntry`. These are simple one-line additions in each block.

**Step 5: Verify compilation again**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go build ./internal/handler/...`
Expected: builds successfully

**Step 6: Commit**

```bash
git add internal/handler/flag_handler.go
git commit -m "feat: record old_value, environment_id, user_email in flag audit entries (#43)"
```

---

### Task 5: Add History Handler (List + GetByID + Restore)

**Files:**
- Create: `internal/handler/history_handler.go`
- Modify: `cmd/togglerino/main.go` (wire up new handler + routes)

**Step 1: Create the history handler**

Create `internal/handler/history_handler.go`:

```go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

type HistoryHandler struct {
	audit        *store.AuditStore
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
}

func NewHistoryHandler(audit *store.AuditStore, flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool) *HistoryHandler {
	return &HistoryHandler{audit: audit, flags: flags, projects: projects, environments: environments, hub: hub, cache: cache, pool: pool}
}

// List handles GET /api/v1/projects/{key}/flags/{flag}/history?env=&limit=50&offset=0
func (h *HistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	if projectKey == "" || flagKey == "" {
		writeError(w, http.StatusBadRequest, "project key and flag key are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Verify flag exists
	if _, err := h.flags.FindByKey(r.Context(), project.ID, flagKey); err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var envID *string
	if envKey := r.URL.Query().Get("env"); envKey != "" {
		env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		envID = &env.ID
	}

	entries, err := h.audit.ListByFlag(r.Context(), project.ID, flagKey, envID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list flag history")
		return
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

// Get handles GET /api/v1/projects/{key}/flags/{flag}/history/{id}
func (h *HistoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	entryID := r.PathValue("id")
	if projectKey == "" || flagKey == "" || entryID == "" {
		writeError(w, http.StatusBadRequest, "project key, flag key, and history id are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	entry, err := h.audit.GetByID(r.Context(), entryID)
	if err != nil {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	// Verify entry belongs to this project and flag
	if entry.ProjectID == nil || *entry.ProjectID != project.ID || entry.EntityID != flagKey {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// Restore handles POST /api/v1/projects/{key}/flags/{flag}/history/{id}/restore
func (h *HistoryHandler) Restore(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	entryID := r.PathValue("id")
	if projectKey == "" || flagKey == "" || entryID == "" {
		writeError(w, http.StatusBadRequest, "project key, flag key, and history id are required")
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

	entry, err := h.audit.GetByID(r.Context(), entryID)
	if err != nil {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	// Verify entry belongs to this project and flag
	if entry.ProjectID == nil || *entry.ProjectID != project.ID || entry.EntityID != flagKey {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	// Only flag_config entries can be restored
	if entry.EntityType != "flag_config" {
		writeError(w, http.StatusBadRequest, "only flag_config entries can be restored")
		return
	}

	// Must have environment_id to know which env to restore to
	if entry.EnvironmentID == nil {
		writeError(w, http.StatusBadRequest, "this entry has no associated environment and cannot be restored")
		return
	}

	// Determine snapshot to restore: for "update" entries, old_value is the state before that change.
	// For "restore" entries, old_value is the state before the restore was applied.
	// We restore to old_value (the state before the recorded change).
	snapshot := entry.OldValue
	if snapshot == nil {
		// Fall back to new_value if old_value isn't available
		snapshot = entry.NewValue
	}
	if snapshot == nil {
		writeError(w, http.StatusBadRequest, "no snapshot available to restore from this entry")
		return
	}

	// Parse the snapshot
	var cfg model.FlagEnvironmentConfig
	if err := json.Unmarshal(snapshot, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse config snapshot")
		return
	}

	// Marshal variants and targeting rules back to JSON for the store method
	variantsJSON, _ := json.Marshal(cfg.Variants)
	rulesJSON, _ := json.Marshal(cfg.TargetingRules)

	// Fetch current config before restore for audit old_value
	oldConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, *entry.EnvironmentID)
	if err != nil {
		slog.Warn("failed to fetch old config for restore audit", "error", err)
	}

	// Apply the restored config using the same store method
	updated, err := h.flags.UpdateEnvironmentConfig(r.Context(), flag.ID, *entry.EnvironmentID, cfg.Enabled, cfg.DefaultVariant, variantsJSON, rulesJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore config")
		return
	}

	// Look up environment key for cache refresh and SSE broadcast
	env, err := h.environments.FindByID(r.Context(), *entry.EnvironmentID)
	if err != nil {
		slog.Warn("failed to look up environment for cache refresh", "error", err)
	}

	// Audit the restore action
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if oldConfig != nil {
			oldVal, _ = json.Marshal(oldConfig)
		}
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: entry.EnvironmentID,
			Action:        "restore",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record restore audit", "error", err)
		}
	}

	// Refresh cache and broadcast SSE
	if env != nil {
		if err := h.cache.Refresh(r.Context(), h.pool, projectKey, env.Key); err != nil {
			slog.Warn("failed to refresh cache after restore", "error", err)
		}
		h.hub.Broadcast(projectKey, env.Key, stream.Event{
			Type:    "flag_update",
			FlagKey: flagKey,
			Value:   updated.Enabled,
			Variant: updated.DefaultVariant,
		})
	}

	writeJSON(w, http.StatusOK, updated)
}
```

**Step 2: Check if EnvironmentStore has FindByID**

The `Restore` handler uses `h.environments.FindByID`. Check if this method exists on `EnvironmentStore`. If not, add it:

```go
// FindByID returns an environment by its ID.
func (s *EnvironmentStore) FindByID(ctx context.Context, id string) (*model.Environment, error) {
	var e model.Environment
	err := s.pool.QueryRow(ctx,
		`SELECT id, project_id, key, name, created_at FROM environments WHERE id = $1`, id,
	).Scan(&e.ID, &e.ProjectID, &e.Key, &e.Name, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding environment by id: %w", err)
	}
	return &e, nil
}
```

Add this to `internal/store/environment_store.go` if it doesn't already exist.

**Step 3: Wire up in main.go**

In `cmd/togglerino/main.go`, after the `auditHandler` initialization (around line 99):

Add handler creation:
```go
historyHandler := handler.NewHistoryHandler(auditStore, flagStore, projectStore, environmentStore, hub, cache, pool)
```

Add routes after the flag routes section (after line 167):
```go
	// Flag history
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/history", wrap(historyHandler.List, sessionAuth))
	mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/history/{id}", wrap(historyHandler.Get, sessionAuth))
	mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/history/{id}/restore", wrap(historyHandler.Restore, sessionAuth))
```

**Step 4: Verify compilation**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go build ./...`
Expected: builds successfully (note: won't actually run without `web/dist/`, but the packages should compile)

**Step 5: Commit**

```bash
git add internal/handler/history_handler.go internal/store/environment_store.go cmd/togglerino/main.go
git commit -m "feat: add history handler with list, get, and restore endpoints (#43)"
```

---

### Task 6: Update Frontend Types and API Client

**Files:**
- Modify: `web/src/api/types.ts:96-106` (AuditEntry interface)

**Step 1: Update AuditEntry interface**

In `web/src/api/types.ts`, update the `AuditEntry` interface to include the new fields:

```typescript
export interface AuditEntry {
  id: string
  project_id?: string
  user_id?: string
  user_email?: string
  environment_id?: string
  action: string
  entity_type: string
  entity_id: string
  old_value?: unknown
  new_value?: unknown
  created_at: string
}
```

**Step 2: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run lint`
Expected: no errors

**Step 3: Commit**

```bash
git add web/src/api/types.ts
git commit -m "feat: add environment_id and user_email to AuditEntry type (#43)"
```

---

### Task 7: Add Tabs to FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Add tab structure**

Wrap the existing page content (from "Environment Configuration" section onwards) in shadcn `Tabs`. The header (breadcrumbs, flag key, settings dropdown, name, metadata, description, owner) stays above tabs. The environment configs move into a "Configuration" tab, and a placeholder "History" tab is added.

Import the Tabs components at the top of the file:
```typescript
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
```

Then wrap the environment configuration section and dialogs with tabs. The `Tabs` component goes right after the owner section and before the mutation error alerts. Configuration tab contains the existing env config content. History tab will be a placeholder for now:

```tsx
<Tabs defaultValue="configuration" className="w-full">
  <TabsList className="mb-6">
    <TabsTrigger value="configuration">Configuration</TabsTrigger>
    <TabsTrigger value="history">History</TabsTrigger>
  </TabsList>

  <TabsContent value="configuration">
    {/* existing environment config content moves here */}
  </TabsContent>

  <TabsContent value="history">
    <div className="text-center py-12 text-muted-foreground/60 text-[13px]">
      History tab — coming next
    </div>
  </TabsContent>
</Tabs>
```

The mutation error alerts and dialogs stay outside the tabs (they're global to the page).

**Step 2: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run lint`
Expected: no errors

**Step 3: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: add tabs to flag detail page with configuration and history tabs (#43)"
```

---

### Task 8: Build FlagHistory Component

**Files:**
- Create: `web/src/components/FlagHistory.tsx`
- Modify: `web/src/pages/FlagDetailPage.tsx` (replace history placeholder)

**Step 1: Create the FlagHistory component**

Create `web/src/components/FlagHistory.tsx`:

```tsx
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { AuditEntry, Environment } from '../api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import ConfigDiff from './ConfigDiff'
import { RotateCcw, ChevronDown, ChevronRight } from 'lucide-react'

const PAGE_SIZE = 50

interface FlagHistoryProps {
  projectKey: string
  flagKey: string
  environments: Environment[]
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now()
  const date = new Date(dateStr).getTime()
  const diff = now - date
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  return new Date(dateStr).toLocaleDateString()
}

function formatAction(action: string): string {
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

export default function FlagHistory({ projectKey, flagKey, environments }: FlagHistoryProps) {
  const queryClient = useQueryClient()
  const [envFilter, setEnvFilter] = useState<string>('all')
  const [offset, setOffset] = useState(0)
  const [allEntries, setAllEntries] = useState<AuditEntry[]>([])
  const [hasMore, setHasMore] = useState(true)
  const [expandedEntries, setExpandedEntries] = useState<Set<string>>(new Set())
  const [restoreEntry, setRestoreEntry] = useState<AuditEntry | null>(null)

  const envParam = envFilter === 'all' ? '' : `&env=${envFilter}`

  const { isLoading, error } = useQuery({
    queryKey: ['projects', projectKey, 'flags', flagKey, 'history', envFilter, offset],
    queryFn: async () => {
      const entries = await api.get<AuditEntry[]>(
        `/projects/${projectKey}/flags/${flagKey}/history?limit=${PAGE_SIZE}&offset=${offset}${envParam}`
      )
      if (offset === 0) {
        setAllEntries(entries)
      } else {
        setAllEntries((prev) => [...prev, ...entries])
      }
      setHasMore(entries.length === PAGE_SIZE)
      return entries
    },
    enabled: !!projectKey && !!flagKey,
  })

  const restoreMutation = useMutation({
    mutationFn: (entryId: string) =>
      api.post(`/projects/${projectKey}/flags/${flagKey}/history/${entryId}/restore`),
    onSuccess: () => {
      setRestoreEntry(null)
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey] })
      // Reset history to refetch from scratch
      setOffset(0)
      setAllEntries([])
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey, 'history'] })
    },
  })

  const handleEnvChange = (value: string) => {
    setEnvFilter(value)
    setOffset(0)
    setAllEntries([])
    setHasMore(true)
  }

  const toggleExpanded = (id: string) => {
    setExpandedEntries((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const envNameMap = new Map(environments.map((e) => [e.id, e.name]))

  const canRestore = (entry: AuditEntry): boolean => {
    return entry.entity_type === 'flag_config' &&
      entry.environment_id != null &&
      (entry.old_value != null || entry.new_value != null)
  }

  const restoreEnvName = restoreEntry?.environment_id
    ? envNameMap.get(restoreEntry.environment_id) ?? 'unknown'
    : 'unknown'

  return (
    <div>
      {/* Environment filter */}
      <div className="flex items-center gap-3 mb-6">
        <span className="text-[11px] text-muted-foreground/50 uppercase tracking-wider font-mono">Environment</span>
        <Select value={envFilter} onValueChange={handleEnvChange}>
          <SelectTrigger className="w-[200px] h-8 text-[13px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All environments</SelectItem>
            {environments.map((env) => (
              <SelectItem key={env.id} value={env.key}>{env.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Error state */}
      {error && allEntries.length === 0 && (
        <Alert variant="destructive">
          <AlertDescription>
            Failed to load history: {error instanceof Error ? error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {/* Loading state */}
      {isLoading && allEntries.length === 0 && (
        <div className="text-center py-12 text-muted-foreground/60 text-[13px] animate-pulse">
          Loading history...
        </div>
      )}

      {/* Empty state */}
      {!isLoading && allEntries.length === 0 && !error && (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No history yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Changes to this flag will appear here.
          </div>
        </div>
      )}

      {/* Timeline */}
      {allEntries.length > 0 && (
        <div className="space-y-2">
          {allEntries.map((entry) => {
            const isExpanded = expandedEntries.has(entry.id)
            const hasDiff = entry.old_value != null && entry.new_value != null
            const hasSnapshot = entry.old_value != null || entry.new_value != null

            return (
              <div
                key={entry.id}
                className="rounded-lg border border-border hover:border-[#d4956a]/20 transition-colors"
              >
                <button
                  className="flex items-center w-full px-4 py-3 text-left cursor-pointer"
                  onClick={() => hasSnapshot && toggleExpanded(entry.id)}
                  disabled={!hasSnapshot}
                >
                  {hasSnapshot ? (
                    isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-muted-foreground mr-3 shrink-0" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-muted-foreground mr-3 shrink-0" />
                    )
                  ) : (
                    <div className="w-4 h-4 mr-3 shrink-0" />
                  )}

                  <div className="flex flex-wrap items-center gap-2 min-w-0 flex-1">
                    <span
                      className="text-xs text-muted-foreground font-mono whitespace-nowrap"
                      title={new Date(entry.created_at).toISOString()}
                    >
                      {formatRelativeTime(entry.created_at)}
                    </span>

                    <Badge variant="secondary" className="font-mono text-[11px]">
                      {formatAction(entry.action)}
                    </Badge>

                    {entry.environment_id && (
                      <Badge variant="outline" className="text-[11px]">
                        {envNameMap.get(entry.environment_id) ?? 'unknown'}
                      </Badge>
                    )}

                    <span className="text-xs text-muted-foreground/60 ml-auto whitespace-nowrap">
                      {entry.user_email ?? (entry.user_id ? entry.user_id.slice(0, 8) + '...' : 'system')}
                    </span>
                  </div>
                </button>

                {/* Expanded diff content */}
                {isExpanded && hasSnapshot && (
                  <div className="px-4 pb-4 border-t border-border/50">
                    <div className="mt-3">
                      {hasDiff ? (
                        <ConfigDiff
                          oldValue={entry.old_value}
                          newValue={entry.new_value}
                          entityType={entry.entity_type}
                        />
                      ) : (
                        <div className="text-[13px] text-muted-foreground/60 italic">
                          Snapshot available (no previous version for comparison)
                        </div>
                      )}
                    </div>

                    {canRestore(entry) && (
                      <div className="mt-3 flex justify-end">
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-[12px]"
                          onClick={(e) => {
                            e.stopPropagation()
                            setRestoreEntry(entry)
                          }}
                        >
                          <RotateCcw className="w-3 h-3 mr-1.5" />
                          Restore this version
                        </Button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Load more */}
      {hasMore && allEntries.length > 0 && (
        <div className="text-center mt-6">
          <Button
            variant="outline"
            onClick={() => setOffset((prev) => prev + PAGE_SIZE)}
            disabled={isLoading}
          >
            {isLoading ? 'Loading...' : 'Load More'}
          </Button>
        </div>
      )}

      {/* Restore confirmation dialog */}
      <Dialog open={restoreEntry !== null} onOpenChange={(open) => !open && setRestoreEntry(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Restore configuration?</DialogTitle>
            <DialogDescription>
              This will apply the configuration from{' '}
              <span className="font-mono text-foreground">
                {restoreEntry && new Date(restoreEntry.created_at).toLocaleString()}
              </span>{' '}
              to <span className="font-medium text-foreground">{restoreEnvName}</span>.
              This creates a new change entry and does not delete any history.
            </DialogDescription>
          </DialogHeader>
          {restoreMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {restoreMutation.error instanceof Error ? restoreMutation.error.message : 'Failed to restore'}
              </AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setRestoreEntry(null)}>
              Cancel
            </Button>
            <Button
              onClick={() => restoreEntry && restoreMutation.mutate(restoreEntry.id)}
              disabled={restoreMutation.isPending}
            >
              {restoreMutation.isPending ? 'Restoring...' : 'Restore'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
```

**Step 2: Wire up in FlagDetailPage.tsx**

Replace the history tab placeholder with the actual component. Import `FlagHistory` and render it:

```tsx
import FlagHistory from '../components/FlagHistory'
```

Replace the history tab content:
```tsx
<TabsContent value="history">
  {environments && environments.length > 0 && (
    <FlagHistory
      projectKey={key!}
      flagKey={flagKey!}
      environments={environments}
    />
  )}
</TabsContent>
```

**Step 3: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run lint`
Expected: may fail because `ConfigDiff` doesn't exist yet — that's expected, we'll create it next

**Step 4: Commit** (even with the lint issue — ConfigDiff is next)

```bash
git add web/src/components/FlagHistory.tsx web/src/pages/FlagDetailPage.tsx
git commit -m "feat: add FlagHistory component with timeline, restore, and env filter (#43)"
```

---

### Task 9: Build ConfigDiff Component (Structured Diff)

**Files:**
- Create: `web/src/components/ConfigDiff.tsx`

**Step 1: Create the ConfigDiff component**

This component takes `oldValue` and `newValue` (the raw JSON from audit entries) and renders a structured, human-readable diff. It handles both `flag` and `flag_config` entity types.

Create `web/src/components/ConfigDiff.tsx`:

```tsx
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { Variant, TargetingRule } from '../api/types'

interface ConfigDiffProps {
  oldValue: unknown
  newValue: unknown
  entityType: string
}

interface FlagConfigSnapshot {
  enabled?: boolean
  default_variant?: string
  variants?: Variant[]
  targeting_rules?: TargetingRule[]
}

interface FlagSnapshot {
  name?: string
  description?: string
  tags?: string[]
  flag_type?: string
  lifecycle_status?: string
  owner_id?: string
}

type DiffLine = {
  type: 'added' | 'removed' | 'changed' | 'unchanged'
  label: string
  oldVal?: string
  newVal?: string
}

function formatValue(val: unknown): string {
  if (val === null || val === undefined) return 'null'
  if (typeof val === 'boolean') return val ? 'true' : 'false'
  if (typeof val === 'string') return val
  if (typeof val === 'number') return String(val)
  return JSON.stringify(val)
}

function formatVariant(v: Variant): string {
  return `${v.key} = ${formatValue(v.value)}`
}

function formatRule(rule: TargetingRule, index: number): string {
  const conditions = rule.conditions
    .map((c) => `${c.attribute} ${c.operator} ${formatValue(c.value)}`)
    .join(' AND ')
  const rollout = rule.percentage_rollout != null ? ` (${rule.percentage_rollout}%)` : ''
  return `Rule ${index + 1}: ${conditions} → ${rule.variant}${rollout}`
}

function diffFlagConfig(oldVal: FlagConfigSnapshot, newVal: FlagConfigSnapshot): DiffLine[] {
  const lines: DiffLine[] = []

  // Enabled
  if (oldVal.enabled !== newVal.enabled) {
    lines.push({
      type: 'changed',
      label: 'Enabled',
      oldVal: formatValue(oldVal.enabled),
      newVal: formatValue(newVal.enabled),
    })
  }

  // Default variant
  if (oldVal.default_variant !== newVal.default_variant) {
    lines.push({
      type: 'changed',
      label: 'Default variant',
      oldVal: oldVal.default_variant ?? 'none',
      newVal: newVal.default_variant ?? 'none',
    })
  }

  // Variants
  const oldVariants = oldVal.variants ?? []
  const newVariants = newVal.variants ?? []
  const oldVarKeys = new Set(oldVariants.map((v) => v.key))
  const newVarKeys = new Set(newVariants.map((v) => v.key))

  for (const v of newVariants) {
    if (!oldVarKeys.has(v.key)) {
      lines.push({ type: 'added', label: `Variant: ${formatVariant(v)}` })
    }
  }
  for (const v of oldVariants) {
    if (!newVarKeys.has(v.key)) {
      lines.push({ type: 'removed', label: `Variant: ${formatVariant(v)}` })
    }
  }
  // Changed variant values
  for (const nv of newVariants) {
    const ov = oldVariants.find((v) => v.key === nv.key)
    if (ov && JSON.stringify(ov.value) !== JSON.stringify(nv.value)) {
      lines.push({
        type: 'changed',
        label: `Variant "${nv.key}"`,
        oldVal: formatValue(ov.value),
        newVal: formatValue(nv.value),
      })
    }
  }

  // Targeting rules — compare by index since rules are ordered
  const oldRules = oldVal.targeting_rules ?? []
  const newRules = newVal.targeting_rules ?? []
  const maxRules = Math.max(oldRules.length, newRules.length)

  for (let i = 0; i < maxRules; i++) {
    const or = oldRules[i]
    const nr = newRules[i]
    if (!or && nr) {
      lines.push({ type: 'added', label: formatRule(nr, i) })
    } else if (or && !nr) {
      lines.push({ type: 'removed', label: formatRule(or, i) })
    } else if (or && nr && JSON.stringify(or) !== JSON.stringify(nr)) {
      lines.push({
        type: 'changed',
        label: `Rule ${i + 1}`,
        oldVal: formatRule(or, i),
        newVal: formatRule(nr, i),
      })
    }
  }

  if (lines.length === 0) {
    lines.push({ type: 'unchanged', label: 'No changes detected' })
  }

  return lines
}

function diffFlag(oldVal: FlagSnapshot, newVal: FlagSnapshot): DiffLine[] {
  const lines: DiffLine[] = []

  if (oldVal.name !== newVal.name) {
    lines.push({ type: 'changed', label: 'Name', oldVal: oldVal.name, newVal: newVal.name })
  }
  if (oldVal.description !== newVal.description) {
    lines.push({ type: 'changed', label: 'Description', oldVal: oldVal.description ?? '', newVal: newVal.description ?? '' })
  }
  if (JSON.stringify(oldVal.tags) !== JSON.stringify(newVal.tags)) {
    lines.push({
      type: 'changed',
      label: 'Tags',
      oldVal: (oldVal.tags ?? []).join(', ') || 'none',
      newVal: (newVal.tags ?? []).join(', ') || 'none',
    })
  }
  if (oldVal.flag_type !== newVal.flag_type) {
    lines.push({ type: 'changed', label: 'Flag type', oldVal: oldVal.flag_type, newVal: newVal.flag_type })
  }
  if (oldVal.lifecycle_status !== newVal.lifecycle_status) {
    lines.push({ type: 'changed', label: 'Lifecycle', oldVal: oldVal.lifecycle_status, newVal: newVal.lifecycle_status })
  }
  if (oldVal.owner_id !== newVal.owner_id) {
    lines.push({ type: 'changed', label: 'Owner', oldVal: oldVal.owner_id ?? 'unassigned', newVal: newVal.owner_id ?? 'unassigned' })
  }

  if (lines.length === 0) {
    lines.push({ type: 'unchanged', label: 'No changes detected' })
  }

  return lines
}

export default function ConfigDiff({ oldValue, newValue, entityType }: ConfigDiffProps) {
  const oldVal = oldValue as Record<string, unknown>
  const newVal = newValue as Record<string, unknown>

  const lines = entityType === 'flag_config'
    ? diffFlagConfig(oldVal as FlagConfigSnapshot, newVal as FlagConfigSnapshot)
    : diffFlag(oldVal as FlagSnapshot, newVal as FlagSnapshot)

  return (
    <div className="space-y-1.5">
      {lines.map((line, i) => (
        <div key={i} className="flex items-start gap-2 text-[13px]">
          {line.type === 'added' && (
            <>
              <Badge variant="outline" className="text-[10px] bg-emerald-500/10 text-emerald-400 border-emerald-500/20 shrink-0">
                added
              </Badge>
              <span className="text-emerald-400">{line.label}</span>
            </>
          )}
          {line.type === 'removed' && (
            <>
              <Badge variant="outline" className="text-[10px] bg-red-500/10 text-red-400 border-red-500/20 shrink-0">
                removed
              </Badge>
              <span className="text-red-400 line-through">{line.label}</span>
            </>
          )}
          {line.type === 'changed' && (
            <>
              <Badge variant="outline" className="text-[10px] bg-amber-500/10 text-amber-400 border-amber-500/20 shrink-0">
                changed
              </Badge>
              <span className="text-muted-foreground">
                {line.label}:{' '}
                <span className={cn('font-mono text-red-400/80 line-through')}>{line.oldVal}</span>
                {' → '}
                <span className={cn('font-mono text-emerald-400')}>{line.newVal}</span>
              </span>
            </>
          )}
          {line.type === 'unchanged' && (
            <span className="text-muted-foreground/40 italic">{line.label}</span>
          )}
        </div>
      ))}
    </div>
  )
}
```

**Step 2: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run lint`
Expected: no errors

**Step 3: Commit**

```bash
git add web/src/components/ConfigDiff.tsx
git commit -m "feat: add ConfigDiff component for structured flag change diffs (#43)"
```

---

### Task 10: Update Existing AuditLogPage to Use New Fields

**Files:**
- Modify: `web/src/pages/AuditLogPage.tsx`

**Step 1: Update user column to show email**

In `web/src/pages/AuditLogPage.tsx`, update the User column (around line 125-127) to prefer `user_email` over the truncated user_id:

```tsx
<span className="text-xs text-muted-foreground font-mono">
  {entry.user_email ?? (entry.user_id ? entry.user_id.slice(0, 8) + '...' : '--')}
</span>
```

**Step 2: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run lint`
Expected: no errors

**Step 3: Commit**

```bash
git add web/src/pages/AuditLogPage.tsx
git commit -m "feat: show user email in audit log page when available (#43)"
```

---

### Task 11: Final Verification

**Step 1: Run Go build**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run build && cd .. && go build ./...`
Expected: builds successfully

**Step 2: Run Go tests (non-DB)**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go test ./internal/evaluation/... ./internal/stream/...`
Expected: PASS

**Step 3: Run frontend lint**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history/web && npm run lint`
Expected: no errors

**Step 4: Run Go tests with DB** (if PostgreSQL is running)

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/flag-history && go test ./internal/store/... -v`
Expected: all audit store tests pass including the new ones

**Step 5: Verify git log looks clean**

Run: `git log --oneline feature/flag-change-history --not main`
Expected: clean sequence of commits
