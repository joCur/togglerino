# Mobile-Friendly Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the Togglerino dashboard fully usable on mobile devices (below 768px) with a slide-out drawer navigation, responsive layouts, and full management parity.

**Architecture:** Add a shadcn/ui Sheet component (slide-out drawer) for mobile navigation. Use a `useIsMobile` hook with `matchMedia` to conditionally render mobile vs desktop layout. Apply responsive Tailwind classes (`md:` breakpoint) throughout pages to adapt tables, forms, and content areas.

**Tech Stack:** React 19, Tailwind CSS v4, Radix UI (Sheet/Dialog primitives via shadcn/ui), lucide-react icons

---

### Task 1: Add shadcn/ui Sheet Component

The Sheet component from shadcn/ui provides the slide-out drawer primitive built on Radix Dialog. This is the foundation for mobile navigation.

**Files:**
- Create: `web/src/components/ui/sheet.tsx`

**Step 1: Install the sheet component**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.claude/worktrees/mobile-friendly/web && npx shadcn@latest add sheet --yes`

**Step 2: Verify the component was created**

Run: `ls -la /Users/jonascurth/Documents/git/togglerino/.claude/worktrees/mobile-friendly/web/src/components/ui/sheet.tsx`
Expected: File exists

**Step 3: Commit**

```bash
git add web/src/components/ui/sheet.tsx web/package.json web/package-lock.json
git commit -m "feat: add shadcn/ui sheet component for mobile drawer"
```

---

### Task 2: Create useIsMobile Hook

A reactive hook that tracks whether the viewport is below the `md` breakpoint (768px). Used by layouts to decide between sidebar and drawer.

**Files:**
- Create: `web/src/hooks/useIsMobile.ts`

**Step 1: Write the hook**

```ts
import { useState, useEffect } from 'react'

const MOBILE_BREAKPOINT = 768

export function useIsMobile() {
  const [isMobile, setIsMobile] = useState(
    typeof window !== 'undefined' ? window.innerWidth < MOBILE_BREAKPOINT : false
  )

  useEffect(() => {
    const mql = window.matchMedia(`(max-width: ${MOBILE_BREAKPOINT - 1}px)`)
    const onChange = (e: MediaQueryListEvent) => setIsMobile(e.matches)
    mql.addEventListener('change', onChange)
    setIsMobile(mql.matches)
    return () => mql.removeEventListener('change', onChange)
  }, [])

  return isMobile
}
```

**Step 2: Commit**

```bash
git add web/src/hooks/useIsMobile.ts
git commit -m "feat: add useIsMobile hook for responsive layout switching"
```

---

### Task 3: Make Topbar Responsive with Hamburger Menu

Add a hamburger icon on mobile. Hide email text. The hamburger triggers the drawer (via callback from parent layout).

**Files:**
- Modify: `web/src/components/Topbar.tsx`

**Step 1: Update Topbar to accept onMenuClick prop and show hamburger on mobile**

The modified Topbar should:
- Accept optional `onMenuClick?: () => void` prop
- Show a hamburger button (Menu icon from lucide-react) on the left before the logo, visible only below `md:` — use classes `md:hidden`
- Hide the email `<span>` below `md:` — add `hidden md:inline`
- Hide the "Log out" button below `md:` — add `hidden md:flex` (it'll be in the drawer instead)
- Keep the avatar always visible

```tsx
import { type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Menu } from 'lucide-react'

interface TopbarProps {
  children?: ReactNode
  onMenuClick?: () => void
}

export default function Topbar({ children, onMenuClick }: TopbarProps) {
  const { user, logout } = useAuth()

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

      <div className="flex items-center gap-3.5">
        <div className="w-7 h-7 rounded-full bg-[#d4956a]/8 border border-[#d4956a]/20 flex items-center justify-center text-[11px] font-semibold text-[#d4956a] font-mono">
          {user?.email?.charAt(0).toUpperCase()}
        </div>
        <span className="hidden md:inline text-xs text-muted-foreground">{user?.email}</span>
        <Button variant="outline" size="sm" className="hidden md:flex" onClick={handleLogout}>
          Log out
        </Button>
      </div>
    </header>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/components/Topbar.tsx
git commit -m "feat: add hamburger menu button and responsive topbar"
```

---

### Task 4: Make OrgLayout Responsive with Mobile Drawer

On mobile, replace the fixed sidebar with a Sheet drawer. The sidebar content (nav links) is extracted into a shared component used by both the static sidebar and the drawer.

**Files:**
- Modify: `web/src/components/OrgLayout.tsx`

**Step 1: Rewrite OrgLayout with mobile drawer**

Key changes:
- Import `useIsMobile`, `Sheet`/`SheetContent` from shadcn
- Add `drawerOpen` state
- Extract the nav link list into a `SidebarNav` local component
- Desktop: render `SidebarNav` in the fixed sidebar `<nav>` (unchanged)
- Mobile: render `SidebarNav` inside a `<Sheet>` that slides from the left
- Pass `onMenuClick` to `Topbar` that sets `drawerOpen(true)`
- Close drawer on nav link click via `useLocation` change detection
- Add user email + logout button to drawer footer on mobile
- Content area: `p-4 md:p-9`

```tsx
import { useState, useEffect } from 'react'
import { Outlet, NavLink, useLocation } from 'react-router-dom'
import { useFlag } from '@togglerino/react'
import { useAuth } from '../hooks/useAuth.ts'
import { useIsMobile } from '../hooks/useIsMobile.ts'
import Topbar from './Topbar.tsx'
import { cn } from '@/lib/utils'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
    isActive
      ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
      : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
  )

function SidebarNav() {
  const isThemeToggleEnabled = useFlag('enable-theme-toggle', false)
  return (
    <>
      <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
        Navigation
      </div>
      <NavLink to="/projects" end className={navLinkClass}>Projects</NavLink>
      <NavLink to="/settings/team" className={navLinkClass}>Team</NavLink>
      {isThemeToggleEnabled && (
        <NavLink to="/settings" end className={navLinkClass}>Settings</NavLink>
      )}
    </>
  )
}

export default function OrgLayout() {
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const location = useLocation()
  const { user, logout } = useAuth()

  // Close drawer on navigation
  useEffect(() => {
    setDrawerOpen(false)
  }, [location.pathname])

  return (
    <div className="flex flex-col min-h-screen">
      <Topbar onMenuClick={() => setDrawerOpen(true)} />

      <div className="flex flex-1">
        {/* Desktop sidebar */}
        {!isMobile && (
          <nav className="w-[200px] bg-card border-r py-5 shrink-0 flex flex-col">
            <SidebarNav />
          </nav>
        )}

        {/* Mobile drawer */}
        {isMobile && (
          <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
            <SheetContent side="left" className="w-[260px] p-0 flex flex-col">
              <nav className="py-5 flex-1 flex flex-col">
                <SidebarNav />
              </nav>
              <div className="border-t p-4 flex flex-col gap-2">
                <div className="text-xs text-muted-foreground truncate">{user?.email}</div>
                <Button variant="outline" size="sm" className="w-full" onClick={() => { logout(); setDrawerOpen(false) }}>
                  Log out
                </Button>
              </div>
            </SheetContent>
          </Sheet>
        )}

        {/* Main content */}
        <main className="flex-1 p-4 md:p-9 overflow-y-auto animate-[fadeIn_300ms_ease]">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

**Step 2: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.claude/worktrees/mobile-friendly/web && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add web/src/components/OrgLayout.tsx
git commit -m "feat: add mobile drawer navigation to OrgLayout"
```

---

### Task 5: Make ProjectLayout Responsive with Mobile Drawer

Same pattern as OrgLayout but with project-specific navigation links.

**Files:**
- Modify: `web/src/components/ProjectLayout.tsx`

**Step 1: Rewrite ProjectLayout with mobile drawer**

Same structure as OrgLayout Task 4, but with the project nav links (Flags, Lifecycle, Environments, Audit Log, Settings) and the ProjectSwitcher shown inside the drawer on mobile. On desktop, ProjectSwitcher stays in the Topbar.

```tsx
import { useState, useEffect } from 'react'
import { Outlet, NavLink, useParams, useLocation } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { useIsMobile } from '../hooks/useIsMobile.ts'
import Topbar from './Topbar.tsx'
import ProjectSwitcher from './ProjectSwitcher.tsx'
import { cn } from '@/lib/utils'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
    isActive
      ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
      : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
  )

export default function ProjectLayout() {
  const { key } = useParams<{ key: string }>()
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const location = useLocation()
  const { user, logout } = useAuth()

  useEffect(() => {
    setDrawerOpen(false)
  }, [location.pathname])

  const navLinks = (
    <>
      <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
        Project
      </div>
      <NavLink to={`/projects/${key}`} end className={navLinkClass}>Flags</NavLink>
      <NavLink to={`/projects/${key}/lifecycle`} className={navLinkClass}>Lifecycle</NavLink>
      <NavLink to={`/projects/${key}/environments`} className={navLinkClass}>Environments</NavLink>
      <NavLink to={`/projects/${key}/audit-log`} className={navLinkClass}>Audit Log</NavLink>
      <NavLink to={`/projects/${key}/settings`} className={navLinkClass}>Settings</NavLink>
    </>
  )

  return (
    <div className="flex flex-col min-h-screen">
      <Topbar onMenuClick={() => setDrawerOpen(true)}>
        {!isMobile && <ProjectSwitcher />}
      </Topbar>

      <div className="flex flex-1">
        {!isMobile && (
          <nav className="w-[200px] bg-card border-r py-5 shrink-0 flex flex-col">
            {navLinks}
          </nav>
        )}

        {isMobile && (
          <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
            <SheetContent side="left" className="w-[260px] p-0 flex flex-col">
              <div className="p-4 border-b">
                <ProjectSwitcher />
              </div>
              <nav className="py-5 flex-1 flex flex-col">
                {navLinks}
              </nav>
              <div className="border-t p-4 flex flex-col gap-2">
                <div className="text-xs text-muted-foreground truncate">{user?.email}</div>
                <Button variant="outline" size="sm" className="w-full" onClick={() => { logout(); setDrawerOpen(false) }}>
                  Log out
                </Button>
              </div>
            </SheetContent>
          </Sheet>
        )}

        <main className="flex-1 p-4 md:p-9 overflow-y-auto animate-[fadeIn_300ms_ease]">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

**Step 2: Verify lint passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.claude/worktrees/mobile-friendly/web && npm run lint`

**Step 3: Commit**

```bash
git add web/src/components/ProjectLayout.tsx
git commit -m "feat: add mobile drawer navigation to ProjectLayout"
```

---

### Task 6: Make ProjectsPage Responsive

The project cards grid needs to work on narrow screens.

**Files:**
- Modify: `web/src/pages/ProjectsPage.tsx`

**Step 1: Update the grid and header for mobile**

Changes:
- Page header: `flex-col gap-3 md:flex-row md:items-center md:justify-between` so title and button stack on mobile
- Grid: change `grid-cols-[repeat(auto-fill,minmax(300px,1fr))]` to `grid-cols-1 md:grid-cols-[repeat(auto-fill,minmax(300px,1fr))]`

In `ProjectsPage.tsx`, change:

```tsx
// Header: from
<div className="flex items-center justify-between mb-8">
// to
<div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-6 md:mb-8">
```

```tsx
// Grid: from
<div className="grid grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-4">
// to
<div className="grid grid-cols-1 sm:grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3 md:gap-4">
```

**Step 2: Commit**

```bash
git add web/src/pages/ProjectsPage.tsx
git commit -m "feat: make ProjectsPage responsive"
```

---

### Task 7: Make ProjectDetailPage Responsive

Flag list filters and the unknown flags table need mobile treatment.

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Update filter bar and header layout**

Changes:
- Header: `flex-col gap-3 md:flex-row md:items-center md:justify-between`
- Filter bar: `flex flex-col md:flex-row gap-2.5` and search input: remove `max-w-[300px]` on mobile, use `md:max-w-[300px] md:flex-1`
- Selects: add `w-full md:w-auto` so they go full-width on mobile
- Unknown flags table: wrap in `overflow-x-auto` for horizontal scrolling

In the filter section, change:
```tsx
// from
<div className="flex gap-2.5 mb-5 mt-5">
  <Input className="flex-1 max-w-[300px]" .../>
// to
<div className="flex flex-col md:flex-row gap-2.5 mb-5 mt-5">
  <Input className="w-full md:flex-1 md:max-w-[300px]" .../>
```

Each `<select>` gets `w-full md:w-auto md:min-w-[130px]` instead of just `min-w-[130px]`.

For the unknown flags table wrapper:
```tsx
// from
<div className="rounded-lg border overflow-hidden mt-5">
// to
<div className="rounded-lg border overflow-x-auto mt-5">
```

**Step 2: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat: make ProjectDetailPage filters and tables responsive"
```

---

### Task 8: Make FlagDetailPage Responsive

The flag header metadata chips should wrap nicely. Environment collapsible sections are already vertical and mostly fine.

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Update metadata chips and breadcrumb**

Changes:
- Metadata chips row: `flex flex-wrap items-center gap-2` (add `flex-wrap`)
- The environment config collapsible sections are already stacked vertically and work on mobile

**Step 2: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: make FlagDetailPage responsive"
```

---

### Task 9: Make ConfigEditor Responsive

Fixed-width inputs need to go full-width on mobile. The "Copy from" section needs to wrap.

**Files:**
- Modify: `web/src/components/ConfigEditor.tsx`

**Step 1: Update input widths and copy section**

Changes:
- Default variant input: change `min-w-[200px] max-w-[300px]` to `w-full md:min-w-[200px] md:max-w-[300px]`
- Default variant select: change `min-w-[160px]` to `w-full md:min-w-[160px] md:w-auto`
- "Copy from" section: change `flex items-center gap-3` to `flex flex-col md:flex-row md:items-center gap-3`
- Select trigger inside copy: change `w-[180px]` to `w-full md:w-[180px]`

**Step 2: Commit**

```bash
git add web/src/components/ConfigEditor.tsx
git commit -m "feat: make ConfigEditor responsive"
```

---

### Task 10: Make RuleBuilder Responsive

Condition rows are horizontal with fixed widths. On mobile they should stack vertically.

**Files:**
- Modify: `web/src/components/RuleBuilder.tsx`

**Step 1: Update condition row layout**

The condition row (attribute + operator + value + remove button) currently uses:
```tsx
<div className="flex items-center gap-1.5 mb-1.5">
```

Change to:
```tsx
<div className="flex flex-col md:flex-row md:items-center gap-1.5 mb-1.5">
```

Also update the fixed-width inputs:
- Attribute input: `w-full md:w-[180px]` instead of `w-[180px]`
- Operator select: `w-full md:w-[170px]` instead of `w-[170px]`
- Value input: keep `flex-1` (already works)

The "Serve variant" row: `flex flex-col md:flex-row md:items-center gap-2.5` and variant input `w-full md:w-[130px]`.

**Step 2: Commit**

```bash
git add web/src/components/RuleBuilder.tsx
git commit -m "feat: make RuleBuilder conditions responsive"
```

---

### Task 11: Make EnvironmentsPage Responsive

Table should scroll horizontally on mobile. Create form should stack.

**Files:**
- Modify: `web/src/pages/EnvironmentsPage.tsx`

**Step 1: Update table wrapper and form layout**

Changes:
- Header: `flex flex-col gap-3 md:flex-row md:justify-between md:items-center`
- Table wrapper: change `overflow-hidden` to `overflow-x-auto`
- Create form: change `flex gap-3 ... items-end` to `flex flex-col md:flex-row gap-3 ... md:items-end`
- Input fields inside form: remove `min-w-[160px]`, add `w-full md:w-auto`

**Step 2: Commit**

```bash
git add web/src/pages/EnvironmentsPage.tsx
git commit -m "feat: make EnvironmentsPage responsive"
```

---

### Task 12: Make SDKKeysPage Responsive

Same pattern: table horizontal scroll, form stacking.

**Files:**
- Modify: `web/src/pages/SDKKeysPage.tsx`

**Step 1: Update table and form**

Changes:
- Header: `flex flex-col gap-3 md:flex-row md:justify-between md:items-center`
- Table wrapper: `overflow-x-auto`
- Create form: `flex flex-col md:flex-row gap-3 ... md:items-end`
- Form buttons row on mobile: ensure Generate + Cancel are side-by-side with `flex flex-row gap-3` inside a wrapper

**Step 2: Commit**

```bash
git add web/src/pages/SDKKeysPage.tsx
git commit -m "feat: make SDKKeysPage responsive"
```

---

### Task 13: Make AuditLogPage Responsive

The 6-column audit log table is the tightest fit. Horizontal scroll is the pragmatic approach.

**Files:**
- Modify: `web/src/pages/AuditLogPage.tsx`

**Step 1: Update table wrapper**

Change:
```tsx
<div className="rounded-lg border overflow-hidden">
```
to:
```tsx
<div className="rounded-lg border overflow-x-auto">
```

Also update the header to stack on mobile:
```tsx
// breadcrumb already wraps naturally via flex-wrap, no change needed
```

**Step 2: Commit**

```bash
git add web/src/pages/AuditLogPage.tsx
git commit -m "feat: make AuditLogPage table horizontally scrollable"
```

---

### Task 14: Make TeamPage Responsive

Invite form and member table need mobile treatment.

**Files:**
- Modify: `web/src/pages/TeamPage.tsx`

**Step 1: Update invite form and tables**

Changes:
- Invite form: change `flex gap-3 items-end flex-wrap` to `flex flex-col md:flex-row gap-3 md:items-end`
- Email input container: remove `min-w-[200px]`, the `flex-1` already works
- Role select container: remove `min-w-[120px]`, add `w-full md:w-auto`
- Invite link row: `flex flex-col md:flex-row gap-2 md:items-center`
- Members table wrapper: `overflow-x-auto`
- Invites table wrapper: `overflow-x-auto`

**Step 2: Commit**

```bash
git add web/src/pages/TeamPage.tsx
git commit -m "feat: make TeamPage responsive"
```

---

### Task 15: Make Dialog Full-Width on Mobile

Update the shared Dialog component so modals are wider on mobile (less wasted margin).

**Files:**
- Modify: `web/src/components/ui/dialog.tsx`

**Step 1: Adjust DialogContent max-width**

The current class has `max-w-[calc(100%-2rem)] ... sm:max-w-lg`. This already works reasonably well. For better mobile UX, change to `max-w-[calc(100%-1rem)] sm:max-w-lg` so there's only 0.5rem margin on each side on very small screens. Also add `max-h-[calc(100dvh-2rem)] overflow-y-auto` for scroll on mobile.

In the `DialogContent` className, change:
```
max-w-[calc(100%-2rem)]
```
to:
```
max-w-[calc(100%-1rem)] max-h-[calc(100dvh-2rem)] overflow-y-auto
```

**Step 2: Commit**

```bash
git add web/src/components/ui/dialog.tsx
git commit -m "feat: improve dialog sizing on mobile"
```

---

### Task 16: Final Lint Check and Build Verification

**Step 1: Run lint**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.claude/worktrees/mobile-friendly/web && npm run lint`
Expected: No errors

**Step 2: Run build**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.claude/worktrees/mobile-friendly/web && npm run build`
Expected: Build succeeds, `web/dist/` created

**Step 3: Fix any issues found, commit if needed**
