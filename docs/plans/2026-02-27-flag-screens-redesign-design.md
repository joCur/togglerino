# Flag List & Detail Screens Redesign

## Problem

Both the flag list and flag detail screens have UX issues:
- **Flag list**: Too many columns, visual noise, environment enable states shown as tiny dots that are hard to read, no summary/overview, clunky filtering
- **Flag detail**: Hard to understand the config evaluation flow, excessive vertical scrolling, environment switching via tabs loses unsaved changes silently, danger zone (archive/delete) is distracting when you just want to edit config

## Design Direction

Inspired by Unleash's card-based approach. Focus on scanability, clear information hierarchy, and making the enable/disable state per environment the most prominent visual element.

## Flag List Page

### Current
Dense table with 6 columns (key, name, type, purpose, tags, environment dots). All information at the same visual weight.

### New: Card-based layout

Each flag renders as a card with clear visual hierarchy:

```
┌─────────────────────────────────────────────────────────┐
│ enable-dark-mode                           boolean      │
│ Enable Dark Mode                                        │
│                                                         │
│  development  [■ ON ]   staging  [□ OFF]                │
│  production   [□ OFF]                                   │
│                                             release     │
└─────────────────────────────────────────────────────────┘
```

**Visual hierarchy (top to bottom):**
1. Flag key (amber monospace, primary identifier) + type badge (top-right, subtle)
2. Flag name (regular text, secondary)
3. Environment enable states as labeled pills: green "ON" / muted "OFF" with environment name — this is the core information
4. Purpose label (bottom-right, subtle)

**What's removed from the list view:**
- Tags — available via filter, visible on detail page
- Lifecycle status — only shown when non-active (amber "potentially stale" / red "stale" badge inline with name)

**Filters:** Single compact toolbar row: search input + purpose dropdown + status dropdown + tag dropdown. Same filter logic, just less visual space.

**Clicking a card** navigates to the flag detail page. Environment pills are read-only on the list.

## Flag Detail Page

### Current
Metadata card → danger zone → environment tabs → vertical form (enable toggle, default variant, variants editor, rule builder, save button). Long vertical scroll, danger zone prominent.

### New: Compact header + environment cards + collapsible config

```
┌─────────────────────────────────────────────────────────┐
│ ← Back to flags                                         │
│                                                         │
│ enable-dark-mode                   [⚙ Flag Settings ▾]  │
│ Enable Dark Mode                                        │
│ boolean · release · active                              │
│ Allow users to switch to dark mode theme                │
│                                                         │
│ ─────────── Environment Configuration ────────────────  │
│                                                         │
│ ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│ │development │  │  staging   │  │ production │         │
│ │   [■ ON]   │  │   [□ OFF]  │  │   [□ OFF]  │         │
│ └────────────┘  └────────────┘  └────────────┘         │
│      ▲ selected                                         │
│                                                         │
│ ┌── Evaluation Flow ──────────────────────────────────┐ │
│ │ Request → Enabled? → Targeting Rules → Default      │ │
│ │           (1 rule)                     variant: off  │ │
│ └─────────────────────────────────────────────────────┘ │
│                                                         │
│ ┌── Configuration: development ───────────────────────┐ │
│ │ Default Variant: [off ▾]                            │ │
│ │ ▸ Variants (2)                                      │ │
│ │ ▸ Targeting Rules (1)                               │ │
│ │ Copy from: [staging ▾] [Apply]                      │ │
│ │                             [Save Configuration]    │ │
│ └─────────────────────────────────────────────────────┘ │
```

### Key changes:

**1. Compact header**
Flag key (large, amber monospace), name below, metadata as inline text (type · purpose · status), description as muted text. One compact block instead of a full card.

**2. Environment selector cards**
Replace tabs with small clickable cards. Each shows environment name + enable toggle. The toggle is clickable directly — toggling it here saves immediately (only the enable state, not the full config). Selected environment is visually highlighted.

**3. Visual evaluation flow (always visible, compact)**
A small horizontal pipeline showing: Request → Enabled? → Targeting Rules (count) → Default Variant. Steps are dimmed when not applicable (e.g., no rules = that node is muted). Shows actual default variant value. ~60-80px tall.

**4. Collapsible config sections**
- Default Variant: always visible at top
- Variants: collapsible, auto-expanded if variants exist
- Targeting Rules: collapsible, auto-expanded if rules exist
- Copy from environment: subtle action at bottom

**5. Danger zone → dropdown menu**
Archive, delete, and staleness actions moved to a "Flag Settings" gear dropdown in the header. No more red panel on the main page.

## Components Changed

| Component | Change |
|-----------|--------|
| `ProjectDetailPage.tsx` | Replace table with card grid, simplify filters |
| `FlagDetailPage.tsx` | New header, environment cards, collapsible sections, evaluation flow |
| `ConfigEditor` (inline) | Extract to own file, add collapsible sections |
| New: `FlagCard.tsx` | Card component for flag list |
| New: `EnvironmentSelector.tsx` | Environment cards with inline toggles |
| New: `EvaluationFlow.tsx` | Visual evaluation pipeline component |
| `VariantEditor.tsx` | No functional change, just wrapped in collapsible |
| `RuleBuilder.tsx` | No functional change, just wrapped in collapsible |

## Non-goals

- No changes to the API or data model
- No changes to the create flag modal
- No changes to the lifecycle board page
- No changes to the unknown flags tab (just moves with the new layout)
