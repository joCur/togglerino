# Flag Templates Design

**Issue:** #50 — Flag templates for common patterns

## Summary

Pre-configured flag templates that pre-fill flag type, value type, default value, variants, targeting rules, and environment defaults when creating a new flag. Templates exist at two scopes: global (shared across projects) and project-specific.

## Data Model

New `flag_templates` table:

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | PK |
| `project_id` | UUID, nullable | NULL = global, set = project-scoped. FK to `projects` ON DELETE CASCADE |
| `key` | text | Unique within scope |
| `name` | text | Display name |
| `description` | text | Template description |
| `flag_type` | text | Pre-filled flag purpose (release, experiment, etc.) |
| `value_type` | text | Pre-filled value type (boolean, string, number, json) |
| `default_value` | jsonb | Pre-filled default value |
| `tags` | text[] | Pre-filled tags |
| `environment_defaults` | jsonb | `{"production": {"enabled": false}, "development": {"enabled": true}}` |
| `variant_config` | jsonb | `{"variants": [...], "default_variant": "...", "targeting_rules": [...]}` |
| `is_system` | bool | True for auto-seeded built-ins (cannot be deleted) |
| `sort_order` | int | Display ordering |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

**Unique constraint:** `(COALESCE(project_id, '00000000-0000-0000-0000-000000000000'), key)` — enforces key uniqueness within global or per-project scope.

### Variant Config Shape

```json
{
  "variants": [
    {"key": "on", "value": true},
    {"key": "off", "value": false}
  ],
  "default_variant": "off",
  "targeting_rules": [
    {"conditions": [], "variant": "on", "percentage_rollout": 0}
  ]
}
```

## Built-in Templates (seeded at startup)

| Template | Flag Type | Value Type | Default | Variants | Rules | Env Defaults |
|----------|-----------|------------|---------|----------|-------|--------------|
| Gradual Rollout | release | boolean | false | on/off | 0% rollout on "on" | dev: enabled, prod: disabled |
| Kill Switch | kill-switch | boolean | true | on | — | all: enabled |
| A/B Test | experiment | string | "control" | control/treatment | 50% rollout | dev: enabled, prod: disabled |
| Permission Gate | permission | boolean | false | on/off | — | all: disabled |

System templates are seeded on startup (upsert by key where `project_id IS NULL`). They can be updated by admins but not deleted.

## API Endpoints

### Global Templates (admin-only for writes)

- `GET /api/v1/templates` — list all global templates
- `POST /api/v1/templates` — create global template (admin)
- `PUT /api/v1/templates/{key}` — update global template (admin, system templates updatable)
- `DELETE /api/v1/templates/{key}` — delete global template (admin, 403 for system templates)

### Project Templates

- `GET /api/v1/projects/{key}/templates` — list project templates + global templates (separate sections)
- `POST /api/v1/projects/{key}/templates` — create project template
- `PUT /api/v1/projects/{key}/templates/{templateKey}` — update project template
- `DELETE /api/v1/projects/{key}/templates/{templateKey}` — delete project template

### Extended Flag Creation

Extend `POST /api/v1/projects/{key}/flags` to accept richer `environment_overrides`:

```json
{
  "key": "my-flag",
  "name": "My Flag",
  "value_type": "boolean",
  "flag_type": "release",
  "default_value": false,
  "environment_overrides": {
    "production": {
      "enabled": false,
      "variants": [{"key": "on", "value": true}, {"key": "off", "value": false}],
      "default_variant": "off",
      "targeting_rules": [{"conditions": [], "variant": "on", "percentage_rollout": 0}]
    }
  }
}
```

The `EnvironmentDefault` model is extended to optionally include `variants`, `default_variant`, and `targeting_rules`. Existing callers that only send `{"enabled": bool}` continue to work unchanged.

## Frontend

### Template Selection in CreateFlagModal

- First view when opening the create flag dialog: template picker
- Two sections: "Global Templates" and "Project Templates"
- Plus "Blank" option (current behavior, no pre-fill)
- Each template as a card with name, description, flag type badge
- Selecting a template pre-fills all form fields; user can modify before submitting

### Template Management Pages

- `/settings/templates` — admin-only, CRUD for global templates
- `/projects/:key/settings/templates` — CRUD for project-specific templates
- Template editor form: name, key, description, flag type, value type, default value, tags, environment defaults, variant config

## Testing Strategy

TDD approach:
1. Model/types first (template struct, validation)
2. Store layer with DB tests (CRUD, seeding, uniqueness)
3. Handler layer with HTTP tests (auth, CRUD endpoints)
4. Extended flag creation (variants in environment overrides)
5. Frontend components (template picker, management pages)
