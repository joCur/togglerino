# Flag Change History with Diff and Rollback

**Issue:** #43
**Date:** 2026-03-03
**Status:** Approved

## Summary

Add a per-flag change history view that shows a timeline of all configuration changes with structured diffs between versions and the ability to rollback to a previous configuration.

## Decisions

- **Architecture:** Query-over-audit-log. No new tables — history is a filtered view of existing `audit_log` data.
- **Old data gap:** Forward-only. Fix recording going forward; pre-existing entries show snapshots but no diffs.
- **Environment tracking:** Add `environment_id` column to `audit_log` table.
- **Page layout:** Add tabs to flag detail page — "Configuration" and "History".
- **Diff style:** Structured field-by-field diff (not raw JSON).

## Database Changes

Single migration (`010_flag_history`):

1. Add nullable `environment_id UUID REFERENCES environments(id) ON DELETE SET NULL` to `audit_log`
2. Add nullable `user_email TEXT` to `audit_log` (denormalized for display after user deletion)
3. Add composite index: `(project_id, entity_id, entity_type, created_at DESC)`

## Backend Changes

### Fix audit recording

`UpdateEnvironmentConfig` handler must fetch the current config before updating, so both `old_value` and `new_value` are recorded. Also populate `environment_id` and `user_email` on audit entries going forward.

### New store methods

- `AuditStore.ListByFlag(ctx, projectID, flagKey, envID *string, limit, offset)` — per-flag history with optional environment filter
- `AuditStore.GetByID(ctx, id)` — single entry for restore flow

### New API endpoints (session-authed)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{key}/flags/{flag}/history?env=&limit=50&offset=0` | List flag change history |
| `GET` | `/api/v1/projects/{key}/flags/{flag}/history/{id}` | Single history entry |
| `POST` | `/api/v1/projects/{key}/flags/{flag}/history/{id}/restore` | Restore previous config |

### Restore flow

1. Fetch audit entry by ID, verify it belongs to this flag/project
2. Verify entry is `flag_config` type with a restorable snapshot
3. Parse snapshot as `FlagEnvironmentConfig`
4. Call existing `UpdateEnvironmentConfig` store method (same validation, cache, SSE path)
5. Record new audit entry with `action: "restore"`, source entry ID in metadata
6. Return updated config

## Model Changes

Add to `AuditEntry`:
- `EnvironmentID *string` (`json:"environment_id,omitempty"`)
- `UserEmail *string` (`json:"user_email,omitempty"`)

New action: `"restore"` — the restore entry stores `old_value` = state before restore, `new_value` = restored snapshot.

## Frontend Changes

### Flag detail page tabs

Convert `FlagDetailPage` to use shadcn `Tabs`:
- **Configuration** — current page content (metadata, description, owner, environment configs)
- **History** — new change history timeline

Header (breadcrumbs, flag key, settings dropdown) stays above tabs.

### History tab

1. **Environment filter dropdown** — "All environments" or specific environment
2. **Timeline** (most recent first), each entry showing:
   - Timestamp (relative + absolute on hover)
   - User email
   - Action badge (created, updated, restored, archived, etc.)
   - Environment badge (if applicable)
   - Expandable structured diff (collapsed by default)
   - "Restore this version" button (for restorable `flag_config` entries)
3. **Load more** pagination

### Structured diff component

Field-by-field comparison:
- `Enabled: false → true`
- `Default variant: "off" → "on"`
- `Added variant: "beta" = {...}`
- `Removed targeting rule: "country equals US → 'us-users'"`
- `Modified rule #2: percentage 50% → 75%`

For entries without `old_value` (pre-fix): show "Snapshot available" with readable summary, no diff.

### Restore confirmation dialog

"This will apply the configuration from {timestamp} to {environment}. This creates a new change entry." — Cancel / Restore buttons.

## Out of Scope

- Backfilling `old_value` for pre-existing audit entries
- History for non-flag entities (projects, segments)
- Comparing two arbitrary versions side-by-side
- Retention/cleanup policies for audit data
