# Scheduled Flag Changes — Design

**Issue**: #32
**Date**: 2026-03-01

## Summary

Add the ability to schedule flag environment config changes to take effect at a future time. Supports timed launches, maintenance windows, and promotional periods.

## Decisions

1. **Full config snapshot** — schedule stores complete `{enabled, default_variant, variants, targeting_rules}`
2. **Multiple schedules** per flag-environment allowed (e.g., enable at 9am, disable at 5pm)
3. **30-second checker interval** — background goroutine polls for due schedules
4. **Execute overwrites** — scheduled change applies regardless of manual changes since scheduling
5. **Auto-cancel on archive/delete** — pending schedules cancelled with reason
6. **No revert-at in V1** — users create two separate schedules
7. **UTC storage** — frontend handles local display conversion
8. **Editable schedules** — pending schedules can have time and payload updated
9. **Schedule from ConfigEditor** — button alongside "Save Configuration"; pending schedules shown on flag detail page
10. **Full stack** — backend + frontend in one pass

## Database

New `scheduled_flag_changes` table:
- `id`, `flag_id` (FK CASCADE), `environment_id` (FK CASCADE), `scheduled_at` (TIMESTAMPTZ)
- `status` (pending/executed/cancelled), `config_snapshot` (JSONB)
- `created_by` (FK SET NULL), `created_at`, `executed_at`, `cancelled_at`, `cancel_reason`
- Partial index on `(scheduled_at) WHERE status = 'pending'`

## Go Packages

- `internal/model/schedule.go` — ScheduledFlagChange, ScheduleStatus, ConfigSnapshotPayload
- `internal/store/schedule_store.go` — CRUD + Execute(tx) + CancelByFlag + ListDue
- `internal/schedule/checker.go` — Background worker (mirrors staleness/checker.go pattern)
- `internal/handler/schedule_handler.go` — List, Create, Update, Cancel

## API Routes (session-authed)

```
GET    /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules
POST   /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules
PUT    /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}
DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}
```

## Execution Flow

1. Checker ticks every 30s, calls `ListDue(now)`
2. For each due schedule: `pool.Begin()` → `Execute(tx, ...)` (applies config + marks executed atomically) → `tx.Commit()`
3. Post-commit (best-effort): cache refresh, SSE broadcast, audit log
4. Optimistic concurrency via `AND status = 'pending'` prevents double-execution

## Frontend

- `ScheduleChangeDialog.tsx` — datetime-local picker modal from ConfigEditor
- `PendingSchedules.tsx` — per-environment list with cancel buttons on FlagDetailPage

## Files

**Create**: migration (up/down), model, store, scheduler, handler, 2 React components
**Modify**: main.go, flag_handler.go, types.ts, ConfigEditor.tsx, FlagDetailPage.tsx, flag_store.go, environment_store.go
