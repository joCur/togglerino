# Bulk Flag Operations Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a bulk operations API and UI for selecting and operating on multiple flags at once (enable/disable, archive, add/remove tags, set owner).

**Architecture:** Single polymorphic `POST /api/v1/projects/{key}/flags/bulk` endpoint with an `action` field. Partial results model — each flag processed independently. New `batch_id` column on `audit_log` links related entries. Frontend adds checkboxes to FlagCards with a fixed bottom action bar.

**Tech Stack:** Go (stdlib `net/http`, `pgx/v5`), React 19, TypeScript, TanStack Query, shadcn/ui, Tailwind CSS v4.

**Design doc:** `docs/plans/2026-03-03-bulk-flag-operations-design.md`

---

### Task 1: Database Migration — Add batch_id to audit_log

**Files:**
- Create: `migrations/012_bulk_operations.up.sql`
- Create: `migrations/012_bulk_operations.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/012_bulk_operations.up.sql
ALTER TABLE audit_log ADD COLUMN batch_id UUID;
CREATE INDEX idx_audit_log_batch_id ON audit_log (batch_id) WHERE batch_id IS NOT NULL;
```

**Step 2: Write the down migration**

```sql
-- migrations/012_bulk_operations.down.sql
DROP INDEX IF EXISTS idx_audit_log_batch_id;
ALTER TABLE audit_log DROP COLUMN IF EXISTS batch_id;
```

**Step 3: Commit**

```bash
git add migrations/012_bulk_operations.up.sql migrations/012_bulk_operations.down.sql
git commit -m "feat: add batch_id column to audit_log (migration 012)"
```

---

### Task 2: Update AuditEntry Model and AuditStore

**Files:**
- Modify: `internal/model/audit.go` (AuditEntry struct, line 8)
- Modify: `internal/store/audit_store.go` (Record method, line 20; ListByProject, line 33; GetByID, line 59; ListByFlag, line 73)

**Step 1: Add BatchID field to AuditEntry**

In `internal/model/audit.go`, add `BatchID` to the struct:

```go
type AuditEntry struct {
	ID            string          `json:"id"`
	ProjectID     *string         `json:"project_id,omitempty"`
	UserID        *string         `json:"user_id,omitempty"`
	UserEmail     *string         `json:"user_email,omitempty"`
	EnvironmentID *string         `json:"environment_id,omitempty"`
	BatchID       *string         `json:"batch_id,omitempty"`
	Action        string          `json:"action"`
	EntityType    string          `json:"entity_type"`
	EntityID      string          `json:"entity_id"`
	OldValue      json.RawMessage `json:"old_value,omitempty"`
	NewValue      json.RawMessage `json:"new_value,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}
```

**Step 2: Update AuditStore.Record to include batch_id**

In `internal/store/audit_store.go`, update the `Record` method's INSERT to include `batch_id`:

```go
func (s *AuditStore) Record(ctx context.Context, entry model.AuditEntry) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO audit_log (project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		entry.ProjectID, entry.UserID, entry.UserEmail, entry.EnvironmentID, entry.BatchID, entry.Action, entry.EntityType, entry.EntityID, entry.OldValue, entry.NewValue,
	)
	if err != nil {
		return fmt.Errorf("recording audit entry: %w", err)
	}
	return nil
}
```

**Step 3: Update all SELECT queries to include batch_id**

Update `ListByProject`, `GetByID`, and `ListByFlag` to SELECT and Scan `batch_id`. Every SELECT that reads `audit_log` needs the new column added. The column list becomes:

```
id, project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value, created_at
```

And every `Scan` call gains `&e.BatchID` after `&e.EnvironmentID`.

**Step 4: Run existing tests to verify backward compatibility**

```bash
go test ./internal/store/... -run TestAuditStore -v
```

Expected: All existing audit tests pass (batch_id is nullable, so existing inserts with nil batch_id still work).

**Step 5: Commit**

```bash
git add internal/model/audit.go internal/store/audit_store.go
git commit -m "feat: add batch_id to AuditEntry model and store"
```

---

### Task 3: Update Frontend AuditEntry Type

**Files:**
- Modify: `web/src/api/types.ts` (AuditEntry interface, line 96)

**Step 1: Add batch_id to the AuditEntry TypeScript interface**

In `web/src/api/types.ts`, add `batch_id` to the `AuditEntry` interface:

```typescript
export interface AuditEntry {
  id: string
  project_id?: string
  user_id?: string
  user_email?: string
  environment_id?: string
  batch_id?: string
  action: string
  entity_type: string
  entity_id: string
  old_value?: unknown
  new_value?: unknown
  created_at: string
}
```

**Step 2: Commit**

```bash
git add web/src/api/types.ts
git commit -m "feat: add batch_id to frontend AuditEntry type"
```

---

### Task 4: Bulk Handler — Backend Implementation

**Files:**
- Modify: `internal/handler/flag_handler.go` (add BulkAction method after SetStaleness, ~line 615)
- Modify: `cmd/togglerino/main.go` (add route, ~line 169)

**Step 1: Add the BulkAction handler method**

Add this method to `internal/handler/flag_handler.go` after the `SetStaleness` method:

```go
// BulkAction handles POST /api/v1/projects/{key}/flags/bulk
func (h *FlagHandler) BulkAction(w http.ResponseWriter, r *http.Request) {
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
		Action         string   `json:"action"`
		FlagKeys       []string `json:"flag_keys"`
		EnvironmentKey string   `json:"environment_key"`
		Tags           []string `json:"tags"`
		OwnerID        *string  `json:"owner_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.FlagKeys) == 0 {
		writeError(w, http.StatusBadRequest, "flag_keys is required and must not be empty")
		return
	}

	validActions := map[string]bool{
		"enable": true, "disable": true, "archive": true,
		"add_tags": true, "remove_tags": true, "set_owner": true,
	}
	if !validActions[req.Action] {
		writeError(w, http.StatusBadRequest, "invalid action: must be one of enable, disable, archive, add_tags, remove_tags, set_owner")
		return
	}

	// Validate action-specific fields
	if (req.Action == "enable" || req.Action == "disable") && req.EnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "environment_key is required for enable/disable actions")
		return
	}
	if (req.Action == "add_tags" || req.Action == "remove_tags") && len(req.Tags) == 0 {
		writeError(w, http.StatusBadRequest, "tags is required and must not be empty for tag actions")
		return
	}

	// Resolve environment for enable/disable
	var env *model.Environment
	if req.Action == "enable" || req.Action == "disable" {
		env, err = h.environments.FindByKey(r.Context(), project.ID, req.EnvironmentKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "environment not found: "+req.EnvironmentKey)
			return
		}
	}

	user := auth.UserFromContext(r.Context())
	batchID := generateUUID()

	type bulkResult struct {
		FlagKey string `json:"flag_key"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	results := make([]bulkResult, 0, len(req.FlagKeys))
	// Track environments that need cache refresh (for enable/disable)
	refreshedEnvs := make(map[string]bool)

	for _, flagKey := range req.FlagKeys {
		flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
		if err != nil {
			results = append(results, bulkResult{FlagKey: flagKey, Error: "flag not found"})
			continue
		}

		var opErr error
		switch req.Action {
		case "enable", "disable":
			opErr = h.bulkEnableDisable(r.Context(), project, flag, env, req.Action == "enable", user, &batchID)
			if opErr == nil {
				refreshedEnvs[req.EnvironmentKey] = true
			}
		case "archive":
			opErr = h.bulkArchive(r.Context(), project, flag, user, &batchID)
		case "add_tags":
			opErr = h.bulkAddTags(r.Context(), project, flag, req.Tags, user, &batchID)
		case "remove_tags":
			opErr = h.bulkRemoveTags(r.Context(), project, flag, req.Tags, user, &batchID)
		case "set_owner":
			opErr = h.bulkSetOwner(r.Context(), project, flag, req.OwnerID, user, &batchID)
		}

		if opErr != nil {
			results = append(results, bulkResult{FlagKey: flagKey, Error: opErr.Error()})
		} else {
			results = append(results, bulkResult{FlagKey: flagKey, Success: true})
		}
	}

	// Deduplicated cache refresh + SSE broadcast for enable/disable
	if env != nil {
		if err := h.cache.Refresh(r.Context(), h.pool, projectKey, req.EnvironmentKey); err != nil {
			slog.Warn("failed to refresh cache after bulk action", "error", err)
		}
		h.hub.Broadcast(projectKey, req.EnvironmentKey, stream.Event{
			Type: "flag_update",
		})
	}

	// For archive actions, refresh all environments
	if req.Action == "archive" {
		envs, err := h.environments.ListByProject(r.Context(), project.ID)
		if err != nil {
			slog.Warn("failed to list environments for bulk archive cache refresh", "error", err)
		} else {
			for _, e := range envs {
				if err := h.cache.Refresh(r.Context(), h.pool, projectKey, e.Key); err != nil {
					slog.Warn("failed to refresh cache", "project", projectKey, "env", e.Key, "error", err)
				}
				h.hub.Broadcast(projectKey, e.Key, stream.Event{Type: "flag_update"})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"batch_id": batchID,
		"results":  results,
	})
}
```

**Step 2: Add the helper methods for each bulk action**

Add these private methods to `internal/handler/flag_handler.go`:

```go
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (h *FlagHandler) bulkEnableDisable(ctx context.Context, project *model.Project, flag *model.Flag, env *model.Environment, enable bool, user *model.User, batchID *string) error {
	if flag.LifecycleStatus == model.LifecycleArchived {
		return fmt.Errorf("flag is archived")
	}

	oldConfig, err := h.flags.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		return fmt.Errorf("failed to get environment config")
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, enable, oldConfig.DefaultVariant,
		mustMarshal(oldConfig.Variants), mustMarshal(oldConfig.TargetingRules))
	if err != nil {
		return fmt.Errorf("failed to update environment config")
	}

	if user != nil {
		oldVal, _ := json.Marshal(oldConfig)
		newVal, _ := json.Marshal(cfg)
		action := "enable"
		if !enable {
			action = "disable"
		}
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			BatchID:       batchID,
			Action:        action,
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkArchive(ctx context.Context, project *model.Project, flag *model.Flag, user *model.User, batchID *string) error {
	if flag.LifecycleStatus == model.LifecycleArchived {
		return fmt.Errorf("flag is already archived")
	}

	updated, err := h.flags.SetLifecycleStatus(ctx, flag.ID, model.LifecycleArchived)
	if err != nil {
		return fmt.Errorf("failed to archive flag")
	}

	// Cancel pending schedules
	if err := h.schedules.CancelByFlag(ctx, flag.ID, "bulk_archived"); err != nil {
		slog.Warn("failed to cancel schedules for bulk archived flag", "flag", flag.Key, "error", err)
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "archive",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkAddTags(ctx context.Context, project *model.Project, flag *model.Flag, tags []string, user *model.User, batchID *string) error {
	existing := make(map[string]bool, len(flag.Tags))
	for _, t := range flag.Tags {
		existing[t] = true
	}
	newTags := append([]string{}, flag.Tags...)
	for _, t := range tags {
		if !existing[t] {
			newTags = append(newTags, t)
		}
	}

	updated, err := h.flags.Update(ctx, flag.ID, flag.Name, flag.Description, newTags, flag.FlagType, flag.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to update tags")
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "add_tags",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkRemoveTags(ctx context.Context, project *model.Project, flag *model.Flag, tags []string, user *model.User, batchID *string) error {
	toRemove := make(map[string]bool, len(tags))
	for _, t := range tags {
		toRemove[t] = true
	}
	newTags := make([]string, 0, len(flag.Tags))
	for _, t := range flag.Tags {
		if !toRemove[t] {
			newTags = append(newTags, t)
		}
	}

	updated, err := h.flags.Update(ctx, flag.ID, flag.Name, flag.Description, newTags, flag.FlagType, flag.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to update tags")
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "remove_tags",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkSetOwner(ctx context.Context, project *model.Project, flag *model.Flag, ownerID *string, user *model.User, batchID *string) error {
	updated, err := h.flags.Update(ctx, flag.ID, flag.Name, flag.Description, flag.Tags, flag.FlagType, ownerID)
	if err != nil {
		return fmt.Errorf("failed to set owner")
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "set_owner",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
```

**Step 3: Add required imports to flag_handler.go**

Add `"crypto/rand"` and `"fmt"` to the import block in `internal/handler/flag_handler.go`.

**Step 4: Register the route in main.go**

In `cmd/togglerino/main.go`, add the bulk route after line 168 (the existing flag routes block):

```go
mux.Handle("POST /api/v1/projects/{key}/flags/bulk", wrap(flagHandler.BulkAction, sessionAuth))
```

**Important:** This MUST be registered BEFORE the `GET /api/v1/projects/{key}/flags/{flag}` route. Go's `net/http` mux matches `bulk` as a `{flag}` wildcard segment if the specific route isn't matched first. Place it after the `GET /api/v1/projects/{key}/flags` list route (line 162) and before the `GET .../{flag}` route (line 163). The final order should be:

```go
// Flags
mux.Handle("POST /api/v1/projects/{key}/flags", wrap(flagHandler.Create, sessionAuth))
mux.Handle("GET /api/v1/projects/{key}/flags", wrap(flagHandler.List, sessionAuth))
mux.Handle("POST /api/v1/projects/{key}/flags/bulk", wrap(flagHandler.BulkAction, sessionAuth))
mux.Handle("GET /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Get, sessionAuth))
// ... rest of flag routes
```

**Step 5: Verify it compiles**

```bash
go build ./cmd/togglerino/...
```

Expected: Clean build with no errors.

**Step 6: Commit**

```bash
git add internal/handler/flag_handler.go cmd/togglerino/main.go
git commit -m "feat: add bulk flag operations API endpoint"
```

---

### Task 5: Add API Client Method and Types for Frontend

**Files:**
- Modify: `web/src/api/client.ts` (add bulk method, after line 42)
- Modify: `web/src/api/types.ts` (add BulkActionRequest and BulkActionResponse types)

**Step 1: Add TypeScript types**

In `web/src/api/types.ts`, add at the end:

```typescript
export type BulkAction = 'enable' | 'disable' | 'archive' | 'add_tags' | 'remove_tags' | 'set_owner'

export interface BulkActionRequest {
  action: BulkAction
  flag_keys: string[]
  environment_key?: string
  tags?: string[]
  owner_id?: string | null
}

export interface BulkActionResult {
  flag_key: string
  success: boolean
  error?: string
}

export interface BulkActionResponse {
  batch_id: string
  results: BulkActionResult[]
}
```

**Step 2: Add the API client method**

In `web/src/api/client.ts`, add a `flags` section to the `api` object (after the `delete` method on line 42, before `segments`):

```typescript
flags: {
  bulk: (projectKey: string, body: BulkActionRequest) =>
    request<BulkActionResponse>(`/projects/${projectKey}/flags/bulk`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
},
```

Update the import at the top of `client.ts`:

```typescript
import type { Condition, Segment, BulkActionRequest, BulkActionResponse } from './types'
```

**Step 3: Run lint to verify**

```bash
cd web && npm run lint
```

Expected: No errors.

**Step 4: Commit**

```bash
git add web/src/api/client.ts web/src/api/types.ts
git commit -m "feat: add bulk operations API client and types"
```

---

### Task 6: BulkActionBar Component

**Files:**
- Create: `web/src/components/BulkActionBar.tsx`

**Step 1: Create the BulkActionBar component**

This is a fixed bottom bar that appears when flags are selected. It contains:
- Selected count display
- Action dropdown (using native select matching existing filter style)
- Environment dropdown (conditional, for enable/disable)
- Tag input (conditional, for add_tags/remove_tags)
- Owner dropdown (conditional, for set_owner)
- Execute button

```tsx
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Environment, User, BulkAction } from '../api/types'

interface Props {
  selectedCount: number
  environments: Environment[]
  users: User[]
  onExecute: (action: BulkAction, params: {
    environmentKey?: string
    tags?: string[]
    ownerId?: string | null
  }) => void
  onClear: () => void
}

export default function BulkActionBar({ selectedCount, environments, users, onExecute, onClear }: Props) {
  const [action, setAction] = useState<BulkAction | ''>('')
  const [envKey, setEnvKey] = useState('')
  const [tagInput, setTagInput] = useState('')
  const [ownerId, setOwnerId] = useState<string>('')

  const needsEnv = action === 'enable' || action === 'disable'
  const needsTags = action === 'add_tags' || action === 'remove_tags'
  const needsOwner = action === 'set_owner'

  const canExecute =
    action !== '' &&
    (!needsEnv || envKey !== '') &&
    (!needsTags || tagInput.trim() !== '') &&
    (!needsOwner || true) // owner can be null (unassign)

  const handleExecute = () => {
    if (!action) return
    onExecute(action, {
      environmentKey: needsEnv ? envKey : undefined,
      tags: needsTags ? tagInput.split(',').map((t) => t.trim()).filter(Boolean) : undefined,
      ownerId: needsOwner ? (ownerId || null) : undefined,
    })
  }

  return (
    <div className="fixed bottom-0 left-0 right-0 z-50 border-t bg-card/95 backdrop-blur-sm px-4 py-3 animate-[slideUp_200ms_ease]">
      <div className="max-w-5xl mx-auto flex items-center gap-3 flex-wrap">
        <span className="text-[13px] text-foreground font-medium whitespace-nowrap">
          {selectedCount} flag{selectedCount !== 1 ? 's' : ''} selected
        </span>

        <select
          className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[140px]"
          value={action}
          onChange={(e) => setAction(e.target.value as BulkAction | '')}
        >
          <option value="">Select action...</option>
          <option value="enable">Enable</option>
          <option value="disable">Disable</option>
          <option value="archive">Archive</option>
          <option value="add_tags">Add Tags</option>
          <option value="remove_tags">Remove Tags</option>
          <option value="set_owner">Set Owner</option>
        </select>

        {needsEnv && (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[140px]"
            value={envKey}
            onChange={(e) => setEnvKey(e.target.value)}
          >
            <option value="">Select environment...</option>
            {environments.map((env) => (
              <option key={env.id} value={env.key}>{env.name}</option>
            ))}
          </select>
        )}

        {needsTags && (
          <Input
            className="w-48"
            placeholder="tag1, tag2..."
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
          />
        )}

        {needsOwner && (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[140px]"
            value={ownerId}
            onChange={(e) => setOwnerId(e.target.value)}
          >
            <option value="">Unassign owner</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>{u.display_name ?? u.email}</option>
            ))}
          </select>
        )}

        <div className="flex items-center gap-2 ml-auto">
          <Button variant="ghost" size="sm" onClick={onClear} className="text-[13px]">
            Clear
          </Button>
          <Button
            size="sm"
            disabled={!canExecute}
            onClick={handleExecute}
            className="text-[13px]"
          >
            Execute
          </Button>
        </div>
      </div>
    </div>
  )
}
```

**Step 2: Add the slideUp keyframe**

The `animate-[slideUp_200ms_ease]` class needs a keyframe. Check if `web/src/index.css` has custom keyframes. If not, add to the `@theme` block:

```css
@keyframes slideUp {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}
```

**Step 3: Run lint**

```bash
cd web && npm run lint
```

**Step 4: Commit**

```bash
git add web/src/components/BulkActionBar.tsx web/src/index.css
git commit -m "feat: add BulkActionBar component"
```

---

### Task 7: BulkConfirmDialog Component

**Files:**
- Create: `web/src/components/BulkConfirmDialog.tsx`

**Step 1: Create the confirmation dialog**

Uses the existing shadcn `Dialog` component. Shows a summary of the action, lists affected flags, and displays results after execution.

```tsx
import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { api } from '../api/client'
import type { BulkAction, BulkActionResponse, BulkActionResult } from '../api/types'

interface Props {
  open: boolean
  onClose: () => void
  projectKey: string
  flagKeys: string[]
  action: BulkAction
  environmentKey?: string
  tags?: string[]
  ownerId?: string | null
  onComplete: () => void
}

const actionLabels: Record<BulkAction, string> = {
  enable: 'Enable',
  disable: 'Disable',
  archive: 'Archive',
  add_tags: 'Add tags to',
  remove_tags: 'Remove tags from',
  set_owner: 'Set owner for',
}

export default function BulkConfirmDialog({
  open,
  onClose,
  projectKey,
  flagKeys,
  action,
  environmentKey,
  tags,
  ownerId,
  onComplete,
}: Props) {
  const [results, setResults] = useState<BulkActionResult[] | null>(null)

  const mutation = useMutation({
    mutationFn: () =>
      api.flags.bulk(projectKey, {
        action,
        flag_keys: flagKeys,
        environment_key: environmentKey,
        tags,
        owner_id: ownerId,
      }),
    onSuccess: (data: BulkActionResponse) => {
      setResults(data.results)
    },
  })

  const handleClose = () => {
    if (results) {
      onComplete()
    }
    setResults(null)
    mutation.reset()
    onClose()
  }

  const summary = `${actionLabels[action]} ${flagKeys.length} flag${flagKeys.length !== 1 ? 's' : ''}${environmentKey ? ` in ${environmentKey}` : ''}?`
  const successCount = results?.filter((r) => r.success).length ?? 0
  const failCount = results?.filter((r) => !r.success).length ?? 0

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-[15px]">
            {results ? 'Bulk Action Results' : 'Confirm Bulk Action'}
          </DialogTitle>
          <DialogDescription className="text-[13px] text-muted-foreground/60">
            {results
              ? `${successCount} succeeded, ${failCount} failed`
              : summary}
          </DialogDescription>
        </DialogHeader>

        {!results ? (
          <>
            {tags && tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-2">
                {tags.map((t) => (
                  <Badge key={t} variant="secondary" className="text-[11px]">{t}</Badge>
                ))}
              </div>
            )}
            <div className="max-h-48 overflow-y-auto space-y-1">
              {flagKeys.map((key) => (
                <div key={key} className="text-[13px] font-mono text-[#d4956a] px-2 py-1 rounded bg-muted/30">
                  {key}
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className="max-h-48 overflow-y-auto space-y-1">
            {results.map((r) => (
              <div key={r.flag_key} className="flex items-center justify-between px-2 py-1 rounded bg-muted/30">
                <span className="text-[13px] font-mono text-[#d4956a]">{r.flag_key}</span>
                {r.success ? (
                  <Badge variant="secondary" className="text-[10px] bg-emerald-500/10 text-emerald-400 border-emerald-500/20">
                    OK
                  </Badge>
                ) : (
                  <Badge variant="secondary" className="text-[10px] bg-red-500/10 text-red-400 border-red-500/20">
                    {r.error}
                  </Badge>
                )}
              </div>
            ))}
          </div>
        )}

        <DialogFooter>
          {!results ? (
            <>
              <Button variant="ghost" onClick={handleClose} disabled={mutation.isPending}>
                Cancel
              </Button>
              <Button onClick={() => mutation.mutate()} disabled={mutation.isPending}>
                {mutation.isPending ? 'Processing...' : 'Confirm'}
              </Button>
            </>
          ) : (
            <Button onClick={handleClose}>Done</Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

**Step 2: Run lint**

```bash
cd web && npm run lint
```

**Step 3: Commit**

```bash
git add web/src/components/BulkConfirmDialog.tsx
git commit -m "feat: add BulkConfirmDialog component"
```

---

### Task 8: Integrate Bulk Selection into ProjectDetailPage

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`
- Modify: `web/src/components/FlagCard.tsx`

**Step 1: Add selection state and checkbox to FlagCard**

In `web/src/components/FlagCard.tsx`, add `selected` and `onSelect` props:

```tsx
interface Props {
  flag: Flag
  environments: Environment[]
  getEnvStatus: (flagKey: string, envId: string) => boolean
  onClick: () => void
  selected?: boolean
  onSelect?: (flagKey: string) => void
}

export default function FlagCard({ flag, environments, getEnvStatus, onClick, selected, onSelect }: Props) {
```

Add a checkbox as the first element inside the outer `<div>`, and wrap the click handler so clicking the checkbox doesn't navigate:

The outer div's `onClick` should check if the click target is the checkbox and skip navigation. The simplest approach: add a checkbox that calls `onSelect` with `stopPropagation`.

In the FlagCard return, add at the very top inside the main div (before `{/* Row 1 */}`):

```tsx
{onSelect && (
  <div className="flex items-center mb-2">
    <input
      type="checkbox"
      checked={selected ?? false}
      onChange={(e) => { e.stopPropagation(); onSelect(flag.key) }}
      onClick={(e) => e.stopPropagation()}
      className="w-4 h-4 rounded border-muted-foreground/30 accent-[#d4956a] cursor-pointer"
    />
  </div>
)}
```

**Step 2: Add selection state and bulk action flow to ProjectDetailPage**

In `web/src/pages/ProjectDetailPage.tsx`:

1. Add imports for `BulkActionBar`, `BulkConfirmDialog`, and the `BulkAction` type
2. Add selection state: `const [selectedFlags, setSelectedFlags] = useState<Set<string>>(new Set())`
3. Add bulk dialog state: `const [bulkDialogOpen, setBulkDialogOpen] = useState(false)` and `const [bulkAction, setBulkAction] = useState<{action: BulkAction, ...} | null>(null)`
4. Add `toggleSelect` function that adds/removes from the Set
5. Add "Select all" / "Deselect all" checkbox in the filter bar
6. Pass `selected` and `onSelect` to each `FlagCard`
7. Render `BulkActionBar` when `selectedFlags.size > 0`
8. Render `BulkConfirmDialog` when `bulkDialogOpen`
9. Reset selection on filter changes (add `selectedFlags` cleanup in a useEffect or clear in the filter handlers)
10. On `onComplete` callback from dialog: invalidate queries and clear selection

The `onExecute` callback from `BulkActionBar` should:
- Set `bulkAction` state with all params
- Open `BulkConfirmDialog`

The `onComplete` callback from `BulkConfirmDialog` should:
- `queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })`
- `queryClient.invalidateQueries({ queryKey: ['projects', key, 'all-configs'] })`
- Clear `selectedFlags`

**Step 3: Run lint**

```bash
cd web && npm run lint
```

**Step 4: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx web/src/components/FlagCard.tsx
git commit -m "feat: integrate bulk flag selection and action bar into flag list"
```

---

### Task 9: Go Test — Bulk Operations Store-Level Test

**Files:**
- Modify: `internal/store/audit_store_test.go` (add batch_id test)

**Step 1: Write a test for audit entries with batch_id**

In `internal/store/audit_store_test.go`, add:

```go
func TestAuditStore_Record_WithBatchID(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	as := store.NewAuditStore(pool)
	ctx := context.Background()

	key := uniqueKey("audit-batch")
	project, err := ps.Create(ctx, key, "Batch ID Project", "testing batch_id")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	batchID := "550e8400-e29b-41d4-a716-446655440000"
	entry := model.AuditEntry{
		ProjectID:  &project.ID,
		BatchID:    &batchID,
		Action:     "enable",
		EntityType: "flag_config",
		EntityID:   "test-flag",
		NewValue:   json.RawMessage(`{"enabled":true}`),
	}

	err = as.Record(ctx, entry)
	if err != nil {
		t.Fatalf("Record with batch_id: %v", err)
	}

	// Verify the batch_id was stored and can be read back
	entries, err := as.ListByProject(ctx, project.ID, 1, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 entry")
	}
	if entries[0].BatchID == nil || *entries[0].BatchID != batchID {
		t.Errorf("BatchID: got %v, want %q", entries[0].BatchID, batchID)
	}
}
```

**Step 2: Run the test**

```bash
go test ./internal/store/... -run TestAuditStore_Record_WithBatchID -v
```

Expected: PASS

**Step 3: Run all store tests to verify no regressions**

```bash
go test ./internal/store/... -v
```

Expected: All tests pass.

**Step 4: Commit**

```bash
git add internal/store/audit_store_test.go
git commit -m "test: add audit store test for batch_id"
```

---

### Task 10: Full Integration Verification

**Step 1: Build the full binary (frontend + backend)**

```bash
cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino
```

Expected: Clean build.

**Step 2: Run all Go tests**

```bash
go test ./...
```

Expected: All tests pass.

**Step 3: Run frontend lint**

```bash
cd web && npm run lint
```

Expected: No errors.

**Step 4: Final commit if any fixes were needed**

If any fixes were made during verification, commit them.
