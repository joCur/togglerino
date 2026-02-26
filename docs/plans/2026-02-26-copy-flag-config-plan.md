# Copy Flag Config Between Environments — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a "Copy from" dropdown to the flag detail page's ConfigEditor that lets users copy variants, targeting rules, and default variant from another environment into the current one.

**Architecture:** Frontend-only change. The `ConfigEditor` component receives all environment configs as a new prop, renders a "Copy from" select dropdown with a confirmation dialog, and on confirm populates its local state from the source config (excluding `enabled`).

**Tech Stack:** React 19, TypeScript, shadcn/ui (Select, Dialog), TanStack Query (existing)

---

### Task 1: Pass all environment configs and environments to ConfigEditor

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx:34-46` (ConfigEditor props)
- Modify: `web/src/pages/FlagDetailPage.tsx:359-369` (ConfigEditor usage)

**Step 1: Add new props to ConfigEditor**

Add `allConfigs` and `environments` to the ConfigEditor function signature and props type:

```tsx
function ConfigEditor({
  config,
  flag,
  envKey,
  projectKey,
  flagKey,
  allConfigs,
  environments,
}: {
  config: FlagEnvironmentConfig | null
  flag: Flag
  envKey: string
  projectKey: string
  flagKey: string
  allConfigs: FlagEnvironmentConfig[]
  environments: Environment[]
}) {
```

**Step 2: Pass the new props at the call site**

In the `environments.map` block (~line 363), update the `<ConfigEditor>` usage:

```tsx
<ConfigEditor
  config={envConfig}
  flag={flag}
  envKey={env.key}
  projectKey={key!}
  flagKey={flagKey!}
  allConfigs={data.environment_configs}
  environments={environments}
/>
```

**Step 3: Verify the app compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): pass environment configs to ConfigEditor for copy feature"
```

---

### Task 2: Add "Copy from" select dropdown to ConfigEditor

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx:1-2` (imports)
- Modify: `web/src/pages/FlagDetailPage.tsx` (inside ConfigEditor, before the Toggle section)

**Step 1: Add Select imports**

Add to the existing imports at the top of the file:

```tsx
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
```

**Step 2: Compute other environments inside ConfigEditor**

After the existing state declarations and before the `return`, add:

```tsx
const otherEnvironments = environments.filter((e) => e.key !== envKey)
```

**Step 3: Add the "Copy from" UI**

Inside the ConfigEditor return, add a section between the opening `<div className="p-6 ...">` and the Toggle `<div>` (before line 80):

```tsx
{/* Copy from environment */}
{otherEnvironments.length > 0 && (
  <div className="flex items-center gap-3 mb-6 p-3 rounded-md bg-secondary/30 border border-dashed">
    <div className="text-[13px] text-muted-foreground whitespace-nowrap">Copy from</div>
    <Select onValueChange={(value) => setCopySourceEnv(value)}>
      <SelectTrigger className="w-[180px]" size="sm">
        <SelectValue placeholder="Select environment" />
      </SelectTrigger>
      <SelectContent>
        {otherEnvironments.map((env) => (
          <SelectItem key={env.key} value={env.key}>
            {env.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  </div>
)}
```

**Step 4: Add state for the copy source**

Add to the state declarations inside ConfigEditor:

```tsx
const [copySourceEnv, setCopySourceEnv] = useState<string | null>(null)
```

**Step 5: Verify the app compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors (copySourceEnv is set but not yet consumed — that's fine, dialog comes next)

**Step 6: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): add 'Copy from' environment select dropdown"
```

---

### Task 3: Add confirmation dialog and apply copy logic

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx` (inside ConfigEditor component)

**Step 1: Add the confirmation dialog**

After the save error Alert block (after the closing `</Alert>` for `updateConfig.error`) and before the closing `</div>` of ConfigEditor, add:

```tsx
{/* Copy Config Confirmation Dialog */}
<Dialog open={copySourceEnv !== null} onOpenChange={(open) => { if (!open) setCopySourceEnv(null) }}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Copy configuration?</DialogTitle>
      <DialogDescription>
        This will replace the current variants, targeting rules, and default variant
        in <span className="font-semibold text-foreground">{envKey}</span> with
        the configuration from <span className="font-semibold text-foreground">{copySourceEnv}</span>.
        The enabled/disabled state will not change.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onClick={() => setCopySourceEnv(null)}>
        Cancel
      </Button>
      <Button onClick={() => {
        if (!copySourceEnv) return
        const sourceEnv = environments.find((e) => e.key === copySourceEnv)
        if (!sourceEnv) return
        const sourceConfig = allConfigs.find((c) => c.environment_id === sourceEnv.id)
        if (!sourceConfig) return
        setVariants(sourceConfig.variants ?? [])
        setRules(sourceConfig.targeting_rules ?? [])
        setDefaultVariant(sourceConfig.default_variant ?? '')
        setCopySourceEnv(null)
      }}>
        Copy Configuration
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

Note: The `Dialog`, `DialogContent`, `DialogDescription`, `DialogFooter`, `DialogHeader`, `DialogTitle` imports already exist at lines 21-27. No new imports needed for this step.

**Step 2: Verify the app compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): add confirmation dialog and copy logic for env config"
```

---

### Task 4: Manual testing and visual verification

**Step 1: Start the dev environment**

Run: `cd web && npm run dev`

**Step 2: Test the feature**

1. Navigate to any flag detail page with multiple environments
2. Switch to an environment tab (e.g., staging)
3. Verify the "Copy from" dropdown appears with the other environments listed
4. Select a source environment from the dropdown
5. Verify the confirmation dialog appears with correct environment names
6. Click "Cancel" — verify dialog closes and nothing changes
7. Select the source again and click "Copy Configuration"
8. Verify variants, targeting rules, and default variant are populated from the source
9. Verify the enabled/disabled toggle did NOT change
10. Click "Save Configuration" to persist
11. Verify the save succeeds

**Step 3: Edge case testing**

- Test with a flag that has only one environment (dropdown should not appear)
- Test copying from an environment with empty config (should clear the fields)
- Test copying when the source has targeting rules with percentage rollouts

**Step 4: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 5: Final commit (if any lint fixes needed)**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "fix(web): lint fixes for copy config feature"
```
