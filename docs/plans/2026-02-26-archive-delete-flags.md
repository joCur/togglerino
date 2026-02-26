# Archive & Delete Feature Flags — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow users to archive feature flags (reversible) and permanently delete archived flags, with proper cache invalidation, SSE notifications, and frontend UI.

**Architecture:** Two-step removal: archive first (via `PUT .../archive`), then hard delete (existing `DELETE` route, guarded to require `archived=true`). The `archived` field already exists in DB and model; the evaluation engine already returns default value with reason "archived". We add the SSE `flag_deleted` event type for SDK notification on deletion.

**Tech Stack:** Go 1.25 stdlib, pgx/v5, React 19, TanStack Query, shadcn/ui, Tailwind CSS v4

---

### Task 1: Add `SetArchived` to FlagStore

**Files:**
- Modify: `internal/store/flag_store.go` (after `Update` method, ~line 158)
- Test: `internal/store/flag_store_test.go`

**Step 1: Write the failing test**

Add to `internal/store/flag_store_test.go`:

```go
func TestFlagStore_SetArchived(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagarchive")
	project, err := ps.Create(ctx, projKey, "Archive Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "archive-me", "Archive Me", "test", model.FlagTypeBoolean, json.RawMessage(`false`), []string{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if flag.Archived {
		t.Fatal("expected newly created flag to not be archived")
	}

	// Archive
	archived, err := fs.SetArchived(ctx, flag.ID, true)
	if err != nil {
		t.Fatalf("SetArchived(true): %v", err)
	}
	if !archived.Archived {
		t.Error("expected Archived to be true")
	}
	if archived.ID != flag.ID {
		t.Errorf("ID: got %q, want %q", archived.ID, flag.ID)
	}

	// Unarchive
	unarchived, err := fs.SetArchived(ctx, flag.ID, false)
	if err != nil {
		t.Fatalf("SetArchived(false): %v", err)
	}
	if unarchived.Archived {
		t.Error("expected Archived to be false")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestFlagStore_SetArchived -v`
Expected: FAIL — `fs.SetArchived` does not exist

**Step 3: Write minimal implementation**

Add to `internal/store/flag_store.go` after the `Update` method:

```go
// SetArchived sets the archived status of a flag.
func (s *FlagStore) SetArchived(ctx context.Context, flagID string, archived bool) (*model.Flag, error) {
	var f model.Flag
	err := s.pool.QueryRow(ctx,
		`UPDATE flags SET archived=$2, updated_at=NOW() WHERE id=$1
		 RETURNING id, project_id, key, name, description, flag_type, default_value, tags, archived, created_at, updated_at`,
		flagID, archived,
	).Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.FlagType, &f.DefaultValue, &f.Tags, &f.Archived, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("setting flag archived status: %w", err)
	}
	if f.Tags == nil {
		f.Tags = []string{}
	}
	return &f, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -run TestFlagStore_SetArchived -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/flag_store.go internal/store/flag_store_test.go
git commit -m "feat: add FlagStore.SetArchived method"
```

---

### Task 2: Add `Type` field to SSE Event and update StreamHandler

**Files:**
- Modify: `internal/stream/hub.go` (line 6-10, `Event` struct)
- Modify: `internal/handler/stream_handler.go` (line 58-60, event writing)

**Step 1: Add `Type` field to `stream.Event`**

In `internal/stream/hub.go`, change the `Event` struct:

```go
// Event represents a flag change event sent to SSE clients.
type Event struct {
	Type    string `json:"type"`
	FlagKey string `json:"flag_key"`
	Value   any    `json:"value"`
	Variant string `json:"variant"`
}
```

**Step 2: Update StreamHandler to use `event.Type` as SSE event name**

In `internal/handler/stream_handler.go`, change line 60:

```go
// Before:
fmt.Fprintf(w, "event: flag_update\ndata: %s\n\n", data)

// After:
eventName := event.Type
if eventName == "" {
    eventName = "flag_update"
}
fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, data)
```

**Step 3: Update existing Broadcast call in `flag_handler.go` (UpdateEnvironmentConfig)**

In `internal/handler/flag_handler.go` at line 360, add `Type: "flag_update"`:

```go
h.hub.Broadcast(projectKey, envKey, stream.Event{
    Type:    "flag_update",
    FlagKey: flagKey,
    Value:   cfg.Enabled,
    Variant: cfg.DefaultVariant,
})
```

**Step 4: Run all Go tests to verify nothing breaks**

Run: `go test ./internal/stream/... ./internal/handler/... -v`
Expected: PASS (all existing tests)

**Step 5: Commit**

```bash
git add internal/stream/hub.go internal/handler/stream_handler.go internal/handler/flag_handler.go
git commit -m "feat: add Type field to SSE Event for event name routing"
```

---

### Task 3: Add Archive handler and fix Delete handler (cache + SSE + guard)

**Files:**
- Modify: `internal/handler/flag_handler.go`
- Modify: `cmd/togglerino/main.go` (add archive route, ~line 136)

**Step 1: Add `refreshAllEnvironments` helper to flag handler**

Add this private method to `internal/handler/flag_handler.go`:

```go
// refreshAllEnvironments refreshes the evaluation cache and broadcasts SSE events
// for all environments in a project after a flag change (archive/delete).
func (h *FlagHandler) refreshAllEnvironments(ctx context.Context, projectKey, projectID, flagKey string, event stream.Event) {
	envs, err := h.environments.ListByProject(ctx, projectID)
	if err != nil {
		slog.Warn("failed to list environments for cache refresh", "error", err)
		return
	}
	for _, env := range envs {
		if err := h.cache.Refresh(ctx, h.pool, projectKey, env.Key); err != nil {
			slog.Warn("failed to refresh cache", "project", projectKey, "env", env.Key, "error", err)
		}
		event.FlagKey = flagKey
		h.hub.Broadcast(projectKey, env.Key, event)
	}
}
```

**Step 2: Add the Archive handler**

Add to `internal/handler/flag_handler.go` (after the `Delete` method):

```go
// Archive handles PUT /api/v1/projects/{key}/flags/{flag}/archive
func (h *FlagHandler) Archive(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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
		Archived bool `json:"archived"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.flags.SetArchived(r.Context(), flag.ID, req.Archived)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update flag archive status")
		return
	}

	// Best-effort audit logging
	action := "archive"
	if !req.Archived {
		action = "unarchive"
	}
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     action,
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache and broadcast for all environments
	h.refreshAllEnvironments(r.Context(), projectKey, project.ID, flagKey, stream.Event{
		Type:    "flag_update",
		Value:   updated.Archived,
		Variant: "",
	})

	writeJSON(w, http.StatusOK, updated)
}
```

**Step 3: Fix the Delete handler**

Replace the existing `Delete` method in `internal/handler/flag_handler.go` with:

```go
// Delete handles DELETE /api/v1/projects/{key}/flags/{flag}
func (h *FlagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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

	// Guard: only archived flags can be deleted
	if !flag.Archived {
		writeError(w, http.StatusConflict, "flag must be archived before it can be deleted")
		return
	}

	if err := h.flags.Delete(r.Context(), flag.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete flag")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "delete",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache and broadcast deletion for all environments
	h.refreshAllEnvironments(r.Context(), projectKey, project.ID, flagKey, stream.Event{
		Type: "flag_deleted",
	})

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Register the archive route**

In `cmd/togglerino/main.go`, add after the existing flag routes (after line 136):

```go
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/archive", wrap(flagHandler.Archive, sessionAuth))
```

**Step 5: Run Go tests**

Run: `go test ./internal/handler/... ./cmd/togglerino/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/handler/flag_handler.go cmd/togglerino/main.go
git commit -m "feat: add archive handler, guard delete behind archived, fix cache/SSE on delete"
```

---

### Task 4: Update JavaScript SDK to handle `flag_deleted` events

**Files:**
- Modify: `sdks/javascript/src/types.ts` (add `FlagDeletedEvent`, update `EventType`)
- Modify: `sdks/javascript/src/client.ts` (~line 417-447, `handleSSEEvent`)
- Test: `sdks/javascript/src/__tests__/client.test.ts`

**Step 1: Add types**

In `sdks/javascript/src/types.ts`, add after `FlagChangeEvent`:

```typescript
/**
 * SSE event emitted when a flag is deleted.
 */
export interface FlagDeletedEvent {
  flagKey: string
}
```

Update `EventType`:

```typescript
export type EventType = 'change' | 'deleted' | 'context_change' | 'error' | 'ready' | 'reconnecting' | 'reconnected'
```

**Step 2: Update `handleSSEEvent` in `sdks/javascript/src/client.ts`**

Replace the `handleSSEEvent` method (~line 417-447):

```typescript
private handleSSEEvent(raw: string): void {
    let eventType = ''
    let data = ''

    for (const line of raw.split('\n')) {
      if (line.startsWith('event:')) {
        eventType = line.slice('event:'.length).trim()
      } else if (line.startsWith('data:')) {
        data = line.slice('data:'.length).trim()
      }
      // Lines starting with ":" are comments (keepalives), ignore them
    }

    if (!data) return

    if (eventType === 'flag_deleted') {
      try {
        const event: FlagDeletedEvent = JSON.parse(data)
        this.flags.delete(event.flagKey)
        this.emit('deleted', event)
      } catch {
        // Ignore malformed SSE data
      }
      return
    }

    if (eventType !== 'flag_update') return

    try {
      const event: FlagChangeEvent = JSON.parse(data)

      // Update the flag in the local cache
      const existing = this.flags.get(event.flagKey)
      this.flags.set(event.flagKey, {
        value: event.value,
        variant: event.variant,
        reason: existing?.reason ?? 'stream_update',
      })

      this.emit('change', event)
    } catch {
      // Ignore malformed SSE data
    }
  }
```

Also add `FlagDeletedEvent` to the import in `sdks/javascript/src/client.ts` (line 5):

```typescript
import type {
  TogglerinoConfig,
  EvaluationContext,
  EvaluationResult,
  FlagChangeEvent,
  FlagDeletedEvent,
  EventType,
} from './types'
```

And add to exports in `sdks/javascript/src/index.ts`:

```typescript
export type {
  TogglerinoConfig,
  EvaluationContext,
  EvaluationResult,
  FlagChangeEvent,
  FlagDeletedEvent,
  EventType,
} from './types'
```

**Step 3: Add test for `flag_deleted` event**

Add to `sdks/javascript/src/__tests__/client.test.ts` after the existing SSE test:

```typescript
it('should handle flag_deleted SSE events by removing the flag', async () => {
    // Initial fetch
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'delete-me': { value: true, variant: 'on', reason: 'default' },
        'keep-me': { value: false, variant: 'off', reason: 'default' },
      })
    )

    // Create a mock ReadableStream that emits a flag_deleted event then closes
    const sseData =
      'event: flag_deleted\ndata: {"flagKey":"delete-me"}\n\n'
    const encoder = new TextEncoder()
    let readerDone = false

    const mockStream = new ReadableStream({
      pull(controller) {
        if (!readerDone) {
          readerDone = true
          controller.enqueue(encoder.encode(sseData))
        } else {
          controller.close()
        }
      },
    })

    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: mockStream,
    } as unknown as Response)

    const client = new TogglerinoClient({
      serverUrl: 'http://localhost:8080',
      sdkKey: 'test-key',
      streaming: true,
    })

    const deletedEvents: unknown[] = []
    client.on('deleted', (e) => deletedEvents.push(e))

    await client.initialize()

    // Wait for SSE to be processed
    await new Promise((r) => setTimeout(r, 50))

    // The deleted flag should be gone
    expect(client.getFlag('delete-me')).toBeUndefined()
    // The other flag should still be there
    expect(client.getFlag('keep-me')).toEqual({ value: false, variant: 'off', reason: 'default' })
    // Deleted event should have been emitted
    expect(deletedEvents).toHaveLength(1)
    expect(deletedEvents[0]).toEqual({ flagKey: 'delete-me' })

    client.destroy()
  })
```

**Step 4: Run SDK tests**

Run: `cd sdks/javascript && npm test`
Expected: PASS

**Step 5: Commit**

```bash
git add sdks/javascript/src/types.ts sdks/javascript/src/client.ts sdks/javascript/src/index.ts sdks/javascript/src/__tests__/client.test.ts
git commit -m "feat(sdk): handle flag_deleted SSE events"
```

---

### Task 5: Frontend — Archive/Unarchive/Delete UI on FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Add archive mutation, delete mutation, and danger zone UI**

In `web/src/pages/FlagDetailPage.tsx`, add imports at the top:

```typescript
import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type {
  Flag,
  Environment,
  FlagEnvironmentConfig,
  Variant,
  TargetingRule,
} from '../api/types.ts'
import VariantEditor from '../components/VariantEditor.tsx'
import RuleBuilder from '../components/RuleBuilder.tsx'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
```

Add `useNavigate` to the component and add the mutations:

```typescript
export default function FlagDetailPage() {
  const { key, flag: flagKey } = useParams<{ key: string; flag: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [selectedEnvKey, setSelectedEnvKey] = useState<string>('')
  const [archiveDialogOpen, setArchiveDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)

  // ... existing queries ...

  const archiveMutation = useMutation({
    mutationFn: (archived: boolean) =>
      api.put<Flag>(`/projects/${key}/flags/${flagKey}/archive`, { archived }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      setArchiveDialogOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/projects/${key}/flags/${flagKey}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      navigate(`/projects/${key}`)
    },
  })
```

Add the danger zone section after the metadata card (after the closing `</div>` of the metadata card at line ~249, before the environment tabs):

```tsx
{/* Danger Zone */}
<div className="p-6 rounded-lg border border-destructive/30 mb-6">
  <div className="font-mono text-[10px] font-medium text-destructive uppercase tracking-wider mb-3">
    Danger Zone
  </div>
  {flag.archived ? (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-[13px] font-medium text-foreground">Unarchive this flag</div>
          <div className="text-xs text-muted-foreground">Restore this flag so it can be evaluated again.</div>
        </div>
        <Button
          variant="outline"
          onClick={() => archiveMutation.mutate(false)}
          disabled={archiveMutation.isPending}
        >
          {archiveMutation.isPending ? 'Unarchiving...' : 'Unarchive'}
        </Button>
      </div>
      <div className="border-t border-destructive/20" />
      <div className="flex items-center justify-between">
        <div>
          <div className="text-[13px] font-medium text-foreground">Delete this flag permanently</div>
          <div className="text-xs text-muted-foreground">This action cannot be undone.</div>
        </div>
        <Button variant="destructive" onClick={() => setDeleteDialogOpen(true)}>
          Delete Flag
        </Button>
      </div>
    </div>
  ) : (
    <div className="flex items-center justify-between">
      <div>
        <div className="text-[13px] font-medium text-foreground">Archive this flag</div>
        <div className="text-xs text-muted-foreground">
          Archived flags return default values and are excluded from evaluation.
        </div>
      </div>
      <Button
        variant="outline"
        className="border-destructive/50 text-destructive hover:bg-destructive/10"
        onClick={() => setArchiveDialogOpen(true)}
      >
        Archive Flag
      </Button>
    </div>
  )}
</div>
```

Add dialogs before the closing `</div>` of the component:

```tsx
{/* Archive Confirmation Dialog */}
<Dialog open={archiveDialogOpen} onOpenChange={setArchiveDialogOpen}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Archive {flag.name}?</DialogTitle>
      <DialogDescription>
        Archived flags return default values and are excluded from targeting evaluation.
        You can unarchive it later.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onClick={() => setArchiveDialogOpen(false)}>
        Cancel
      </Button>
      <Button
        variant="destructive"
        onClick={() => archiveMutation.mutate(true)}
        disabled={archiveMutation.isPending}
      >
        {archiveMutation.isPending ? 'Archiving...' : 'Archive'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

{/* Delete Confirmation Dialog */}
<Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Permanently delete {flag.name}?</DialogTitle>
      <DialogDescription>
        This will permanently remove the flag and all its environment configurations.
        This action cannot be undone.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
        Cancel
      </Button>
      <Button
        variant="destructive"
        onClick={() => deleteMutation.mutate()}
        disabled={deleteMutation.isPending}
      >
        {deleteMutation.isPending ? 'Deleting...' : 'Delete Permanently'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

Also add an "Archived" badge in the metadata card title, after `{flag.name}`:

```tsx
<div className="text-xl font-semibold text-foreground mb-1 tracking-tight">
  {flag.name}
  {flag.archived && (
    <Badge variant="secondary" className="ml-2 text-xs align-middle">Archived</Badge>
  )}
</div>
```

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): add archive/unarchive/delete UI on flag detail page"
```

---

### Task 6: Frontend — Show archived badge in flag list on ProjectDetailPage

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Add archived badge to flag list rows**

In `web/src/pages/ProjectDetailPage.tsx`, in the `TableBody` section, update the flag name cell (around line 168):

```tsx
<TableCell className="text-[13px] text-foreground">
  <span className={flag.archived ? 'opacity-50' : ''}>
    {flag.name}
  </span>
  {flag.archived && (
    <Badge variant="secondary" className="ml-2 text-[10px]">Archived</Badge>
  )}
</TableCell>
```

Also dim the key cell for archived flags:

```tsx
<TableCell>
  <span className={`font-mono text-xs text-[#d4956a] tracking-wide ${flag.archived ? 'opacity-50' : ''}`}>{flag.key}</span>
</TableCell>
```

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat(web): show archived badge in flag list"
```

---

### Task 7: Final verification

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Run SDK tests**

Run: `cd sdks/javascript && npm test`
Expected: PASS

**Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 4: Build the full binary to verify go:embed**

Run: `cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: Build succeeds
