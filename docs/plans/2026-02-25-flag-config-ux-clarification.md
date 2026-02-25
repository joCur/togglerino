# Flag Configuration UX Clarification — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add contextual inline descriptions to every section of the flag configuration screen, expand the operator dropdown from 9 to 14 operators with grouped categories, and add contextual input placeholders.

**Architecture:** Pure frontend changes across 4 files. Each component gets a `descriptionStyle` const for consistent muted helper text. The `RuleBuilder` gets the biggest change: restructured `OPERATOR_GROUPS` array with `<optgroup>` rendering, plus conditional logic for value placeholder/visibility based on selected operator. `FlagDetailPage` gets a helper function `getDefaultVariantDescription(flagType)` for type-specific text.

**Tech Stack:** React 19, TypeScript, inline styles (project convention — no CSS modules), `t` theme tokens from `web/src/theme.ts`

**Design doc:** `docs/plans/2026-02-25-flag-config-ux-clarification-design.md`

---

### Task 1: Add inline description to RolloutSlider

**Files:**
- Modify: `web/src/components/RolloutSlider.tsx`

This is the simplest component — one description line when the rollout is enabled.

**Step 1: Add the description text below the slider**

In `RolloutSlider.tsx`, add a description `<div>` inside the `{enabled && (...)}` block, after the range slider row. The full `{enabled && (...)}` block becomes:

```tsx
{enabled && (
  <>
    <div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginBottom: 8 }}>
      Gradually roll out this variant to a percentage of users. Uses consistent hashing — the same user always gets the same result.
    </div>
    <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
      {/* existing slider + percentage display unchanged */}
    </div>
  </>
)}
```

**Step 2: Verify lint passes**

Run: `cd web && npx eslint src/components/RolloutSlider.tsx`
Expected: No errors

**Step 3: Verify build passes**

Run: `cd web && npx tsc -b --noEmit`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/components/RolloutSlider.tsx
git commit -m "feat(web): add inline description to rollout slider"
```

---

### Task 2: Add type-specific descriptions and improved empty state to VariantEditor

**Files:**
- Modify: `web/src/components/VariantEditor.tsx`

**Step 1: Add the description text below the "Variants" label**

After the existing label `<div>` at line 72, add a type-specific description. Use a helper function or inline ternary:

```tsx
<div style={{ fontSize: 13, fontWeight: 500, color: t.textPrimary }}>Variants</div>
<div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginBottom: 4 }}>
  {flagType === 'boolean'
    ? 'Define the on/off states for this flag. Each variant has a key (referenced in targeting rules) and a boolean value.'
    : flagType === 'string'
    ? 'Define the possible string values this flag can return. Each variant has a key (referenced in targeting rules) and a string value.'
    : flagType === 'number'
    ? 'Define the possible numeric values this flag can return. Each variant has a key (referenced in targeting rules) and a number value.'
    : 'Define the possible JSON payloads this flag can return. Each variant has a key (referenced in targeting rules) and a JSON value.'}
</div>
```

**Step 2: Update the empty state text**

Replace the current empty state block (lines 74-91) with improved text:

- Boolean: `"No variants defined."` + the existing `Add boolean defaults` link (keep as-is)
- Non-boolean: `"No variants defined. Add variants to use in targeting rules and as the default value."`

The boolean case keeps the existing "Add boolean defaults" clickable link. For non-boolean, replace the generic text.

```tsx
{variants.length === 0 && (
  <div style={{ fontSize: 12, color: t.textMuted, fontStyle: 'italic' }}>
    {flagType === 'boolean' ? (
      <>
        No variants defined.{' '}
        <span
          style={{ color: t.accent, cursor: 'pointer', fontStyle: 'normal', transition: 'color 200ms ease' }}
          onClick={addDefaults}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.accentLight }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.accent }}
        >
          Add boolean defaults
        </span>
      </>
    ) : (
      'No variants defined. Add variants to use in targeting rules and as the default value.'
    )}
  </div>
)}
```

**Step 3: Verify lint passes**

Run: `cd web && npx eslint src/components/VariantEditor.tsx`
Expected: No errors

**Step 4: Verify build passes**

Run: `cd web && npx tsc -b --noEmit`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/components/VariantEditor.tsx
git commit -m "feat(web): add type-specific descriptions to variant editor"
```

---

### Task 3: Expand operators and add descriptions/placeholders to RuleBuilder

**Files:**
- Modify: `web/src/components/RuleBuilder.tsx`

This is the largest task — 3 changes: grouped operators, section descriptions, contextual placeholders.

**Step 1: Replace the OPERATORS array with grouped OPERATOR_GROUPS**

Replace lines 11-21 (the current `OPERATORS` array) with:

```tsx
const OPERATOR_GROUPS = [
  {
    label: 'Comparison',
    operators: [
      { value: 'equals', label: 'equals' },
      { value: 'not_equals', label: 'not equals' },
    ],
  },
  {
    label: 'String',
    operators: [
      { value: 'contains', label: 'contains' },
      { value: 'not_contains', label: 'not contains' },
      { value: 'starts_with', label: 'starts with' },
      { value: 'ends_with', label: 'ends with' },
    ],
  },
  {
    label: 'List',
    operators: [
      { value: 'in', label: 'in (comma-separated)' },
      { value: 'not_in', label: 'not in (comma-separated)' },
    ],
  },
  {
    label: 'Numeric',
    operators: [
      { value: 'greater_than', label: '> greater than' },
      { value: 'less_than', label: '< less than' },
      { value: 'gte', label: '>= greater or equal' },
      { value: 'lte', label: '<= less or equal' },
    ],
  },
  {
    label: 'Presence',
    operators: [
      { value: 'exists', label: 'exists' },
      { value: 'not_exists', label: 'not exists' },
    ],
  },
  {
    label: 'Pattern',
    operators: [
      { value: 'matches', label: 'matches (regex)' },
    ],
  },
]
```

**Step 2: Update the operator `<select>` to use `<optgroup>`**

Replace the operator select rendering (lines 150-158) with:

```tsx
<select
  style={{ ...selectStyle, width: 170 }}
  value={cond.operator}
  onChange={(e) => updateCondition(ruleIdx, condIdx, { operator: e.target.value })}
>
  {OPERATOR_GROUPS.map((group) => (
    <optgroup key={group.label} label={group.label}>
      {group.operators.map((op) => (
        <option key={op.value} value={op.value}>{op.label}</option>
      ))}
    </optgroup>
  ))}
</select>
```

Note: Widen from `width: 130` to `width: 170` to accommodate longer labels like ">= greater or equal".

**Step 3: Add contextual value placeholder and hide for exists/not_exists**

Replace the value `<input>` (lines 159-166) with conditional rendering:

```tsx
{cond.operator !== 'exists' && cond.operator !== 'not_exists' && (
  <input
    style={inputStyle}
    placeholder={
      cond.operator === 'in' || cond.operator === 'not_in'
        ? 'comma-separated values'
        : 'Value'
    }
    value={String(cond.value ?? '')}
    onChange={(e) => updateCondition(ruleIdx, condIdx, { value: e.target.value })}
    onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
    onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
  />
)}
```

**Step 4: Update attribute placeholder**

Change the attribute input placeholder (line 144) from `"Attribute"` to `"e.g. user_id, email, plan"`.

**Step 5: Add section description below "Targeting Rules" label**

After line 98 (the "Targeting Rules" label), add:

```tsx
<div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginBottom: 4 }}>
  Rules are evaluated top to bottom — the first matching rule wins. If no rule matches, the default variant is served.
</div>
```

**Step 6: Add conditions description and "Serve variant" description**

After the "Conditions" label (line 137-139), add:

```tsx
<div style={{ fontSize: 11, color: t.textMuted, lineHeight: 1.5, marginBottom: 6, fontStyle: 'italic' }}>
  All conditions must match (AND logic). Attributes are properties from your SDK's evaluation context.
</div>
```

Replace the "Serve variant:" label text (line 199) to include a description:

```tsx
<div style={{ display: 'flex', flexDirection: 'column', gap: 4, marginBottom: 12 }}>
  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
    <span style={{ fontSize: 12, color: t.textSecondary, whiteSpace: 'nowrap' }}>Serve variant:</span>
    {/* existing dropdown/input unchanged */}
  </div>
  <div style={{ fontSize: 11, color: t.textMuted, fontStyle: 'italic' }}>
    The variant returned when this rule matches.
  </div>
</div>
```

**Step 7: Verify lint passes**

Run: `cd web && npx eslint src/components/RuleBuilder.tsx`
Expected: No errors

**Step 8: Verify build passes**

Run: `cd web && npx tsc -b --noEmit`
Expected: No errors

**Step 9: Commit**

```bash
git add web/src/components/RuleBuilder.tsx
git commit -m "feat(web): expand operators to 14 with optgroups, add rule descriptions and contextual placeholders"
```

---

### Task 4: Add toggle and default variant descriptions to FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Add description below the enabled/disabled toggle**

After line 116 (the `{enabled ? 'Enabled' : 'Disabled'} in {envKey}` span), before the closing `</div>` of the toggle container, add a description that changes based on `enabled` state:

```tsx
<span style={{ fontSize: 14, fontWeight: 500, color: t.textPrimary }}>
  {enabled ? 'Enabled' : 'Disabled'} in {envKey}
</span>
<div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5 }}>
  {enabled
    ? 'Targeting rules and variants are active. Users are evaluated against rules below.'
    : 'All SDK evaluations return the default variant. Targeting rules are ignored.'}
</div>
```

Note: The toggle container's layout needs to change from `alignItems: 'center'` to allow wrapping. Change the toggle container to use `flexWrap: 'wrap'` or restructure with a wrapper. The cleanest approach: wrap the text span + description in a `<div>` so they stack vertically next to the toggle switch.

Updated toggle container structure:

```tsx
<div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, marginBottom: 24, padding: 16, borderRadius: t.radiusMd, backgroundColor: t.bgElevated, border: `1px solid ${t.border}` }}>
  {/* toggle switch div unchanged */}
  <div>
    <span style={{ fontSize: 14, fontWeight: 500, color: t.textPrimary }}>
      {enabled ? 'Enabled' : 'Disabled'} in {envKey}
    </span>
    <div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginTop: 4 }}>
      {enabled
        ? 'Targeting rules and variants are active. Users are evaluated against rules below.'
        : 'All SDK evaluations return the default variant. Targeting rules are ignored.'}
    </div>
  </div>
</div>
```

**Step 2: Add type-specific description below "Default Variant" label**

After line 122-123 (the "Default Variant" label), add:

```tsx
<div style={{ fontSize: 13, fontWeight: 500, color: t.textPrimary, marginBottom: 4 }}>
  Default Variant
</div>
<div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginBottom: 10 }}>
  {flag.flag_type === 'boolean'
    ? "The value returned when no targeting rule matches. For boolean flags, this is typically 'on' or 'off'."
    : flag.flag_type === 'string'
    ? 'The string value returned when no targeting rule matches.'
    : flag.flag_type === 'number'
    ? 'The numeric value returned when no targeting rule matches.'
    : 'The JSON payload returned when no targeting rule matches.'}
</div>
```

Note: Change the existing label's `marginBottom: 10` to `marginBottom: 4` so the description sits closer to the label, then the description has `marginBottom: 10` before the input.

**Step 3: Verify lint passes**

Run: `cd web && npx eslint src/pages/FlagDetailPage.tsx`
Expected: No errors

**Step 4: Verify build passes**

Run: `cd web && npx tsc -b --noEmit`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): add toggle and default variant inline descriptions"
```

---

### Task 5: Full build verification and visual check

**Step 1: Run full frontend lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 2: Run full frontend build**

Run: `cd web && npm run build`
Expected: Build succeeds, output in `web/dist/`

**Step 3: Visual verification with dev server**

Run: `cd web && npm run dev`
Then navigate to a flag detail page and verify:

- [ ] Toggle shows contextual description (different for enabled vs disabled)
- [ ] Default Variant shows type-specific description
- [ ] Variants section shows type-specific description
- [ ] Variants empty state is improved (boolean shows "Add boolean defaults", others show fuller text)
- [ ] Targeting Rules section shows evaluation order description
- [ ] Conditions sub-label shows AND logic explanation
- [ ] Operator dropdown shows all 14 operators in 6 groups with `<optgroup>` labels
- [ ] Attribute placeholder shows "e.g. user_id, email, plan"
- [ ] Value placeholder shows "comma-separated values" for in/not_in operators
- [ ] Value input is hidden for exists/not_exists operators
- [ ] Serve variant shows description text
- [ ] Rollout slider shows description when enabled
- [ ] All description text uses consistent muted styling

**Step 4: Commit any fixes if needed**

If visual check reveals issues, fix and commit.
