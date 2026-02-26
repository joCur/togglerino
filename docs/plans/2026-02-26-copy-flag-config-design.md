# Copy Flag Config Between Environments

## Problem

Setting up the same flag configuration (variants, targeting rules, rollout percentages) in every environment is repetitive. Users need a way to copy a flag's config from one environment to another.

## Decision

Frontend-only copy. The flag detail page already loads all environment configs in one API call. The copy reads the source config from already-loaded data, populates the target environment's editor state, and the user saves via the existing PUT endpoint.

## Scope

- Single flag config copy on the flag detail page
- Copy variants, targeting rules, and default variant
- Do NOT copy the enabled/disabled state (safety for production)
- Confirmation dialog before overwriting

## Design

### Data Flow

1. `FlagDetailPage` fetches all `environment_configs[]` for the flag (existing behavior)
2. `ConfigEditor` receives the current config + all configs for other environments
3. User picks a source environment from a "Copy from" select dropdown
4. Confirmation dialog: "Copy config from **{env}**? This will replace the current variants, targeting rules, and default variant."
5. On confirm: populate local state (`variants`, `targetingRules`, `defaultVariant`) from source — keep `enabled` unchanged
6. User reviews populated fields and clicks Save (existing PUT flow)

### UI Changes

**`ConfigEditor` component in `FlagDetailPage.tsx`:**
- Add `allConfigs` and `environments` props
- Add a "Copy from" `<Select>` listing other environments (exclude current)
- Add confirmation `<Dialog>` with source env name and description of what changes
- On confirm, set `variants`, `rules`, and `defaultVariant` state from source config

### No Backend Changes

- No new endpoints, store methods, or migrations
- Audit log captures the saved state regardless of how it was populated
- Cache/SSE/evaluation unaffected

## Approach

Chosen over a backend copy endpoint because:
- Data is already in the browser
- User can review and tweak before saving
- Minimal code change, no API surface increase
- Backend endpoint can be added later if API-driven copy is needed
