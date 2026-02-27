# Flag Types, Lifecycle & Staleness Tracking

## Overview

Add Unleash-inspired flag types (purpose categories), unified lifecycle status tracking, automatic staleness detection, and a per-project kanban lifecycle board. Per-project configurable lifetimes for each flag type.

## Flag Types (Purpose)

Each flag gets a purpose type that determines its expected lifetime and use case:

| Type | Default Lifetime | Description |
|------|-----------------|-------------|
| `release` | 40 days | Manage deployment of new/incomplete features |
| `experiment` | 40 days | A/B testing and multivariate experiments |
| `operational` | 7 days | Technical migration switches |
| `kill-switch` | permanent (null) | Graceful degradation controls |
| `permission` | permanent (null) | Role/entitlement-based access |

Default: `release`. Changeable after creation (triggers staleness re-evaluation).

## Unified Lifecycle Status

Replace the existing `archived` boolean with a single `lifecycle_status` field:

```
active → potentially_stale → stale → archived
```

- **active** — within expected lifetime, normal operation
- **potentially_stale** — exceeded expected lifetime (set by background job)
- **stale** — auto-promoted from potentially_stale after 14-day grace period, or manually marked by user
- **archived** — explicitly archived by user (evaluation returns default value)

Existing archive/unarchive behavior preserved — endpoints set `lifecycle_status = 'archived'` / `lifecycle_status = 'active'`. Delete guard checks `lifecycle_status = 'archived'`.

## Schema Changes

### Migration: rename flag_type → value_type

```sql
ALTER TABLE flags RENAME COLUMN flag_type TO value_type;
```

### Migration: add flag_type (purpose) and lifecycle_status

```sql
-- Add flag type (purpose)
ALTER TABLE flags ADD COLUMN flag_type TEXT NOT NULL DEFAULT 'release'
    CHECK (flag_type IN ('release', 'experiment', 'operational', 'kill-switch', 'permission'));

-- Replace archived boolean with lifecycle_status
ALTER TABLE flags ADD COLUMN lifecycle_status TEXT NOT NULL DEFAULT 'active'
    CHECK (lifecycle_status IN ('active', 'potentially_stale', 'stale', 'archived'));

-- Migrate archived flags
UPDATE flags SET lifecycle_status = 'archived' WHERE archived = TRUE;

-- Drop archived column
ALTER TABLE flags DROP COLUMN archived;

-- Track when lifecycle status last changed
ALTER TABLE flags ADD COLUMN lifecycle_status_changed_at TIMESTAMPTZ;
```

### Migration: project_settings table

```sql
CREATE TABLE project_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL UNIQUE REFERENCES projects(id) ON DELETE CASCADE,
    settings JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Settings JSONB structure:

```json
{
  "flag_lifetimes": {
    "release": 40,
    "experiment": 40,
    "operational": 7,
    "kill-switch": null,
    "permission": null
  }
}
```

`null` means permanent (never stale). Missing key = use hardcoded default.

## Background Staleness Checker

New `internal/staleness/` package.

```go
type Checker struct {
    flagStore       FlagStore
    settingsStore   ProjectSettingsStore
    auditStore      AuditStore
    interval        time.Duration  // default: 1 hour
}
```

### Tick logic

1. Load all non-archived flags with project settings
2. For each flag:
   - Look up expected lifetime from flag_type + project settings (fall back to global defaults)
   - If lifetime is null (permanent) → skip
   - If `created_at + lifetime < now()` and `lifecycle_status = 'active'` → set `potentially_stale`, record audit event
   - If `lifecycle_status = 'potentially_stale'` and `lifecycle_status_changed_at + 14 days < now()` → set `stale`, record audit event
3. Manually-set `stale` flags (user action) are never reverted

### Lifecycle

- Started in `main.go` alongside HTTP server
- Graceful shutdown via context cancellation
- Runs immediately on startup, then every interval

### Audit events

Action: `staleness_change`, entity_type: `flag`. Old/new values contain lifecycle_status.

## API Changes

### Breaking: rename flag_type → value_type

All request/response bodies rename `flag_type` to `value_type` for the data type field.

### Flag creation

**POST /api/v1/projects/{key}/flags** — new fields:

- `flag_type` (string, optional, default `"release"`) — purpose type

### Flag response

All flag responses gain:

- `flag_type` — purpose type (release/experiment/operational/kill-switch/permission)
- `value_type` — renamed from old flag_type (boolean/string/number/json)
- `lifecycle_status` — active/potentially_stale/stale/archived
- `lifecycle_status_changed_at` — timestamp or null

### Flag update

**PUT /api/v1/projects/{key}/flags/{flag}** — `flag_type` is updatable.

### Manual staleness override

**PUT /api/v1/projects/{key}/flags/{flag}/staleness**

```json
{ "status": "stale" }
```

One-way operation. Returns updated flag. Records audit event.

### Flag list query params

- `?staleness=active,potentially_stale,stale,archived` — filter by lifecycle status
- `?flag_type=release,experiment` — filter by purpose type

### Project settings

**GET /api/v1/projects/{key}/settings** — returns project settings including flag_lifetimes

**PUT /api/v1/projects/{key}/settings** — update project settings

```json
{
  "flag_lifetimes": {
    "release": 40,
    "experiment": 40,
    "operational": 7,
    "kill-switch": null,
    "permission": null
  }
}
```

## Frontend Changes

### New route: Lifecycle Board

`/projects/:key/lifecycle` — kanban-style board with 4 columns:

| **Active** | **Potentially Stale** | **Stale** | **Archived** |
|---|---|---|---|
| lifecycle_status = active | lifecycle_status = potentially_stale | lifecycle_status = stale | lifecycle_status = archived |

Each flag card shows:
- Flag name + key (monospace)
- Flag type badge (purpose) with distinct colors
- Value type indicator
- Tags
- Age since creation
- For potentially_stale: days since marked

Card actions (buttons, no drag-and-drop):
- Click → navigate to flag detail
- Potentially stale cards: "Mark as Stale" button
- Stale cards: "Archive" button

### CreateFlagModal changes

- New "Flag Type" (purpose) dropdown at top with descriptions and expected lifetimes
- Default: Release
- Existing type selector renamed to "Value Type"

### FlagDetailPage changes

- Flag type badge (colored by purpose)
- Lifecycle status badge (green/amber/red/gray for active/potentially_stale/stale/archived)
- "Mark as Stale" button when potentially_stale
- Archive/unarchive behavior unchanged (sets lifecycle_status)

### Flag list changes

- Lifecycle status badge per row
- Flag type badge per row
- Filter options: by flag type and lifecycle status

### Project Settings changes

New "Flag Lifetimes" section:
- Input per flag type (days), with defaults pre-filled
- null/empty = permanent (no staleness tracking for that type)

## Evaluation Engine Impact

- Replace `flag.Archived` check with `flag.LifecycleStatus == "archived"`
- No other evaluation changes — staleness is informational only, does not affect flag evaluation

## SDK Impact

- Rename `flag_type` → `value_type` in SDK responses (breaking change for SDK consumers)
- New `flag_type`, `lifecycle_status` fields available but informational only
- No changes to evaluation behavior
