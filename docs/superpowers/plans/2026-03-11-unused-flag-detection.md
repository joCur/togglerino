# Unused Flag Detection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Track SDK evaluation activity per flag and surface unused flags in the UI, with optional lifecycle integration.

**Architecture:** Add `last_evaluated_at` column to flags table, batch-flush evaluation timestamps from an in-memory tracker every 60s, extend the staleness checker with an opt-in evaluation-based criterion, and add UI signals to the flag list/detail pages.

**Tech Stack:** Go 1.25 (stdlib), PostgreSQL, pgx/v5, React 19, TypeScript, TanStack Query, Tailwind CSS v4, shadcn/ui

---

## File Structure

**New files:**
- `internal/evaluation/tracker.go` — batched evaluation tracker
- `internal/evaluation/tracker_test.go` — tracker tests
- `migrations/026_last_evaluated_at.up.sql` — schema migration
- `migrations/026_last_evaluated_at.down.sql` — rollback migration
- `web/src/lib/date.ts` — shared `formatRelativeTime` utility (extracted from ProjectDetailPage)

**Modified files:**
- `internal/model/flag.go` — add `LastEvaluatedAt` field
- `internal/model/project_settings.go` — add `UnevaluatedStaleAfterDays` field
- `internal/store/flag_store.go` — add `UpdateLastEvaluatedAt`, update queries to include column
- `internal/store/project_settings_store.go` — parse new JSONB field
- `internal/staleness/checker.go` — add evaluation-based staleness criterion
- `internal/staleness/checker_test.go` — tests for new criterion
- `internal/handler/evaluate_handler.go` — integrate tracker
- `internal/handler/project_settings_handler.go` — handle new setting in GET/PUT
- `cmd/togglerino/main.go` — wire tracker, shutdown ordering
- `web/src/api/types.ts` — add `last_evaluated_at` to Flag, `unevaluated_stale_after_days` to settings
- `web/src/api/client.ts` — add `unevaluated_days` filter param
- `web/src/components/FlagCard.tsx` — show evaluation indicator
- `web/src/pages/FlagDetailPage.tsx` — show evaluation badge
- `web/src/pages/ProjectDetailPage.tsx` — add evaluation filter dropdown
- `web/src/pages/settings/FlagLifetimesTab.tsx` — add unevaluated staleness setting

---

## Chunk 1: Backend — Schema, Model, Store, Tracker

### Task 1: Migration + Model

**Files:**
- Create: `migrations/026_last_evaluated_at.up.sql`
- Create: `migrations/026_last_evaluated_at.down.sql`
- Modify: `internal/model/flag.go:64-81`
- Modify: `internal/model/project_settings.go:41-47`

- [ ] **Step 1: Create migration files**

Create `migrations/026_last_evaluated_at.up.sql`:
```sql
ALTER TABLE flags ADD COLUMN last_evaluated_at TIMESTAMPTZ NULL;
```

Create `migrations/026_last_evaluated_at.down.sql`:
```sql
ALTER TABLE flags DROP COLUMN IF EXISTS last_evaluated_at;
```

- [ ] **Step 2: Add `LastEvaluatedAt` to Flag model**

In `internal/model/flag.go`, add after the `UpdatedAt` field (line 77):
```go
LastEvaluatedAt          *time.Time      `json:"last_evaluated_at"`
```

- [ ] **Step 3: Add `UnevaluatedStaleAfterDays` to ProjectSettings model**

In `internal/model/project_settings.go`, add to `ProjectSettings` struct after `EnvironmentDefaults` (line 45):
```go
UnevaluatedStaleAfterDays *int                          `json:"unevaluated_stale_after_days,omitempty"`
```

- [ ] **Step 4: Commit**

```bash
git add migrations/026_last_evaluated_at.up.sql migrations/026_last_evaluated_at.down.sql internal/model/flag.go internal/model/project_settings.go
git commit -m "feat(model): add last_evaluated_at to flags and unevaluated_stale_after_days to settings (#88)"
```

### Task 2: Flag Store — Update Queries + New Method

**Files:**
- Modify: `internal/store/flag_store.go`

- [ ] **Step 1: Update `ListByProject` SELECT and Scan**

In `internal/store/flag_store.go:118`, add `f.last_evaluated_at` after `f.owner_id` in the SELECT clause.

In the Scan call (line 174), add `&f.LastEvaluatedAt` after `&f.OwnerID`.

- [ ] **Step 2: Update `FindByKey` SELECT and Scan**

In `internal/store/flag_store.go:209`, add `f.last_evaluated_at` after `f.owner_id` in the SELECT clause.

In the Scan call (line 215), add `&f.LastEvaluatedAt` after `&f.OwnerID`.

- [ ] **Step 3: Update `ListNonArchived` SELECT and Scan**

In `internal/store/flag_store.go:266`, add `last_evaluated_at` after `owner_id` in the SELECT clause.

In the Scan call (line 276), add `&f.LastEvaluatedAt` after `&f.OwnerID`.

- [ ] **Step 4: Update `Create` RETURNING and Scan**

In `internal/store/flag_store.go:37`, add `last_evaluated_at` after `owner_id` in the RETURNING clause.

In the Scan call (line 39), add `&f.LastEvaluatedAt` after `&f.OwnerID`.

- [ ] **Step 5: Update `Update` RETURNING and Scan**

In `internal/store/flag_store.go:234`, add `last_evaluated_at` after `owner_id` in the RETURNING clause.

In the Scan call (line 236), add `&f.LastEvaluatedAt` after `&f.OwnerID`.

- [ ] **Step 6: Update `SetLifecycleStatus` RETURNING and Scan**

In `internal/store/flag_store.go:251`, add `last_evaluated_at` after `owner_id` in the RETURNING clause.

In the Scan call (line 253), add `&f.LastEvaluatedAt` after `&f.OwnerID`.

- [ ] **Step 7: Add `UpdateLastEvaluatedAt` method**

Add to `internal/store/flag_store.go`:
```go
// UpdateLastEvaluatedAt batch-updates the last_evaluated_at timestamp for the given flag IDs.
func (s *FlagStore) UpdateLastEvaluatedAt(ctx context.Context, flagIDs []string) error {
	if len(flagIDs) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE flags SET last_evaluated_at = NOW() WHERE id = ANY($1)`,
		flagIDs,
	)
	if err != nil {
		return fmt.Errorf("updating last_evaluated_at: %w", err)
	}
	return nil
}
```

- [ ] **Step 8: Add `unevaluated_days` filter to `ListByProject`**

Add a new parameter `unevaluatedDays string` to `ListByProject` (after `owner`). Update the method signature:
```go
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string, owner string, unevaluatedDays string, limit, offset int) ([]model.Flag, int, error) {
```

After the owner filter block (~line 157), add:
```go
if unevaluatedDays != "" {
	if unevaluatedDays == "never" {
		query += " AND f.last_evaluated_at IS NULL"
	} else {
		query += fmt.Sprintf(" AND (f.last_evaluated_at IS NULL OR f.last_evaluated_at < NOW() - ($%d || ' days')::INTERVAL)", argIdx)
		args = append(args, unevaluatedDays)
		argIdx++
	}
}
```

Also update `ListAllByProject` to pass the new parameter through (add `unevaluatedDays string` parameter).

- [ ] **Step 9: Update all callers of `ListByProject` and `ListAllByProject`**

Search for all callers and add the empty string `""` for the new `unevaluatedDays` parameter where not applicable (flag handler List, bulk, lifecycle handler, etc.).

- [ ] **Step 10: Commit**

```bash
git add internal/store/flag_store.go
git commit -m "feat(store): add last_evaluated_at to flag queries and unevaluated_days filter (#88)"
```

### Task 3: Evaluation Tracker

**Files:**
- Create: `internal/evaluation/tracker.go`
- Create: `internal/evaluation/tracker_test.go`

- [ ] **Step 1: Write failing test for Track and Flush**

Create `internal/evaluation/tracker_test.go`:
```go
package evaluation

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockFlagUpdater struct {
	mu      sync.Mutex
	batches [][]string
}

func (m *mockFlagUpdater) UpdateLastEvaluatedAt(_ context.Context, flagIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(flagIDs))
	copy(cp, flagIDs)
	m.batches = append(m.batches, cp)
	return nil
}

func TestTracker_Track_And_Flush(t *testing.T) {
	store := &mockFlagUpdater{}
	tracker := NewTracker(store, 24*time.Hour) // long interval — we flush manually

	tracker.Track("flag-1")
	tracker.Track("flag-2")
	tracker.Track("flag-1") // duplicate should be deduplicated

	tracker.flush()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.batches) != 1 {
		t.Fatalf("expected 1 flush batch, got %d", len(store.batches))
	}
	if len(store.batches[0]) != 2 {
		t.Errorf("expected 2 unique flag IDs, got %d", len(store.batches[0]))
	}
}

func TestTracker_Flush_Empty_NoDBCall(t *testing.T) {
	store := &mockFlagUpdater{}
	tracker := NewTracker(store, 24*time.Hour)

	tracker.flush()

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.batches) != 0 {
		t.Errorf("expected no flush for empty tracker, got %d batches", len(store.batches))
	}
}

func TestTracker_Stop_FinalFlush(t *testing.T) {
	store := &mockFlagUpdater{}
	tracker := NewTracker(store, 24*time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	go tracker.Start(ctx)

	tracker.Track("flag-final")
	cancel()
	time.Sleep(50 * time.Millisecond) // allow goroutine to finish

	tracker.Stop()

	store.mu.Lock()
	defer store.mu.Unlock()

	// Should have at least 1 batch from Stop's final flush
	total := 0
	for _, b := range store.batches {
		total += len(b)
	}
	if total == 0 {
		t.Error("expected final flush to write flag-final, got 0 flags")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go test ./internal/evaluation/... -run TestTracker -v`
Expected: FAIL (NewTracker not defined)

- [ ] **Step 3: Implement tracker**

Create `internal/evaluation/tracker.go`:
```go
package evaluation

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// FlagEvaluationUpdater is the interface for batch-updating last_evaluated_at timestamps.
type FlagEvaluationUpdater interface {
	UpdateLastEvaluatedAt(ctx context.Context, flagIDs []string) error
}

// Tracker batches flag evaluation events and periodically flushes them to the database.
type Tracker struct {
	store    FlagEvaluationUpdater
	interval time.Duration

	mu      sync.Mutex
	pending map[string]struct{}
}

// NewTracker creates a new evaluation tracker.
func NewTracker(store FlagEvaluationUpdater, interval time.Duration) *Tracker {
	return &Tracker{
		store:    store,
		interval: interval,
		pending:  make(map[string]struct{}),
	}
}

// Track records that a flag was evaluated. Safe for concurrent use.
func (t *Tracker) Track(flagID string) {
	t.mu.Lock()
	t.pending[flagID] = struct{}{}
	t.mu.Unlock()
}

// Start runs the periodic flush loop. Blocks until ctx is cancelled.
func (t *Tracker) Start(ctx context.Context) {
	slog.Info("evaluation tracker started", "interval", t.interval)

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("evaluation tracker stopped")
			return
		case <-ticker.C:
			t.flush()
		}
	}
}

// Stop performs a final flush to ensure no tracked evaluations are lost.
func (t *Tracker) Stop() {
	t.flush()
}

func (t *Tracker) flush() {
	t.mu.Lock()
	if len(t.pending) == 0 {
		t.mu.Unlock()
		return
	}
	batch := t.pending
	t.pending = make(map[string]struct{})
	t.mu.Unlock()

	ids := make([]string, 0, len(batch))
	for id := range batch {
		ids = append(ids, id)
	}

	if err := t.store.UpdateLastEvaluatedAt(context.Background(), ids); err != nil {
		slog.Error("evaluation tracker: failed to flush", "count", len(ids), "error", err)
	} else {
		slog.Debug("evaluation tracker: flushed", "count", len(ids))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go test ./internal/evaluation/... -run TestTracker -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/evaluation/tracker.go internal/evaluation/tracker_test.go
git commit -m "feat(evaluation): add batched evaluation tracker (#88)"
```

### Task 4: Handler Integration + Wiring

**Files:**
- Modify: `internal/handler/evaluate_handler.go`
- Modify: `cmd/togglerino/main.go`

- [ ] **Step 1: Add tracker to EvaluateHandler**

In `internal/handler/evaluate_handler.go`, add `tracker *evaluation.Tracker` field to `EvaluateHandler` struct (line 23).

Update `NewEvaluateHandler` to accept `tracker *evaluation.Tracker` parameter and set it.

- [ ] **Step 2: Track evaluations in EvaluateAll**

In `EvaluateAll` (line 60), after the `for flagKey, fd := range flags` loop (after line 90), add:
```go
if h.tracker != nil {
	for _, fd := range flags {
		h.tracker.Track(fd.Flag.ID)
	}
}
```

- [ ] **Step 3: Track evaluations in EvaluateSingle**

In `EvaluateSingle` (line 101), after the flag is found and evaluated (before `writeJSON`), add:
```go
if h.tracker != nil {
	h.tracker.Track(fd.Flag.ID)
}
```
This should go after `fd, ok := h.cache.GetFlag(...)` succeeds (i.e., after the `!ok` block).

- [ ] **Step 4: Wire tracker in main.go**

In `cmd/togglerino/main.go`:

After cache/engine creation (~line 99), add:
```go
evalTracker := evaluation.NewTracker(flagStore, 60*time.Second)
```

After the existing `go` launches (~line 149), add:
```go
go evalTracker.Start(ctx)
```

Update the `NewEvaluateHandler` call (~line 169) to pass `evalTracker`.

In the shutdown sequence (~line 482), add after `cancelCtx()` but before `hub.Close()` and `pool.Close()`:
```go
evalTracker.Stop()
```

- [ ] **Step 5: Verify compilation**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go build ./internal/... ./cmd/...`
Expected: No errors (may fail on cmd due to embed, but internal should pass)

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go vet ./internal/...`
Expected: No issues

- [ ] **Step 6: Commit**

```bash
git add internal/handler/evaluate_handler.go cmd/togglerino/main.go
git commit -m "feat(handler): integrate evaluation tracker into evaluate endpoints (#88)"
```

### Task 5: Project Settings Store — Parse New Field

**Files:**
- Modify: `internal/store/project_settings_store.go`

- [ ] **Step 1: Update the `raw` struct in `Get`, `Upsert`, and `GetAll`**

In all three methods, the `raw` struct used for JSON unmarshaling needs `UnevaluatedStaleAfterDays`:
```go
var raw struct {
	FlagLifetimes             map[model.FlagType]*int            `json:"flag_lifetimes"`
	EnvironmentDefaults       map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
	UnevaluatedStaleAfterDays *int                                `json:"unevaluated_stale_after_days,omitempty"`
}
```

After unmarshaling, add:
```go
ps.UnevaluatedStaleAfterDays = raw.UnevaluatedStaleAfterDays
```

This applies to:
- `Get` method (~line 36-46)
- `Upsert` method (~line 97-105) — the RETURNING parse section
- `GetAll` method (~line 124-133)
- `UpsertEnvironmentDefaults` method (~line 185-193)

- [ ] **Step 2: Add `UpsertFlagSettings` method**

Add a new method that atomically writes both `flag_lifetimes` and `unevaluated_stale_after_days`, preserving `environment_defaults`:
```go
// UpsertFlagSettings creates or updates the flag_lifetimes and unevaluated_stale_after_days settings,
// preserving other settings (environment_defaults).
func (s *ProjectSettingsStore) UpsertFlagSettings(ctx context.Context, projectID string, flagLifetimes map[model.FlagType]*int, unevaluatedDays *int) (*model.ProjectSettings, error) {
	var existingJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT settings FROM project_settings WHERE project_id = $1`,
		projectID,
	).Scan(&existingJSON)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("reading existing settings: %w", err)
	}

	var full map[string]json.RawMessage
	if len(existingJSON) > 0 {
		if err := json.Unmarshal(existingJSON, &full); err != nil {
			return nil, fmt.Errorf("unmarshaling existing settings: %w", err)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}

	lifetimesJSON, err := json.Marshal(flagLifetimes)
	if err != nil {
		return nil, fmt.Errorf("marshaling flag lifetimes: %w", err)
	}
	full["flag_lifetimes"] = lifetimesJSON

	if unevaluatedDays != nil {
		daysJSON, _ := json.Marshal(unevaluatedDays)
		full["unevaluated_stale_after_days"] = daysJSON
	} else {
		delete(full, "unevaluated_stale_after_days")
	}

	mergedJSON, err := json.Marshal(full)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged settings: %w", err)
	}

	var ps model.ProjectSettings
	var returnedJSON []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO project_settings (project_id, settings)
		 VALUES ($1, $2)
		 ON CONFLICT (project_id) DO UPDATE SET settings = $2, updated_at = NOW()
		 RETURNING id, project_id, settings, updated_at`,
		projectID, mergedJSON,
	).Scan(&ps.ID, &ps.ProjectID, &returnedJSON, &ps.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting flag settings: %w", err)
	}

	var raw struct {
		FlagLifetimes             map[model.FlagType]*int            `json:"flag_lifetimes"`
		EnvironmentDefaults       map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
		UnevaluatedStaleAfterDays *int                                `json:"unevaluated_stale_after_days,omitempty"`
	}
	if err := json.Unmarshal(returnedJSON, &raw); err != nil {
		return nil, fmt.Errorf("unmarshaling upserted settings: %w", err)
	}
	ps.FlagLifetimes = raw.FlagLifetimes
	ps.EnvironmentDefaults = raw.EnvironmentDefaults
	ps.UnevaluatedStaleAfterDays = raw.UnevaluatedStaleAfterDays
	return &ps, nil
}
```

Note: The existing `Upsert` method is kept for backward compatibility (used by other callers that only update lifetimes). The handler (Task 7) will call `UpsertFlagSettings` instead, which atomically writes both fields in a single read-merge-write.

- [ ] **Step 3: Commit**

```bash
git add internal/store/project_settings_store.go
git commit -m "feat(store): parse unevaluated_stale_after_days from project settings JSONB (#88)"
```

## Chunk 2: Staleness Checker + Settings Handler

### Task 6: Staleness Checker — Evaluation-Based Criterion

**Files:**
- Modify: `internal/staleness/checker.go`
- Modify: `internal/staleness/checker_test.go`

- [ ] **Step 1: Write failing test — unevaluated flag promoted when setting enabled**

Add to `internal/staleness/checker_test.go`:
```go
func TestTick_UnevaluatedFlag_PromotedWhenSettingEnabled(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Flag is only 5 days old (within 40-day release lifetime)
	// But has never been evaluated, and project has unevaluated_stale_after_days=7
	// 5 < 7, so should NOT be promoted yet
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("young-unused", "proj-eval", model.FlagTypeRelease, model.LifecycleActive, now.Add(-5*24*time.Hour), nil),
		},
	}
	settings := &mockSettingsStore{
		settings: map[string]*model.ProjectSettings{
			"proj-eval": {
				ProjectID:                 "proj-eval",
				UnevaluatedStaleAfterDays: intPtr(7),
			},
		},
	}
	c := &Checker{
		flags:    flags,
		settings: settings,
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotion (flag younger than unevaluated threshold), got %d", len(flags.promoted))
	}
}

func TestTick_UnevaluatedFlag_OlderThanThreshold_Promoted(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Flag is 10 days old, never evaluated, threshold is 7 days
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("old-unused", "proj-eval", model.FlagTypeRelease, model.LifecycleActive, now.Add(-10*24*time.Hour), nil),
		},
	}
	settings := &mockSettingsStore{
		settings: map[string]*model.ProjectSettings{
			"proj-eval": {
				ProjectID:                 "proj-eval",
				UnevaluatedStaleAfterDays: intPtr(7),
			},
		},
	}
	c := &Checker{
		flags:    flags,
		settings: settings,
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 1 {
		t.Fatalf("expected 1 promotion, got %d", len(flags.promoted))
	}
	if flags.promoted[0].status != model.LifecyclePotentiallyStale {
		t.Errorf("expected potentially_stale, got %s", flags.promoted[0].status)
	}
}

func TestTick_RecentlyEvaluatedFlag_NotPromoted(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	evaluated := now.Add(-3 * 24 * time.Hour) // evaluated 3 days ago
	f := makeFlag("recently-used", "proj-eval", model.FlagTypeRelease, model.LifecycleActive, now.Add(-30*24*time.Hour), nil)
	f.LastEvaluatedAt = &evaluated
	flags := &mockFlagStore{flags: []model.Flag{f}}
	settings := &mockSettingsStore{
		settings: map[string]*model.ProjectSettings{
			"proj-eval": {
				ProjectID:                 "proj-eval",
				UnevaluatedStaleAfterDays: intPtr(7),
			},
		},
	}
	c := &Checker{
		flags:    flags,
		settings: settings,
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotion for recently evaluated flag, got %d", len(flags.promoted))
	}
}

func TestTick_PermanentFlagType_UnevaluatedSetting_StillPromoted(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Kill-switch has nil lifetime (permanent for age-based) but should still
	// be promoted by evaluation-based staleness if setting is enabled
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("permanent-unused", "proj-eval", model.FlagTypeKillSwitch, model.LifecycleActive, now.Add(-30*24*time.Hour), nil),
		},
	}
	settings := &mockSettingsStore{
		settings: map[string]*model.ProjectSettings{
			"proj-eval": {
				ProjectID:                 "proj-eval",
				UnevaluatedStaleAfterDays: intPtr(7),
			},
		},
	}
	c := &Checker{
		flags:    flags,
		settings: settings,
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 1 {
		t.Fatalf("expected 1 promotion for permanent flag with unevaluated setting, got %d", len(flags.promoted))
	}
	if flags.promoted[0].status != model.LifecyclePotentiallyStale {
		t.Errorf("expected potentially_stale, got %s", flags.promoted[0].status)
	}
}

func TestTick_UnevaluatedSettingDisabled_NoPromotion(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	// Flag is old and never evaluated, but setting is not configured
	flags := &mockFlagStore{
		flags: []model.Flag{
			makeFlag("unused-no-setting", "proj-nosetting", model.FlagTypeRelease, model.LifecycleActive, now.Add(-10*24*time.Hour), nil),
		},
	}
	c := &Checker{
		flags:    flags,
		settings: &mockSettingsStore{},
		audit:    &mockAudit{},
		cache:    &mockCache{},
		now:      func() time.Time { return now },
	}

	c.tick(context.Background())

	if len(flags.promoted) != 0 {
		t.Errorf("expected no promotion when unevaluated setting is disabled, got %d", len(flags.promoted))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go test ./internal/staleness/... -run TestTick_Unevaluated -v`
Expected: FAIL (new tests reference `LastEvaluatedAt` and `UnevaluatedStaleAfterDays` which exist in model but aren't used in checker logic yet; specifically `TestTick_UnevaluatedFlag_OlderThanThreshold_Promoted` should fail because the checker doesn't check evaluation status)

- [ ] **Step 3: Implement evaluation-based staleness check**

In `internal/staleness/checker.go`, modify the `tick` method. After the existing `switch f.LifecycleStatus` block (around line 114), add a second check for unevaluated flags. The complete updated loop body:

```go
for _, f := range flags {
	settings := allSettings[f.ProjectID]
	ps := &model.ProjectSettings{FlagLifetimes: nil}
	if settings != nil {
		ps = settings
	}

	lifetime := ps.GetLifetime(f.FlagType)

	// Age-based staleness (existing logic)
	if lifetime != nil {
		expectedEnd := f.CreatedAt.Add(time.Duration(*lifetime) * 24 * time.Hour)

		switch f.LifecycleStatus {
		case model.LifecycleActive:
			if now.After(expectedEnd) {
				c.promote(ctx, f, model.LifecyclePotentiallyStale)
				promoted++
				continue
			}
		case model.LifecyclePotentiallyStale:
			if f.LifecycleStatusChangedAt != nil && now.After(f.LifecycleStatusChangedAt.Add(gracePeriod)) {
				c.promote(ctx, f, model.LifecycleStale)
				promoted++
				continue
			}
		}
	}

	// Evaluation-based staleness (opt-in)
	if f.LifecycleStatus == model.LifecycleActive && ps.UnevaluatedStaleAfterDays != nil && *ps.UnevaluatedStaleAfterDays > 0 {
		threshold := time.Duration(*ps.UnevaluatedStaleAfterDays) * 24 * time.Hour
		// Only apply to flags older than the threshold (don't flag brand-new flags)
		if now.After(f.CreatedAt.Add(threshold)) {
			unevaluated := f.LastEvaluatedAt == nil || now.After(f.LastEvaluatedAt.Add(threshold))
			if unevaluated {
				c.promote(ctx, f, model.LifecyclePotentiallyStale)
				promoted++
			}
		}
	}
}
```

Note: The `continue` statements ensure a flag promoted by age-based logic isn't also checked by evaluation-based logic in the same tick. The permanent flag type check (`lifetime == nil`) was previously a `continue` — it now only skips the age-based block but still allows evaluation-based checks.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go test ./internal/staleness/... -v`
Expected: ALL PASS (both old and new tests)

- [ ] **Step 5: Commit**

```bash
git add internal/staleness/checker.go internal/staleness/checker_test.go
git commit -m "feat(staleness): add evaluation-based staleness criterion (#88)"
```

### Task 7: Project Settings Handler — GET/PUT for New Setting

**Files:**
- Modify: `internal/handler/project_settings_handler.go`

- [ ] **Step 1: Update `Get` handler to include `unevaluated_stale_after_days`**

In `internal/handler/project_settings_handler.go`, update the `Get` method's response (~line 50):
```go
resp := map[string]any{
	"flag_lifetimes": merged,
}
if settings != nil && settings.UnevaluatedStaleAfterDays != nil {
	resp["unevaluated_stale_after_days"] = *settings.UnevaluatedStaleAfterDays
}
writeJSON(w, http.StatusOK, resp)
```

- [ ] **Step 2: Update `Update` handler to accept and save `unevaluated_stale_after_days`**

In `internal/handler/project_settings_handler.go`, update the `Update` method's request struct (~line 69):
```go
var req struct {
	FlagLifetimes             map[model.FlagType]*int `json:"flag_lifetimes"`
	UnevaluatedStaleAfterDays *int                    `json:"unevaluated_stale_after_days"`
}
```

Note: No `omitempty` tag — Go's JSON decoder will set `*int` to `nil` for both omitted and explicitly null values. The frontend sends `0` to disable (treated as "off" below).

Replace the existing Upsert call and response (~line 88-97) with:
```go
// Normalize: 0 or negative means disabled (nil)
unevalDays := req.UnevaluatedStaleAfterDays
if unevalDays != nil && *unevalDays <= 0 {
	unevalDays = nil
}

settings, err := h.settings.UpsertFlagSettings(r.Context(), project.ID, req.FlagLifetimes, unevalDays)
if err != nil {
	slog.Error("failed to update project settings", "error", err)
	writeError(w, http.StatusInternalServerError, "failed to update project settings")
	return
}

resp := map[string]any{
	"flag_lifetimes": settings.FlagLifetimes,
}
if settings.UnevaluatedStaleAfterDays != nil {
	resp["unevaluated_stale_after_days"] = *settings.UnevaluatedStaleAfterDays
}
writeJSON(w, http.StatusOK, resp)
```

This uses a single `UpsertFlagSettings` method (see Task 5) that writes both `flag_lifetimes` and `unevaluated_stale_after_days` atomically.

- [ ] **Step 3: Commit**

```bash
git add internal/handler/project_settings_handler.go
git commit -m "feat(handler): expose unevaluated_stale_after_days in project settings API (#88)"
```

### Task 8: Flag Handler — Pass `unevaluated_days` Filter

**Files:**
- Modify: `internal/handler/flag_handler.go`

- [ ] **Step 1: Update `List` handler to read and pass `unevaluated_days` query param**

In the `List` handler, read the new query parameter and pass it to `ListByProject`:
```go
unevaluatedDays := r.URL.Query().Get("unevaluated_days")
```

Pass it as the new parameter to `s.flags.ListByProject(...)`.

- [ ] **Step 2: Verify all tests pass**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go test ./internal/evaluation/... ./internal/staleness/... -v`
Expected: ALL PASS

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go vet ./internal/...`
Expected: No issues

- [ ] **Step 3: Commit**

```bash
git add internal/handler/flag_handler.go
git commit -m "feat(handler): add unevaluated_days filter to flag list endpoint (#88)"
```

## Chunk 3: Frontend

### Task 9: TypeScript Types + API Client

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

- [ ] **Step 1: Add `last_evaluated_at` to Flag type**

In `web/src/api/types.ts`, add after `lifecycle_status_changed_at` (line 66):
```typescript
last_evaluated_at: string | null
```

- [ ] **Step 2: Add `unevaluated_stale_after_days` to ProjectFlagSettings**

In `web/src/api/types.ts`, update `ProjectFlagSettings` (line 74):
```typescript
export interface ProjectFlagSettings {
  flag_lifetimes: Record<FlagPurpose, number | null>
  unevaluated_stale_after_days?: number | null
}
```

- [ ] **Step 3: Add `unevaluated_days` filter to API client**

In `web/src/api/client.ts`, update the `flags.list` params type to include `unevaluated_days`:
```typescript
list: (projectKey: string, params?: { search?: string; tag?: string; lifecycle_status?: string; flag_type?: string; include?: string; limit?: number; offset?: number; unevaluated_days?: string }) => {
```

Add after the existing param setters:
```typescript
if (params?.unevaluated_days) search.set('unevaluated_days', params.unevaluated_days)
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat(web): add last_evaluated_at type and unevaluated_days filter to API client (#88)"
```

### Task 10: Flag Card — Evaluation Indicator

**Files:**
- Modify: `web/src/components/FlagCard.tsx`

- [ ] **Step 1: Add evaluation indicator to Row 4**

In `web/src/components/FlagCard.tsx`, import `formatRelativeTime` or add a local helper. Update Row 4 (line 100-117) to include evaluation info between owner and purpose:

Add before the closing `</div>` of Row 4 (before the purpose span):
```tsx
{!isArchived && (
  <span className={cn(
    'text-[11px]',
    flag.last_evaluated_at
      ? 'text-muted-foreground/50'
      : 'text-amber-400/70',
  )}>
    {flag.last_evaluated_at
      ? `Evaluated ${formatRelativeTime(flag.last_evaluated_at)}`
      : 'Never evaluated'}
  </span>
)}
```

Extract `formatRelativeTime` from `ProjectDetailPage.tsx` (line 23-37) into a shared utility at `web/src/lib/date.ts`:
```typescript
export function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSecs < 60) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 30) return `${diffDays}d ago`
  return date.toLocaleDateString()
}
```

Import it in `FlagCard.tsx`, `FlagDetailPage.tsx`, and `ProjectDetailPage.tsx` (replacing the local copy in ProjectDetailPage).

- [ ] **Step 2: Commit**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection
git add web/src/components/FlagCard.tsx
git commit -m "feat(web): show evaluation indicator on flag cards (#88)"
```

### Task 11: Flag Detail Page — Evaluation Badge

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

- [ ] **Step 1: Add evaluation badge to metadata chips**

In `web/src/pages/FlagDetailPage.tsx`, in the metadata chips section (~line 278-303), add after the tags section (before the closing `</div>`):

```tsx
<span>&middot;</span>
<Badge
  variant="secondary"
  className={cn(
    'text-[11px]',
    flag.last_evaluated_at
      ? 'bg-muted text-muted-foreground'
      : 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  )}
>
  {flag.last_evaluated_at
    ? `Last evaluated ${formatRelativeTime(flag.last_evaluated_at)}`
    : 'Never evaluated'}
</Badge>
```

Import `formatRelativeTime` from `@/lib/date` (created in Task 10).

- [ ] **Step 2: Commit**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): show evaluation badge on flag detail page (#88)"
```

### Task 12: Flag List — Evaluation Filter

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

- [ ] **Step 1: Add evaluation filter state**

In `web/src/pages/ProjectDetailPage.tsx`, add state for the new filter (after the existing filter states, ~line 46):
```typescript
const [evaluationFilter, setEvaluationFilter] = useState('')
```

- [ ] **Step 2: Pass filter to API call and query key**

Find the `useInfiniteQuery` call that fetches flags and add `unevaluated_days: evaluationFilter || undefined` to the params. Also add `evaluationFilter` to the `queryKey` array so changing the filter triggers a refetch.

- [ ] **Step 3: Add filter dropdown to the filter bar**

Add a new Select dropdown in the filter bar (alongside existing tag/purpose/status/owner filters). Follow the same pattern:
```tsx
<Select value={evaluationFilter} onValueChange={setEvaluationFilter}>
  <SelectTrigger className="w-[180px] h-8 text-[13px]">
    <SelectValue placeholder="Evaluation" />
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="all">All</SelectItem>
    <SelectItem value="never">Never evaluated</SelectItem>
    <SelectItem value="7">Not in 7 days</SelectItem>
    <SelectItem value="30">Not in 30 days</SelectItem>
    <SelectItem value="90">Not in 90 days</SelectItem>
  </SelectContent>
</Select>
```

Handle the `"all"` value by converting it to `""` in the `onValueChange`.

- [ ] **Step 4: Commit**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat(web): add evaluation filter to flag list page (#88)"
```

### Task 13: Settings UI — Unevaluated Staleness

**Files:**
- Modify: `web/src/pages/settings/FlagLifetimesTab.tsx`

- [ ] **Step 1: Add state for `unevaluatedStaleDays`**

In `web/src/pages/settings/FlagLifetimesTab.tsx`, add state:
```typescript
const [unevaluatedDays, setUnevaluatedDays] = useState<number | null>(null)
```

Initialize from fetched data in the existing `if (data && !initialized)` block:
```typescript
setUnevaluatedDays(data.unevaluated_stale_after_days ?? null)
```

- [ ] **Step 2: Include in save mutation**

Update the mutation to include the new field:
```typescript
mutationFn: (params: { flagLifetimes: Record<string, number | null>; unevaluatedStaleDays: number | null }) =>
  api.put(`/projects/${key}/settings/flags`, {
    flag_lifetimes: params.flagLifetimes,
    unevaluated_stale_after_days: params.unevaluatedStaleDays ?? 0, // 0 = disabled
  }),
```

Update `handleSave`:
```typescript
const handleSave = () => updateMutation.mutate({ flagLifetimes: lifetimes, unevaluatedStaleDays: unevaluatedDays })
```

- [ ] **Step 3: Add UI section**

Add below the existing flag lifetimes section (before the Save button):
```tsx
<div className="border-t border-border mt-4 pt-4">
  <div className="text-sm font-semibold text-foreground mb-1">
    Evaluation-Based Staleness
  </div>
  <div className="text-xs text-muted-foreground mb-3">
    Optionally mark flags as potentially stale if they haven't been evaluated by any SDK within a time window.
  </div>
  <div className="flex flex-col md:flex-row md:items-center gap-2 md:gap-4">
    <div className="md:w-[200px]">
      <div className="text-[13px] font-medium text-foreground">Unevaluated threshold</div>
      <div className="text-[11px] text-muted-foreground">Flags not evaluated within this window are marked potentially stale</div>
    </div>
    <div className="flex flex-wrap items-center gap-2">
      {unevaluatedDays === null ? (
        <Input className="w-full md:w-[120px]" value="Disabled" disabled />
      ) : (
        <Input
          className="w-full md:w-[120px]"
          type="number"
          min={1}
          value={unevaluatedDays}
          onChange={(e) => {
            const num = parseInt(e.target.value, 10)
            if (!isNaN(num) && num > 0) setUnevaluatedDays(num)
          }}
        />
      )}
      <span className="text-xs text-muted-foreground">
        {unevaluatedDays === null ? '' : 'days'}
      </span>
      <button
        type="button"
        className="text-[11px] text-muted-foreground hover:text-foreground transition-colors"
        onClick={() => setUnevaluatedDays(unevaluatedDays === null ? 30 : null)}
      >
        {unevaluatedDays === null ? 'Enable' : 'Disable'}
      </button>
    </div>
  </div>
</div>
```

- [ ] **Step 4: Verify frontend builds**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection/web && npm run build`
Expected: Build succeeds

- [ ] **Step 5: Run frontend lint**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection/web && npm run lint`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection
git add web/src/pages/settings/FlagLifetimesTab.tsx
git commit -m "feat(web): add unevaluated staleness threshold to project settings UI (#88)"
```

## Chunk 4: Final Verification

### Task 14: Full Test Suite + Lint

- [ ] **Step 1: Run all Go tests**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go test ./internal/evaluation/... ./internal/staleness/...`
Expected: ALL PASS

- [ ] **Step 2: Run go vet**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go vet ./internal/...`
Expected: No issues

- [ ] **Step 3: Run frontend build + lint**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection/web && npm run build && npm run lint`
Expected: Both succeed

- [ ] **Step 4: Verify all callers compile**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/88-unused-flag-detection && go build ./internal/...`
Expected: No errors
