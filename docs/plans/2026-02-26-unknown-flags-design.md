# Unknown Flags Feature Design

## Overview

When SDKs try to evaluate flags that don't exist in a project, Togglerino will automatically track those requests. Users can view unknown flags in a dedicated tab on the Flags overview page. This helps spot typos, misconfigurations, or stale references in application code.

## Database Schema

New migration `003_unknown_flags`:

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

- Unique constraint on `(project_id, environment_id, flag_key)` enables upsert pattern
- `dismissed_at` as nullable timestamp for soft dismiss (reversible, timestamped)
- Partial index on non-dismissed rows for the primary query path
- CASCADE on project/environment delete for auto-cleanup
- Same flag key unknown in multiple environments = separate rows

## Backend

### UnknownFlagStore

New store in `internal/store/` with methods:

- **Upsert(ctx, projectID, environmentID, flagKey)** — `INSERT ... ON CONFLICT DO UPDATE SET request_count = request_count + 1, last_seen_at = now(), dismissed_at = NULL`. Clears dismissed_at on resurfacing.
- **ListByProject(ctx, projectID)** — Returns rows where `dismissed_at IS NULL`, joined with environments for env key/name. Ordered by `last_seen_at DESC`.
- **Dismiss(ctx, id)** — Sets `dismissed_at = now()`.
- **DeleteByProjectAndKey(ctx, projectID, flagKey)** — Hard-deletes all rows matching project + flag key (for auto-cleanup on flag creation).

### Tracking Hook

In `EvaluateSingle` handler: when `cache.GetFlag()` returns false, call `unknownFlagStore.Upsert()` best-effort (goroutine, log errors, don't fail the request). Same pattern as audit log. SDK key middleware already provides projectID and environmentID in context.

### Auto-Cleanup

In the create-flag handler: after creating a flag, call `unknownFlagStore.DeleteByProjectAndKey(projectID, flagKey)` to remove unknown entries across all environments.

### Management API Endpoints (session-authed)

- `GET /api/v1/projects/{key}/unknown-flags` — list active unknown flags
- `DELETE /api/v1/projects/{key}/unknown-flags/{id}` — dismiss an unknown flag

## Frontend

### ProjectDetailPage Changes

Tab bar with "Flags" (existing) and "Unknown Flags" (new). Unknown flags tab shows count badge when active (e.g., "Unknown Flags (3)").

### Unknown Flags Tab

Table columns: Flag Key (monospace), Environment (badge), Requests, First Seen (relative), Last Seen (relative), Actions.

Actions per row:
- **Create Flag** — navigates to flag creation form with key pre-filled
- **Dismiss** — calls DELETE endpoint, optimistic update via TanStack Query invalidation

Empty state: "No unknown flags detected. Unknown flags appear here when your SDKs try to evaluate flags that don't exist in this project."

### Data Fetching

New TanStack Query hook `useUnknownFlags(projectKey)` calling the GET endpoint.

## Edge Cases

- **Dismissed flags resurfacing**: Upsert clears `dismissed_at` if SDK keeps requesting a dismissed flag. Intentional — signals the stale reference hasn't been fixed.
- **High-frequency unknown evaluations**: Upsert is a single atomic SQL statement, no read-before-write race. Best-effort goroutine never blocks SDK response.
- **EvaluateAll**: No unknown tracking — SDK doesn't request specific keys, so no "unknown" concept.
- **Flag creation auto-cleanup**: Deletes across all environments since the flag will now be in the cache.

## Testing

- Go: unit tests for store (upsert idempotency, dismiss, auto-cleanup) and handler (upsert called on cache miss, not on hit)
- Frontend: standard patterns, no special test requirements
