# Flag Types, Lifecycle & Staleness Tracking — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add flag purpose types, unified lifecycle status, automatic staleness detection, per-project lifetime settings, and a kanban lifecycle board.

**Architecture:** Schema migration renames `flag_type` → `value_type`, introduces `flag_type` (purpose), replaces `archived` bool with `lifecycle_status` enum, adds `project_settings` table. New `internal/staleness` package runs an hourly background checker. Frontend gains a lifecycle kanban board and flag type picker.

**Tech Stack:** Go 1.25 (stdlib), PostgreSQL, pgx/v5, React 19, TypeScript, TanStack Query, Tailwind CSS, shadcn/ui

---

### Task 1: Database Migration — Rename flag_type to value_type

**Files:**
- Create: `migrations/003_rename_flag_type.up.sql`
- Create: `migrations/003_rename_flag_type.down.sql`

**Step 1: Write the up migration**

```sql
-- 003_rename_flag_type.up.sql
ALTER TABLE flags RENAME COLUMN flag_type TO value_type;
```

**Step 2: Write the down migration**

```sql
-- 003_rename_flag_type.down.sql
ALTER TABLE flags RENAME COLUMN value_type TO flag_type;
```

**Step 3: Verify migration applies**

Run: `docker compose up -d && go test ./internal/store/... -count=1 -run TestFlagStore_Create -v`
Expected: Compilation errors (Go code still references `flag_type` column). That's expected — we'll fix the Go code in Task 3.

**Step 4: Commit**

```
feat(db): rename flag_type column to value_type

Prepares for introducing flag_type as the purpose/category field
(release, experiment, operational, kill-switch, permission).
```

---

### Task 2: Database Migration — Add lifecycle status, flag purpose, project settings

**Files:**
- Create: `migrations/004_flag_lifecycle.up.sql`
- Create: `migrations/004_flag_lifecycle.down.sql`

**Step 1: Write the up migration**

```sql
-- 004_flag_lifecycle.up.sql

-- Add flag purpose type
ALTER TABLE flags ADD COLUMN flag_type TEXT NOT NULL DEFAULT 'release'
    CHECK (flag_type IN ('release', 'experiment', 'operational', 'kill-switch', 'permission'));

-- Add lifecycle status (replaces archived boolean)
ALTER TABLE flags ADD COLUMN lifecycle_status TEXT NOT NULL DEFAULT 'active'
    CHECK (lifecycle_status IN ('active', 'potentially_stale', 'stale', 'archived'));
ALTER TABLE flags ADD COLUMN lifecycle_status_changed_at TIMESTAMPTZ;

-- Migrate archived flags to lifecycle_status
UPDATE flags SET lifecycle_status = 'archived', lifecycle_status_changed_at = updated_at WHERE archived = TRUE;

-- Drop archived column
ALTER TABLE flags DROP COLUMN archived;

-- Project settings table
CREATE TABLE project_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Step 2: Write the down migration**

```sql
-- 004_flag_lifecycle.down.sql

-- Restore archived column
ALTER TABLE flags ADD COLUMN archived BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE flags SET archived = TRUE WHERE lifecycle_status = 'archived';

-- Drop new columns
ALTER TABLE flags DROP COLUMN lifecycle_status_changed_at;
ALTER TABLE flags DROP COLUMN lifecycle_status;
ALTER TABLE flags DROP COLUMN flag_type;

-- Drop project settings table
DROP TABLE IF EXISTS project_settings;
```

**Step 3: Commit**

```
feat(db): add flag lifecycle status, purpose type, and project settings

Adds flag_type (purpose: release/experiment/operational/kill-switch/
permission), lifecycle_status (active/potentially_stale/stale/archived)
replacing the archived boolean, and project_settings table for
per-project flag lifetime configuration.
```

---

### Task 3: Update Go Model — Flag struct and types

**Files:**
- Modify: `internal/model/flag.go`

**Step 1: Update the model**

Replace the entire content of `internal/model/flag.go` with:

```go
package model

import (
	"encoding/json"
	"time"
)

// ValueType describes the data type of a flag's value.
type ValueType string

const (
	ValueTypeBoolean ValueType = "boolean"
	ValueTypeString  ValueType = "string"
	ValueTypeNumber  ValueType = "number"
	ValueTypeJSON    ValueType = "json"
)

// FlagType describes the purpose/category of a flag.
type FlagType string

const (
	FlagTypeRelease     FlagType = "release"
	FlagTypeExperiment  FlagType = "experiment"
	FlagTypeOperational FlagType = "operational"
	FlagTypeKillSwitch  FlagType = "kill-switch"
	FlagTypePermission  FlagType = "permission"
)

// LifecycleStatus describes the lifecycle state of a flag.
type LifecycleStatus string

const (
	LifecycleActive           LifecycleStatus = "active"
	LifecyclePotentiallyStale LifecycleStatus = "potentially_stale"
	LifecycleStale            LifecycleStatus = "stale"
	LifecycleArchived         LifecycleStatus = "archived"
)

type Flag struct {
	ID                      string          `json:"id"`
	ProjectID               string          `json:"project_id"`
	Key                     string          `json:"key"`
	Name                    string          `json:"name"`
	Description             string          `json:"description"`
	ValueType               ValueType       `json:"value_type"`
	FlagType                FlagType        `json:"flag_type"`
	DefaultValue            json.RawMessage `json:"default_value"`
	Tags                    []string        `json:"tags"`
	LifecycleStatus         LifecycleStatus `json:"lifecycle_status"`
	LifecycleStatusChangedAt *time.Time      `json:"lifecycle_status_changed_at"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

// rest of the file (FlagEnvironmentConfig, Variant, TargetingRule, etc.) stays exactly the same
```

Keep `FlagEnvironmentConfig`, `Variant`, `TargetingRule`, `Condition`, `Operator` constants, `EvaluationContext`, and `EvaluationResult` unchanged.

**Step 2: Verify compilation fails with expected errors**

Run: `go build ./...`
Expected: Many compile errors in store, handler, evaluation, and cache code referencing old field names (`flag.Archived`, `flag.FlagType` used as old type, `model.FlagTypeBoolean` etc.). This confirms we need to update those files next.

**Step 3: Commit**

```
feat(model): update Flag struct for lifecycle and purpose types

Renames FlagType→ValueType for data types, adds FlagType for purpose
(release/experiment/operational/kill-switch/permission), replaces
Archived bool with LifecycleStatus enum and LifecycleStatusChangedAt.
```

---

### Task 4: Create ProjectSettings model

**Files:**
- Create: `internal/model/project_settings.go`

**Step 1: Write the model**

```go
package model

// DefaultFlagLifetimes returns the default expected lifetimes (in days) per flag type.
// nil means permanent (never stale).
func DefaultFlagLifetimes() map[FlagType]*int {
	return map[FlagType]*int{
		FlagTypeRelease:     intPtr(40),
		FlagTypeExperiment:  intPtr(40),
		FlagTypeOperational: intPtr(7),
		FlagTypeKillSwitch:  nil,
		FlagTypePermission:  nil,
	}
}

func intPtr(n int) *int { return &n }

// ProjectSettings holds per-project configuration.
type ProjectSettings struct {
	ID            string                `json:"id"`
	ProjectID     string                `json:"project_id"`
	FlagLifetimes map[FlagType]*int     `json:"flag_lifetimes"`
	UpdatedAt     string                `json:"updated_at"`
}

// GetLifetime returns the expected lifetime in days for a flag type,
// using the project setting if available, otherwise the global default.
func (ps *ProjectSettings) GetLifetime(ft FlagType) *int {
	if ps != nil && ps.FlagLifetimes != nil {
		if v, ok := ps.FlagLifetimes[ft]; ok {
			return v
		}
	}
	return DefaultFlagLifetimes()[ft]
}
```

**Step 2: Commit**

```
feat(model): add ProjectSettings model with flag lifetime defaults
```

---

### Task 5: Update FlagStore — SQL queries and method signatures

**Files:**
- Modify: `internal/store/flag_store.go`

This is the largest single change. Every SQL query and Scan call that touches the `flags` table needs updating.

**Step 1: Update the flag column list constant**

The recurring column list in all queries changes from:
```
id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at
```
to:
```
id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at
```

**Step 2: Update Create method**

Change the `Create` method signature — the parameter `flagType model.FlagType` becomes `valueType model.ValueType`, and add `flagType model.FlagType`:

```go
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string) (*model.Flag, error) {
```

Update the INSERT query:
```sql
INSERT INTO flags (project_id, key, name, description, value_type, flag_type, default_value, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at
```

Update the Scan call:
```go
.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt)
```

Update the parameters list (now 8 instead of 7):
```go
projectID, key, name, description, valueType, flagType, defaultValue, tags,
```

**Step 3: Update ListByProject — add lifecycle_status and flag_type filters**

Change the method signature:
```go
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string) ([]model.Flag, error) {
```

Update the SELECT column list and add filter conditions:
```go
query := `SELECT id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at
    FROM flags WHERE project_id = $1`
args := []any{projectID}
argIdx := 2

if tag != "" {
    query += fmt.Sprintf(" AND $%d = ANY(tags)", argIdx)
    args = append(args, tag)
    argIdx++
}

if search != "" {
    query += fmt.Sprintf(" AND (key ILIKE '%%' || $%d || '%%' OR name ILIKE '%%' || $%d || '%%')", argIdx, argIdx)
    args = append(args, search)
    argIdx++
}

if lifecycleStatus != "" {
    query += fmt.Sprintf(" AND lifecycle_status = $%d", argIdx)
    args = append(args, lifecycleStatus)
    argIdx++
}

if flagType != "" {
    query += fmt.Sprintf(" AND flag_type = $%d", argIdx)
    args = append(args, flagType)
    argIdx++
}
```

Update the Scan call in the rows loop:
```go
rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt)
```

**Step 4: Update FindByKey**

Update SELECT and Scan to use the new column list (same pattern as above).

**Step 5: Update Update method — add flag_type (purpose) as updatable**

Change the signature:
```go
func (s *FlagStore) Update(ctx context.Context, flagID, name, description string, tags []string, flagType model.FlagType) (*model.Flag, error) {
```

Update the SQL:
```sql
UPDATE flags SET name=$2, description=$3, tags=$4, flag_type=$5, updated_at=NOW() WHERE id=$1
RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at
```

Update Scan to use new columns.

**Step 6: Replace SetArchived with SetLifecycleStatus**

```go
func (s *FlagStore) SetLifecycleStatus(ctx context.Context, flagID string, status model.LifecycleStatus) (*model.Flag, error) {
    var f model.Flag
    err := s.pool.QueryRow(ctx,
        `UPDATE flags SET lifecycle_status=$2, lifecycle_status_changed_at=NOW(), updated_at=NOW() WHERE id=$1
         RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at`,
        flagID, status,
    ).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt)
    if err != nil {
        return nil, fmt.Errorf("setting flag lifecycle status: %w", err)
    }
    if f.Tags == nil {
        f.Tags = []string{}
    }
    return &f, nil
}
```

**Step 7: Add ListNonArchived for staleness checker**

```go
// ListNonArchived returns all flags that are not archived, for the staleness checker.
func (s *FlagStore) ListNonArchived(ctx context.Context) ([]model.Flag, error) {
    rows, err := s.pool.Query(ctx,
        `SELECT id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at
         FROM flags WHERE lifecycle_status != 'archived'`)
    if err != nil {
        return nil, fmt.Errorf("listing non-archived flags: %w", err)
    }
    defer rows.Close()

    var flags []model.Flag
    for rows.Next() {
        var f model.Flag
        if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt); err != nil {
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

**Step 8: Commit**

```
feat(store): update FlagStore for lifecycle status and flag purpose

Renames SQL columns, adds lifecycle_status/flag_type filters to
ListByProject, replaces SetArchived with SetLifecycleStatus,
adds ListNonArchived for staleness checker.
```

---

### Task 6: Create ProjectSettingsStore

**Files:**
- Create: `internal/store/project_settings_store.go`

**Step 1: Write the store**

```go
package store

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/togglerino/togglerino/internal/model"
)

type ProjectSettingsStore struct {
    pool *pgxpool.Pool
}

func NewProjectSettingsStore(pool *pgxpool.Pool) *ProjectSettingsStore {
    return &ProjectSettingsStore{pool: pool}
}

// Get returns the project settings for a project. Returns nil (no error) if no settings exist yet.
func (s *ProjectSettingsStore) Get(ctx context.Context, projectID string) (*model.ProjectSettings, error) {
    var ps model.ProjectSettings
    var settingsJSON []byte
    err := s.pool.QueryRow(ctx,
        `SELECT id, project_id, settings, updated_at FROM project_settings WHERE project_id = $1`,
        projectID,
    ).Scan(&ps.ID, &ps.ProjectID, &settingsJSON, &ps.UpdatedAt)
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, nil
        }
        return nil, fmt.Errorf("getting project settings: %w", err)
    }

    var raw struct {
        FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
    }
    if len(settingsJSON) > 0 {
        json.Unmarshal(settingsJSON, &raw)
    }
    ps.FlagLifetimes = raw.FlagLifetimes
    return &ps, nil
}

// Upsert creates or updates project settings.
func (s *ProjectSettingsStore) Upsert(ctx context.Context, projectID string, flagLifetimes map[model.FlagType]*int) (*model.ProjectSettings, error) {
    settings := struct {
        FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
    }{FlagLifetimes: flagLifetimes}

    settingsJSON, err := json.Marshal(settings)
    if err != nil {
        return nil, fmt.Errorf("marshaling settings: %w", err)
    }

    var ps model.ProjectSettings
    var returnedJSON []byte
    err = s.pool.QueryRow(ctx,
        `INSERT INTO project_settings (project_id, settings)
         VALUES ($1, $2)
         ON CONFLICT (project_id) DO UPDATE SET settings = $2, updated_at = NOW()
         RETURNING id, project_id, settings, updated_at`,
        projectID, settingsJSON,
    ).Scan(&ps.ID, &ps.ProjectID, &returnedJSON, &ps.UpdatedAt)
    if err != nil {
        return nil, fmt.Errorf("upserting project settings: %w", err)
    }

    var raw struct {
        FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
    }
    json.Unmarshal(returnedJSON, &raw)
    ps.FlagLifetimes = raw.FlagLifetimes
    return &ps, nil
}

// GetAll returns all project settings (for staleness checker bulk load).
func (s *ProjectSettingsStore) GetAll(ctx context.Context) (map[string]*model.ProjectSettings, error) {
    rows, err := s.pool.Query(ctx, `SELECT id, project_id, settings, updated_at FROM project_settings`)
    if err != nil {
        return nil, fmt.Errorf("listing project settings: %w", err)
    }
    defer rows.Close()

    result := make(map[string]*model.ProjectSettings)
    for rows.Next() {
        var ps model.ProjectSettings
        var settingsJSON []byte
        if err := rows.Scan(&ps.ID, &ps.ProjectID, &settingsJSON, &ps.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scanning project settings: %w", err)
        }
        var raw struct {
            FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
        }
        json.Unmarshal(settingsJSON, &raw)
        ps.FlagLifetimes = raw.FlagLifetimes
        result[ps.ProjectID] = &ps
    }
    return result, rows.Err()
}
```

**Step 2: Commit**

```
feat(store): add ProjectSettingsStore for per-project flag lifetimes
```

---

### Task 7: Update Evaluation Engine and Cache

**Files:**
- Modify: `internal/evaluation/engine.go` (line 21)
- Modify: `internal/evaluation/cache.go` (lines 39-44, 160-184)
- Modify: `internal/evaluation/engine_test.go` (lines 19-25, and all `makeFlag` callers)

**Step 1: Update engine.go**

Change line 21 from:
```go
if flag.Archived {
```
to:
```go
if flag.LifecycleStatus == model.LifecycleArchived {
```

**Step 2: Update cache.go baseFlagQuery**

Replace the `baseFlagQuery` constant (lines 39-49) — change `f.flag_type` to `f.value_type`, add `f.flag_type` (purpose), replace `f.archived` with `f.lifecycle_status, f.lifecycle_status_changed_at`:

```go
const baseFlagQuery = `
SELECT
    p.key AS project_key,
    e.key AS env_key,
    f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at,
    fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant, fec.variants, fec.targeting_rules, fec.updated_at
FROM flags f
JOIN projects p ON p.id = f.project_id
JOIN flag_environment_configs fec ON fec.flag_id = f.id
JOIN environments e ON e.id = fec.environment_id
`
```

**Step 3: Update scanFlagRow**

Update the Scan call (lines 160-184) to scan the new columns:

```go
err = row.Scan(
    &projectKey,
    &envKey,
    // Flag fields
    &fd.Flag.ID,
    &fd.Flag.ProjectID,
    &fd.Flag.Key,
    &fd.Flag.Name,
    &fd.Flag.Description,
    &fd.Flag.ValueType,
    &fd.Flag.FlagType,
    &fd.Flag.DefaultValue,
    &fd.Flag.Tags,
    &fd.Flag.LifecycleStatus,
    &fd.Flag.LifecycleStatusChangedAt,
    &fd.Flag.CreatedAt,
    &fd.Flag.UpdatedAt,
    // FlagEnvironmentConfig fields
    &fd.Config.ID,
    &fd.Config.FlagID,
    &fd.Config.EnvironmentID,
    &fd.Config.Enabled,
    &fd.Config.DefaultVariant,
    &variantsJSON,
    &targetingRulesJSON,
    &fecUpdatedAt,
)
```

**Step 4: Update engine_test.go**

Update the `makeFlag` helper:
```go
func makeFlag(key string, defaultValue any, lifecycleStatus model.LifecycleStatus) *model.Flag {
    return &model.Flag{
        Key:             key,
        DefaultValue:    rawJSON(defaultValue),
        LifecycleStatus: lifecycleStatus,
    }
}
```

Update every `makeFlag` call:
- `makeFlag("test-flag", false, false)` → `makeFlag("test-flag", false, model.LifecycleActive)`
- `makeFlag("test-flag", "default-val", true)` → `makeFlag("test-flag", "default-val", model.LifecycleArchived)`
- And so on for all test functions.

**Step 5: Run tests**

Run: `go test ./internal/evaluation/... -v`
Expected: All tests pass.

**Step 6: Commit**

```
feat(evaluation): update engine and cache for lifecycle status

Engine checks LifecycleStatus instead of Archived boolean.
Cache query scans new columns (value_type, flag_type, lifecycle_status,
lifecycle_status_changed_at).
```

---

### Task 8: Update FlagHandler

**Files:**
- Modify: `internal/handler/flag_handler.go`

**Step 1: Update Create handler**

In the `Create` method, update the request struct:
```go
var req struct {
    Key          string          `json:"key"`
    Name         string          `json:"name"`
    Description  string          `json:"description"`
    ValueType    model.ValueType `json:"value_type"`
    FlagType     model.FlagType  `json:"flag_type"`
    DefaultValue json.RawMessage `json:"default_value"`
    Tags         []string        `json:"tags"`
}
```

Update the defaults:
```go
if req.ValueType == "" {
    req.ValueType = model.ValueTypeBoolean
}
if req.FlagType == "" {
    req.FlagType = model.FlagTypeRelease
}
```

Update the `Create` call:
```go
flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.ValueType, req.FlagType, req.DefaultValue, req.Tags)
```

**Step 2: Update List handler**

Add new query params:
```go
tag := r.URL.Query().Get("tag")
search := r.URL.Query().Get("search")
lifecycleStatus := r.URL.Query().Get("lifecycle_status")
flagType := r.URL.Query().Get("flag_type")

flags, err := h.flags.ListByProject(r.Context(), project.ID, tag, search, lifecycleStatus, flagType)
```

**Step 3: Update Update handler**

Add `flag_type` to the request struct and pass to store:
```go
var req struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Tags        []string       `json:"tags"`
    FlagType    model.FlagType `json:"flag_type"`
}
```

```go
flagTypeToUse := req.FlagType
if flagTypeToUse == "" {
    flagTypeToUse = flag.FlagType
}
updated, err := h.flags.Update(r.Context(), flag.ID, req.Name, req.Description, req.Tags, flagTypeToUse)
```

**Step 4: Update Delete handler**

Change the guard from:
```go
if !flag.Archived {
```
to:
```go
if flag.LifecycleStatus != model.LifecycleArchived {
```

**Step 5: Update Archive handler**

Replace the `Archive` method to use `SetLifecycleStatus`:

```go
func (h *FlagHandler) Archive(w http.ResponseWriter, r *http.Request) {
    // ... projectKey, flagKey, project, flag lookup stays the same ...

    var req struct {
        Archived bool `json:"archived"`
    }
    if err := readJSON(r, &req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    var status model.LifecycleStatus
    if req.Archived {
        status = model.LifecycleArchived
    } else {
        status = model.LifecycleActive
    }

    updated, err := h.flags.SetLifecycleStatus(r.Context(), flag.ID, status)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to update flag lifecycle status")
        return
    }

    action := "archive"
    if !req.Archived {
        action = "unarchive"
    }
    // ... audit log same pattern, use updated instead of flag for new value ...

    h.refreshAllEnvironments(r.Context(), projectKey, project.ID, flagKey, stream.Event{
        Type:    "flag_update",
        Value:   updated.LifecycleStatus == model.LifecycleArchived,
        Variant: "",
    })

    writeJSON(w, http.StatusOK, updated)
}
```

**Step 6: Add SetStaleness handler**

```go
// SetStaleness handles PUT /api/v1/projects/{key}/flags/{flag}/staleness
func (h *FlagHandler) SetStaleness(w http.ResponseWriter, r *http.Request) {
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

    flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "flag not found")
        return
    }

    var req struct {
        Status string `json:"status"`
    }
    if err := readJSON(r, &req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.Status != "stale" {
        writeError(w, http.StatusBadRequest, "only 'stale' status is accepted")
        return
    }

    updated, err := h.flags.SetLifecycleStatus(r.Context(), flag.ID, model.LifecycleStale)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to update staleness")
        return
    }

    if user := auth.UserFromContext(r.Context()); user != nil {
        oldVal, _ := json.Marshal(flag)
        newVal, _ := json.Marshal(updated)
        h.audit.Record(r.Context(), model.AuditEntry{
            ProjectID:  &project.ID,
            UserID:     &user.ID,
            Action:     "staleness_change",
            EntityType: "flag",
            EntityID:   flag.Key,
            OldValue:   oldVal,
            NewValue:   newVal,
        })
    }

    writeJSON(w, http.StatusOK, updated)
}
```

**Step 7: Commit**

```
feat(handler): update FlagHandler for lifecycle and flag purpose

Updates Create/Update/List/Delete/Archive handlers for new schema.
Adds SetStaleness endpoint for manual stale marking.
```

---

### Task 9: Create ProjectSettingsHandler

**Files:**
- Create: `internal/handler/project_settings_handler.go`

**Step 1: Write the handler**

```go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/togglerino/togglerino/internal/model"
    "github.com/togglerino/togglerino/internal/store"
)

type ProjectSettingsHandler struct {
    settings *store.ProjectSettingsStore
    projects *store.ProjectStore
}

func NewProjectSettingsHandler(settings *store.ProjectSettingsStore, projects *store.ProjectStore) *ProjectSettingsHandler {
    return &ProjectSettingsHandler{settings: settings, projects: projects}
}

// Get handles GET /api/v1/projects/{key}/settings
func (h *ProjectSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

    settings, err := h.settings.Get(r.Context(), project.ID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get project settings")
        return
    }

    if settings == nil {
        // Return defaults
        writeJSON(w, http.StatusOK, map[string]any{
            "flag_lifetimes": model.DefaultFlagLifetimes(),
        })
        return
    }

    // Merge with defaults for any missing keys
    merged := model.DefaultFlagLifetimes()
    if settings.FlagLifetimes != nil {
        for k, v := range settings.FlagLifetimes {
            merged[k] = v
        }
    }

    writeJSON(w, http.StatusOK, map[string]any{
        "flag_lifetimes": merged,
    })
}

// Update handles PUT /api/v1/projects/{key}/settings
func (h *ProjectSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
        FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    settings, err := h.settings.Upsert(r.Context(), project.ID, req.FlagLifetimes)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to update project settings")
        return
    }

    writeJSON(w, http.StatusOK, map[string]any{
        "flag_lifetimes": settings.FlagLifetimes,
    })
}
```

**Step 2: Commit**

```
feat(handler): add ProjectSettingsHandler for flag lifetime config
```

---

### Task 10: Create Staleness Checker

**Files:**
- Create: `internal/staleness/checker.go`

**Step 1: Write the checker**

```go
package staleness

import (
    "context"
    "encoding/json"
    "log/slog"
    "time"

    "github.com/togglerino/togglerino/internal/model"
    "github.com/togglerino/togglerino/internal/store"
)

type Checker struct {
    flags    *store.FlagStore
    settings *store.ProjectSettingsStore
    audit    *store.AuditStore
    interval time.Duration
}

func NewChecker(flags *store.FlagStore, settings *store.ProjectSettingsStore, audit *store.AuditStore, interval time.Duration) *Checker {
    return &Checker{flags: flags, settings: settings, audit: audit, interval: interval}
}

// Run starts the staleness checker loop. Blocks until ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
    slog.Info("staleness checker started", "interval", c.interval)

    // Run immediately on startup
    c.tick(ctx)

    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            slog.Info("staleness checker stopped")
            return
        case <-ticker.C:
            c.tick(ctx)
        }
    }
}

const gracePeriod = 14 * 24 * time.Hour // 14 days

func (c *Checker) tick(ctx context.Context) {
    flags, err := c.flags.ListNonArchived(ctx)
    if err != nil {
        slog.Error("staleness checker: failed to list flags", "error", err)
        return
    }

    allSettings, err := c.settings.GetAll(ctx)
    if err != nil {
        slog.Error("staleness checker: failed to load settings", "error", err)
        return
    }

    now := time.Now()
    for _, f := range flags {
        settings := allSettings[f.ProjectID]
        ps := &model.ProjectSettings{FlagLifetimes: nil}
        if settings != nil {
            ps = settings
        }

        lifetime := ps.GetLifetime(f.FlagType)
        if lifetime == nil {
            // Permanent flag type — skip
            continue
        }

        expectedEnd := f.CreatedAt.Add(time.Duration(*lifetime) * 24 * time.Hour)

        switch f.LifecycleStatus {
        case model.LifecycleActive:
            if now.After(expectedEnd) {
                c.promote(ctx, f, model.LifecyclePotentiallyStale)
            }
        case model.LifecyclePotentiallyStale:
            if f.LifecycleStatusChangedAt != nil && now.After(f.LifecycleStatusChangedAt.Add(gracePeriod)) {
                c.promote(ctx, f, model.LifecycleStale)
            }
        case model.LifecycleStale:
            // Already stale — nothing to do
        }
    }
}

func (c *Checker) promote(ctx context.Context, flag model.Flag, newStatus model.LifecycleStatus) {
    updated, err := c.flags.SetLifecycleStatus(ctx, flag.ID, newStatus)
    if err != nil {
        slog.Error("staleness checker: failed to update status",
            "flag", flag.Key, "to", newStatus, "error", err)
        return
    }

    oldVal, _ := json.Marshal(map[string]string{"lifecycle_status": string(flag.LifecycleStatus)})
    newVal, _ := json.Marshal(map[string]string{"lifecycle_status": string(updated.LifecycleStatus)})

    if err := c.audit.Record(ctx, model.AuditEntry{
        ProjectID:  &flag.ProjectID,
        Action:     "staleness_change",
        EntityType: "flag",
        EntityID:   flag.Key,
        OldValue:   oldVal,
        NewValue:   newVal,
    }); err != nil {
        slog.Warn("staleness checker: failed to record audit", "error", err)
    }

    slog.Info("staleness checker: promoted flag",
        "flag", flag.Key, "from", flag.LifecycleStatus, "to", newStatus)
}
```

**Step 2: Commit**

```
feat(staleness): add background checker for flag lifecycle

Hourly goroutine promotes active flags to potentially_stale when
expected lifetime exceeded, then to stale after 14-day grace period.
Records audit events for all transitions.
```

---

### Task 11: Wire everything in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add imports**

Add to imports:
```go
"github.com/togglerino/togglerino/internal/staleness"
```

**Step 2: Add new stores** (after line 59, auditStore)

```go
projectSettingsStore := store.NewProjectSettingsStore(pool)
```

**Step 3: Add staleness checker** (after hub creation, line 64)

```go
stalenessChecker := staleness.NewChecker(flagStore, projectSettingsStore, auditStore, 1*time.Hour)
```

**Step 4: Start staleness checker goroutine**

Create a cancellable context and start the checker before the server. Replace `ctx := context.Background()` (line 40) with:

```go
ctx, cancelCtx := context.WithCancel(context.Background())
defer cancelCtx()
```

After the cache load (after line 69), start the checker:
```go
go stalenessChecker.Run(ctx)
```

**Step 5: Add new handlers**

After auditHandler (line 78):
```go
projectSettingsHandler := handler.NewProjectSettingsHandler(projectSettingsStore, projectStore)
```

**Step 6: Add new routes**

After the existing flag routes (line 137), add:
```go
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/staleness", wrap(flagHandler.SetStaleness, sessionAuth))
```

After the audit log route (line 140), add:
```go
mux.Handle("GET /api/v1/projects/{key}/settings/flags", wrap(projectSettingsHandler.Get, sessionAuth))
mux.Handle("PUT /api/v1/projects/{key}/settings/flags", wrap(projectSettingsHandler.Update, sessionAuth))
```

**Step 7: Cancel checker on shutdown**

In the shutdown section, before `hub.Close()` (line 205), add:
```go
cancelCtx()
```

**Step 8: Run all Go tests**

Run: `go test ./... -v`
Expected: Compilation succeeds. Store tests may need updates (see Task 12).

**Step 9: Commit**

```
feat(main): wire staleness checker and settings handler

Starts background staleness checker, registers project settings
and flag staleness endpoints, adds graceful shutdown for checker.
```

---

### Task 12: Update Go Tests

**Files:**
- Modify: `internal/store/flag_store_test.go`
- Create: `internal/store/project_settings_store_test.go`

**Step 1: Update flag_store_test.go**

Update all `Create` calls to include the new `flagType` parameter:
```go
// Old:
fs.Create(ctx, project.ID, "dark-mode", "Dark Mode", "Toggle dark mode", model.FlagTypeBoolean, defaultValue, []string{"ui", "frontend"})
// New:
fs.Create(ctx, project.ID, "dark-mode", "Dark Mode", "Toggle dark mode", model.ValueTypeBoolean, model.FlagTypeRelease, defaultValue, []string{"ui", "frontend"})
```

Update all assertions that reference `flag.FlagType` → `flag.ValueType`:
```go
// Old:
if flag.FlagType != model.FlagTypeBoolean {
// New:
if flag.ValueType != model.ValueTypeBoolean {
```

Update assertions that reference `flag.Archived`:
```go
// Old:
if flag.Archived {
// New:
if flag.LifecycleStatus != model.LifecycleActive {
```

Update `TestFlagStore_SetArchived` → rename to `TestFlagStore_SetLifecycleStatus`:
```go
func TestFlagStore_SetLifecycleStatus(t *testing.T) {
    // ... setup same ...
    if flag.LifecycleStatus != model.LifecycleActive {
        t.Fatal("expected newly created flag to be active")
    }

    archived, err := fs.SetLifecycleStatus(ctx, flag.ID, model.LifecycleArchived)
    if err != nil {
        t.Fatalf("SetLifecycleStatus(archived): %v", err)
    }
    if archived.LifecycleStatus != model.LifecycleArchived {
        t.Error("expected lifecycle_status to be archived")
    }

    unarchived, err := fs.SetLifecycleStatus(ctx, flag.ID, model.LifecycleActive)
    if err != nil {
        t.Fatalf("SetLifecycleStatus(active): %v", err)
    }
    if unarchived.LifecycleStatus != model.LifecycleActive {
        t.Error("expected lifecycle_status to be active")
    }
}
```

Update `ListByProject` calls to include new empty string parameters:
```go
// Old:
fs.ListByProject(ctx, project.ID, "", "")
// New:
fs.ListByProject(ctx, project.ID, "", "", "", "")
```

Update `Update` calls to include flag type:
```go
// Old:
fs.Update(ctx, created.ID, "New Name", "new description", []string{"new", "updated"})
// New:
fs.Update(ctx, created.ID, "New Name", "new description", []string{"new", "updated"}, model.FlagTypeRelease)
```

**Step 2: Write project_settings_store_test.go**

```go
package store_test

import (
    "context"
    "testing"

    "github.com/togglerino/togglerino/internal/model"
    "github.com/togglerino/togglerino/internal/store"
)

func TestProjectSettingsStore_GetNonExistent(t *testing.T) {
    pool := testPool(t)
    ps := store.NewProjectStore(pool)
    ss := store.NewProjectSettingsStore(pool)
    ctx := context.Background()

    projKey := uniqueKey("settingsnone")
    project, err := ps.Create(ctx, projKey, "No Settings", "test")
    if err != nil {
        t.Fatalf("creating project: %v", err)
    }

    settings, err := ss.Get(ctx, project.ID)
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if settings != nil {
        t.Error("expected nil settings for project with no settings")
    }
}

func TestProjectSettingsStore_Upsert(t *testing.T) {
    pool := testPool(t)
    ps := store.NewProjectStore(pool)
    ss := store.NewProjectSettingsStore(pool)
    ctx := context.Background()

    projKey := uniqueKey("settingsupsert")
    project, err := ps.Create(ctx, projKey, "Upsert Settings", "test")
    if err != nil {
        t.Fatalf("creating project: %v", err)
    }

    days30 := 30
    lifetimes := map[model.FlagType]*int{
        model.FlagTypeRelease: &days30,
    }

    settings, err := ss.Upsert(ctx, project.ID, lifetimes)
    if err != nil {
        t.Fatalf("Upsert: %v", err)
    }

    if settings.ProjectID != project.ID {
        t.Errorf("ProjectID: got %q, want %q", settings.ProjectID, project.ID)
    }
    if settings.FlagLifetimes == nil {
        t.Fatal("expected non-nil FlagLifetimes")
    }
    if *settings.FlagLifetimes[model.FlagTypeRelease] != 30 {
        t.Errorf("release lifetime: got %d, want 30", *settings.FlagLifetimes[model.FlagTypeRelease])
    }

    // Upsert again to update
    days20 := 20
    lifetimes[model.FlagTypeRelease] = &days20

    updated, err := ss.Upsert(ctx, project.ID, lifetimes)
    if err != nil {
        t.Fatalf("Upsert update: %v", err)
    }
    if *updated.FlagLifetimes[model.FlagTypeRelease] != 20 {
        t.Errorf("release lifetime after update: got %d, want 20", *updated.FlagLifetimes[model.FlagTypeRelease])
    }

    // Read back
    readBack, err := ss.Get(ctx, project.ID)
    if err != nil {
        t.Fatalf("Get after upsert: %v", err)
    }
    if *readBack.FlagLifetimes[model.FlagTypeRelease] != 20 {
        t.Errorf("release lifetime after read: got %d, want 20", *readBack.FlagLifetimes[model.FlagTypeRelease])
    }
}

func TestProjectSettingsStore_GetAll(t *testing.T) {
    pool := testPool(t)
    ps := store.NewProjectStore(pool)
    ss := store.NewProjectSettingsStore(pool)
    ctx := context.Background()

    projKey := uniqueKey("settingsall")
    project, err := ps.Create(ctx, projKey, "All Settings", "test")
    if err != nil {
        t.Fatalf("creating project: %v", err)
    }

    days10 := 10
    _, err = ss.Upsert(ctx, project.ID, map[model.FlagType]*int{
        model.FlagTypeOperational: &days10,
    })
    if err != nil {
        t.Fatalf("Upsert: %v", err)
    }

    all, err := ss.GetAll(ctx)
    if err != nil {
        t.Fatalf("GetAll: %v", err)
    }

    if all[project.ID] == nil {
        t.Fatal("expected settings for project")
    }
    if *all[project.ID].FlagLifetimes[model.FlagTypeOperational] != 10 {
        t.Errorf("operational lifetime: got %d, want 10", *all[project.ID].FlagLifetimes[model.FlagTypeOperational])
    }
}
```

**Step 3: Run all Go tests**

Run: `go test ./... -v`
Expected: All tests pass.

**Step 4: Commit**

```
test: update store tests for lifecycle status and add settings tests
```

---

### Task 13: Frontend — Update TypeScript Types

**Files:**
- Modify: `web/src/api/types.ts`

**Step 1: Update the Flag interface**

```typescript
export type ValueType = 'boolean' | 'string' | 'number' | 'json'
export type FlagPurpose = 'release' | 'experiment' | 'operational' | 'kill-switch' | 'permission'
export type LifecycleStatus = 'active' | 'potentially_stale' | 'stale' | 'archived'

export interface Flag {
  id: string
  project_id: string
  key: string
  name: string
  description: string
  value_type: ValueType
  flag_type: FlagPurpose
  default_value: unknown
  tags: string[]
  lifecycle_status: LifecycleStatus
  lifecycle_status_changed_at: string | null
  created_at: string
  updated_at: string
}

export interface ProjectFlagSettings {
  flag_lifetimes: Record<FlagPurpose, number | null>
}
```

**Step 2: Commit**

```
feat(web): update Flag type for lifecycle and purpose fields
```

---

### Task 14: Frontend — Update CreateFlagModal

**Files:**
- Modify: `web/src/components/CreateFlagModal.tsx`

**Step 1: Add flag purpose selector**

Add a `FLAG_PURPOSES` constant above the existing `FLAG_TYPES`:
```typescript
const FLAG_PURPOSES = [
  { value: 'release', label: 'Release', description: 'Deploy new features', lifetime: '40 days' },
  { value: 'experiment', label: 'Experiment', description: 'A/B testing', lifetime: '40 days' },
  { value: 'operational', label: 'Operational', description: 'Technical migration', lifetime: '7 days' },
  { value: 'kill-switch', label: 'Kill Switch', description: 'Graceful degradation', lifetime: 'Permanent' },
  { value: 'permission', label: 'Permission', description: 'Access control', lifetime: 'Permanent' },
]
```

Rename `FLAG_TYPES` → `VALUE_TYPES`.

Add `flagPurpose` state:
```typescript
const [flagPurpose, setFlagPurpose] = useState('release')
```

Update `resetAndClose` to also reset `flagPurpose`.

Add purpose selector before the value type selector in the form. Rename the existing "Type" label to "Value Type".

Update the mutation payload to send both:
```typescript
mutation.mutate({
  key, name, description,
  value_type: flagType,
  flag_type: flagPurpose,
  default_value: getDefaultValueParsed(),
  tags: parsedTags,
})
```

**Step 2: Commit**

```
feat(web): add flag purpose selector to CreateFlagModal
```

---

### Task 15: Frontend — Update Flag List (ProjectDetailPage)

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Replace `flag.archived` references with `flag.lifecycle_status`**

Change:
```tsx
flag.archived ? 'opacity-50' : ''
```
to:
```tsx
flag.lifecycle_status === 'archived' ? 'opacity-50' : ''
```

Update the archived badge:
```tsx
{flag.lifecycle_status !== 'active' && (
  <Badge variant="secondary" className="ml-2 text-[10px]">
    {flag.lifecycle_status === 'archived' ? 'Archived' :
     flag.lifecycle_status === 'stale' ? 'Stale' :
     flag.lifecycle_status === 'potentially_stale' ? 'Potentially Stale' : ''}
  </Badge>
)}
```

**Step 2: Replace `flag.flag_type` with `flag.value_type` in the type badge**

```tsx
<Badge variant="secondary" className="font-mono text-[11px]">{flag.value_type}</Badge>
```

**Step 3: Add flag purpose badge column**

Add a "Purpose" column to the table header and body.

**Step 4: Commit**

```
feat(web): update flag list for lifecycle status and purpose
```

---

### Task 16: Frontend — Update FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Replace all `flag.archived` references**

Change `flag.archived` → `flag.lifecycle_status === 'archived'` throughout the component.

**Step 2: Update the type badge**

Change `flag.flag_type` → `flag.value_type`.

**Step 3: Add lifecycle status badge and purpose badge to metadata card**

Add after the type badge:
```tsx
<div>
  <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-1">
    Purpose
  </div>
  <Badge variant="secondary" className="text-xs">{flag.flag_type}</Badge>
</div>
<div>
  <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-1">
    Status
  </div>
  <Badge
    variant="secondary"
    className={cn(
      'text-xs',
      flag.lifecycle_status === 'active' && 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
      flag.lifecycle_status === 'potentially_stale' && 'bg-amber-500/10 text-amber-400 border-amber-500/20',
      flag.lifecycle_status === 'stale' && 'bg-red-500/10 text-red-400 border-red-500/20',
      flag.lifecycle_status === 'archived' && 'bg-muted text-muted-foreground',
    )}
  >
    {flag.lifecycle_status.replace('_', ' ')}
  </Badge>
</div>
```

**Step 4: Add "Mark as Stale" button**

When `flag.lifecycle_status === 'potentially_stale'`, show a button in the danger zone:
```tsx
const stalenessMutation = useMutation({
  mutationFn: () => api.put(`/projects/${key}/flags/${flagKey}/staleness`, { status: 'stale' }),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
  },
})
```

**Step 5: Update ConfigEditor `flag.flag_type` references**

In `ConfigEditor`, change `flag.flag_type` → `flag.value_type` (used for default variant description text).

**Step 6: Commit**

```
feat(web): update FlagDetailPage for lifecycle and purpose
```

---

### Task 17: Frontend — Update ProjectSettingsPage with Flag Lifetimes

**Files:**
- Modify: `web/src/pages/ProjectSettingsPage.tsx`

**Step 1: Add FlagLifetimesSettings component**

Add a new card component between "General Settings" and "Members" that:
- Fetches settings from `GET /api/v1/projects/{key}/settings/flags`
- Shows input fields for each flag type (release, experiment, operational, kill-switch, permission)
- Empty / null = permanent
- Saves via `PUT /api/v1/projects/{key}/settings/flags`

**Step 2: Commit**

```
feat(web): add flag lifetime settings to ProjectSettingsPage
```

---

### Task 18: Frontend — Create LifecycleBoardPage

**Files:**
- Create: `web/src/pages/LifecycleBoardPage.tsx`

**Step 1: Write the kanban board page**

Fetches all flags via `GET /api/v1/projects/{key}/flags` and groups them into 4 columns by `lifecycle_status`:
- **Active** (green header)
- **Potentially Stale** (amber header)
- **Stale** (red header)
- **Archived** (gray header)

Each card shows:
- Flag name + key (monospace)
- Purpose badge (flag_type) with color
- Value type indicator
- Tags
- Age since creation (`X days old`)
- For potentially_stale: days since marked
- Action buttons: "Mark as Stale" (on potentially_stale), "Archive" (on stale)

Use existing shadcn Card + Badge + Button components. No drag-and-drop.

**Step 2: Commit**

```
feat(web): add lifecycle kanban board page
```

---

### Task 19: Frontend — Add Route and Navigation

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/ProjectLayout.tsx`

**Step 1: Add route in App.tsx**

Import `LifecycleBoardPage` and add route under `ProjectLayout`:
```tsx
<Route path="lifecycle" element={<LifecycleBoardPage />} />
```

**Step 2: Add navigation link in ProjectLayout.tsx**

Add a "Lifecycle" NavLink between "Flags" and "Environments" (between lines 34 and 35):
```tsx
<NavLink
  to={`/projects/${key}/lifecycle`}
  className={({ isActive }) =>
    cn(
      'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
      isActive
        ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
        : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
    )
  }
>
  Lifecycle
</NavLink>
```

**Step 3: Commit**

```
feat(web): add lifecycle board route and navigation link
```

---

### Task 20: Frontend — Update VariantEditor prop name

**Files:**
- Modify: `web/src/components/VariantEditor.tsx`

**Step 1: Rename prop**

If `VariantEditor` accepts a `flagType` prop, rename it to `valueType` for clarity (it uses the value type, not the purpose type). Update the call site in `FlagDetailPage.tsx` accordingly.

**Step 2: Commit**

```
refactor(web): rename VariantEditor flagType prop to valueType
```

---

### Task 21: Final Integration Test

**Step 1: Build the full stack**

```bash
cd web && npm install && npm run build && cd ..
go build -o togglerino ./cmd/togglerino
```

**Step 2: Run Go tests**

```bash
go test ./... -v
```

**Step 3: Run frontend lint**

```bash
cd web && npm run lint
```

**Step 4: Run SDK tests**

```bash
cd sdks/javascript && npm test
cd sdks/react && npm test
```

**Step 5: Manual smoke test**

Start the app with `docker compose up`, verify:
1. Create a flag with purpose type "release"
2. See lifecycle status "active" in flag detail
3. See lifecycle board with flag in "Active" column
4. Configure flag lifetimes in project settings
5. Archive a flag via detail page, see it in "Archived" column

**Step 6: Final commit if any fixes needed**

```
fix: address integration test issues
```
