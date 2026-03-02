# Environment Defaults for New Flags

Issue: #51

## Problem

Every new flag follows the same pattern: enable in dev for testing, keep disabled in staging/production until ready. Currently this requires manually configuring each environment after flag creation.

## Design

### Data Model

Extend the existing `project_settings.settings` JSONB column with an `environment_defaults` key:

```json
{
  "flag_lifetimes": { ... },
  "environment_defaults": {
    "development": { "enabled": true },
    "staging": { "enabled": false },
    "production": { "enabled": false }
  }
}
```

Hardcoded fallback defaults when no project settings exist or an environment key is missing:
- `development` → `enabled: true`
- All other environments → `enabled: false`

### API

New endpoints under the existing project settings pattern:

- `GET /api/v1/projects/{key}/settings/environments` — returns merged defaults (hardcoded fallbacks + custom settings) for all project environments
- `PUT /api/v1/projects/{key}/settings/environments` — update environment defaults

Flag creation endpoint change (`POST /api/v1/projects/{key}/flags`):

- Accept optional `environment_overrides` field: `{ "staging": { "enabled": true } }`
- Server loads project environment defaults, merges with hardcoded fallbacks, applies request overrides
- Uses resolved `enabled` values when creating `flag_environment_configs`

### Backend Flow (Flag Creation)

1. Handler receives flag create request with optional `environment_overrides`
2. Load project environment defaults from `ProjectSettingsStore`
3. Merge: hardcoded fallback → project defaults → request overrides
4. Pass per-environment enabled state to `FlagStore.Create()`
5. Store creates `flag_environment_configs` using resolved values instead of hardcoded `false`

### Frontend

**Project Settings page** — new "Environment Defaults" section:
- Table of environments with an enabled/disabled toggle for each
- Save button calls `PUT /settings/environments`

**Create Flag Modal** — new collapsible "Environment Configuration" section:
- Each environment shown with a toggle pre-filled from project defaults
- User can override before creating
- Collapsed by default with summary text (e.g., "dev: on, staging: off, prod: off")

### Decisions

- **Enabled state only** — no default_variant configuration (would need to vary by value_type, deferred)
- **JSONB storage** — reuses existing project_settings pattern, no migration needed
- **Keyed by environment key** — readable, unique per project, avoids UUID indirection
- **Hardcoded fallback** — development=enabled by default, everything else disabled
