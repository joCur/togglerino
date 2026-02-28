# Flag List & Detail Screens Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the flag list page (card-based layout) and flag detail page (environment cards, evaluation flow, collapsible config sections, danger zone in dropdown) for better scanability and UX.

**Architecture:** Pure frontend changes. Replace the table in `ProjectDetailPage` with card grid. Restructure `FlagDetailPage` with compact header, environment selector cards, always-visible evaluation flow diagram, and collapsible config sections. Extract `ConfigEditor` to its own file. Move danger zone actions to a dropdown menu.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, shadcn/ui (New York style), Radix UI, lucide-react icons.

---

### Task 1: Add missing shadcn/ui components (collapsible + dropdown-menu)

**Files:**
- Create: `web/src/components/ui/collapsible.tsx`
- Create: `web/src/components/ui/dropdown-menu.tsx`

**Step 1: Install collapsible component**

Run: `cd web && npx shadcn@latest add collapsible -y`

**Step 2: Install dropdown-menu component**

Run: `cd web && npx shadcn@latest add dropdown-menu -y`

**Step 3: Verify components exist**

Run: `ls web/src/components/ui/collapsible.tsx web/src/components/ui/dropdown-menu.tsx`
Expected: Both files listed

**Step 4: Commit**

```bash
git add web/src/components/ui/collapsible.tsx web/src/components/ui/dropdown-menu.tsx web/package.json web/package-lock.json
git commit -m "feat(web): add collapsible and dropdown-menu shadcn components"
```

---

### Task 2: Create FlagCard component for the flag list

**Files:**
- Create: `web/src/components/FlagCard.tsx`

**Step 1: Create the FlagCard component**

This is a read-only card showing flag identity + environment enable states. Props: `flag: Flag`, `environments: Environment[]`, `getEnvStatus: (flagKey: string, envId: string) => boolean`, `onClick: () => void`.

```tsx
import type { Flag, Environment } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Props {
  flag: Flag
  environments: Environment[]
  getEnvStatus: (flagKey: string, envId: string) => boolean
  onClick: () => void
}

export default function FlagCard({ flag, environments, getEnvStatus, onClick }: Props) {
  const isArchived = flag.lifecycle_status === 'archived'

  return (
    <div
      onClick={onClick}
      className={cn(
        'p-4 rounded-lg border bg-card cursor-pointer transition-all duration-200',
        'hover:border-[#d4956a]/40 hover:shadow-[0_0_12px_rgba(212,149,106,0.06)]',
        isArchived && 'opacity-60',
      )}
    >
      {/* Row 1: Key + Type */}
      <div className="flex items-center justify-between mb-1">
        <span className="font-mono text-sm text-[#d4956a] tracking-wide">{flag.key}</span>
        <Badge variant="secondary" className="font-mono text-[11px]">{flag.value_type}</Badge>
      </div>

      {/* Row 2: Name + lifecycle badge */}
      <div className="flex items-center gap-2 mb-3">
        <span className="text-[13px] text-muted-foreground">{flag.name}</span>
        {flag.lifecycle_status !== 'active' && flag.lifecycle_status !== 'archived' && (
          <Badge
            variant="secondary"
            className={cn(
              'text-[10px]',
              flag.lifecycle_status === 'stale' && 'bg-red-500/10 text-red-400 border-red-500/20',
              flag.lifecycle_status === 'potentially_stale' && 'bg-amber-500/10 text-amber-400 border-amber-500/20',
            )}
          >
            {flag.lifecycle_status === 'stale' ? 'Stale' : 'Potentially Stale'}
          </Badge>
        )}
        {isArchived && (
          <Badge variant="secondary" className="text-[10px]">Archived</Badge>
        )}
      </div>

      {/* Row 3: Environment status pills */}
      <div className="flex flex-wrap gap-2 mb-2">
        {environments?.map((env) => {
          const enabled = getEnvStatus(flag.key, env.id)
          return (
            <span
              key={env.id}
              className={cn(
                'inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-medium',
                enabled
                  ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                  : 'bg-muted/50 text-muted-foreground/60 border border-transparent',
              )}
            >
              <span
                className={cn(
                  'w-1.5 h-1.5 rounded-full',
                  enabled ? 'bg-emerald-400' : 'bg-muted-foreground/40',
                )}
              />
              {env.name}
              <span className="font-mono text-[10px] ml-0.5">
                {enabled ? 'ON' : 'OFF'}
              </span>
            </span>
          )
        })}
      </div>

      {/* Row 4: Purpose */}
      <div className="flex justify-end">
        <span className="text-[11px] text-muted-foreground/50 capitalize">{flag.flag_type}</span>
      </div>
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/components/FlagCard.tsx
git commit -m "feat(web): add FlagCard component for card-based flag list"
```

---

### Task 3: Rewrite ProjectDetailPage to use card grid

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Replace table with card grid**

Replace the table section (the `<div className="rounded-lg border overflow-hidden">` block containing `<Table>`) with a grid of `FlagCard` components. Keep all existing filter logic, unknown flags tab, and create flag modal unchanged.

Key changes:
- Import `FlagCard` from `../components/FlagCard.tsx`
- Remove `Table, TableBody, TableCell, TableHead, TableHeader, TableRow` imports
- Replace the table rendering block (lines ~224-301) with:

```tsx
<div className="grid grid-cols-1 md:grid-cols-2 gap-3">
  {filtered.map((flag) => (
    <FlagCard
      key={flag.id}
      flag={flag}
      environments={environments ?? []}
      getEnvStatus={getEnvStatus}
      onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
    />
  ))}
</div>
```

**Step 2: Verify the page renders**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

**Step 3: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat(web): replace flag table with card grid on project page"
```

---

### Task 4: Create EvaluationFlow component

**Files:**
- Create: `web/src/components/EvaluationFlow.tsx`

**Step 1: Create the evaluation flow pipeline component**

A compact horizontal pipeline that shows how flag evaluation works for the current environment config. Shows connected nodes: Enabled? → Targeting Rules → Default Variant. Dims inactive paths.

```tsx
import type { FlagEnvironmentConfig } from '../api/types.ts'
import { cn } from '@/lib/utils'

interface Props {
  config: FlagEnvironmentConfig | null
}

export default function EvaluationFlow({ config }: Props) {
  const enabled = config?.enabled ?? false
  const ruleCount = config?.targeting_rules?.length ?? 0
  const defaultVariant = config?.default_variant ?? '—'
  const hasRules = ruleCount > 0

  return (
    <div className="flex items-center gap-0 px-4 py-3 rounded-lg bg-secondary/30 border border-dashed text-[11px] font-mono overflow-x-auto">
      {/* Request node */}
      <span className="text-muted-foreground whitespace-nowrap">Request</span>
      <Arrow />

      {/* Enabled check */}
      <span
        className={cn(
          'px-2 py-1 rounded border whitespace-nowrap',
          enabled
            ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
            : 'bg-red-500/10 text-red-400 border-red-500/20',
        )}
      >
        {enabled ? 'Enabled ✓' : 'Disabled ✗'}
      </span>
      <Arrow />

      {/* Targeting rules */}
      <span
        className={cn(
          'px-2 py-1 rounded border whitespace-nowrap',
          !enabled && 'opacity-30',
          enabled && hasRules
            ? 'bg-[#d4956a]/10 text-[#d4956a] border-[#d4956a]/20'
            : 'bg-muted/50 text-muted-foreground/40 border-muted-foreground/10',
        )}
      >
        {hasRules ? `${ruleCount} rule${ruleCount > 1 ? 's' : ''}` : 'No rules'}
      </span>
      <Arrow />

      {/* Default variant */}
      <span className="px-2 py-1 rounded border bg-muted/50 text-foreground border-muted-foreground/10 whitespace-nowrap">
        Default: <span className="text-[#d4956a]">{defaultVariant || '—'}</span>
      </span>
    </div>
  )
}

function Arrow() {
  return (
    <span className="text-muted-foreground/30 mx-1.5 select-none shrink-0">→</span>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/components/EvaluationFlow.tsx
git commit -m "feat(web): add EvaluationFlow visual pipeline component"
```

---

### Task 5: Create EnvironmentSelector component

**Files:**
- Create: `web/src/components/EnvironmentSelector.tsx`

**Step 1: Create the environment selector card component**

Shows environments as clickable cards with inline enable toggles. Toggling the switch saves just the enable state immediately via PUT.

```tsx
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment, FlagEnvironmentConfig } from '../api/types.ts'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

interface Props {
  environments: Environment[]
  configs: FlagEnvironmentConfig[]
  selectedEnvKey: string
  onSelectEnv: (envKey: string) => void
  projectKey: string
  flagKey: string
}

export default function EnvironmentSelector({
  environments,
  configs,
  selectedEnvKey,
  onSelectEnv,
  projectKey,
  flagKey,
}: Props) {
  const queryClient = useQueryClient()

  const toggleMutation = useMutation({
    mutationFn: ({ envKey, config }: { envKey: string; config: FlagEnvironmentConfig }) =>
      api.put(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}`, {
        enabled: !config.enabled,
        default_variant: config.default_variant,
        variants: config.variants,
        targeting_rules: config.targeting_rules,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey] })
    },
  })

  return (
    <div className="flex flex-wrap gap-3">
      {environments.map((env) => {
        const config = configs.find((c) => c.environment_id === env.id)
        const enabled = config?.enabled ?? false
        const isSelected = env.key === selectedEnvKey

        return (
          <div
            key={env.id}
            onClick={() => onSelectEnv(env.key)}
            className={cn(
              'flex flex-col items-center gap-2 px-5 py-3 rounded-lg border cursor-pointer transition-all duration-200 min-w-[120px]',
              isSelected
                ? 'border-[#d4956a]/60 bg-[#d4956a]/5 shadow-[0_0_12px_rgba(212,149,106,0.08)]'
                : 'border-border hover:border-muted-foreground/30',
            )}
          >
            <span className={cn(
              'text-[13px] font-medium',
              isSelected ? 'text-foreground' : 'text-muted-foreground',
            )}>
              {env.name}
            </span>
            <div
              onClick={(e) => {
                e.stopPropagation()
                if (config) toggleMutation.mutate({ envKey: env.key, config })
              }}
            >
              <Switch
                checked={enabled}
                disabled={!config || toggleMutation.isPending}
              />
            </div>
            <span className={cn(
              'text-[10px] font-mono font-medium',
              enabled ? 'text-emerald-400' : 'text-muted-foreground/50',
            )}>
              {enabled ? 'ON' : 'OFF'}
            </span>
          </div>
        )
      })}
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/components/EnvironmentSelector.tsx
git commit -m "feat(web): add EnvironmentSelector card component with inline toggles"
```

---

### Task 6: Extract ConfigEditor to its own file with collapsible sections

**Files:**
- Create: `web/src/components/ConfigEditor.tsx`

**Step 1: Extract and enhance ConfigEditor**

Move the inline `ConfigEditor` from `FlagDetailPage.tsx` to its own file. Add collapsible sections for Variants and Targeting Rules. Remove the enable toggle (now handled by EnvironmentSelector). Move "Copy from environment" to the bottom as a subtle action.

```tsx
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type {
  Flag,
  Environment,
  FlagEnvironmentConfig,
  Variant,
  TargetingRule,
} from '../api/types.ts'
import VariantEditor from './VariantEditor.tsx'
import RuleBuilder from './RuleBuilder.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'

interface Props {
  config: FlagEnvironmentConfig | null
  flag: Flag
  envKey: string
  projectKey: string
  flagKey: string
  allConfigs: FlagEnvironmentConfig[]
  environments: Environment[]
}

export default function ConfigEditor({
  config,
  flag,
  envKey,
  projectKey,
  flagKey,
  allConfigs,
  environments,
}: Props) {
  const queryClient = useQueryClient()
  const [defaultVariant, setDefaultVariant] = useState(config?.default_variant ?? '')
  const [variants, setVariants] = useState<Variant[]>(config?.variants ?? [])
  const [rules, setRules] = useState<TargetingRule[]>(config?.targeting_rules ?? [])
  const [saved, setSaved] = useState(false)
  const [copySourceEnv, setCopySourceEnv] = useState<string | null>(null)
  const [copyKey, setCopyKey] = useState(0)
  const [variantsOpen, setVariantsOpen] = useState((config?.variants ?? []).length > 0)
  const [rulesOpen, setRulesOpen] = useState((config?.targeting_rules ?? []).length > 0)

  const otherEnvironments = environments.filter((e) => e.key !== envKey)

  const updateConfig = useMutation({
    mutationFn: (data: {
      enabled: boolean
      default_variant: string
      variants: Variant[]
      targeting_rules: TargetingRule[]
    }) => api.put(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey] })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const handleSave = () => {
    updateConfig.mutate({
      enabled: config?.enabled ?? false,
      default_variant: defaultVariant,
      variants,
      targeting_rules: rules,
    })
  }

  return (
    <div className="p-6 rounded-lg bg-card border">
      <div className="text-[13px] font-medium text-muted-foreground mb-4">
        Configuration: <span className="text-foreground">{envKey}</span>
      </div>

      {/* Default Variant */}
      <div className="mb-6">
        <div className="text-[13px] font-medium text-foreground mb-1">Default Variant</div>
        <div className="text-xs text-muted-foreground leading-relaxed mb-2.5">
          {flag.value_type === 'boolean'
            ? "Served when no targeting rule matches. Typically 'on' or 'off'."
            : `The ${flag.value_type} value returned when no targeting rule matches.`}
        </div>
        {variants.length > 0 ? (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[160px]"
            value={defaultVariant}
            onChange={(e) => setDefaultVariant(e.target.value)}
          >
            <option value="">-- Select --</option>
            {variants.map((v) => (
              <option key={v.key} value={v.key}>{v.key}</option>
            ))}
          </select>
        ) : (
          <Input
            className="min-w-[200px] max-w-[300px]"
            placeholder="Variant key"
            value={defaultVariant}
            onChange={(e) => setDefaultVariant(e.target.value)}
          />
        )}
      </div>

      {/* Variants (collapsible) */}
      <Collapsible open={variantsOpen} onOpenChange={setVariantsOpen} className="mb-6">
        <CollapsibleTrigger className="flex items-center gap-2 w-full text-left group">
          <ChevronRight className={cn(
            'w-4 h-4 text-muted-foreground transition-transform duration-200',
            variantsOpen && 'rotate-90',
          )} />
          <span className="text-[13px] font-medium text-foreground">
            Variants
            <span className="text-muted-foreground/60 font-normal ml-1.5">
              ({variants.length})
            </span>
          </span>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-3 pl-6">
          <VariantEditor
            variants={variants}
            valueType={flag.value_type}
            onChange={setVariants}
          />
        </CollapsibleContent>
      </Collapsible>

      {/* Targeting Rules (collapsible) */}
      <Collapsible open={rulesOpen} onOpenChange={setRulesOpen} className="mb-6">
        <CollapsibleTrigger className="flex items-center gap-2 w-full text-left group">
          <ChevronRight className={cn(
            'w-4 h-4 text-muted-foreground transition-transform duration-200',
            rulesOpen && 'rotate-90',
          )} />
          <span className="text-[13px] font-medium text-foreground">
            Targeting Rules
            <span className="text-muted-foreground/60 font-normal ml-1.5">
              ({rules.length})
            </span>
          </span>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-3 pl-6">
          <RuleBuilder
            rules={rules}
            variants={variants}
            onChange={setRules}
          />
        </CollapsibleContent>
      </Collapsible>

      {/* Copy from environment */}
      {otherEnvironments.length > 0 && (
        <div className="flex items-center gap-3 mb-6 p-3 rounded-md bg-secondary/30 border border-dashed">
          <div className="text-[13px] text-muted-foreground whitespace-nowrap">Copy from</div>
          <Select key={copyKey} onValueChange={(value) => setCopySourceEnv(value)}>
            <SelectTrigger className="w-[180px]" size="sm">
              <SelectValue placeholder="Select environment" />
            </SelectTrigger>
            <SelectContent>
              {otherEnvironments.map((env) => (
                <SelectItem key={env.key} value={env.key}>{env.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* Save */}
      <div className="flex items-center gap-3">
        <Button onClick={handleSave} disabled={updateConfig.isPending}>
          {updateConfig.isPending ? 'Saving...' : 'Save Configuration'}
        </Button>
        {saved && (
          <span className="text-[13px] text-emerald-400 animate-[fadeIn_200ms_ease]">
            Saved ✓
          </span>
        )}
      </div>

      {updateConfig.error && (
        <Alert variant="destructive" className="mt-3">
          <AlertDescription>
            Failed to save: {updateConfig.error instanceof Error ? updateConfig.error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {/* Copy Config Confirmation Dialog */}
      <Dialog open={copySourceEnv !== null} onOpenChange={(open) => { if (!open) { setCopySourceEnv(null); setCopyKey((k) => k + 1) } }}>
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
            <Button variant="outline" onClick={() => { setCopySourceEnv(null); setCopyKey((k) => k + 1) }}>
              Cancel
            </Button>
            <Button onClick={() => {
              if (!copySourceEnv) return
              const sourceEnv = environments.find((e) => e.key === copySourceEnv)
              if (!sourceEnv) return
              const sourceConfig = allConfigs.find((c) => c.environment_id === sourceEnv.id)
              if (!sourceConfig) return
              setVariants(structuredClone(sourceConfig.variants ?? []))
              setRules(structuredClone(sourceConfig.targeting_rules ?? []))
              setDefaultVariant(sourceConfig.default_variant ?? '')
              setCopySourceEnv(null)
              setCopyKey((k) => k + 1)
            }}>
              Copy Configuration
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/components/ConfigEditor.tsx
git commit -m "feat(web): extract ConfigEditor with collapsible variants and rules sections"
```

---

### Task 7: Rewrite FlagDetailPage with new layout

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Rewrite FlagDetailPage**

Replace the entire `FlagDetailPage.tsx` with the new layout:
- Compact header (key large/amber, name below, metadata inline, description muted)
- "Flag Settings" dropdown menu (gear icon) for danger zone actions
- EnvironmentSelector cards with inline toggles
- EvaluationFlow always visible
- ConfigEditor for selected environment (imported from separate file)
- Remove inline ConfigEditor function
- Keep all mutations (archive, delete, staleness) and dialogs

The new structure:
1. Back link
2. Header row: flag key (left) + Flag Settings dropdown (right)
3. Flag name, metadata chips (type · purpose · status), description
4. "Environment Configuration" section label
5. EnvironmentSelector component
6. EvaluationFlow component
7. ConfigEditor component
8. Archive/Delete confirmation dialogs (kept, just moved visually)

Key imports to add: `EnvironmentSelector`, `EvaluationFlow`, `ConfigEditor` (from components), `DropdownMenu` components, `Settings` and `Trash2` and `Archive` icons from lucide-react.

Key imports to remove: `Switch`, `Tabs, TabsContent, TabsList, TabsTrigger`, `Input` (if unused), `VariantEditor`, `RuleBuilder`, `Select/SelectContent/SelectItem/SelectTrigger/SelectValue`.

Remove the entire inline `ConfigEditor` function (lines 42-237) and the `FlagDetailResponse` interface (move to component or keep as needed).

**Step 2: Verify no type errors**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

**Step 3: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): redesign flag detail page with env cards, eval flow, and collapsible config"
```

---

### Task 8: Visual polish and cleanup

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx` — remove unused table imports if any remain
- Modify: `web/src/components/VariantEditor.tsx` — remove the header/description text (now shown by the collapsible trigger in ConfigEditor)
- Modify: `web/src/components/RuleBuilder.tsx` — remove the header/description text (now shown by the collapsible trigger in ConfigEditor)

**Step 1: Clean up VariantEditor**

Remove the "Variants" heading and description text from `VariantEditor.tsx` (lines 61-69). The collapsible wrapper in ConfigEditor now provides the section heading with a count badge.

**Step 2: Clean up RuleBuilder**

Remove the "Targeting Rules" heading and description text from `RuleBuilder.tsx` (lines 114-117). Same reason.

**Step 3: Clean up ProjectDetailPage imports**

Remove `Table, TableBody, TableCell, TableHead, TableHeader, TableRow` import if not already removed.

**Step 4: Verify everything compiles**

Run: `cd web && npx tsc --noEmit`
Expected: No type errors

**Step 5: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx web/src/components/VariantEditor.tsx web/src/components/RuleBuilder.tsx
git commit -m "refactor(web): clean up redundant headings from VariantEditor and RuleBuilder"
```

---

### Task 9: Manual visual testing

**Step 1: Start the dev server**

Run: `cd web && npm run dev`

**Step 2: Test flag list page**

Navigate to a project with flags. Verify:
- Cards render in a 2-column grid
- Each card shows: flag key (amber), name, environment status pills (green ON / muted OFF), type badge, purpose label
- Filtering by search, tag, purpose, status all work
- Clicking a card navigates to flag detail
- Unknown flags tab still works (if enabled)
- Empty state shows correctly

**Step 3: Test flag detail page**

Navigate to a flag. Verify:
- Compact header shows key, name, metadata chips, description
- Flag Settings dropdown (gear icon) shows archive/delete/stale actions
- Environment cards show with toggles — clicking a toggle enables/disables immediately
- Evaluation flow shows the pipeline with correct counts
- Selected environment config shows with collapsible variants and rules
- Collapsible sections auto-expand when content exists
- Save button works
- Copy from environment works
- Archive/delete confirmations work

**Step 4: Commit any fixes**

```bash
git add -A
git commit -m "fix(web): visual polish for flag list and detail redesign"
```
