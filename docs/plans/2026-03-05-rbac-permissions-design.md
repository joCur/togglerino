# RBAC: Built-in Roles & Project-Scoped Permissions

**Issue**: #36 (partial — custom roles deferred to follow-up)
**Date**: 2026-03-05

## Summary

Expand the two-role system (`admin`/`member`) with granular permissions and project-scoped role assignments. Adds three built-in project roles (`admin`, `editor`, `viewer`), an org-wide base project role setting, and per-project role overrides. Modeled after GitHub's org/repo permission separation.

## Permission Model

### Organization-Level Permissions

Derived from the user's global role (`admin` or `member`). Not project-scoped.

| Permission | `admin` | `member` |
|---|---|---|
| `org:users:manage` | yes | no |
| `org:oidc:manage` | yes | no |
| `org:projects:create` | yes | no |
| `org:projects:delete` | yes | no |

### Project-Level Permissions

Derived from the user's effective project role. Three built-in project roles:

| Permission | `admin` | `editor` | `viewer` |
|---|---|---|---|
| `flags:read` | yes | yes | yes |
| `flags:write` | yes | yes | no |
| `environments:read` | yes | yes | yes |
| `environments:write` | yes | yes | no |
| `sdk_keys:manage` | yes | yes | no |
| `segments:write` | yes | yes | no |
| `templates:manage` | yes | yes | no |
| `project:settings` | yes | no | no |

## Role Resolution

```
effective project role = project-specific override ?? org base project role ?? "none"
```

- Global `admin` users bypass project role checks entirely (full access to all projects).
- `none` means no access — the project is hidden from the user.
- The org-wide base project role determines the default for `member` users without an explicit project assignment.

### Examples

| Global role | Base project role | Project override | Effective |
|---|---|---|---|
| `admin` | any | any | full access (bypass) |
| `member` | `editor` | none set | `editor` |
| `member` | `viewer` | `editor` on Project A | `editor` on A, `viewer` elsewhere |
| `member` | `none` | `viewer` on Project A | `viewer` on A, no access elsewhere |

## Database Changes

### New: `org_settings` table

```sql
CREATE TABLE org_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Seed: INSERT INTO org_settings (key, value) VALUES ('base_project_role', 'editor');
```

### New: `project_members` table

```sql
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
```

### No changes to `users` table

The `role` column remains `admin`/`member`. The CHECK constraint is unchanged.

## Permission Enforcement

Middleware-based, applied per route in `main.go`:

- `RequireOrgPermission(permission)` — checks global role against org-level permission map.
- `RequireProjectPermission(permission)` — extracts `{key}` from URL path, resolves effective project role, checks permission. Returns 403 if denied, 404 if no access (to avoid leaking project existence).

Global `admin` users pass all project permission checks automatically.

### Project Listing Filter

`GET /api/v1/projects` returns only projects the user can access:
- Global `admin`: all projects.
- `member` with base role != `none`: all projects.
- `member` with base role `none`: only projects with explicit `project_members` entry.

## API Changes

### New Endpoints

| Method | Path | Permission | Description |
|---|---|---|---|
| `GET` | `/api/v1/settings/base-project-role` | `org:users:manage` | Get base project role |
| `PUT` | `/api/v1/settings/base-project-role` | `org:users:manage` | Set base project role |
| `GET` | `/api/v1/projects/{key}/members` | `project:settings` | List project members |
| `POST` | `/api/v1/projects/{key}/members` | `project:settings` | Add member with role |
| `PUT` | `/api/v1/projects/{key}/members/{userId}` | `project:settings` | Update member role |
| `DELETE` | `/api/v1/projects/{key}/members/{userId}` | `project:settings` | Remove member |
| `GET` | `/api/v1/management/users/{id}/projects` | `org:users:manage` | User's project assignments |
| `PUT` | `/api/v1/management/users/{id}/projects` | `org:users:manage` | Bulk update user's project assignments |

### Modified Endpoints

- `GET /api/v1/projects` — filtered by accessible projects.
- All project-scoped mutation routes — gated by `RequireProjectPermission`.
- `GET /api/v1/auth/me` — include effective permissions in response (for frontend UI gating).

## UI Changes

### Project Settings → Members Tab (extend existing)

- List members with project-specific role (badge showing "explicit" vs "inherited from base")
- Add member: search users + role selector dropdown
- Change role / remove member
- Project admins only (gated by `project:settings`)

### Team Page → User Detail (extend existing)

- Section showing user's project assignments with roles
- Add/edit project role assignments from user-centric view
- Org admins only

### Org Settings (admin)

- Base project role selector: `admin` / `editor` / `viewer` / `none`
- Explanation text about what the setting controls

### Navigation

- Hide projects the user has no access to in sidebar/project list
- Disable/hide mutation UI elements for `viewer` role (e.g., flag toggle, create button)
- Show read-only indicators where appropriate

## Migration Strategy

- Migration adds `org_settings` and `project_members` tables.
- Seeds `base_project_role = 'editor'` — preserves current behavior where all members can edit.
- No changes to existing user records or the `users.role` column.
- Zero behavior change on upgrade. Admins opt into restrictions by changing the base project role or adding per-project overrides.

## Out of Scope (Follow-Up)

- Custom role creation/editing UI (issue TBD)
- Per-environment permissions (e.g., "can edit flags in staging but not production")
- API key scoping by permissions
