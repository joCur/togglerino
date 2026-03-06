# Flag Lifecycle Dashboard Design

Issue: #37

## Summary

Replace the existing per-project lifecycle kanban board (`/projects/:key/lifecycle`) with a comprehensive lifecycle dashboard providing actionable visibility into flag health, staleness, and technical debt.

## Overview Section

Four stat cards showing flag counts by lifecycle status:
- Active (emerald)
- Potentially Stale (amber)
- Stale (red)
- Archived (muted)

Health score displayed as percentage with color-coded badge:
- Formula: `(active_flags / total_non_archived_flags) * 100`
- Green: >80%, Amber: 50-80%, Red: <50%

## Staleness Trends Chart

- Area chart (recharts) showing flag counts by lifecycle status over time
- Data from `lifecycle_snapshots` table, populated daily by a background job
- Default view: last 30 days
- Dark-theme compatible, uses project accent colors

## Action Queue

Prioritized list of flags needing attention, two sections:

1. **Stale** — flags recommended for archival, sorted by age (oldest first)
2. **Potentially Stale** — flags to evaluate, sorted by age (oldest first)

Each item shows: flag name, key, type badge, age, time in current status.

Inline actions:
- "Archive" button for stale flags
- "Mark as Stale" button for potentially stale flags

## Bulk Actions

- Checkbox selection on action queue items
- "Archive selected" bulk action (reuses existing `POST /projects/{key}/flags/bulk` endpoint)

## Filters

- **Lifecycle status** — toggle which statuses to show
- **Flag type** — filter by flag type (release, experiment, operational, kill-switch, permission)
- Default sort: oldest first

## Backend Changes

### New table: `lifecycle_snapshots`

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

### New background job: Snapshot recorder

- Runs daily (similar pattern to staleness checker)
- Counts flags by lifecycle status per project
- Inserts one row per project per day (upsert to handle restarts)
- Runs alongside the existing staleness checker in main.go

### New API endpoints

1. `GET /api/v1/projects/{key}/lifecycle/summary`
   - Returns current flag counts by lifecycle status + health score
   - Computed live from flag table (not snapshots)
   - Response: `{ active: N, potentially_stale: N, stale: N, archived: N, health_score: float }`

2. `GET /api/v1/projects/{key}/lifecycle/trends?days=30`
   - Returns snapshot data for the trends chart
   - Response: `[{ date: "2026-03-01", active: N, potentially_stale: N, stale: N, archived: N }, ...]`

### No changes to existing endpoints

- Flag list with filters: `GET /api/v1/projects/{key}/flags?lifecycle_status=...&flag_type=...`
- Bulk operations: `POST /api/v1/projects/{key}/flags/bulk`
- Archive: `PUT /api/v1/projects/{key}/flags/{flag}/archive`
- Mark stale: `PUT /api/v1/projects/{key}/flags/{flag}/staleness`

## Frontend Changes

### Dependencies

- Add `recharts` to web/package.json

### Components

Replace `LifecycleBoardPage.tsx` with new dashboard containing:
- Stat cards (shadcn Card component)
- Health score badge
- Trends area chart (recharts AreaChart)
- Action queue table with checkboxes (shadcn Table, Checkbox, Badge, Button)
- Filter controls (shadcn Select/DropdownMenu)

### Data fetching

- TanStack Query for summary and trends endpoints
- Reuse existing flag list query with lifecycle_status filter for action queue
- Invalidate on archive/mark-stale mutations

## What stays the same

- Staleness checker (background auto-promotion)
- Flag lifecycle statuses and transitions
- Per-project flag lifetime configuration
- All existing API endpoints
- Project navigation structure (Lifecycle link already exists)
