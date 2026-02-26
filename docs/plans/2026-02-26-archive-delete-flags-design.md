# Archive & Delete Feature Flags — Design

**Date**: 2026-02-26
**Status**: Approved

## Problem

Feature flags cannot be deleted from the UI. The backend `DELETE` route exists but has two bugs (no cache invalidation, no SSE notification) and no frontend UI. Users need a safe way to remove flags they no longer need.

## Approach

Two-step removal: archive first (reversible), then permanently delete (irreversible). The `archived` field already exists on the `flags` table and model, and the evaluation engine already handles it (returns default value with reason "archived").

## Design

### 1. Archive/Unarchive (Backend)

- **Store**: `FlagStore.SetArchived(ctx, flagID string, archived bool) (*Flag, error)` — `UPDATE flags SET archived=$2, updated_at=NOW() WHERE id=$1 RETURNING ...`
- **Handler**: `FlagHandler.Archive` — `PUT /api/v1/projects/{key}/flags/{flag}/archive`
  - Request body: `{"archived": true}` or `{"archived": false}`
  - Refreshes evaluation cache for all project environments
  - Broadcasts SSE `flag_update` events per environment (archived flags evaluate to default)
  - Records audit entry: action `"archive"` or `"unarchive"`, entity_type `"flag"`
- **Route**: Registered with `sessionAuth` (any role can archive/unarchive)

### 2. Delete — Guarded by Archive

- Existing `FlagHandler.Delete` gets a **guard**: reject with 409 Conflict if `flag.Archived == false`
- After successful DB delete:
  - Refresh cache for all project environments (evicts the deleted flag)
  - Broadcast SSE `flag_deleted` event per environment

### 3. SSE Event Type

Add `Type` field to `stream.Event`:

```go
type Event struct {
    Type    string `json:"type"`    // "flag_update" or "flag_deleted"
    FlagKey string `json:"flagKey"`
    Value   any    `json:"value"`
    Variant string `json:"variant"`
}
```

`StreamHandler` uses `event.Type` as the SSE event name (defaults to `"flag_update"` for backwards compatibility). Existing SDKs ignore unknown event types — no breaking change.

### 4. Frontend

**FlagDetailPage** — Danger zone section at bottom of metadata card:
- Not archived: "Archive Flag" button (warning style) with confirmation dialog
- Archived: "Archived" badge + "Unarchive" button + "Delete Permanently" button (destructive) with confirmation dialog
- After archive/delete: navigate to project page, invalidate queries

**ProjectDetailPage** — Archived flags shown dimmed with "Archived" badge in the flag table.

### 5. Cache Invalidation Helper

Both archive and delete need to refresh cache for all environments in a project. Extract a helper in the flag handler:

```go
func (h *FlagHandler) refreshAllEnvironments(ctx context.Context, projectKey string, projectID string) { ... }
```

Queries environments for the project, calls `cache.Refresh` per environment. Best-effort (logs warnings on failure).

## Non-Goals

- Filtering archived flags from the list view (can be added later)
- Soft-delete with auto-purge (YAGNI)
- Admin-only restriction on delete (matches current behavior)
