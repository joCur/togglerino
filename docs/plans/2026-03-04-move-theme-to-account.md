# Move Theme to Account Page — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move the theme mode selector from the global settings page to the user account page, and redirect the user dropdown "Settings" link to the account page.

**Architecture:** Pure frontend refactoring — cut the theme selector JSX + `themes` array from `SettingsPage.tsx`, paste into `AccountPage.tsx` as a new Card section. Update `Topbar.tsx` dropdown to point "Settings" at `/account`. Clean up `OrgLayout.tsx` sidebar nav visibility and `SettingsPage.tsx` redirect logic since theme is no longer there.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, shadcn/ui, React Router v7

---

### Task 1: Add theme selector to AccountPage

**Files:**
- Modify: `web/src/pages/AccountPage.tsx`

**Step 1: Add useTheme import and themes array**

Add after existing imports at top of `AccountPage.tsx`:

```tsx
import { useTheme, type Theme } from '@/hooks/useTheme'
import { cn } from '@/lib/utils'
```

Add the `themes` array (currently in `SettingsPage.tsx`) before the `AccountPage` component:

```tsx
const themes: { value: Theme; label: string; description: string; icon: React.ReactNode }[] = [
  {
    value: 'light',
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
    value: 'dark',
    label: 'Dark',
    description: 'Easy on the eyes',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
      </svg>
    ),
  },
  {
    value: 'system',
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
```

**Step 2: Add theme Card to AccountPage JSX**

Inside the component, add `const { theme, setTheme, isThemeToggleEnabled } = useTheme()` after the existing hooks.

Add a new Card section between the "SSO Identity" card and the "Account Info" card (before `{/* Account Info */}`):

```tsx
        {/* Appearance */}
        {isThemeToggleEnabled && (
          <Card className="mb-5">
            <CardContent className="p-6">
              <div className="text-sm font-semibold text-foreground mb-1">
                Appearance
              </div>
              <p className="text-[13px] text-muted-foreground/60 mb-4">Choose how the dashboard looks to you.</p>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
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
            </CardContent>
          </Card>
        )}
```

**Step 3: Verify it builds**

Run: `cd web && npm run build`
Expected: Build succeeds with no TypeScript errors.

**Step 4: Commit**

```bash
git add web/src/pages/AccountPage.tsx
git commit -m "feat: add theme selector to account page"
```

---

### Task 2: Remove theme selector from SettingsPage

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

**Step 1: Remove theme-related code from SettingsPage**

Remove the `useTheme` import, the entire `themes` array, and the theme toggle JSX block. Remove the `cn` import (no longer needed). Remove the `isThemeToggleEnabled` variable and the redirect logic that depends on it.

The page should now only show OIDC settings for admins, and redirect non-admins:

```tsx
import { Navigate } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import OIDCSettingsTab from './settings/OIDCSettingsTab'

export default function SettingsPage() {
  const { user } = useAuth()

  if (user?.role !== 'admin') {
    return <Navigate to="/projects" replace />
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-lg font-semibold mb-1">Settings</h1>
      <p className="text-sm text-muted-foreground mb-8">Organization settings.</p>

      <OIDCSettingsTab />
    </div>
  )
}
```

**Step 2: Verify it builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "refactor: remove theme selector from global settings page"
```

---

### Task 3: Update Topbar dropdown and OrgLayout sidebar

**Files:**
- Modify: `web/src/components/Topbar.tsx`
- Modify: `web/src/components/OrgLayout.tsx`

**Step 1: Update Topbar — change "Settings" to link to /account**

In `Topbar.tsx`, the dropdown currently has two items when theme toggle is enabled: "Account" linking to `/account` and "Settings" linking to `/settings`. Since theme is now on the account page, remove the conditional "Settings" dropdown item entirely — the Account link already covers user-scoped settings. Also remove the `useFlag` import and `isThemeToggleEnabled` variable since they're no longer needed. Remove the `Settings` icon import from lucide-react.

Updated file:

```tsx
import { type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ChevronDown, User as UserIcon, LogOut, Menu } from 'lucide-react'

interface TopbarProps {
  children?: ReactNode
  onMenuClick?: () => void
}

export default function Topbar({ children, onMenuClick }: TopbarProps) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = async () => {
    try {
      await logout()
    } catch {
      // ignore
    }
  }

  return (
    <header className="flex items-center justify-between px-4 md:px-6 h-[52px] bg-card border-b shrink-0">
      <div className="flex items-center gap-3 md:gap-4">
        {onMenuClick && (
          <Button
            variant="ghost"
            size="sm"
            className="md:hidden h-8 w-8 p-0"
            onClick={onMenuClick}
          >
            <Menu className="w-5 h-5" />
          </Button>
        )}
        <Link to="/projects" className="flex items-center gap-2.5 no-underline">
          <svg width="20" height="12" viewBox="0 0 20 12" fill="none">
            <rect width="20" height="12" rx="6" fill="#d4956a" opacity="0.25" />
            <circle cx="14" cy="6" r="4" fill="#d4956a" />
          </svg>
          <span className="font-mono text-sm font-semibold text-[#d4956a] tracking-wide">
            togglerino
          </span>
        </Link>
        {children}
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted/50 transition-colors outline-none">
            <div className="w-7 h-7 rounded-full bg-[#d4956a]/8 border border-[#d4956a]/20 flex items-center justify-center text-[11px] font-semibold text-[#d4956a] font-mono">
              {user?.email?.charAt(0).toUpperCase()}
            </div>
            <span className="hidden md:inline text-xs text-muted-foreground max-w-[150px] truncate">
              {user?.display_name || user?.email}
            </span>
            <ChevronDown className="hidden md:block h-3 w-3 text-muted-foreground" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem onClick={() => navigate('/account')}>
            <UserIcon className="mr-2 h-4 w-4" />
            Account
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleLogout}>
            <LogOut className="mr-2 h-4 w-4" />
            Log out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>
  )
}
```

**Step 2: Update OrgLayout sidebar — show Settings nav only for admins**

In `OrgLayout.tsx`, the sidebar currently shows a "Settings" link conditionally on `isThemeToggleEnabled`. Since theme is gone from that page, the Settings page is now admin-only (OIDC config). Change the condition so "Settings" shows only for admins. Remove the `useFlag` import from `SidebarNav` since it's no longer needed, and pass the user role instead.

In `SidebarNav`, replace the `isThemeToggleEnabled` check with an `isAdmin` prop:

```tsx
function SidebarNav({ onNavigate, isAdmin }: { onNavigate?: () => void; isAdmin?: boolean }) {
  return (
    <>
      <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
        Navigation
      </div>
      <NavLink to="/projects" end className={navLinkClass} onClick={onNavigate}>Projects</NavLink>
      <NavLink to="/settings/team" className={navLinkClass} onClick={onNavigate}>Team</NavLink>
      {isAdmin && (
        <NavLink to="/settings" end className={navLinkClass} onClick={onNavigate}>Settings</NavLink>
      )}
    </>
  )
}
```

Update the two `<SidebarNav>` usages in `OrgLayout` to pass `isAdmin={user?.role === 'admin'}`:

```tsx
<SidebarNav isAdmin={user?.role === 'admin'} />
```

and:

```tsx
<SidebarNav onNavigate={closeDrawer} isAdmin={user?.role === 'admin'} />
```

Remove the `useFlag` import from `@togglerino/react` at the top of the file (it's no longer used).

**Step 3: Verify it builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 4: Run lint**

Run: `cd web && npm run lint`
Expected: No errors (confirm no unused imports).

**Step 5: Commit**

```bash
git add web/src/components/Topbar.tsx web/src/components/OrgLayout.tsx
git commit -m "refactor: update navigation after theme move to account page"
```

---

### Task 4: Final verification

**Step 1: Full build**

Run: `cd web && npm run build`
Expected: Clean build, no warnings.

**Step 2: Lint check**

Run: `cd web && npm run lint`
Expected: No errors.

**Step 3: Verify no remaining references to theme in SettingsPage**

Search for `useTheme` in `SettingsPage.tsx` — should not exist.
Search for `themes` array in `SettingsPage.tsx` — should not exist.

**Step 4: Verify no unused imports**

Check that `@togglerino/react` `useFlag` is not imported in `Topbar.tsx` or `OrgLayout.tsx` (no longer needed in those files).
