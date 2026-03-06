# Kill Switch Dashboard Design

## Summary

Dedicated dashboard at `/projects/:key/kill-switches` showing all kill-switch type flags with inline per-environment toggles and confirmation dialogs. Optimized for fast incident response.

## Backend Changes

### Migration: Add `updated_by` to `flag_environment_configs`

- New column `updated_by UUID REFERENCES users(id)` (nullable for existing rows)
- Update flag handler to set `updated_by` from session user on config updates
- Update `FlagEnvironmentConfig` model to include `updated_by` and joined user display name

## Frontend Changes

### New page: `KillSwitchDashboardPage.tsx`

- Route: `/projects/:key/kill-switches`
- Nav link in `ProjectLayout.tsx` sidebar

### Layout

- **Summary bar**: Count of kill switches by state (e.g., "3 active in production, 1 disabled")
- **Table**: One row per kill switch flag
  - Flag name + key
  - One `Switch` toggle per environment column (dev / staging / production)
  - Green = enabled, red/muted = disabled
  - "Last changed" below each toggle: relative time + user display name
- **Confirmation dialog**: On toggle click, shows "Disable {flag-name} in {environment}?" with Confirm/Cancel

### Data Fetching

- `api.flags.list(key, { flag_type: 'kill-switch' })` for flag list
- `api.get('/projects/{key}/environments')` for environments
- Per-flag env configs (same pattern as ProjectDetailPage)

### Toggle Mutation

- Reuse existing `PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}` with `enabled` flipped
- Invalidate queries on success

### Permissions

- Toggle shown/enabled only when user has `flags:write`
- Read-only users see status but cannot toggle

## Out of Scope

- Keyboard shortcuts
- SSE real-time updates on dashboard
- Bulk toggle operations
