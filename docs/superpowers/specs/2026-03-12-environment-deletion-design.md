# Allow Deletion of Default Environments

**Issue**: #123
**Date**: 2026-03-12
**Status**: Approved

## Summary

Allow users to delete any environment, including the auto-created defaults (`development`, `staging`, `production`), with a guard preventing deletion of the last environment. Database cascades handle cleanup of dependent records. Frontend shows a confirmation dialog when the environment has active SDK keys.

## Scope

- New `DELETE /api/v1/projects/{key}/environments/{envKey}` endpoint
- Frontend confirmation dialog with SDK key warning
- Audit log and webhook event for environment deletion
- Cache invalidation for the deleted environment
- **Out of scope**: Active SSE client disconnection (clients time out naturally), customizable default environments (#126), SDK changes

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Permission level | `project:settings` | Consistent with other destructive project-level operations (webhook management) |
| Last-environment guard | Backend-enforced 409 | A project must always have at least one environment |
| SDK key warning | Frontend-only confirmation dialog | Simpler than backend `?force=true` flow; frontend already has SDK key data available |
| Cascade cleanup | Database `ON DELETE CASCADE` | `flag_environment_configs`, `sdk_keys`, `project_environment_access`, and `scheduled_changes` already cascade on environment deletion — no manual cleanup needed |
| Scheduled changes | Silently cascade-deleted | Acceptable — the entire environment is being removed, per-schedule audit trail is unnecessary |
| Locked flag configs | Allowed to delete | `project:settings` is the highest project-level permission gate; locking protects against accidental flag changes, not environment-level operations |
| SSE handling | Let clients time out | No hub changes needed; SDK clients hit auth failures on reconnect since their SDK key is cascade-deleted — same end result as active disconnect with less complexity |
| Cache invalidation | New `Evict` method on Cache | Must remove entries from both `data` and `overrides` maps for the deleted `projectKey:envKey` |

## Backend Changes

### New endpoint

`DELETE /api/v1/projects/{key}/environments/{envKey}`

**Handler flow:**
1. Extract project key and environment key from URL
2. Check `project:settings` permission
3. Look up environment by project ID + environment key (404 if not found)
4. Count environments for the project — reject with 409 if this is the last one (count + delete in a single transaction to prevent race conditions)
5. Delete the environment (database cascades handle dependent records)
6. Evict evaluation cache entries for `projectKey:envKey` (both `data` and `overrides` maps)
7. Record audit log entry (`action: "delete"`, `entity_type: "environment"`, old state snapshot)
8. Dispatch `environment.deleted` webhook event
9. Return 204 No Content

**Error responses:**
- 404 — environment not found
- 409 — cannot delete last environment (`{"error": "cannot delete the last environment"}`)
- 403 — insufficient permissions

### Store changes

- Add `CountByProject(ctx, projectID) (int, error)` method to `EnvironmentStore` to support the last-environment check
- The existing `Delete(id)` method is already sufficient
- The count check + delete should run in a single transaction to prevent concurrent deletions leaving a project with zero environments

### Handler dependency changes

`EnvironmentHandler` currently has `environments`, `projects`, and `webhooks` fields. Add:
- `audit *store.AuditStore` — for recording deletion events
- `cache *evaluation.Cache` — for evicting entries after deletion

Update the constructor in `main.go` accordingly.

### Cache changes

Add an `Evict(projectKey, envKey string)` method to `evaluation.Cache` that deletes the key from both the `data` and `overrides` maps. The existing `Refresh` method is insufficient — it re-queries and would set an empty map rather than removing the key.

### Webhook event

Add `EventEnvironmentDeleted = "environment.deleted"` to `webhook/event.go`.

### Audit log

Record with:
- `action`: `"delete"` (matches existing convention — `"create"`, `"update"`, `"delete"`)
- `entity_type`: `"environment"`
- `entity_id`: environment UUID
- `old_value`: JSON snapshot of the deleted environment
- `new_value`: `nil`

## Frontend Changes

### Environment list page

Add a delete button (trash icon or dropdown menu action) per environment row.

**On click:**
1. Fetch SDK key count for the environment (or use already-cached data if available)
2. If SDK keys exist: show confirmation dialog — "This environment has N active SDK keys. Deleting it will revoke all keys and remove all flag configurations for this environment. This cannot be undone."
3. If no SDK keys: show simpler confirmation — "Delete environment X? All flag configurations for this environment will be removed. This cannot be undone."
4. If this is the last environment: the backend returns 409; show an error toast "Cannot delete the last environment"

**On confirm:**
- Call `DELETE /api/v1/projects/{key}/environments/{envKey}`
- Invalidate TanStack Query caches for environments and flags (flag data includes per-env configs)
- Show success toast
- If the deleted environment was selected/active in the UI, navigate to the first remaining environment

### Delete button visibility

Only show the delete button for users with `project:settings` permission (consistent with how other settings-level actions are gated in the UI).

## Route Registration

Add to the project-scoped routes in `main.go`:

```
DELETE /api/v1/projects/{key}/environments/{envKey}
```

Protected by `RequireProjectPermission("project:settings")`.

## Documentation

Update `docs-site/docs/dashboard/` with environment deletion instructions and `docs-site/docs/api-reference/` with the new endpoint.
