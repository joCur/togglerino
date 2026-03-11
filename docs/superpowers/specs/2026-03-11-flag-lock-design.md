# Flag Lock — Prevent Accidental Changes in an Environment

**Date:** 2026-03-11
**Issue:** #53
**Status:** Draft

## Summary

Allow locking a flag's configuration in a specific environment to prevent accidental changes. A locked flag cannot be toggled, and its config cannot be modified until explicitly unlocked by an admin.

## Motivation

Some flags must not change in production — e.g., "this flag must stay OFF until legal approves" or "do not touch during the holiday freeze." Currently anyone with write access can change any flag at any time. Locking adds a safety layer.

## Design Decisions

- **Flag-level locks only** — no environment-wide locks. Each lock targets a specific flag in a specific environment. For code freezes, use bulk lock to lock multiple flags at once. This is simpler and more flexible than environment-level locks (individual flags can be unlocked independently).
- **Admin-only lock/unlock** — uses the existing `project:settings` permission (which only project admins and org admins have). Avoids the "locker is on vacation" problem of per-user unlock restrictions.
- **Lock means locked** — no admin override bypass. To make changes, explicitly unlock first. This keeps the audit trail clean (unlock → change → re-lock are distinct events).
- **Optional reason** — max 255 characters. Not required, but encouraged for audit trail clarity.

## Data Model

Add four columns to the existing `flag_environment_configs` table:

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `locked` | `BOOLEAN NOT NULL` | `false` | Whether config is locked |
| `locked_by` | `UUID FK → users` | `NULL` | User who locked it |
| `locked_at` | `TIMESTAMPTZ` | `NULL` | When it was locked |
| `lock_reason` | `TEXT` | `NULL` | Optional reason (max 255 chars, validated in handler) |

No new tables. Migration `027_flag_lock.up.sql` / `027_flag_lock.down.sql` adds the columns (026 is already taken by `026_last_evaluated_at`).

## API Endpoints

### Lock a flag in an environment

```
POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/lock
```

**Auth:** Session, requires `project:settings` permission.

**Request body:**
```json
{
  "reason": "Holiday code freeze"
}
```

`reason` is optional. Max 255 characters; returns 400 if exceeded.

**Success response (200):** Returns the full `FlagEnvironmentConfig` JSON (same shape as the config update response), including the new lock fields:
```json
{
  "id": "...",
  "locked": true,
  "locked_by": "user-uuid",
  "locked_by_user": { "id": "...", "email": "admin@acme.com", "display_name": "Admin" },
  "locked_at": "2026-03-11T12:00:00Z",
  "lock_reason": "Holiday code freeze"
}
```

**Error responses:**
- 400: Reason exceeds 255 characters
- 404: Project, flag, or environment not found
- 409: Flag is already locked in this environment

### Unlock a flag in an environment

```
DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/lock
```

**Auth:** Session, requires `project:settings` permission.

**Success response (200):**
```json
{
  "locked": false
}
```

**Error responses:**
- 404: Project, flag, or environment not found
- 409: Flag is not locked in this environment

### Bulk lock flags in an environment

```
POST /api/v1/projects/{key}/flags/bulk-lock
```

**Auth:** Session, requires `project:settings` permission.

**Request body:**
```json
{
  "flag_keys": ["payment-gateway", "new-checkout"],
  "environment_key": "production",
  "reason": "Holiday code freeze"
}
```

**Success response (200):**
```json
{
  "locked": 2,
  "already_locked": 0,
  "errors": []
}
```

Partial success: locks as many as possible, reports errors for individual failures.

**Error responses:**
- 400: Empty flag_keys, missing environment_key, reason exceeds 255 chars
- 404: Project or environment not found

### Bulk unlock flags in an environment

```
POST /api/v1/projects/{key}/flags/bulk-unlock
```

**Auth:** Session, requires `project:settings` permission.

**Request body:**
```json
{
  "flag_keys": ["payment-gateway", "new-checkout"],
  "environment_key": "production"
}
```

**Success response (200):**
```json
{
  "unlocked": 2,
  "already_unlocked": 0,
  "errors": []
}
```

**Error responses:**
- 400: Empty flag_keys, missing environment_key
- 404: Project or environment not found

## Lock Enforcement

The following mutations check lock status and return **409 Conflict** if the flag is locked in the target environment:

| Mutation | Lock check |
|----------|-----------|
| Toggle enabled/disabled | Flag locked in that env? |
| Update config (variants, rules, default variant) | Flag locked in that env? |
| Promote INTO environment | Flag locked in target env? |
| Archive flag | Flag locked in ANY env? |
| Delete flag | Blocked transitively — delete requires archived status, and archive checks locks in all envs |
| Bulk toggle | Flag locked in that env? (per flag) |
| Bulk archive | Flag locked in ANY env? (per flag) |
| Scheduled changes | Checker skips locked configs, marks schedule as `failed` with reason "flag locked", logs warning |

Promote FROM a locked environment is allowed (the source is not being modified).

**Staleness lifecycle transitions** (active → potentially_stale → stale) are NOT blocked by locks. Lifecycle status is a flag-level property independent of per-environment config.

**Evaluation cache**: Lock state is NOT part of the evaluation cache. Locks only affect management API mutations, not SDK flag evaluation. No cache invalidation is needed for lock/unlock operations.

**Environment access controls**: Lock enforcement applies regardless of the user's environment access level. Even a user with full environment write access is blocked by a lock — the lock check runs before any mutation logic.

**Bulk lock scope**: Bulk lock/unlock requires explicit `flag_keys`. There is no "lock all flags in environment" shorthand — the frontend handles this by fetching the flag list first and passing all keys. This avoids accidentally locking newly created flags.

**409 response body** (uses existing `writeError` pattern with details in message):
```json
{
  "error": "flag is locked in this environment by admin@acme.com: Holiday code freeze"
}
```

Lock check happens after authentication/authorization but before any mutation logic, in the handler.

## Audit Logging

Lock and unlock events are recorded using the existing best-effort audit pattern:

| Action | Entity Type | Details |
|--------|------------|---------|
| `lock` | `flag_config` | `new_value` includes `locked_by`, `lock_reason`, `environment_key` |
| `unlock` | `flag_config` | `old_value` includes previous lock info |

Bulk lock records one audit entry per flag locked.

## Webhook Events

New event types dispatched through existing webhook system:

- `flag.config.locked` — fired when a flag is locked
- `flag.config.unlocked` — fired when a flag is unlocked

Payload follows the same structure as `flag.config.updated`.

## Frontend Changes

### Flag Detail Page — Environment Section

**Unlocked state:**
- Lock button (padlock icon + "Lock") in the environment section header, visible to admins only
- All controls function normally

**Locked state:**
- Red "Locked" badge next to environment name
- Lock banner below header: padlock icon, "Locked by {email} — {reason}", relative timestamp
- All config controls dimmed/disabled (toggle, variant editor, rule editor, save button)
- Promote button disabled for this environment as a target
- Unlock button replaces lock button (visible to admins only)

### Lock Dialog

Modal shown when clicking "Lock":
- Displays flag key and environment name for confirmation
- Optional reason text input (255 char limit with counter)
- Cancel and "Lock Flag" (red) buttons

### Flag List

No changes to the flag list view. Lock status is per-environment and only relevant on the detail page.

## Model Changes

Add lock fields to `FlagEnvironmentConfig` struct:

```go
type FlagEnvironmentConfig struct {
    // ... existing fields ...
    Locked       bool       `json:"locked"`
    LockedBy     *string    `json:"locked_by,omitempty"`
    LockedByUser *FlagOwner `json:"locked_by_user,omitempty"`
    LockedAt     *time.Time `json:"locked_at,omitempty"`
    LockReason   *string    `json:"lock_reason,omitempty"`
}
```

## Store Changes

New methods on `FlagStore`:

- `LockEnvironmentConfig(ctx, flagID, envID, userID, reason) (*FlagEnvironmentConfig, error)` — sets locked=true, locked_by, locked_at, lock_reason
- `UnlockEnvironmentConfig(ctx, flagID, envID) (*FlagEnvironmentConfig, error)` — sets locked=false, clears lock fields
- `IsLockedInAnyEnvironment(ctx, flagID) (bool, error)` — for archive check

Existing `GetEnvironmentConfig` and scan helpers updated to read new columns.

## Handler Changes

New handler methods:

- `LockEnvironmentConfig` — validates inputs, calls store, audit logs, dispatches webhook
- `UnlockEnvironmentConfig` — validates inputs, calls store, audit logs, dispatches webhook
- `BulkLockFlags` — iterates flag keys, locks each, collects results
- `BulkUnlockFlags` — iterates flag keys, unlocks each, collects results

Modified handlers (add lock check at top):

- `UpdateEnvironmentConfig` — check `cfg.Locked` before proceeding
- `ArchiveFlag` — call `IsLockedInAnyEnvironment` before proceeding
- `PromoteEnvironmentConfig` — check lock on target environment before proceeding
- `BulkAction` toggle path — check lock before toggling
- `BulkAction` archive path — check `IsLockedInAnyEnvironment` before archiving

Modified background workers:

- `schedule.Checker` — skip execution if target flag is locked in target environment, mark schedule as `failed` with reason, log warning

## Testing Strategy

TDD approach — tests written before implementation:

1. **Store tests**: Lock/unlock operations, lock state persistence, IsLockedInAnyEnvironment
2. **Handler tests**: Lock/unlock endpoints, 409 enforcement on all mutation paths, permission checks, bulk lock, reason validation
3. **Frontend**: Lock/unlock UI interactions, disabled state rendering, dialog behavior
