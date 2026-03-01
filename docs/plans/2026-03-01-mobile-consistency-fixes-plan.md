# Mobile Consistency Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix 6 mobile responsiveness inconsistencies to ensure a polished, consistent mobile experience across all screens.

**Architecture:** Pure CSS/Tailwind changes plus one `useIsMobile` hook addition. No new components, no API changes.

**Tech Stack:** React 19, Tailwind CSS v4, shadcn/ui, existing `useIsMobile` hook

---

### Task 1: Login & Auth Pages — Full-Screen on Mobile

**Files:**
- Modify: `web/src/pages/LoginPage.tsx:30-31`
- Modify: `web/src/pages/SetupPage.tsx:43-44`
- Modify: `web/src/pages/AcceptInvitePage.tsx:47-48`
- Modify: `web/src/pages/ResetPasswordPage.tsx:47-48`

**Step 1: Update LoginPage card classes**

In `LoginPage.tsx`, change the outer wrapper and card div (lines 30-31):

```tsx
// Line 30 — outer wrapper: add px-6 for mobile margin
<div className="flex items-center justify-center min-h-screen px-6 md:px-0 bg-background bg-[radial-gradient(ellipse_60%_50%_at_50%_40%,rgba(212,149,106,0.04)_0%,transparent_70%)]">
// Line 31 — card: remove border/shadow/bg on mobile, reduce padding
<div className="w-full max-w-[400px] p-6 md:p-10 rounded-2xl md:bg-card md:border md:shadow-lg animate-[fadeInUp_400ms_ease]">
```

**Step 2: Apply same pattern to SetupPage, AcceptInvitePage, ResetPasswordPage**

Each file has the identical card structure. Apply the same two class changes:
- Outer div: add `px-6 md:px-0`
- Card div: change `p-10` to `p-6 md:p-10`, change `bg-card border shadow-lg` to `md:bg-card md:border md:shadow-lg`

**Step 3: Verify with lint**

Run: `cd web && npm run lint`
Expected: No new errors

**Step 4: Commit**

```bash
git add web/src/pages/LoginPage.tsx web/src/pages/SetupPage.tsx web/src/pages/AcceptInvitePage.tsx web/src/pages/ResetPasswordPage.tsx
git commit -m "feat: make auth pages full-screen on mobile"
```

---

### Task 2: Projects Page — FAB for Create Project

**Files:**
- Modify: `web/src/pages/ProjectsPage.tsx:1-2,7,42,78-79`

**Step 1: Add imports**

Add `useIsMobile` hook and `Plus` icon to imports at top of file:

```tsx
import { useIsMobile } from '@/hooks/useIsMobile'
import { Plus } from 'lucide-react'
```

**Step 2: Add hook call**

Inside the `ProjectsPage` component, after `const [modalOpen, setModalOpen] = useState(false)` (line 13), add:

```tsx
const isMobile = useIsMobile()
```

**Step 3: Conditionally hide inline button on mobile**

Change line 42 from:
```tsx
<Button onClick={() => setModalOpen(true)}>Create Project</Button>
```
to:
```tsx
{!isMobile && <Button onClick={() => setModalOpen(true)}>Create Project</Button>}
```

**Step 4: Add FAB before closing `</div>` at line 79**

Insert before the closing tag, after `<CreateProjectModal>`:

```tsx
{isMobile && (
  <button
    onClick={() => setModalOpen(true)}
    className="fixed bottom-6 right-6 z-50 w-14 h-14 rounded-full bg-[#d4956a] text-white shadow-lg flex items-center justify-center hover:bg-[#e0a87a] active:scale-95 transition-all"
    aria-label="Create Project"
  >
    <Plus className="w-6 h-6" />
  </button>
)}
```

**Step 5: Verify with lint**

Run: `cd web && npm run lint`
Expected: No new errors

**Step 6: Commit**

```bash
git add web/src/pages/ProjectsPage.tsx
git commit -m "feat: use FAB for Create Project on mobile"
```

---

### Task 3: Dialogs — Wider Horizontal Margin

**Files:**
- Modify: `web/src/components/ui/dialog.tsx:62`

**Step 1: Update max-width calc**

In `dialog.tsx` line 62, within the long className string, change:
```
max-w-[calc(100%-1rem)]
```
to:
```
max-w-[calc(100%-2rem)]
```

This gives 16px margin per side instead of 8px.

**Step 2: Verify with lint**

Run: `cd web && npm run lint`
Expected: No new errors

**Step 3: Commit**

```bash
git add web/src/components/ui/dialog.tsx
git commit -m "feat: increase dialog horizontal margin on mobile"
```

---

### Task 4: LifecycleBoardPage — Responsive Grid

**Files:**
- Modify: `web/src/pages/LifecycleBoardPage.tsx:139`

**Step 1: Update grid classes**

Change line 139 from:
```tsx
<div className="grid grid-cols-4 gap-4">
```
to:
```tsx
<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
```

**Step 2: Verify with lint**

Run: `cd web && npm run lint`
Expected: No new errors

**Step 3: Commit**

```bash
git add web/src/pages/LifecycleBoardPage.tsx
git commit -m "feat: make lifecycle board grid responsive"
```

---

### Task 5: SettingsPage — Responsive Theme Grid

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx:64`

**Step 1: Update grid classes**

Change line 64 from:
```tsx
<div className="grid grid-cols-3 gap-3">
```
to:
```tsx
<div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
```

**Step 2: Verify with lint**

Run: `cd web && npm run lint`
Expected: No new errors

**Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: make settings theme grid responsive"
```

---

### Task 6: ProjectSettingsPage — Responsive Flag Lifetimes Form

**Files:**
- Modify: `web/src/pages/ProjectSettingsPage.tsx:75-107`

**Step 1: Update the form row layout**

Change line 75 from:
```tsx
<div key={purpose} className="flex items-center gap-4">
```
to:
```tsx
<div key={purpose} className="flex flex-col md:flex-row md:items-center gap-2 md:gap-4">
```

**Step 2: Make label width responsive**

Change line 76 from:
```tsx
<div className="w-[140px]">
```
to:
```tsx
<div className="md:w-[140px]">
```

**Step 3: Make input width responsive**

Change lines 83 and 89 — both `w-[120px]` inputs — from:
```tsx
className="w-[120px]"
```
to:
```tsx
className="w-full md:w-[120px]"
```

**Step 4: Wrap the input row for mobile stacking**

Change line 80 from:
```tsx
<div className="flex items-center gap-2">
```
to:
```tsx
<div className="flex flex-wrap items-center gap-2">
```

**Step 5: Verify with lint**

Run: `cd web && npm run lint`
Expected: No new errors

**Step 6: Commit**

```bash
git add web/src/pages/ProjectSettingsPage.tsx
git commit -m "feat: make project settings flag lifetimes responsive"
```

---

### Task 7: Final Lint Check

**Step 1: Run full lint**

Run: `cd web && npm run lint`
Expected: Clean pass, no errors
