# Bulk Flag Operations Design

**Issue:** #49
**Date:** 2026-03-03

## Summary

Enable selecting and operating on multiple flags at once from the flag list page. Supports enable/disable, archive, add/remove tags, and set owner. Critical for incident response and coordinated releases.

## API

### Endpoint

`POST /api/v1/projects/{key}/flags/bulk` (session-authed)

### Request

```json
{
  "action": "enable | disable | archive | add_tags | remove_tags | set_owner",
  "flag_keys": ["flag-a", "flag-b"],
  "environment_key": "production",    // required for enable/disable
  "tags": ["tag-a"],                  // required for add_tags/remove_tags
  "owner_id": "uuid"                  // required for set_owner, null to unassign
}
```

### Response

Partial results model — each flag processed independently, failures don't block others.

```json
{
  "batch_id": "uuid",
  "results": [
    {"flag_key": "flag-a", "success": true},
    {"flag_key": "flag-c", "success": false, "error": "flag is archived"}
  ]
}
```

### Validation Rules

| Action | Required Fields | Constraints |
|--------|----------------|-------------|
| enable/disable | environment_key | Skips archived flags (error) |
| archive | — | Only active/stale flags |
| add_tags | tags | Non-empty tags array |
| remove_tags | tags | Non-empty tags array |
| set_owner | owner_id (nullable) | Valid user ID or null |

## Database

### Migration: `012_bulk_operations`

Add `batch_id` column to `audit_log`:

```sql
ALTER TABLE audit_log ADD COLUMN batch_id UUID;
CREATE INDEX idx_audit_log_batch_id ON audit_log (batch_id) WHERE batch_id IS NOT NULL;
```

No other schema changes — tags and owner_id already exist on flags.

## Backend Flow

1. Handler validates request (action + required fields)
2. Generates batch_id UUID
3. Iterates flag_keys, performing each operation:
   - **enable/disable**: Fetch env config → update `enabled` → audit log
   - **archive**: `SetLifecycleStatus("archived")` → audit log
   - **add_tags/remove_tags**: Fetch flag → modify tags array → update flag → audit log
   - **set_owner**: Update `owner_id` on flag → audit log
4. Each audit entry shares the batch_id
5. Cache refresh + SSE broadcast deduplicated per affected environment
6. Return aggregated results

### Cache & SSE

Collect unique `(projectKey, envKey)` pairs affected during iteration. After all operations complete, refresh cache and broadcast once per unique pair.

For tag/owner changes (metadata only, no evaluation impact): skip cache refresh and SSE broadcast.

## Frontend

### Flag List Changes

- Checkbox on each FlagCard (left side)
- "Select all" checkbox in filter bar
- Selection state managed in page component, reset on filter changes

### Bulk Action Bar

Fixed bar at bottom of screen, visible when 1+ flags selected:
- Selected count display
- Action dropdown: Enable, Disable, Archive, Add Tags, Remove Tags, Set Owner
- Environment dropdown (shown only for Enable/Disable)
- Execute button

### Confirmation Dialog

- Summary message: "Enable 5 flags in production?"
- List of affected flag keys
- Confirm / Cancel
- After execution: show results with success/failure per flag
- Auto-refetch flag list on close

## Audit Log

Each flag operation creates an individual audit entry with:
- `action`: matches the bulk action (e.g., "enable", "archive")
- `entity_type`: "flag" or "flag_config" (for enable/disable)
- `batch_id`: shared UUID linking all entries in the batch
- `old_value` / `new_value`: JSON snapshots as usual

## Decisions

- **Single polymorphic endpoint** over separate endpoints per action
- **Partial results** over all-or-nothing transactions
- **batch_id as DB column** over embedded in JSON
- **Environment selector in action bar** dropdown, not in confirmation dialog
- **Full scope**: all 5 actions in first pass
