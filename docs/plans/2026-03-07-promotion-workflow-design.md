# Environment Promotion Workflow Design

Issue: #47

## Overview

Add a structured "promote config" workflow that copies a flag's environment config from one environment to another (e.g., staging to production) with a confirmation step, diff preview, and audit trail.

## Decisions

| Decision | Choice |
|----------|--------|
| Environment ordering | `sort_order` column on `environments` table |
| What gets copied | `default_variant`, `variants`, `targeting_rules` — `enabled` is preserved on target |
| Diff preview | Client-side (fetch both configs, compare in browser) |
| Permissions | `flags:write` (same as config update) |
| Direction | Forward-only (target `sort_order` > source `sort_order`) |
| Skipping | Allowed (user picks any higher-order environment) |
| Audit | New `"promoted"` action with source env in new_value snapshot |

## Database Changes

Add `sort_order` integer column to `environments`:

```sql
ALTER TABLE environments ADD COLUMN sort_order integer NOT NULL DEFAULT 0;

WITH ranked AS (
  SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY created_at) - 1 AS rn
  FROM environments
)
UPDATE environments SET sort_order = ranked.rn FROM ranked WHERE environments.id = ranked.id;
```

No new tables. Audit log already supports the needed fields — use action `"promoted"` with `entity_type: "flag_config"`.

## API Design

### Promote endpoint

```
POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote
```

Request body:

```json
{
  "target_environment": "production"
}
```

Server-side logic:

1. Resolve project, flag, source env, target env
2. Validate target env `sort_order > source sort_order` (400 if not)
3. Fetch source config (`default_variant`, `variants`, `targeting_rules`)
4. Fetch target config (for audit old_value snapshot)
5. Update target config: copy `default_variant`, `variants`, `targeting_rules` from source; preserve target's `enabled` state
6. Audit log: action `"promoted"`, entity_type `"flag_config"`, entity_id = flag key, environment_id = target env ID, old_value = target's previous config, new_value = target's new config with source env key for traceability
7. Invalidate cache + notify SSE hub for target env
8. Return the updated target config

Permission: `flags:write` on the project.

### Environment reorder endpoint

```
PUT /api/v1/projects/{key}/environments/order
```

Request body:

```json
{
  "environment_ids": ["id1", "id2", "id3"]
}
```

Sets `sort_order` based on array position. Existing `GET /api/v1/projects/{key}/environments` returns `sort_order` in the response.

## Frontend Design

### Promote button

On the flag environment config panel, a "Promote to" button with a dropdown of environments with higher `sort_order`. Disabled if current env is last in order.

### Promote dialog

1. Fetch target env's config (existing API data from flag detail)
2. Show diff of what will change in the target:
   - `default_variant`: old vs. new
   - `variants`: added/removed/changed
   - `targeting_rules`: added/removed/changed
   - `enabled`: shown as "preserved (unchanged)"
3. "Confirm Promotion" button triggers the POST
4. On success: invalidate TanStack Query cache, show success toast

### Environment reorder

In project environment settings, drag-to-reorder (or up/down arrows) for environment order. Calls `PUT .../environments/order` on change.

### Audit log display

The `"promoted"` action renders as "Promoted flag config from {source} to {target}" in the audit log list.

## Out of Scope

- Bulk promote (all flags from one env to another)
- Backward promotion (target sort_order < source sort_order)
- Dedicated `flags:promote` permission
