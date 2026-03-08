# Custom Role Creation and Management

**Issue**: #83
**Date**: 2026-03-08
**Builds on**: #36 (built-in roles and project-scoped permissions)
**Future integration**: #99 (environment-scoped permissions layer on top)

## Summary

Replace hardcoded project role definitions with a database-driven model. The three built-in roles (`admin`, `editor`, `viewer`) become immutable rows in a new `roles` table. Admins can create custom roles with arbitrary combinations of the 8 project-level permissions. Custom roles are org-wide and usable anywhere built-in roles are used today (project member assignments, base project role).

## Database Schema

### New: `roles` table

```sql
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    permissions TEXT[] NOT NULL,
    is_built_in BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Seeded with three built-in roles matching current hardcoded permissions:

| Name | Permissions | `is_built_in` |
|------|------------|---------------|
| `admin` | all 8 | `true` |
| `editor` | all except `project:settings` | `true` |
| `viewer` | `flags:read`, `environments:read` | `true` |

### Modified: `project_members` table

- Drop `CHECK (role IN ('admin', 'editor', 'viewer'))` constraint
- Add `FOREIGN KEY (role) REFERENCES roles(name) ON UPDATE CASCADE`

### Unchanged: `org_settings`

Base project role validation moves from hardcoded switch to DB lookup (app-level, already the case).

## API

### New Endpoints (admin-only, `org:users:manage`)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/roles` | List all roles |
| `POST` | `/api/v1/roles` | Create custom role |
| `GET` | `/api/v1/roles/{name}` | Get single role |
| `PUT` | `/api/v1/roles/{name}` | Update custom role (403 for built-in) |
| `DELETE` | `/api/v1/roles/{name}` | Delete custom role (403 for built-in, 409 if in use) |

### Request Shape

```json
{
  "name": "qa-engineer",
  "description": "Can read flags and environments, manage SDK keys",
  "permissions": ["flags:read", "environments:read", "sdk_keys:manage"]
}
```

### Validation

- `name`: required, lowercase alphanumeric + hyphens, 2-50 chars, unique
- `permissions`: non-empty subset of the 8 known project permissions
- Built-in roles: reject PUT (403) and DELETE (403)
- Delete in-use role: reject with 409

### Modified Endpoints

- Project member and base project role endpoints accept any valid `roles.name`
- `GET /api/v1/auth/me` uses dynamic permission lookup from roles table

## Backend Changes

### New

- `internal/store/role_store.go` — CRUD + `IsInUse()` check
- `internal/handler/role_handler.go` — HTTP handlers for 5 endpoints

### Modified

- `internal/model/permission.go` — Remove hardcoded `projectRolePermissions` map and `ValidProjectRole()`. Keep permission constants. Add `ValidPermission()`.
- `internal/auth/resolver.go` — `BuildRoleResolver` takes `RoleStore`, returns `ResolvedRole` (name + permissions) instead of just role name
- `internal/auth/middleware.go` — `RequireProjectPermission` checks permission against `ResolvedRole.Permissions` slice
- `internal/handler/project_member_handler.go` — Validate role against DB
- `internal/store/org_settings_store.go` — Validate base_project_role against DB
- `cmd/togglerino/main.go` — Wire `RoleStore`, update dependency injection

### Cache

Roles loaded into memory at startup, refreshed on mutation. Avoids DB hit per permission check.

## Frontend Changes

### New

- `/settings/roles` page — table of all roles, create/edit dialog with permission matrix (8 checkboxes), delete with 409 handling
- Route added to admin settings navigation

### Modified

- `MembersTab.tsx`, `TeamPage.tsx` — fetch roles from `GET /api/v1/roles` instead of hardcoded arrays
- `usePermissions.ts` — `ProjectRole` type from union to `string`, permission checks unchanged (server-resolved)

## Migration Strategy

Single migration `017_custom_roles`:
1. Create `roles` table with built-in seeds
2. Drop CHECK constraint on `project_members.role`
3. Add FK to `roles(name)` with `ON UPDATE CASCADE`

Zero breaking changes — existing data references built-in role names which are seeded.

## Future: #99 Integration

Environment-scoped permissions will layer on top as a separate concept (e.g., `role_environment_restrictions` table that narrows `flags:write` to specific environments). The `TEXT[]` permissions column and role resolution pipeline support this without structural changes to the roles table.
