# Tailwind CSS + shadcn/ui Migration Design

## Problem

The togglerino dashboard uses inline `style={{}}` objects for all styling. This causes:

- Repetitive style objects (buttons, inputs, labels copy-pasted across 22 TSX files)
- Hover/focus states require JS event handlers (`onMouseEnter`/`onMouseLeave`/`onFocus`/`onBlur`)
- No pre-built UI components — modals, tables, dropdowns, sliders all built from scratch
- Hard to maintain consistency across pages

## Decision

Adopt **Tailwind CSS v4** (Vite plugin, CSS-first config) + **shadcn/ui** (copy-paste component library on Radix UI).

### Why This Approach

- Tailwind eliminates inline styles with utility classes, handles hover/focus via CSS
- shadcn/ui provides accessible, pre-built components (Dialog, Table, Tabs, Select, etc.)
- Components are copied into the codebase — full control, no library lock-in
- CSS variable theming integrates cleanly with existing dark theme
- Tailwind v4 purges unused CSS — small production bundle
- Open to visual refresh — shadcn's clean aesthetic fits internal tooling well

### Alternatives Considered

1. **Mantine** — Richer component set but more opinionated, heavier, less customizable
2. **Tailwind only** — Lighter but doesn't solve the "pre-built components" need

## Setup

- Tailwind CSS v4 via `@tailwindcss/vite` plugin
- shadcn/ui CLI to scaffold `components/ui/`, `lib/utils.ts`, `components.json`
- Theme via CSS custom properties in `index.css` (shadcn convention: `--background`, `--foreground`, `--primary`, etc.)
- Keep Sora + Fira Code fonts, configure as `--font-sans` and `--font-mono`

## shadcn/ui Components

| Component | Replaces |
|-----------|----------|
| Button | Custom `<button>` elements + hover handlers |
| Input | Custom `<input>` elements + focus handlers |
| Label | Custom `<label>` elements |
| Select | Custom `<select>` dropdowns |
| Dialog | CreateProjectModal, CreateFlagModal |
| Table | Flag lists, SDK keys, audit log tables |
| Tabs | Environment tab switcher (FlagDetailPage) |
| Badge | Flag type badges, tag pills |
| Card | Surface containers |
| Slider | RolloutSlider |
| Checkbox | Rollout checkboxes |
| Alert | Success/error inline messages |
| DropdownMenu | Project switcher, action menus |
| Separator | Dividers |
| Sonner (toast) | Save success feedback |

## Migration Strategy

Incremental, page by page:

1. **Foundation:** Install Tailwind + shadcn, set up theme CSS variables, add `cn()` utility
2. **Shared components:** Migrate Topbar, OrgLayout, ProjectLayout
3. **Page by page:** Convert each page — replace inline styles with Tailwind classes, swap custom elements for shadcn components
4. **Cleanup:** Remove `theme.ts`, simplify `index.css`

## What Changes

- All inline `style={{}}` objects → Tailwind utility classes
- All JS hover/focus handlers → CSS pseudoclass utilities (`hover:`, `focus:`)
- `theme.ts` → CSS custom properties
- Custom form elements → shadcn components

## What Stays

- File/folder structure (pages/, components/, hooks/, api/)
- React Query data fetching
- Form state management
- Routing
- Business logic in RuleBuilder, VariantEditor, ConfigEditor
- Noise grain overlay in index.css
