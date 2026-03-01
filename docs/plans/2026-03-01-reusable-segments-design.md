# Reusable Segments for Targeting Rules

**Date:** 2026-03-01
**Issue:** #31

## Summary

Add reusable segments — named, project-scoped groups of targeting conditions that can be shared across multiple flags. Eliminates duplication when the same audience (e.g. "beta users", "enterprise customers") is targeted by many flags.

## Design Decisions

- **Project-scoped:** A segment is defined once per project and referenced by any flag in any environment.
- **Segment as a condition operator:** Uses a new `segment_match` operator in the existing `Condition` type. A condition `{operator: "segment_match", value: "beta-users"}` references a segment by key. No changes to `Condition`, `TargetingRule`, or `FlagEnvironmentConfig` structs.
- **AND-only logic:** Segment conditions use the same AND logic as targeting rules. No OR groups.
- **No nesting:** Segments cannot reference other segments. Enforced at write time — segment conditions must not use `segment_match`.
- **Delete safety:** Cannot delete a segment referenced by active flags. Returns 409 with referencing flag list.

## Data Model

### New `segments` table

```sql
CREATE TABLE segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    conditions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key)
);
```

### New Go model

```go
type Segment struct {
    ID          string      `json:"id"`
    ProjectID   string      `json:"project_id"`
    Key         string      `json:"key"`
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Conditions  []Condition `json:"conditions"`
    CreatedAt   time.Time   `json:"created_at"`
    UpdatedAt   time.Time   `json:"updated_at"`
}
```

### New operator

`OperatorSegmentMatch = "segment_match"` added to the existing operator constants.

## Evaluation Engine

### Cache extension

Add a segment map to `Cache`:

```go
segments map[string]map[string]model.Segment  // projectKey -> segmentKey -> Segment
```

`LoadAll` and `Refresh` load segments alongside flags. Segment data is project-scoped.

### Evaluation logic

In `matchesAllConditions`, when `cond.Operator == "segment_match"`:

1. Look up segment by key from the cache
2. If not found, condition fails (no match)
3. Recursively evaluate the segment's conditions against the same context
4. No recursion risk because segments cannot contain `segment_match` conditions

No changes to consistent hashing, rollout logic, or evaluate handlers.

## API Routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects/{key}/segments` | List segments |
| `POST` | `/api/v1/projects/{key}/segments` | Create segment |
| `GET` | `/api/v1/projects/{key}/segments/{segmentKey}` | Get segment |
| `PUT` | `/api/v1/projects/{key}/segments/{segmentKey}` | Update segment |
| `DELETE` | `/api/v1/projects/{key}/segments/{segmentKey}` | Delete segment (409 if referenced) |
| `GET` | `/api/v1/projects/{key}/segments/{segmentKey}/usage` | List referencing flags |

### Validation

- Segment key: lowercase alphanumeric + hyphens, 3-64 chars
- Conditions must not contain `segment_match` operator
- At least one condition required

### Audit logging

Segment create/update/delete events recorded in `audit_log` with old/new JSON snapshots.

### Cache invalidation

After segment mutations, refresh segment cache for that project. SSE hub broadcasts a refresh event for all environments in the project.

## Frontend

### New route

`/projects/:key/segments` — segment list and management, added to project navigation sidebar.

### Segment list page

Table: segment key, name, condition count, last updated. "Create Segment" button opens dialog with key, name, description fields + condition builder.

### Segment editor

Reuses existing condition row UI from `RuleBuilder` (attribute combobox, operator select, value input). Filters out `segment_match` from operator dropdown to prevent nesting.

### Segment usage view

"Used by" section on segment editor showing flags that reference the segment.

### Rule builder extension

- "Segment" group added to operator dropdown in `RuleBuilder.tsx`
- When `segment_match` selected: attribute/value inputs replaced by segment picker combobox
- Selected segment displayed as labeled badge with segment name

### No changes needed

`ConfigEditor`, `FlagDetailPage`, save mutation — `targeting_rules` JSON already accepts any condition shape.
