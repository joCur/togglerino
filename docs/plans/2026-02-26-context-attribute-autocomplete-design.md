# Context Attribute Autocomplete Design

## Problem

When creating targeting rules, users must manually type context attribute names (e.g., `country`, `plan`, `user_id`) into a free-text input. This leads to typos, inconsistency, and discoverability issues — users don't know which attributes their SDKs are actually sending.

## Solution

Track context attributes seen in SDK evaluation requests and surface them as autocomplete suggestions in the rule builder's attribute field.

## Decisions

- **Data source**: SDK evaluation requests (tracks real usage, not just what's configured in rules)
- **Scope**: Per project (shared across all environments within a project)
- **Performance**: Async best-effort tracking (goroutine, same pattern as audit log)
- **Storage**: Dedicated `context_attributes` table with `last_seen_at` timestamp
- **Frontend**: Combobox (dropdown + freetext) using shadcn/ui Popover + Command pattern

## Database

New migration `003_context_attributes`:

```sql
CREATE TABLE context_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, name)
);

CREATE INDEX idx_context_attributes_project ON context_attributes(project_id);
```

Upsert pattern: `INSERT ... ON CONFLICT (project_id, name) DO UPDATE SET last_seen_at = NOW()`

## Backend

### Store (`internal/store/context_attribute_store.go`)

- `UpsertAttributes(ctx, projectID string, names []string) error` — bulk upsert with updated `last_seen_at`
- `ListByProject(ctx, projectID string) ([]ContextAttribute, error)` — alphabetically ordered

### Tracking (`internal/handler/evaluate_handler.go`)

After parsing evaluation context, if `len(ctx.Attributes) > 0`:
- Extract attribute keys
- Fire goroutine: `go s.contextAttributeStore.UpsertAttributes(bgCtx, projectID, attrNames)`
- Log errors, never fail the evaluation request

The handler already resolves the project from the SDK key, so no extra DB lookup needed.

### API Endpoint

```
GET /api/v1/projects/{key}/context-attributes
```

Session-authed (management API). Returns:

```json
[
  { "name": "country", "last_seen_at": "2026-02-26T10:00:00Z" },
  { "name": "plan", "last_seen_at": "2026-02-25T08:30:00Z" }
]
```

Sorted alphabetically by name.

## Frontend

### RuleBuilder attribute field

Replace the `<Input>` for condition attributes in `web/src/components/RuleBuilder.tsx` with a combobox:

- Fetch attributes from `GET /api/v1/projects/{key}/context-attributes` via TanStack Query
- shadcn/ui Combobox pattern (Popover + Command component)
- Filter suggestions as user types
- Allow custom values not in the list
- Placeholder: "e.g. user_id, email, plan"

### API client

Add `getContextAttributes(projectKey)` to `web/src/api/client.ts`.
