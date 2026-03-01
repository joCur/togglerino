# Mobile Consistency Fixes Design

## Context

The mobile-friendly branch has good responsive foundations (drawer navigation, useIsMobile hook, responsive tables), but several screens still have desktop-first patterns that feel inconsistent or cramped on mobile.

## Fixes

### Fix 1: Login & Auth Pages — Full-Screen on Mobile

**Affected files**: `LoginPage.tsx`, `SetupPage.tsx`, `AcceptInvitePage.tsx`, `ResetPasswordPage.tsx`

**Current**: Floating card with `max-w-[400px]`, `p-10`, border/shadow — looks like a shrunken desktop page on mobile.

**Change**: On mobile (< md), remove card border/shadow/background. Content fills the screen with `px-6` horizontal padding and reduced vertical padding. Logo and form vertically centered via flexbox. Desktop (>= md) keeps the current floating card style unchanged.

### Fix 2: Projects Page — FAB for Create Project

**Affected file**: `ProjectsPage.tsx`

**Current**: Full-width inline "Create Project" button in the header area.

**Change**: Import `useIsMobile`. On mobile, hide the inline button and show a FAB (floating action button) matching the existing Create Flag FAB pattern: fixed bottom-6 right-6, w-14 h-14, rounded-full, amber `bg-[#d4956a]`, Plus icon. Desktop keeps the inline button.

### Fix 3: Dialogs — Wider Horizontal Margin

**Affected file**: `web/src/components/ui/dialog.tsx`

**Current**: `max-w-[calc(100%-1rem)]` gives only 8px margin per side.

**Change**: Increase to `max-w-[calc(100%-2rem)]` for 16px margin per side. Single line change, fixes all dialogs globally.

### Fix 4: LifecycleBoardPage — Responsive Grid

**Affected file**: `LifecycleBoardPage.tsx`

**Current**: `grid-cols-4` — forces 4 cramped columns on mobile.

**Change**: `grid-cols-1 sm:grid-cols-2 lg:grid-cols-4` — stacks on mobile, 2 columns on tablet, 4 on desktop.

### Fix 5: SettingsPage — Responsive Theme Grid

**Affected file**: `SettingsPage.tsx`

**Current**: `grid-cols-3` — theme buttons cramped on mobile.

**Change**: `grid-cols-1 sm:grid-cols-3`.

### Fix 6: ProjectSettingsPage — Responsive Form Layout

**Affected file**: `ProjectSettingsPage.tsx`

**Current**: Fixed-width labels (`w-[140px]`, `w-[120px]`) in horizontal flex rows that don't stack on mobile.

**Change**: Add `flex-col md:flex-row` to the form rows. Remove fixed label widths on mobile (use `w-full md:w-[140px]` pattern). Inputs stretch to fill on mobile.
