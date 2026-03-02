# Settings Page Tabbed Navigation Design

Issue: #62

## Problem

The project settings page (`/projects/:key/settings`) renders all settings sections on a single long page. As more sections are added (members, integrations), this becomes unwieldy and hard to navigate.

## Solution

Split the settings page into tabbed sub-routes. Each tab gets its own URL path and renders independently.

## Routes

```
/projects/:key/settings              → redirect to /settings/general
/projects/:key/settings/general      → project name, description, danger zone
/projects/:key/settings/lifetimes    → per-flag-type staleness thresholds
/projects/:key/settings/environments → default enabled state per environment
/projects/:key/settings/members      → placeholder ("coming soon")
```

## File Structure

```
web/src/pages/
├── ProjectSettingsPage.tsx           → layout: breadcrumbs + header + tab bar + <Outlet />
├── settings/
│   ├── GeneralSettingsTab.tsx        → GeneralSettings + DangerZone (extracted)
│   ├── FlagLifetimesTab.tsx          → FlagLifetimesSettings (extracted)
│   ├── EnvironmentDefaultsTab.tsx    → EnvironmentDefaultsSettings (extracted)
│   └── MembersTab.tsx                → "coming soon" stub
```

## Tab Navigation

Uses React Router `NavLink` elements styled identically to the existing `TabsList variant="line"` / `TabsTrigger` components from shadcn/ui. Active tab is determined by current route (via NavLink's `isActive`), not Radix state. This gives URL-based navigation while maintaining visual consistency with the tabs on ProjectDetailPage.

## Routing Config (App.tsx)

The current single `<Route path="settings" element={<ProjectSettingsPage />} />` becomes a parent with child routes:

```tsx
<Route path="settings" element={<ProjectSettingsPage />}>
  <Route index element={<Navigate to="general" replace />} />
  <Route path="general" element={<GeneralSettingsTab />} />
  <Route path="lifetimes" element={<FlagLifetimesTab />} />
  <Route path="environments" element={<EnvironmentDefaultsTab />} />
  <Route path="members" element={<MembersTab />} />
</Route>
```

## What Changes

- `ProjectSettingsPage.tsx` shrinks to a layout component (~50 lines)
- Three existing sub-components extracted into separate files (no logic changes)
- DangerZone moves into GeneralSettingsTab
- Members stub preserved as its own tab
- App.tsx routing updated with nested routes

## What Stays the Same

- All API calls, query keys, mutation logic
- Card-based layout within each tab
- Success/error message handling
- 640px max-width content area
- Breadcrumb style and page header
- Sidebar navigation link (still points to `/projects/:key/settings`)
