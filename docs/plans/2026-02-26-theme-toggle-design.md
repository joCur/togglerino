# Theme Toggle Design

## Overview

Add light/dark/system theme switching to the Togglerino web dashboard, controlled by Togglerino's own feature flags hosted at `flags.curth.dev`.

## Feature Flags

Two flags on `flags.curth.dev` (SDK key: `sdk_37e55bbb1ae453f80d0d97b253a551a8`):

1. **`enable-theme-toggle`** (boolean, default: `false`) — Gates feature visibility. When off, settings nav item and page are hidden, app stays dark-only.
2. **`theme-default`** (string: `"dark"` | `"light"` | `"system"`) — Sets the default theme for users who haven't made a choice. Only relevant when the toggle is enabled.

## SDK Integration

Add `@togglerino/react` (and `@togglerino/sdk`) as dependencies in `web/`. Wrap the app with `TogglerioProvider` in `App.tsx`, configured with:

- `serverUrl`: `https://flags.curth.dev`
- `sdkKey`: `sdk_37e55bbb1ae453f80d0d97b253a551a8`

## Theme Resolution

Priority order (highest first):

1. User's explicit choice in `localStorage` (`togglerino-theme` key)
2. `theme-default` flag value
3. Fallback: `"dark"`

For `"system"` mode, use `window.matchMedia('(prefers-color-scheme: dark)')` with a listener for OS-level changes.

## CSS Strategy: Tailwind `dark` Class

Restructure `web/src/index.css` from always-dark to class-based dark mode:

- **`:root`** — light theme CSS custom properties (new)
- **`.dark`** — dark theme CSS custom properties (current values moved here)
- Tailwind CSS v4 configured with `darkMode: 'class'` equivalent (via `@custom-variant dark (&:where(.dark, .dark *))`)

shadcn/ui components already include `dark:` variant classes, so they'll work automatically.

## ThemeProvider

New React context provider (`web/src/components/ThemeProvider.tsx`):

- Reads `useFlag('enable-theme-toggle', false)` and `useFlag('theme-default', 'dark')`
- Manages resolved theme state
- Toggles `.dark` class on `document.documentElement`
- Listens to `prefers-color-scheme` media query when mode is `"system"`
- Persists user choice to localStorage
- Exposes `{ theme, setTheme, resolvedTheme }` via context

## UI

### Settings Page (`/settings`)

- New route at `/settings` with a `SettingsPage` component
- Three selectable cards: Light, Dark, System
- Active card highlighted with accent border
- Each card shows a mini preview/icon of the theme

### Navigation

- "Settings" NavLink added to `OrgLayout` sidebar, below "Team"
- Conditionally rendered: only visible when `enable-theme-toggle` flag is `true`

## Light Theme Colors

Define light-mode CSS custom properties using oklch() to match the existing dark theme structure. Key mappings:

- `--background`: near-white
- `--foreground`: near-black
- `--card`: white/light gray
- `--primary`, `--secondary`, `--muted`, `--accent`: light-appropriate values
- Accent color `#d4956a` stays the same across both themes

## Component Considerations

- shadcn/ui `dark:` prefixes handle most components automatically
- Hardcoded colors (e.g., `bg-white/[0.03]` hover states in sidebar) need light-mode alternatives
- The noise grain overlay opacity may need adjustment for light mode
- Scrollbar colors need light variants
