# Mobile Flag Screens Responsiveness — Design

**Date:** 2026-03-01
**Status:** Approved

## Problem

The flag list and flag detail screens have mobile responsiveness issues:

1. **Flag list header** takes too much vertical space on mobile — the "Create Flag" button sits full-width between the title and tabs, pushing content down.
2. **Flag detail EvaluationFlow** — the horizontal arrow diagram ("Request → Enabled → No rules → Default: on") gets cut off on narrow screens.
3. **Flag detail VariantEditor** — variant rows with fixed-width key inputs can push content off-screen.

## Design

### 1. Flag List — Floating Action Button (FAB)

- **Mobile**: Hide the inline "Create Flag" button. Render a fixed FAB (`+`) in the bottom-right corner.
  - Position: `fixed bottom-6 right-6 z-50`
  - Style: circular, amber accent background (`#d4956a`), white `+` icon, shadow
  - Opens the same `CreateFlagModal`
- **Desktop**: No change — keep existing inline button.
- Use `useIsMobile()` hook for conditional rendering.

### 2. Flag Detail — EvaluationFlow Simplified Summary

- **Mobile**: Replace the horizontal arrow diagram with a condensed text summary.
  - Format: `Enabled · 0 rules · Default: on` (single line, wraps naturally)
  - Same information, no horizontal overflow
- **Desktop**: Keep current horizontal flow diagram unchanged.
- Use `useIsMobile()` hook for conditional rendering.

### 3. Flag Detail — VariantEditor Mobile Layout

- **Mobile**: Stack key input and value input vertically within each variant row (`flex-col`).
- **Desktop**: Keep horizontal layout (`flex items-center`) as-is.
- Use Tailwind responsive classes (`flex-col md:flex-row`).

## Files to Modify

- `web/src/pages/ProjectDetailPage.tsx` — FAB + hide inline button on mobile
- `web/src/components/EvaluationFlow.tsx` — mobile summary view
- `web/src/components/VariantEditor.tsx` — mobile stacking layout
