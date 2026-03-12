# Environment Comparison — Side-by-Side Flag Config Diff

**Issue**: #46
**Date**: 2026-03-11
**Status**: Approved

## Summary

Add a "Compare" tab to the flag detail page that shows a single flag's environment configurations side-by-side in a columnar grid. Enables teams to spot configuration drift across environments (dev/staging/prod) at a glance.

## Scope

- Single flag, all environments compared simultaneously
- Frontend-only feature — no backend changes, no new API endpoints
- Read-only view (no inline editing)
- New tab on the existing flag detail page
- **Out of scope**: Scheduled changes are not shown in the comparison view (v1)

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Comparison scope | Single flag, all environments | Fills the gap — flag detail page shows envs stacked vertically today |
| Location | New "Compare" tab on flag detail page | Keeps context close, no navigation friction |
| Diff highlighting | Inline value badges | Pinpoints which specific cell is the outlier |
| Targeting rules display | Summary count + expandable detail panel | Keeps grid scannable, full detail on demand |
| Differences filter | "Show differences only" toggle | Full view by default, filter to drift quickly |
| Actions | Read-only | Simple; users switch to Configuration tab to edit |
| Architecture | Pure frontend diffing (Approach 1) | All data already available client-side via existing Flag API |
| Tab state | URL search param (`?tab=compare`) | Makes the Compare view linkable and shareable |

## Component Structure

The existing flag detail page has tabs: `Configuration` and `History`. We add `Compare` as a third tab.

```
FlagDetailPage
├── Tab bar: [Configuration] [Compare] [History]
├── (Configuration tab) → existing accordion view (unchanged)
├── (Compare tab) → <CompareTab />
│     ├── "Show differences only" toggle
│     ├── ComparisonGrid
│     │     ├── Header row: environment names (sorted by sort_order)
│     │     ├── Enabled row
│     │     ├── Default variant row
│     │     ├── Variants row (count + expandable)
│     │     └── Rules row (count + expandable detail panel)
│     └── Diff utility functions
└── (History tab) → existing history view (unchanged)
```

### New Files

- `web/src/components/CompareTab.tsx` — tab content component
- `web/src/lib/flag-diff.ts` — pure diff utility functions
- `web/src/lib/__tests__/flag-diff.test.ts` — unit tests for diff logic

### Modified Files

- `web/src/pages/FlagDetailPage.tsx` — add "Compare" tab trigger and content, sync tab state to URL search param

### Props

`CompareTab` receives:
- `environments: Environment[]` — project environments (already fetched by the page)
- `environmentConfigs: FlagEnvironmentConfig[]` — flag's per-env configs (from the flag query response)

Both are already available in `FlagDetailPage` — no new API calls needed.

### Single-Environment Projects

When `environments.length < 2`, hide the Compare tab entirely. Comparison is meaningless with a single environment.

## Diff Logic (`flag-diff.ts`)

Pure functions that compare `FlagEnvironmentConfig[]` and return per-field diff metadata.

### Types

```typescript
type DiffStatus = "match" | "differs"

type FieldDiff = {
  status: DiffStatus
  values: Map<string, any> // environmentId → display value
}

type VariantDiff = {
  status: DiffStatus
  perVariant: Map<string, FieldDiff> // variantKey → per-env values
}

type ComparisonResult = {
  enabled: FieldDiff
  defaultVariant: FieldDiff
  variants: VariantDiff       // overall status + per-variant detail for expansion
  rules: FieldDiff            // overall status based on full rule equality
}
```

### Missing Config Handling

A `FlagEnvironmentConfig` may not exist for a given environment. When config is missing (the existing page already handles this with a `?? null` fallback), treat it as:
- `enabled`: `false`
- `defaultVariant`: `""` (empty)
- `variants`: `[]`
- `targetingRules`: `[]`

Display missing configs with a muted "Not configured" label in the grid cell.

### Comparison Strategy

| Field | Method | Display |
|-------|--------|---------|
| Enabled | Direct boolean compare | `ON` (green) / `OFF` (red) |
| Default variant | String equality | Variant key string |
| Variants | Deep equality per variant key — compare sorted `[key, JSON.stringify(value)]` pairs | "N variants" count |
| Targeting rules | Deep equality — canonicalize each rule (sort condition arrays by attribute, serialize with sorted keys) then compare | "N rules" count |

**Canonical serialization**: To avoid false positives from property ordering, all objects are serialized with `JSON.stringify(obj, Object.keys(obj).sort())` before comparison. This ensures `{key: "a", value: true}` and `{value: true, key: "a"}` compare as equal.

### Differences Filter

When "show differences only" is active, hide rows where `status === "match"`. If all rows match, display: "All environments have identical configuration."

## UI Specification

### Grid Layout

- CSS Grid: auto-width label column + one `1fr` column per environment
- Environments ordered by `sort_order` (development → staging → production)
- Horizontal scroll on narrow screens

### Rows

| Row | Per-cell content | Diff badge |
|-----|-----------------|------------|
| Enabled | `ON` / `OFF` with color | Minority value gets colored badge |
| Default variant | Variant key string | Outlier values get amber badge |
| Variants | "N variants" count | Different counts get amber badge |
| Rules | "N rules" count | Different counts get amber badge |
| Lock status | Locked / Unlocked | Informational only, no diff |
| Last updated | Relative time + user name | Informational only, no diff |

### Expandable Sections

Variants and Rules rows are clickable to toggle a detail panel below:

- **Variants detail**: Each variant rendered as key → value per environment column. Per-variant rows that differ across environments get amber badges on the outlier values.
- **Rules detail**: Each rule rendered as a card showing conditions, target variant, and rollout percentage. Rules use amber left-border accent. Environments with no rules show "No targeting rules" in muted text.

### Styling

- Uses existing shadcn/ui components: Badge, Collapsible, Switch
- Tailwind CSS classes following existing patterns
- Amber accent (`#d4956a`) for diff badges, consistent with design system
- Dark theme only (matches existing app)

### Empty States

- All identical: "All environments have identical configuration" (when filter is active and nothing differs)
- No rules: "No targeting rules" in expanded detail for environments with zero rules
- No variants beyond default: show the single variant inline
- Missing config: "Not configured" in muted text

## Testing Strategy

- **Unit tests** for `flag-diff.ts`: test each comparison function with matching configs, differing configs, edge cases (empty rules, single environment, missing configs, property order variations)
- **Component tests** for `CompareTab.tsx`: render with mock data, verify grid structure, test toggle filter, test expandable sections
- TDD approach: write diff utility tests first, then component tests, then implement
