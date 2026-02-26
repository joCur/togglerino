# Theme Toggle Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add light/dark/system theme switching to the web dashboard, gated by Togglerino feature flags (`enable-theme-toggle` boolean + `default-theme` string).

**Architecture:** Install `@togglerino/sdk` and `@togglerino/react` as local deps in `web/`. Wrap app with `TogglerioProvider` → `ThemeProvider`. CSS restructured from always-dark to class-based (`dark` class on `<html>`). New `/settings` route with theme picker, conditionally shown via feature flag.

**Tech Stack:** React 19, Tailwind CSS v4, shadcn/ui, `@togglerino/react`, `@togglerino/sdk`

---

### Task 1: Install SDK dependencies

**Files:**
- Modify: `web/package.json`

**Step 1: Build the SDKs (required for local file: references)**

Both SDKs need a `dist/` folder to be importable.

Run:
```bash
cd sdks/javascript && npm install && npm run build
cd sdks/react && npm install && npm run build
```

**Step 2: Install SDKs in web**

Run:
```bash
cd web && npm install ../../sdks/javascript ../../sdks/react
```

This adds local `file:` references for `@togglerino/sdk` and `@togglerino/react` to `web/package.json`.

**Step 3: Verify imports resolve**

Run:
```bash
cd web && npx tsc --noEmit 2>&1 | head -20
```
Expected: No errors related to `@togglerino/sdk` or `@togglerino/react`.

**Step 4: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "feat(web): add @togglerino/sdk and @togglerino/react dependencies"
```

---

### Task 2: Add TogglerioProvider to app

**Files:**
- Modify: `web/src/App.tsx`

**Step 1: Wrap the app with TogglerioProvider**

In `web/src/App.tsx`, add the import and wrap `QueryClientProvider` with `TogglerioProvider`:

```tsx
import { TogglerioProvider } from '@togglerino/react'
```

Add the provider config and wrap the existing JSX:

```tsx
const togglerinoConfig = {
  serverUrl: 'https://flags.curth.dev',
  sdkKey: 'sdk_37e55bbb1ae453f80d0d97b253a551a8',
}

function App() {
  return (
    <TogglerioProvider config={togglerinoConfig}>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Routes>
            <Route path="/invite/:token" element={<AcceptInvitePage />} />
            <Route path="/reset-password/:token" element={<ResetPasswordPage />} />
            <Route path="*" element={<AuthRouter />} />
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </TogglerioProvider>
  )
}
```

**Step 2: Verify the app compiles**

Run:
```bash
cd web && npx tsc --noEmit
```
Expected: No errors.

**Step 3: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): wrap app with TogglerioProvider for feature flags"
```

---

### Task 3: Restructure CSS for light/dark mode

**Files:**
- Modify: `web/src/index.css`

**Step 1: Update the custom-variant**

The current line 4 is:
```css
@custom-variant dark (&:is(.dark *));
```

Change to also match the `.dark` element itself:
```css
@custom-variant dark (&:where(.dark, .dark *));
```

**Step 2: Move current dark values into `.dark` class, add light values to `:root`**

Replace the current `:root` block (lines 51-85) with light-mode values in `:root` and dark-mode values in `.dark`:

```css
/*
 * Light theme — default when no .dark class on <html>.
 * Based on shadcn neutral palette with warm accent adjustments.
 */
:root {
  --radius: 0.625rem;
  --background: oklch(0.985 0 0);
  --foreground: oklch(0.145 0 0);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.145 0 0);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.145 0 0);
  --primary: oklch(0.205 0 0);
  --primary-foreground: oklch(0.985 0 0);
  --secondary: oklch(0.961 0 0);
  --secondary-foreground: oklch(0.205 0 0);
  --muted: oklch(0.961 0 0);
  --muted-foreground: oklch(0.45 0 0);
  --accent: oklch(0.961 0 0);
  --accent-foreground: oklch(0.205 0 0);
  --destructive: oklch(0.577 0.245 27.325);
  --destructive-foreground: oklch(0.985 0 0);
  --border: oklch(0 0 0 / 10%);
  --input: oklch(0 0 0 / 15%);
  --ring: oklch(0.556 0 0);
  --chart-1: oklch(0.488 0.243 264.376);
  --chart-2: oklch(0.696 0.17 162.48);
  --chart-3: oklch(0.769 0.188 70.08);
  --chart-4: oklch(0.627 0.265 303.9);
  --chart-5: oklch(0.645 0.246 16.439);
  --sidebar: oklch(0.975 0 0);
  --sidebar-foreground: oklch(0.145 0 0);
  --sidebar-primary: oklch(0.488 0.243 264.376);
  --sidebar-primary-foreground: oklch(0.985 0 0);
  --sidebar-accent: oklch(0.961 0 0);
  --sidebar-accent-foreground: oklch(0.205 0 0);
  --sidebar-border: oklch(0 0 0 / 10%);
  --sidebar-ring: oklch(0.556 0 0);
}

/*
 * Dark theme — applied when .dark class is on <html>.
 * Original dark-only values from the initial design.
 */
.dark {
  --background: oklch(0.145 0 0);
  --foreground: oklch(0.985 0 0);
  --card: oklch(0.178 0 0);
  --card-foreground: oklch(0.985 0 0);
  --popover: oklch(0.178 0 0);
  --popover-foreground: oklch(0.985 0 0);
  --primary: oklch(0.922 0 0);
  --primary-foreground: oklch(0.205 0 0);
  --secondary: oklch(0.269 0 0);
  --secondary-foreground: oklch(0.985 0 0);
  --muted: oklch(0.269 0 0);
  --muted-foreground: oklch(0.708 0 0);
  --accent: oklch(0.269 0 0);
  --accent-foreground: oklch(0.985 0 0);
  --destructive: oklch(0.704 0.191 22.216);
  --destructive-foreground: oklch(0.985 0 0);
  --border: oklch(1 0 0 / 10%);
  --input: oklch(1 0 0 / 15%);
  --ring: oklch(0.556 0 0);
  --chart-1: oklch(0.488 0.243 264.376);
  --chart-2: oklch(0.696 0.17 162.48);
  --chart-3: oklch(0.769 0.188 70.08);
  --chart-4: oklch(0.627 0.265 303.9);
  --chart-5: oklch(0.645 0.246 16.439);
  --sidebar: oklch(0.205 0 0);
  --sidebar-foreground: oklch(0.985 0 0);
  --sidebar-primary: oklch(0.488 0.243 264.376);
  --sidebar-primary-foreground: oklch(0.985 0 0);
  --sidebar-accent: oklch(0.269 0 0);
  --sidebar-accent-foreground: oklch(0.985 0 0);
  --sidebar-border: oklch(1 0 0 / 10%);
  --sidebar-ring: oklch(0.556 0 0);
}
```

**Step 3: Update hardcoded dark-mode styles**

Several global styles assume dark mode. Make them theme-aware:

Scrollbar thumb:
```css
::-webkit-scrollbar-thumb {
  background: oklch(0 0 0 / 6%);
  border-radius: 3px;
}
::-webkit-scrollbar-thumb:hover {
  background: oklch(0 0 0 / 12%);
}
.dark ::-webkit-scrollbar-thumb {
  background: oklch(1 0 0 / 6%);
}
.dark ::-webkit-scrollbar-thumb:hover {
  background: oklch(1 0 0 / 12%);
}
```

Selection:
```css
::selection {
  background: rgba(212, 149, 106, 0.25);
}
.dark ::selection {
  color: #fff;
}
```

Range slider track:
```css
input[type='range'] {
  background: oklch(0 0 0 / 8%);
}
.dark input[type='range'] {
  background: oklch(1 0 0 / 8%);
}
```

Placeholder:
```css
::placeholder {
  color: oklch(0 0 0 / 18%);
}
.dark ::placeholder {
  color: oklch(1 0 0 / 18%);
}
```

Option:
```css
option {
  background: #fff;
  color: #1a1a1e;
}
.dark option {
  background: #111114;
  color: #e4e4e8;
}
```

Noise grain: keep as-is (works on both).

**Step 4: Verify it compiles and the dark class still renders correctly**

Run:
```bash
cd web && npm run build
```
Expected: Build succeeds.

**Step 5: Commit**

```bash
git add web/src/index.css
git commit -m "refactor(web): restructure CSS for light/dark class-based theming"
```

---

### Task 4: Create ThemeProvider

**Files:**
- Create: `web/src/components/ThemeProvider.tsx`

**Step 1: Create the ThemeProvider component**

```tsx
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { useFlag } from '@togglerino/react'

type Theme = 'dark' | 'light' | 'system'

interface ThemeContextValue {
  theme: Theme
  setTheme: (theme: Theme) => void
  resolvedTheme: 'dark' | 'light'
  isThemeToggleEnabled: boolean
}

const ThemeContext = createContext<ThemeContextValue | undefined>(undefined)

const STORAGE_KEY = 'togglerino-theme'

function getSystemTheme(): 'dark' | 'light' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function isValidTheme(value: string): value is Theme {
  return value === 'dark' || value === 'light' || value === 'system'
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const isThemeToggleEnabled = useFlag('enable-theme-toggle', false)
  const defaultTheme = useFlag('default-theme', 'dark')

  const [theme, setThemeState] = useState<Theme>(() => {
    if (!isThemeToggleEnabled) return 'dark'
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && isValidTheme(stored)) return stored
    return isValidTheme(defaultTheme) ? defaultTheme as Theme : 'dark'
  })

  const [resolvedTheme, setResolvedTheme] = useState<'dark' | 'light'>(() => {
    if (!isThemeToggleEnabled) return 'dark'
    return theme === 'system' ? getSystemTheme() : theme
  })

  const setTheme = (newTheme: Theme) => {
    setThemeState(newTheme)
    localStorage.setItem(STORAGE_KEY, newTheme)
  }

  // When feature flag is off, force dark
  useEffect(() => {
    if (!isThemeToggleEnabled) {
      setResolvedTheme('dark')
      return
    }
    if (theme === 'system') {
      setResolvedTheme(getSystemTheme())
    } else {
      setResolvedTheme(theme)
    }
  }, [isThemeToggleEnabled, theme])

  // Listen for OS theme changes when in "system" mode
  useEffect(() => {
    if (!isThemeToggleEnabled || theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => setResolvedTheme(e.matches ? 'dark' : 'light')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [isThemeToggleEnabled, theme])

  // Apply .dark class to <html>
  useEffect(() => {
    const root = document.documentElement
    if (resolvedTheme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
  }, [resolvedTheme])

  return (
    <ThemeContext.Provider value={{ theme, setTheme, resolvedTheme, isThemeToggleEnabled }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
```

**Step 2: Verify it compiles**

Run:
```bash
cd web && npx tsc --noEmit
```
Expected: No errors.

**Step 3: Commit**

```bash
git add web/src/components/ThemeProvider.tsx
git commit -m "feat(web): add ThemeProvider with feature flag integration"
```

---

### Task 5: Wire ThemeProvider into App

**Files:**
- Modify: `web/src/App.tsx`

**Step 1: Import and add ThemeProvider**

Add import:
```tsx
import { ThemeProvider } from './components/ThemeProvider.tsx'
```

Wrap inside `TogglerioProvider` (ThemeProvider uses `useFlag`, so it must be inside TogglerioProvider):

```tsx
function App() {
  return (
    <TogglerioProvider config={togglerinoConfig}>
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <Routes>
              <Route path="/invite/:token" element={<AcceptInvitePage />} />
              <Route path="/reset-password/:token" element={<ResetPasswordPage />} />
              <Route path="*" element={<AuthRouter />} />
            </Routes>
          </BrowserRouter>
        </QueryClientProvider>
      </ThemeProvider>
    </TogglerioProvider>
  )
}
```

**Step 2: Verify it compiles**

Run:
```bash
cd web && npx tsc --noEmit
```
Expected: No errors.

**Step 3: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): wire ThemeProvider into app component tree"
```

---

### Task 6: Create SettingsPage

**Files:**
- Create: `web/src/pages/SettingsPage.tsx`

**Step 1: Create the settings page with theme picker**

```tsx
import { useTheme } from '@/components/ThemeProvider'
import { cn } from '@/lib/utils'

const themes = [
  {
    value: 'light' as const,
    label: 'Light',
    description: 'A clean, bright interface',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2" />
        <path d="M12 20v2" />
        <path d="m4.93 4.93 1.41 1.41" />
        <path d="m17.66 17.66 1.41 1.41" />
        <path d="M2 12h2" />
        <path d="M20 12h2" />
        <path d="m6.34 17.66-1.41 1.41" />
        <path d="m19.07 4.93-1.41 1.41" />
      </svg>
    ),
  },
  {
    value: 'dark' as const,
    label: 'Dark',
    description: 'Easy on the eyes',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
      </svg>
    ),
  },
  {
    value: 'system' as const,
    label: 'System',
    description: 'Follows your OS setting',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect width="20" height="14" x="2" y="3" rx="2" />
        <line x1="8" x2="16" y1="21" y2="21" />
        <line x1="12" x2="12" y1="17" y2="21" />
      </svg>
    ),
  },
]

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()

  return (
    <div className="max-w-2xl">
      <h1 className="text-lg font-semibold mb-1">Settings</h1>
      <p className="text-sm text-muted-foreground mb-8">Manage your preferences.</p>

      <div>
        <h2 className="text-sm font-medium mb-1">Appearance</h2>
        <p className="text-xs text-muted-foreground mb-4">Choose how the dashboard looks to you.</p>

        <div className="grid grid-cols-3 gap-3">
          {themes.map((t) => (
            <button
              key={t.value}
              onClick={() => setTheme(t.value)}
              className={cn(
                'flex flex-col items-center gap-2.5 rounded-lg border p-5 text-center transition-all duration-200 cursor-pointer',
                theme === t.value
                  ? 'border-[#d4956a] bg-[#d4956a]/8 ring-1 ring-[#d4956a]/30'
                  : 'border-border bg-card hover:bg-accent/50'
              )}
            >
              <div className={cn(
                'text-muted-foreground transition-colors',
                theme === t.value && 'text-[#d4956a]'
              )}>
                {t.icon}
              </div>
              <div>
                <div className={cn(
                  'text-sm font-medium',
                  theme === t.value && 'text-[#d4956a]'
                )}>
                  {t.label}
                </div>
                <div className="text-xs text-muted-foreground mt-0.5">
                  {t.description}
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
```

**Step 2: Verify it compiles**

Run:
```bash
cd web && npx tsc --noEmit
```
Expected: No errors.

**Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat(web): add SettingsPage with theme picker UI"
```

---

### Task 7: Add settings route and conditional nav

**Files:**
- Modify: `web/src/App.tsx` (add route)
- Modify: `web/src/components/OrgLayout.tsx` (add conditional sidebar link)

**Step 1: Add the route in App.tsx**

Import SettingsPage:
```tsx
import SettingsPage from './pages/SettingsPage.tsx'
```

Add the route inside the `OrgLayout` group, after the team route:
```tsx
<Route element={<OrgLayout />}>
  <Route path="/projects" element={<ProjectsPage />} />
  <Route path="/settings/team" element={<TeamPage />} />
  <Route path="/settings" element={<SettingsPage />} />
</Route>
```

**Important:** Place `/settings` after `/settings/team` so the more specific route matches first.

**Step 2: Add conditional nav in OrgLayout.tsx**

Import `useFlag`:
```tsx
import { useFlag } from '@togglerino/react'
```

Inside `OrgLayout`, read the flag:
```tsx
const isThemeToggleEnabled = useFlag('enable-theme-toggle', false)
```

Add the Settings NavLink conditionally, after the Team NavLink (same styling pattern):
```tsx
{isThemeToggleEnabled && (
  <NavLink
    to="/settings"
    end
    className={({ isActive }) =>
      cn(
        'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
        isActive
          ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
          : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-white/[0.03]'
      )
    }
  >
    Settings
  </NavLink>
)}
```

**Step 3: Verify it compiles**

Run:
```bash
cd web && npx tsc --noEmit
```
Expected: No errors.

**Step 4: Commit**

```bash
git add web/src/App.tsx web/src/components/OrgLayout.tsx
git commit -m "feat(web): add settings route and conditional nav link"
```

---

### Task 8: Fix hardcoded dark-mode colors in components

**Files:**
- Modify: `web/src/components/OrgLayout.tsx` (sidebar hover uses `bg-white/[0.03]`)

**Step 1: Replace hardcoded white/black with theme-aware alternatives**

In `OrgLayout.tsx`, the sidebar NavLink hover state uses `hover:bg-white/[0.03]` which is invisible in light mode. Change to:
```
hover:bg-foreground/[0.03]
```

This applies to both NavLink instances (Projects, Team) and the new Settings NavLink.

**Step 2: Verify it compiles**

Run:
```bash
cd web && npx tsc --noEmit
```
Expected: No errors.

**Step 3: Commit**

```bash
git add web/src/components/OrgLayout.tsx
git commit -m "fix(web): use theme-aware hover colors in sidebar"
```

---

### Task 9: Ensure dark class is set on initial load

**Files:**
- Modify: `web/index.html`

**Step 1: Add `dark` class to `<html>` tag**

This prevents a flash of light theme on initial page load before React mounts. The ThemeProvider will manage it from there.

Change:
```html
<html lang="en">
```
To:
```html
<html lang="en" class="dark">
```

**Step 2: Add inline script to set theme before React mounts**

Add this script in `<head>`, before any CSS:
```html
<script>
  (function() {
    var stored = localStorage.getItem('togglerino-theme');
    if (stored === 'light') {
      document.documentElement.classList.remove('dark');
    } else if (stored === 'system') {
      if (!window.matchMedia('(prefers-color-scheme: dark)').matches) {
        document.documentElement.classList.remove('dark');
      }
    }
  })();
</script>
```

This prevents flash-of-wrong-theme (FOWT). Default is dark (class already on html), only removes if user explicitly chose light or system-resolves-to-light.

**Step 3: Commit**

```bash
git add web/index.html
git commit -m "fix(web): prevent flash of wrong theme on page load"
```

---

### Task 10: Manual testing and visual QA

**Step 1: Start the dev server**

Run:
```bash
cd web && npm run dev
```

**Step 2: Test with flag disabled**

On `flags.curth.dev`, ensure `enable-theme-toggle` is `false`. Verify:
- [ ] App renders in dark mode (same as before)
- [ ] No "Settings" link in sidebar
- [ ] `/settings` route redirects to `/projects`

**Step 3: Test with flag enabled**

On `flags.curth.dev`, set `enable-theme-toggle` to `true` and `default-theme` to `dark`. Verify:
- [ ] "Settings" link appears in sidebar
- [ ] Settings page shows three theme cards
- [ ] Clicking "Light" switches to light theme immediately
- [ ] Clicking "Dark" switches back to dark
- [ ] Clicking "System" follows OS preference
- [ ] Accent color (#d4956a) remains consistent in both themes
- [ ] Refreshing the page preserves the selected theme (localStorage)
- [ ] Sidebar, topbar, cards, tables, buttons all look correct in light mode

**Step 4: Test `default-theme` flag**

Clear localStorage (`localStorage.removeItem('togglerino-theme')`). Change `default-theme` to `light` on the server. Reload — app should start in light mode.

**Step 5: Final build check**

Run:
```bash
cd web && npm run build && npm run lint
```
Expected: Both pass.

**Step 6: Commit any final fixes from QA**
