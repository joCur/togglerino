# Flag Configuration UX Clarification

**Date:** 2026-02-25
**Status:** Approved

## Problem

The feature toggle configuration screen provides no contextual help. Users don't understand what "variants" are, what they can configure for conditions, or what "disabled in environment" means. This is especially problematic for mixed technical teams (developers + product managers).

## Approach

**Contextual inline descriptions** — short, always-visible helper text below each section label. Descriptions adapt based on flag type (boolean, string, number, json) where relevant. Additionally, expose all 14 backend operators (currently only 9 are shown) with grouped `<optgroup>` categories and contextual input placeholders.

## Design

### 1. Enabled/Disabled Toggle

Add a muted description line below the toggle:

- **Enabled:** "Targeting rules and variants are active. Users are evaluated against rules below."
- **Disabled:** "All SDK evaluations return the default variant. Targeting rules are ignored."

### 2. Default Variant

Add type-specific description below the "Default Variant" label:

| Flag Type | Description |
|-----------|-------------|
| boolean | "The value returned when no targeting rule matches. For boolean flags, this is typically 'on' or 'off'." |
| string | "The string value returned when no targeting rule matches." |
| number | "The numeric value returned when no targeting rule matches." |
| json | "The JSON payload returned when no targeting rule matches." |

### 3. Variants

Add type-specific description below the "Variants" label:

| Flag Type | Description |
|-----------|-------------|
| boolean | "Define the on/off states for this flag. Each variant has a key (referenced in targeting rules) and a boolean value." |
| string | "Define the possible string values this flag can return. Each variant has a key (referenced in targeting rules) and a string value." |
| number | "Define the possible numeric values this flag can return. Each variant has a key (referenced in targeting rules) and a number value." |
| json | "Define the possible JSON payloads this flag can return. Each variant has a key (referenced in targeting rules) and a JSON value." |

Empty state improvements:

- Boolean: "No variants defined. Add boolean defaults (on/off) or create custom variants."
- Others: "No variants defined. Add variants to use in targeting rules and as the default value."

### 4. Targeting Rules & Conditions

**Section description:** "Rules are evaluated top to bottom — the first matching rule wins. If no rule matches, the default variant is served."

**Conditions sub-label:** "All conditions in a rule must match (AND logic). Attributes are properties from the evaluation context passed by your SDK (e.g. user_id, email, plan, country)."

**Operator dropdown — all 14 operators grouped with `<optgroup>`:**

| Group | Operators |
|-------|-----------|
| Comparison | equals, not equals |
| String | contains, not contains, starts with, ends with |
| List | in (comma-separated), not in (comma-separated) |
| Numeric | > greater than, < less than, >= greater or equal, <= less or equal |
| Presence | exists, not exists |
| Pattern | matches (regex) |

**Contextual placeholders:**
- Attribute field: "e.g. user_id, email, plan"
- Value field for `in`/`not_in` operators: "comma-separated values"
- Value field hidden for `exists`/`not_exists` operators

### 5. Percentage Rollout

When checkbox is checked, show: "Gradually roll out this variant to a percentage of users. Uses consistent hashing — the same user always gets the same result."

### 6. Serve Variant

Description: "The variant returned when this rule matches."

## Files to Modify

| File | Changes |
|------|---------|
| `web/src/pages/FlagDetailPage.tsx` | Add inline descriptions for toggle, default variant (pass `flagType`) |
| `web/src/components/VariantEditor.tsx` | Add type-specific description and improved empty state |
| `web/src/components/RuleBuilder.tsx` | Add rule description, grouped operators (9 → 14), contextual placeholders, conditions explanation |
| `web/src/components/RolloutSlider.tsx` | Add rollout description when enabled |

## Styling

All description text uses: `fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginBottom: 8`. Consistent with the "Ink & Ember" theme.

## Scope Exclusions

- No tooltips or collapsible sections
- No validation feedback or preview/dry-run
- No rule naming or commenting
- No autocomplete for attributes
