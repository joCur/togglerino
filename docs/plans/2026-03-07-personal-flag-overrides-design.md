# Personal Flag Overrides Design

Issue: #48

## Problem

Developers need to test features behind disabled flags in shared environments (e.g., staging). Today this requires either enabling the flag for everyone (risky) or adding a targeting rule with their user ID (clutters config). Personal overrides provide a clean, scoped solution.

## Core Concepts

- **App Identity** — a per-project mapping from a dashboard user to their application user ID (the `userId` sent in SDK evaluation context)
- **Override** — a value override for a specific flag + environment + user, with optional expiry
- **Evaluation precedence** — archived > disabled > **personal override** > targeting rules > default variant

Overrides are resolved server-side by matching `context.userId` against stored app identities. No SDK changes required.

## Data Model

### `user_app_identities`

| Column | Type | Description |
|--------|------|-------------|
| `user_id` | UUID FK → users | Dashboard user |
| `project_id` | UUID FK → projects | Project scope |
| `app_user_id` | TEXT | User's identity in their application |
| `created_at` | TIMESTAMP | |
| `updated_at` | TIMESTAMP | |

- Primary key: `(user_id, project_id)`
- Unique constraint: `(project_id, app_user_id)` — prevents two dashboard users claiming the same app identity

### `flag_overrides`

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `user_id` | UUID FK → users | Who created it |
| `flag_id` | UUID FK → flags | Which flag |
| `environment_id` | UUID FK → environments | Which environment |
| `value` | JSONB | The override value |
| `expires_at` | TIMESTAMP | Nullable — null means no expiry |
| `created_at` | TIMESTAMP | |

- Unique constraint: `(user_id, flag_id, environment_id)` — one override per user per flag per environment

No changes to existing tables.

## API Endpoints

All session-authed. Override endpoints return 400 if no app identity configured for the project.

### App Identity

- `PUT /api/v1/projects/{key}/app-identity` — set app user ID for this project
- `GET /api/v1/projects/{key}/app-identity` — get app user ID for this project
- `DELETE /api/v1/projects/{key}/app-identity` — remove app identity

### Overrides

- `PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/override` — set override
- `DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/override` — remove override
- `PUT /api/v1/projects/{key}/flags/{flag}/override` — set override for all environments
- `DELETE /api/v1/projects/{key}/flags/{flag}/override` — remove from all environments
- `GET /api/v1/overrides/me` — list all active overrides across projects

### Override Request Body

```json
{
  "value": <any>,
  "duration": "1h" | "8h" | "24h" | "7d" | null
}
```

Default duration: `24h`. Null means no expiry.

## Evaluation Flow

```
archived? → disabled? → PERSONAL OVERRIDE? → targeting rules → default
```

1. `EvaluateHandler` receives SDK request with `context.userId`
2. Look up `flag_overrides` where `app_user_id` matches `context.userId` for the relevant project + environment
3. If a non-expired override exists, return it with `reason: "override"`
4. Otherwise, continue normal evaluation

### Caching

Overrides stored in the in-memory cache alongside flags and segments. Cache keyed by `projectKey:envKey` mapping `appUserID:flagKey` to override value. Cache invalidated when overrides are created or deleted.

### Expired Override Cleanup

A periodic goroutine (similar to the staleness checker) deletes expired overrides from the database. The evaluation engine also checks `expires_at` at read time so expired overrides are never served between cleanup runs.

## Frontend UX

### Flag Detail Page

- "Override for me" toggle in each environment's config section
- On first click: inline prompt for app identity if not yet configured (saved for the project)
- When active: shows override value, expiry time, remove button
- "Apply to all environments" checkbox
- Duration picker: 1h, 8h, 24h (default), 7d, No expiry

### Override Indicator

- Badge/icon on flag list rows when an active override exists
- Tooltip: "You have a personal override active"

### My Overrides Page

- Route: `/overrides` (accessible from user menu)
- Table: project, flag, environment, override value, expires at, remove action
- Bulk "Remove all" button

### App Identity Setup

- Configured inline on first override attempt (no separate settings page required)
- Also viewable/editable from project context

## Privacy

Only the owning user can see and manage their own overrides. No admin visibility for now.

## Design Decisions

- **Server-side over client-side**: Unlike LaunchDarkly (client-side toolbar/dev-server), overrides are applied during server evaluation. This works across all SDKs (including server-side) without SDK changes.
- **App identity per-project**: Users may have different application user IDs across projects. Per-project mapping handles this.
- **Default 24h expiry**: Prevents forgotten overrides from lingering. Developers can extend or disable expiry when needed.
- **Separate from flag config**: Overrides don't pollute targeting rules and don't generate audit log noise for flag config changes.
- **Unique app identity per project**: Two dashboard users cannot claim the same app user ID within a project, preventing override conflicts.
