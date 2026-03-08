# Environment-Scoped Permissions for Flag Config Updates

Issue: #99

## Problem

Project roles (`admin`, `editor`, `viewer`) grant uniform write access across all environments. There is no way to allow a user to configure flags in `development` and `staging` but prevent them from modifying `production` flag configurations.

## Design

### Data Model

New table `project_environment_access` stores per-project, per-role environment allow-lists:

```sql
CREATE TABLE project_environment_access (
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role_name      TEXT NOT NULL REFERENCES roles(name) ON UPDATE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, role_name, environment_id)
);
```

**Semantics:**
- No rows for a project+role = unrestricted access to all environments (backwards-compatible)
- Rows exist = role can only write flag environment configs for the listed environments

### Permission Enforcement

A new `CheckEnvironmentAccess` check is added after `RequireProjectPermission` confirms `flags:write` on the `UpdateEnvironmentConfig` handler:

1. Resolved project role is stored in request context by `RequireProjectPermission`
2. Look up `project_environment_access` rows for the project + resolved role
3. No rows → allow (unrestricted)
4. Rows exist but target environment not listed → 403 Forbidden
5. Org admins bypass entirely (existing behavior in `RequireProjectPermission`)

Project-level admins are subject to restrictions if explicitly configured; by default they have no rows and are unrestricted.

### API

**Endpoint:** `GET /PUT /api/v1/projects/{key}/environment-access`

**Permission:** `project:settings`

**GET response:**
```json
{
  "restrictions": [
    {
      "role_name": "editor",
      "environment_ids": ["uuid-1", "uuid-2"]
    }
  ],
  "environments": [
    {"id": "uuid-1", "key": "development", "name": "Development"},
    {"id": "uuid-2", "key": "staging", "name": "Staging"},
    {"id": "uuid-3", "key": "production", "name": "Production"}
  ]
}
```

**PUT request:**
```json
{
  "restrictions": [
    {
      "role_name": "editor",
      "environment_ids": ["uuid-1", "uuid-2"]
    }
  ]
}
```

Replaces all restrictions for the project atomically. Omitting a role removes its restrictions.

**Validation:**
- Role names must exist in `roles` table
- Environment IDs must belong to the project

### Frontend

Environment-role matrix grid in Project Settings:
- Rows = environments, columns = roles
- Each cell is a toggle
- All enabled by default (unrestricted); toggling off creates restrictions
- Save applies the full matrix via PUT

### Audit Log

Changes to environment access restrictions are recorded with old/new state snapshots.

### Migration

New migration `023_environment_access` creates the table. Existing projects are unaffected (no rows = full access).

## Decisions

- **Allow-list model** with NULL/no-rows meaning unrestricted (backwards-compatible)
- **Per-project role-to-environment mapping** (not per-user, not on the role definition itself)
- **Org admins bypass** environment restrictions entirely
- **Project admins subject to restrictions** if explicitly configured
- **Dedicated API endpoint** for managing the full restriction matrix
- **Layered enforcement** — environment access is an additional check on top of `flags:write`, not a replacement
