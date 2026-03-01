# Mobile-Friendly Togglerino Dashboard

## Problem

The Togglerino dashboard is entirely desktop-oriented. The fixed 200px sidebar consumes ~54% of a 375px mobile screen. There are no responsive breakpoints, no mobile navigation patterns, and tables/forms use fixed widths that break on small screens. Users cannot manage feature flags on mobile devices.

## Goals

- Full management parity: everything the desktop can do, mobile can do too
- No regression to desktop layout — changes are additive via responsive breakpoints
- No new dependencies — use Tailwind responsive utilities and existing Radix primitives

## Design

### Breakpoint

All mobile adaptations trigger below **768px** (Tailwind's `md:` breakpoint). Desktop layout is unchanged above this threshold.

### Navigation: Slide-Out Drawer

**Current (desktop):** Fixed 200px sidebar always visible alongside content.

**Mobile (below 768px):**
- Sidebar hidden by default
- Topbar gains a **hamburger icon** (left side) that opens the sidebar as a **slide-out drawer**
- Drawer overlays content from the left with a dimmed backdrop
- Closes on: backdrop tap, navigation link tap, or swipe left
- Built with Radix Dialog (already a dependency via shadcn/ui) — provides focus trapping, scroll lock, and accessible dismiss

### Topbar Changes (Mobile)

- **Left:** Hamburger icon + Togglerino logo
- **Right:** User avatar only (email hidden, shown inside drawer instead)
- Logout button moves into the drawer's footer

### Layout Changes

| Element | Desktop (>= 768px) | Mobile (< 768px) |
|---------|-------------------|-------------------|
| Sidebar | Fixed 200px, always visible | Hidden, slide-out drawer |
| Content padding | `p-9` | `p-4` |
| Page headers | As-is | Sticky, smaller text |
| Dialogs/Modals | Centered overlay, max-width constrained | Near-full-screen (`inset-2`) |

### Page-Specific Adaptations

**Projects Page:**
- Grid switches to single column on mobile (already partially works via `auto-fill,minmax(300px,1fr)`)

**Flag List (ProjectDetailPage):**
- Table view replaced with **stacked card layout** on mobile
- Each flag becomes a card showing: name, type badge, environment status indicators
- Filter bar wraps vertically: search full-width on top, selects below

**Flag Detail Page:**
- Environment config sections stack vertically, full-width
- Config editor fields stack vertically (label above input)
- Rule builder conditions stack vertically

**SDK Keys / Environments Pages:**
- Table rows become cards on mobile
- Actions (copy, delete) shown as card footer buttons

**Audit Log:**
- Compact card entries with timestamp, action, and user
- Expandable details on tap for full JSON diff

**Team Page:**
- User list becomes cards instead of table rows
- Role badge and actions shown per-card

### Responsive Table Strategy

Create a shared pattern (CSS class or wrapper) for table-to-card transitions:
- Desktop: standard `<table>` layout
- Mobile: each `<tr>` becomes a card with `<td>` values labeled and stacked
- Implemented via Tailwind's `hidden md:table-cell` / `md:hidden` pattern on duplicate markup, or a `ResponsiveTable` component

### Form & Input Adaptations

- All fixed-width inputs (`min-w-[200px]`, `max-w-[300px]`) become `w-full` on mobile
- Select dropdowns go full-width
- Button groups stack vertically on mobile
- Touch targets: minimum 44px tap area on interactive elements

### Dialogs on Mobile

- Expand to near-full-screen (`inset-2` or similar) instead of centered small modal
- Scroll internally for long content
- Close button clearly visible in top-right

## Components to Create/Modify

### New Components
- `MobileDrawer` — slide-out sidebar wrapper (Radix Dialog based)
- `MobileNav` — hamburger trigger button

### Modified Components
- `Topbar.tsx` — add hamburger icon, hide email on mobile
- `OrgLayout.tsx` — conditional sidebar vs drawer based on breakpoint
- `ProjectLayout.tsx` — same as OrgLayout
- `ProjectDetailPage.tsx` — responsive flag table/cards
- `FlagDetailPage.tsx` — responsive config editor layout
- `ConfigEditor.tsx` — vertical stacking on mobile
- `RuleBuilder.tsx` — vertical stacking on mobile
- `EnvironmentsPage.tsx` — responsive table
- `SDKKeysPage.tsx` — responsive table
- `AuditLogPage.tsx` — responsive entries
- `TeamPage.tsx` — responsive user list
- Dialog component (`ui/dialog.tsx`) — full-screen variant on mobile

## Out of Scope

- Native mobile app or PWA capabilities
- Offline support
- Touch gestures beyond standard tap/scroll
- Responsive images (none in the dashboard currently)
