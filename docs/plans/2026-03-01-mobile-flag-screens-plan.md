# Mobile Flag Screens Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix mobile responsiveness on flag list and flag detail screens — FAB for create button, simplified EvaluationFlow, stacked VariantEditor.

**Architecture:** Three isolated component changes. FAB uses `useIsMobile()` hook + conditional rendering. EvaluationFlow renders a condensed text summary on mobile. VariantEditor switches to stacked layout via Tailwind responsive classes.

**Tech Stack:** React 19, Tailwind CSS v4, lucide-react (Plus icon), existing `useIsMobile` hook.

---

### Task 1: FAB for Create Flag on mobile

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Add imports**

Add `useIsMobile` hook and `Plus` icon to the existing imports at the top of `ProjectDetailPage.tsx`:

```tsx
import { useIsMobile } from '@/hooks/useIsMobile'
import { Plus } from 'lucide-react'
```

**Step 2: Call the hook**

Inside the `ProjectDetailPage` component, after the existing state declarations (after line 42), add:

```tsx
const isMobile = useIsMobile()
```

**Step 3: Conditionally hide inline button on mobile**

Replace lines 147-150 (the header div containing title + button):

```tsx
<div className="flex items-center justify-between mb-6">
  <h1 className="text-[22px] font-semibold text-foreground tracking-tight">{key}</h1>
  {!isMobile && <Button onClick={() => setModalOpen(true)}>Create Flag</Button>}
</div>
```

**Step 4: Add FAB at the bottom of the component**

Right before the closing `</div>` of the component return (before the final `</div>` at line 316), add the FAB:

```tsx
{isMobile && (
  <button
    onClick={() => setModalOpen(true)}
    className="fixed bottom-6 right-6 z-50 w-14 h-14 rounded-full bg-[#d4956a] text-white shadow-lg flex items-center justify-center hover:bg-[#e0a87a] active:scale-95 transition-all"
    aria-label="Create Flag"
  >
    <Plus className="w-6 h-6" />
  </button>
)}
```

**Step 5: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 6: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat: add floating action button for Create Flag on mobile"
```

---

### Task 2: Simplified EvaluationFlow on mobile

**Files:**
- Modify: `web/src/components/EvaluationFlow.tsx`

**Step 1: Add import**

Add `useIsMobile` to imports at the top of the file:

```tsx
import { useIsMobile } from '@/hooks/useIsMobile'
```

**Step 2: Add mobile summary branch**

Inside the `EvaluationFlow` component, after the existing variable declarations (after line 12), add:

```tsx
const isMobile = useIsMobile()
```

Then replace the return statement. The component should return either the mobile summary or the existing desktop flow:

```tsx
if (isMobile) {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 px-3 py-2.5 rounded-lg bg-secondary/30 border border-dashed text-[11px] font-mono">
      <span
        className={cn(
          'px-2 py-0.5 rounded',
          enabled
            ? 'bg-emerald-500/10 text-emerald-400'
            : 'bg-red-500/10 text-red-400',
        )}
      >
        {enabled ? 'Enabled' : 'Disabled'}
      </span>
      <span className="text-muted-foreground/30">·</span>
      <span className="text-muted-foreground">
        {hasRules ? `${ruleCount} rule${ruleCount > 1 ? 's' : ''}` : '0 rules'}
      </span>
      <span className="text-muted-foreground/30">·</span>
      <span className="text-muted-foreground">
        Default: <span className="text-[#d4956a]">{defaultVariant || '—'}</span>
      </span>
    </div>
  )
}
```

Keep the existing desktop return as-is (the `<div className="flex items-center gap-0 ...">` block).

**Step 3: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/components/EvaluationFlow.tsx
git commit -m "feat: simplified EvaluationFlow summary on mobile"
```

---

### Task 3: Stacked VariantEditor on mobile

**Files:**
- Modify: `web/src/components/VariantEditor.tsx`

**Step 1: Update variant row layout**

Change line 80 — the variant row container — from:

```tsx
<div key={i} className="flex items-center gap-2">
```

To:

```tsx
<div key={i} className="flex flex-col md:flex-row md:items-center gap-2">
```

**Step 2: Make key input full-width on mobile**

Change line 82 — the key Input — from:

```tsx
<Input
  className="flex-none w-[110px] font-mono text-xs"
```

To:

```tsx
<Input
  className="w-full md:flex-none md:w-[110px] font-mono text-xs"
```

**Step 3: Make remove button full-width on mobile**

Change lines 112-119 — the Remove button — from:

```tsx
<Button
  variant="destructive"
  size="sm"
  className="shrink-0 text-[11px] px-2.5 h-7"
  onClick={() => remove(i)}
>
  Remove
</Button>
```

To:

```tsx
<Button
  variant="destructive"
  size="sm"
  className="shrink-0 text-[11px] px-2.5 h-7 self-end md:self-auto"
  onClick={() => remove(i)}
>
  Remove
</Button>
```

**Step 4: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/components/VariantEditor.tsx
git commit -m "feat: stack VariantEditor rows vertically on mobile"
```

---

### Task 4: Visual verification

**Step 1: Start dev server**

Run: `cd web && npm run dev`

**Step 2: Verify in browser**

Open the app in a mobile viewport (375px wide) and verify:
- Flag list: no "Create Flag" button in header, FAB visible in bottom-right corner, FAB opens create modal
- Flag detail: EvaluationFlow shows condensed summary (e.g. "Enabled · 0 rules · Default: on")
- Flag detail: VariantEditor rows stack vertically with full-width inputs
- Desktop (>768px): everything looks the same as before (inline button, horizontal flow, horizontal variant rows)
