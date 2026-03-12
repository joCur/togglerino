# Environment Comparison Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Compare" tab to the flag detail page showing side-by-side environment config diffs for a single flag.

**Architecture:** Pure frontend feature — no backend changes. A `flag-diff.ts` utility computes diffs from existing `FlagEnvironmentConfig[]` data. A `CompareTab.tsx` component renders the comparison grid with expandable detail panels. The existing `FlagDetailPage.tsx` gains a third tab.

**Tech Stack:** React 19, TypeScript, Tailwind CSS v4, shadcn/ui (Badge, Collapsible, Switch, Tabs), Vitest (new dev dependency for web package)

**Spec:** `docs/superpowers/specs/2026-03-11-environment-comparison-design.md`

**Worktree:** `.worktrees/env-compare` on branch `feat/environment-comparison`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `web/package.json` | Modify | Add vitest dev dependency |
| `web/vitest.config.ts` | Create | Vitest config with path aliases matching vite.config.ts |
| `web/src/lib/flag-diff.ts` | Create | Pure diff utility: types + comparison functions |
| `web/src/lib/__tests__/flag-diff.test.ts` | Create | Unit tests for diff logic |
| `web/src/components/CompareTab.tsx` | Create | Comparison grid UI component |
| `web/src/pages/FlagDetailPage.tsx` | Modify:383-608 | Add Compare tab trigger + content, URL search param sync |

---

## Chunk 1: Test Infrastructure + Diff Logic

### Task 1: Add Vitest to web package

The web package has no test framework. The JS/React SDKs use Vitest, so we follow that convention.

**Files:**
- Modify: `web/package.json`
- Create: `web/vitest.config.ts`
- Refer to: `web/vite.config.ts` (for path alias config)

- [ ] **Step 1: Install vitest**

Run from the worktree:
```bash
cd .worktrees/env-compare/web && npm install -D vitest
```

- [ ] **Step 2: Add test script to package.json**

In `web/package.json`, add to `"scripts"`:
```json
"test": "vitest run",
"test:watch": "vitest"
```

- [ ] **Step 3: Create vitest.config.ts**

Create `web/vitest.config.ts`:
```typescript
import { defineConfig } from 'vitest/config'
import path from 'path'

export default defineConfig({
  test: {
    environment: 'node',
    passWithNoTests: true,
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
})
```

Note: Use `environment: 'node'` since `flag-diff.ts` is pure logic with no DOM dependencies. If component tests are added later, switch to `jsdom`.

- [ ] **Step 4: Verify vitest runs**

```bash
cd .worktrees/env-compare/web && npm test
```

Expected: vitest runs, finds no tests, exits with code 0 (thanks to `passWithNoTests: true`).

- [ ] **Step 5: Commit**

```bash
git -C .worktrees/env-compare add web/package.json web/package-lock.json web/vitest.config.ts
git -C .worktrees/env-compare commit -m "chore: add vitest to web package for frontend unit tests"
```

---

### Task 2: Write failing tests for canonical serialization helper

The diff logic needs a `canonicalize` function that serializes objects with sorted keys (recursively) to avoid false positives from property ordering.

**Files:**
- Create: `web/src/lib/__tests__/flag-diff.test.ts`
- Will implement in: `web/src/lib/flag-diff.ts`

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/__tests__/flag-diff.test.ts`:
```typescript
import { describe, it, expect } from 'vitest'
import { canonicalize } from '../flag-diff'

describe('canonicalize', () => {
  it('serializes objects with sorted keys', () => {
    const a = canonicalize({ z: 1, a: 2 })
    const b = canonicalize({ a: 2, z: 1 })
    expect(a).toBe(b)
  })

  it('handles nested objects with different key order', () => {
    const a = canonicalize({ outer: { z: 1, a: 2 } })
    const b = canonicalize({ outer: { a: 2, z: 1 } })
    expect(a).toBe(b)
  })

  it('handles arrays (preserves order)', () => {
    const a = canonicalize([{ b: 1, a: 2 }, { d: 3, c: 4 }])
    const b = canonicalize([{ a: 2, b: 1 }, { c: 4, d: 3 }])
    expect(a).toBe(b)
  })

  it('handles primitives', () => {
    expect(canonicalize('hello')).toBe('"hello"')
    expect(canonicalize(42)).toBe('42')
    expect(canonicalize(true)).toBe('true')
    expect(canonicalize(null)).toBe('null')
  })

  it('handles undefined values by omitting them', () => {
    const a = canonicalize({ a: 1, b: undefined })
    const b = canonicalize({ a: 1 })
    expect(a).toBe(b)
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: FAIL — `canonicalize` is not exported from `../flag-diff` (module not found).

- [ ] **Step 3: Implement canonicalize**

Create `web/src/lib/flag-diff.ts`:
```typescript
/**
 * Recursively serializes a value with sorted object keys.
 * Ensures {a:1, b:2} and {b:2, a:1} produce identical strings.
 */
export function canonicalize(value: unknown): string {
  if (value === null || value === undefined) {
    return JSON.stringify(value ?? null)
  }
  if (Array.isArray(value)) {
    return '[' + value.map(canonicalize).join(',') + ']'
  }
  if (typeof value === 'object') {
    const obj = value as Record<string, unknown>
    const keys = Object.keys(obj).sort()
    const entries = keys
      .filter((k) => obj[k] !== undefined)
      .map((k) => JSON.stringify(k) + ':' + canonicalize(obj[k]))
    return '{' + entries.join(',') + '}'
  }
  return JSON.stringify(value)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: All 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git -C .worktrees/env-compare add web/src/lib/flag-diff.ts web/src/lib/__tests__/flag-diff.test.ts
git -C .worktrees/env-compare commit -m "feat: add canonicalize helper for order-independent object comparison"
```

---

### Task 3: Write failing tests for compareEnabled and compareDefaultVariant

**Files:**
- Modify: `web/src/lib/__tests__/flag-diff.test.ts`
- Modify: `web/src/lib/flag-diff.ts`

- [ ] **Step 1: Write the failing tests**

Append to `web/src/lib/__tests__/flag-diff.test.ts`:
```typescript
import { canonicalize, compareEnabled, compareDefaultVariant } from '../flag-diff'
import type { FlagEnvironmentConfig } from '@/api/types'

// Helper to create a minimal config for testing
function makeConfig(overrides: Partial<FlagEnvironmentConfig> & { environment_id: string }): FlagEnvironmentConfig {
  return {
    id: 'cfg-' + overrides.environment_id,
    flag_id: 'flag-1',
    enabled: false,
    default_variant: 'off',
    variants: [],
    targeting_rules: [],
    updated_at: '2026-01-01T00:00:00Z',
    locked: false,
    ...overrides,
  }
}

describe('compareEnabled', () => {
  it('returns match when all environments have same enabled state', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true }),
      makeConfig({ environment_id: 'env-2', enabled: true }),
    ]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-1')).toBe(true)
    expect(result.values.get('env-2')).toBe(true)
  })

  it('returns differs when enabled states differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true }),
      makeConfig({ environment_id: 'env-2', enabled: false }),
    ]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('treats missing config as disabled', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true }),
    ]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.values.get('env-2')).toBe(false)
  })

  it('returns match for single environment', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true }),
    ]
    const result = compareEnabled(configs, ['env-1'])
    expect(result.status).toBe('match')
  })
})

describe('compareDefaultVariant', () => {
  it('returns match when all variants are the same', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', default_variant: 'on' }),
      makeConfig({ environment_id: 'env-2', default_variant: 'on' }),
    ]
    const result = compareDefaultVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('returns differs when variants differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', default_variant: 'on' }),
      makeConfig({ environment_id: 'env-2', default_variant: 'off' }),
    ]
    const result = compareDefaultVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('treats missing config as empty string', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', default_variant: 'on' }),
    ]
    const result = compareDefaultVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.values.get('env-2')).toBe('')
  })
})
```

Update the import line at the top of the file:
```typescript
import { canonicalize, compareEnabled, compareDefaultVariant } from '../flag-diff'
```

- [ ] **Step 2: Run tests to verify the new ones fail**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: New tests FAIL — `compareEnabled` and `compareDefaultVariant` are not exported.

- [ ] **Step 3: Implement compareEnabled and compareDefaultVariant**

Add to `web/src/lib/flag-diff.ts`:
```typescript
import type { FlagEnvironmentConfig } from '@/api/types'

export type DiffStatus = 'match' | 'differs'

export type FieldDiff = {
  status: DiffStatus
  values: Map<string, unknown>
}

function getConfig(configs: FlagEnvironmentConfig[], envId: string): FlagEnvironmentConfig | null {
  return configs.find((c) => c.environment_id === envId) ?? null
}

export function compareEnabled(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    values.set(envId, config?.enabled ?? false)
  }
  const allValues = [...values.values()]
  const status: DiffStatus = allValues.every((v) => v === allValues[0]) ? 'match' : 'differs'
  return { status, values }
}

export function compareDefaultVariant(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    values.set(envId, config?.default_variant ?? '')
  }
  const allValues = [...values.values()]
  const status: DiffStatus = allValues.every((v) => v === allValues[0]) ? 'match' : 'differs'
  return { status, values }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git -C .worktrees/env-compare add web/src/lib/flag-diff.ts web/src/lib/__tests__/flag-diff.test.ts
git -C .worktrees/env-compare commit -m "feat: add compareEnabled and compareDefaultVariant diff functions"
```

---

### Task 4: Write failing tests for compareVariants

**Files:**
- Modify: `web/src/lib/__tests__/flag-diff.test.ts`
- Modify: `web/src/lib/flag-diff.ts`

- [ ] **Step 1: Write the failing tests**

Append to test file and update imports:
```typescript
import { canonicalize, compareEnabled, compareDefaultVariant, compareVariants } from '../flag-diff'
import type { VariantDiff } from '../flag-diff'
```

```typescript
describe('compareVariants', () => {
  it('returns match when all environments have identical variants', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        variants: [{ key: 'on', value: true }, { key: 'off', value: false }],
      }),
      makeConfig({
        environment_id: 'env-2',
        variants: [{ key: 'on', value: true }, { key: 'off', value: false }],
      }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('returns differs when variant values differ', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        variants: [{ key: 'on', value: true }],
      }),
      makeConfig({
        environment_id: 'env-2',
        variants: [{ key: 'on', value: false }],
      }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.perVariant.get('on')?.status).toBe('differs')
  })

  it('returns differs when variant sets differ', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        variants: [{ key: 'on', value: true }, { key: 'off', value: false }],
      }),
      makeConfig({
        environment_id: 'env-2',
        variants: [{ key: 'on', value: true }],
      }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.perVariant.get('off')?.status).toBe('differs')
    expect(result.perVariant.get('off')?.values.get('env-2')).toBeUndefined()
  })

  it('handles missing config as empty variants', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        variants: [{ key: 'on', value: true }],
      }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('compares variant values independent of property order', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        variants: [{ key: 'config', value: { theme: 'dark', size: 'large' } }],
      }),
      makeConfig({
        environment_id: 'env-2',
        variants: [{ key: 'config', value: { size: 'large', theme: 'dark' } }],
      }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })
})
```

- [ ] **Step 2: Run tests to verify new ones fail**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: FAIL — `compareVariants` is not exported.

- [ ] **Step 3: Implement compareVariants**

Add to `web/src/lib/flag-diff.ts`:
```typescript
export type VariantDiff = {
  status: DiffStatus
  perVariant: Map<string, FieldDiff>
}

export function compareVariants(configs: FlagEnvironmentConfig[], environmentIds: string[]): VariantDiff {
  // Collect all variant keys across all environments
  const allKeys = new Set<string>()
  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    for (const v of config?.variants ?? []) {
      allKeys.add(v.key)
    }
  }

  const perVariant = new Map<string, FieldDiff>()
  let overallDiffers = false

  for (const variantKey of allKeys) {
    const values = new Map<string, unknown>()
    const serialized: string[] = []

    for (const envId of environmentIds) {
      const config = getConfig(configs, envId)
      const variant = config?.variants?.find((v) => v.key === variantKey)
      if (variant) {
        values.set(envId, variant.value)
        serialized.push(canonicalize(variant.value))
      } else {
        // Variant doesn't exist in this environment — leave it out of the map
        serialized.push('__MISSING__')
      }
    }

    const allSame = serialized.every((s) => s === serialized[0])
    const status: DiffStatus = allSame ? 'match' : 'differs'
    if (!allSame) overallDiffers = true
    perVariant.set(variantKey, { status, values })
  }

  return {
    status: overallDiffers ? 'differs' : 'match',
    perVariant,
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git -C .worktrees/env-compare add web/src/lib/flag-diff.ts web/src/lib/__tests__/flag-diff.test.ts
git -C .worktrees/env-compare commit -m "feat: add compareVariants diff function with per-variant detail"
```

---

### Task 5: Write failing tests for compareRules and compareFlag (top-level)

**Files:**
- Modify: `web/src/lib/__tests__/flag-diff.test.ts`
- Modify: `web/src/lib/flag-diff.ts`

- [ ] **Step 1: Write the failing tests**

Update imports:
```typescript
import {
  canonicalize,
  compareEnabled,
  compareDefaultVariant,
  compareVariants,
  compareRules,
  compareFlag,
} from '../flag-diff'
import type { VariantDiff, ComparisonResult } from '../flag-diff'
```

Append tests:
```typescript
describe('compareRules', () => {
  it('returns match when all environments have identical rules', () => {
    const rule = {
      conditions: [{ attribute: 'country', operator: 'equals', value: 'US' }],
      variant: 'on',
      percentage_rollout: 100,
    }
    const configs = [
      makeConfig({ environment_id: 'env-1', targeting_rules: [rule] }),
      makeConfig({ environment_id: 'env-2', targeting_rules: [rule] }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('returns differs when rule counts differ', () => {
    const rule = {
      conditions: [{ attribute: 'country', operator: 'equals', value: 'US' }],
      variant: 'on',
    }
    const configs = [
      makeConfig({ environment_id: 'env-1', targeting_rules: [rule] }),
      makeConfig({ environment_id: 'env-2', targeting_rules: [] }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('returns differs when rule content differs', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        targeting_rules: [{ conditions: [{ attribute: 'plan', operator: 'equals', value: 'pro' }], variant: 'on' }],
      }),
      makeConfig({
        environment_id: 'env-2',
        targeting_rules: [{ conditions: [{ attribute: 'plan', operator: 'equals', value: 'enterprise' }], variant: 'on' }],
      }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('compares rules independent of condition property order', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        targeting_rules: [{ conditions: [{ attribute: 'x', operator: 'eq', value: '1' }], variant: 'on' }],
      }),
      makeConfig({
        environment_id: 'env-2',
        targeting_rules: [{ conditions: [{ value: '1', operator: 'eq', attribute: 'x' }], variant: 'on' }],
      }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('treats missing config as no rules', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        targeting_rules: [{ conditions: [], variant: 'on' }],
      }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.values.get('env-2')).toEqual([])
  })
})

describe('compareFlag', () => {
  it('returns a full ComparisonResult', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        enabled: true,
        default_variant: 'on',
        variants: [{ key: 'on', value: true }],
        targeting_rules: [],
      }),
      makeConfig({
        environment_id: 'env-2',
        enabled: false,
        default_variant: 'on',
        variants: [{ key: 'on', value: true }],
        targeting_rules: [],
      }),
    ]
    const result = compareFlag(configs, ['env-1', 'env-2'])
    expect(result.enabled.status).toBe('differs')
    expect(result.defaultVariant.status).toBe('match')
    expect(result.variants.status).toBe('match')
    expect(result.rules.status).toBe('match')
  })

  it('handles empty configs array', () => {
    const result = compareFlag([], ['env-1', 'env-2'])
    expect(result.enabled.status).toBe('match') // both default to false
    expect(result.defaultVariant.status).toBe('match') // both default to ''
  })
})
```

- [ ] **Step 2: Run tests to verify new ones fail**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: FAIL — `compareRules` and `compareFlag` not exported.

- [ ] **Step 3: Implement compareRules and compareFlag**

Add to `web/src/lib/flag-diff.ts`:
```typescript
export type ComparisonResult = {
  enabled: FieldDiff
  defaultVariant: FieldDiff
  variants: VariantDiff
  rules: FieldDiff
}

export function compareRules(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  const serialized: string[] = []

  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    const rules = config?.targeting_rules ?? []
    values.set(envId, rules)
    serialized.push(canonicalize(rules))
  }

  const status: DiffStatus = serialized.every((s) => s === serialized[0]) ? 'match' : 'differs'
  return { status, values }
}

export function compareFlag(configs: FlagEnvironmentConfig[], environmentIds: string[]): ComparisonResult {
  return {
    enabled: compareEnabled(configs, environmentIds),
    defaultVariant: compareDefaultVariant(configs, environmentIds),
    variants: compareVariants(configs, environmentIds),
    rules: compareRules(configs, environmentIds),
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd .worktrees/env-compare/web && npx vitest run src/lib/__tests__/flag-diff.test.ts
```

Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git -C .worktrees/env-compare add web/src/lib/flag-diff.ts web/src/lib/__tests__/flag-diff.test.ts
git -C .worktrees/env-compare commit -m "feat: add compareRules, compareFlag and complete diff logic (#46)"
```

---

## Chunk 2: CompareTab Component + FlagDetailPage Integration

### Task 6: Create CompareTab component with basic grid

**Files:**
- Create: `web/src/components/CompareTab.tsx`

- [ ] **Step 1: Create the CompareTab component**

Create `web/src/components/CompareTab.tsx`:
```tsx
import { useState } from 'react'
import { cn } from '@/lib/utils'
import { compareFlag } from '@/lib/flag-diff'
import type { ComparisonResult, FieldDiff, VariantDiff } from '@/lib/flag-diff'
import type { Environment, FlagEnvironmentConfig, TargetingRule } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ChevronRight } from 'lucide-react'

interface CompareTabProps {
  environments: Environment[]
  environmentConfigs: FlagEnvironmentConfig[]
}

function DiffBadge({ children, differs }: { children: React.ReactNode; differs: boolean }) {
  if (!differs) {
    return <span className="text-[13px] text-foreground">{children}</span>
  }
  return (
    <Badge variant="outline" className="bg-amber-950/50 text-amber-400 border-amber-800 font-mono text-xs">
      {children}
    </Badge>
  )
}

function EnabledCell({ value, differs }: { value: boolean; differs: boolean }) {
  if (differs) {
    return (
      <Badge
        variant="outline"
        className={cn(
          'font-mono text-xs',
          value
            ? 'bg-green-950/50 text-green-400 border-green-800'
            : 'bg-red-950/50 text-red-400 border-red-800'
        )}
      >
        {value ? 'ON' : 'OFF'}
      </Badge>
    )
  }
  return (
    <span className={cn('text-[13px]', value ? 'text-green-400' : 'text-red-400')}>
      {value ? 'ON' : 'OFF'}
    </span>
  )
}

function NotConfigured() {
  return <span className="text-muted-foreground/50 text-xs italic">Not configured</span>
}

function RuleCard({ rule, index }: { rule: TargetingRule; index: number }) {
  return (
    <div className="rounded border border-border/50 bg-card/50 p-2 border-l-2 border-l-amber-600/60">
      <div className="text-[10px] text-amber-600/80 font-mono mb-1">Rule {index + 1}</div>
      {rule.conditions.map((c, i) => (
        <div key={i} className="text-xs text-foreground">
          <span className="text-foreground">{c.attribute}</span>{' '}
          <span className="text-muted-foreground">{c.operator}</span>{' '}
          <span className="text-blue-400">{typeof c.value === 'string' ? c.value : JSON.stringify(c.value)}</span>
        </div>
      ))}
      <div className="text-[10px] text-muted-foreground mt-1">
        → {rule.variant}
        {rule.percentage_rollout != null && ` (${rule.percentage_rollout}%)`}
      </div>
    </div>
  )
}

export default function CompareTab({ environments, environmentConfigs }: CompareTabProps) {
  const [showDiffsOnly, setShowDiffsOnly] = useState(false)
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())

  const sortedEnvs = [...environments].sort((a, b) => a.sort_order - b.sort_order)
  const envIds = sortedEnvs.map((e) => e.id)
  const comparison = compareFlag(environmentConfigs, envIds)

  const toggleRow = (row: string) => {
    setExpandedRows((prev) => {
      const next = new Set(prev)
      if (next.has(row)) next.delete(row)
      else next.add(row)
      return next
    })
  }

  const getConfig = (envId: string) =>
    environmentConfigs.find((c) => c.environment_id === envId) ?? null

  const allMatch =
    comparison.enabled.status === 'match' &&
    comparison.defaultVariant.status === 'match' &&
    comparison.variants.status === 'match' &&
    comparison.rules.status === 'match'

  const gridCols = `160px repeat(${sortedEnvs.length}, minmax(0, 1fr))`

  function shouldShow(field: FieldDiff | VariantDiff) {
    return !showDiffsOnly || field.status === 'differs'
  }

  return (
    <div className="space-y-4">
      {/* Differences toggle */}
      <div className="flex items-center gap-2">
        <Switch
          id="diff-toggle"
          checked={showDiffsOnly}
          onCheckedChange={setShowDiffsOnly}
        />
        <Label htmlFor="diff-toggle" className="text-xs text-muted-foreground cursor-pointer">
          Show differences only
        </Label>
      </div>

      {showDiffsOnly && allMatch ? (
        <div className="py-8 text-center text-muted-foreground/60 text-[13px]">
          All environments have identical configuration.
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-x-auto">
          <div className="min-w-[500px]">
            {/* Header row */}
            <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
              <div className="bg-card p-3 text-xs font-mono text-muted-foreground uppercase tracking-wider" />
              {sortedEnvs.map((env) => (
                <div key={env.id} className="bg-card p-3 text-xs font-medium text-foreground">
                  {env.name}
                </div>
              ))}
            </div>

            {/* Enabled row */}
            {shouldShow(comparison.enabled) && (
              <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                <div className="bg-background p-3 text-xs text-muted-foreground">Enabled</div>
                {sortedEnvs.map((env) => {
                  const config = getConfig(env.id)
                  if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                  return (
                    <div key={env.id} className="bg-background p-3">
                      <EnabledCell
                        value={comparison.enabled.values.get(env.id) as boolean}
                        differs={comparison.enabled.status === 'differs'}
                      />
                    </div>
                  )
                })}
              </div>
            )}

            {/* Default variant row */}
            {shouldShow(comparison.defaultVariant) && (
              <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                <div className="bg-background p-3 text-xs text-muted-foreground">Default variant</div>
                {sortedEnvs.map((env) => {
                  const config = getConfig(env.id)
                  if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                  const value = comparison.defaultVariant.values.get(env.id) as string
                  return (
                    <div key={env.id} className="bg-background p-3">
                      <DiffBadge differs={comparison.defaultVariant.status === 'differs'}>
                        {value || '—'}
                      </DiffBadge>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Variants row (expandable) */}
            {shouldShow(comparison.variants) && (
              <Collapsible open={expandedRows.has('variants')} onOpenChange={() => toggleRow('variants')}>
                <CollapsibleTrigger asChild>
                  <div className="grid gap-px bg-border cursor-pointer hover:bg-muted/20" style={{ gridTemplateColumns: gridCols }}>
                    <div className="bg-background p-3 text-xs text-muted-foreground flex items-center gap-1">
                      <ChevronRight className={cn('h-3 w-3 transition-transform', expandedRows.has('variants') && 'rotate-90')} />
                      Variants
                    </div>
                    {sortedEnvs.map((env) => {
                      const config = getConfig(env.id)
                      if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                      const count = config.variants.length
                      return (
                        <div key={env.id} className="bg-background p-3">
                          <DiffBadge differs={comparison.variants.status === 'differs'}>
                            {count} variant{count !== 1 ? 's' : ''}
                          </DiffBadge>
                        </div>
                      )
                    })}
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="border-t border-amber-800/30">
                    {[...comparison.variants.perVariant.entries()].map(([variantKey, diff]) => (
                      <div key={variantKey} className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                        <div className="bg-muted/30 p-3 pl-8 text-xs text-muted-foreground font-mono">{variantKey}</div>
                        {sortedEnvs.map((env) => {
                          const value = diff.values.get(env.id)
                          return (
                            <div key={env.id} className="bg-muted/30 p-3">
                              {value !== undefined ? (
                                <DiffBadge differs={diff.status === 'differs'}>
                                  <span className="font-mono text-xs">{typeof value === 'string' ? value : JSON.stringify(value)}</span>
                                </DiffBadge>
                              ) : (
                                <span className="text-muted-foreground/40 text-xs">—</span>
                              )}
                            </div>
                          )
                        })}
                      </div>
                    ))}
                  </div>
                </CollapsibleContent>
              </Collapsible>
            )}

            {/* Rules row (expandable) */}
            {shouldShow(comparison.rules) && (
              <Collapsible open={expandedRows.has('rules')} onOpenChange={() => toggleRow('rules')}>
                <CollapsibleTrigger asChild>
                  <div className="grid gap-px bg-border cursor-pointer hover:bg-muted/20" style={{ gridTemplateColumns: gridCols }}>
                    <div className="bg-background p-3 text-xs text-muted-foreground flex items-center gap-1">
                      <ChevronRight className={cn('h-3 w-3 transition-transform', expandedRows.has('rules') && 'rotate-90')} />
                      Targeting rules
                    </div>
                    {sortedEnvs.map((env) => {
                      const config = getConfig(env.id)
                      if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                      const count = config.targeting_rules.length
                      return (
                        <div key={env.id} className="bg-background p-3">
                          <DiffBadge differs={comparison.rules.status === 'differs'}>
                            {count} rule{count !== 1 ? 's' : ''}
                          </DiffBadge>
                        </div>
                      )
                    })}
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="border-t border-amber-800/30">
                    <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                      <div className="bg-muted/30 p-3 text-[10px] text-muted-foreground uppercase">Details</div>
                      {sortedEnvs.map((env) => {
                        const config = getConfig(env.id)
                        const rules = config?.targeting_rules ?? []
                        return (
                          <div key={env.id} className="bg-muted/30 p-3 space-y-2">
                            {rules.length === 0 ? (
                              <span className="text-muted-foreground/40 text-xs italic">No targeting rules</span>
                            ) : (
                              rules.map((rule, i) => <RuleCard key={i} rule={rule} index={i} />)
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                </CollapsibleContent>
              </Collapsible>
            )}

            {/* Lock status row (informational, no diff) */}
            <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
              <div className="bg-background p-3 text-xs text-muted-foreground">Lock status</div>
              {sortedEnvs.map((env) => {
                const config = getConfig(env.id)
                if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                return (
                  <div key={env.id} className="bg-background p-3 text-xs">
                    {config.locked ? (
                      <span className="text-amber-400">Locked{config.lock_reason ? ` — ${config.lock_reason}` : ''}</span>
                    ) : (
                      <span className="text-muted-foreground">Unlocked</span>
                    )}
                  </div>
                )
              })}
            </div>

            {/* Last updated row (informational, no diff) */}
            <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
              <div className="bg-background p-3 text-xs text-muted-foreground">Last updated</div>
              {sortedEnvs.map((env) => {
                const config = getConfig(env.id)
                if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                return (
                  <div key={env.id} className="bg-background p-3 text-xs text-muted-foreground">
                    {config.updated_by_user
                      ? `${config.updated_by_user.display_name ?? config.updated_by_user.email}`
                      : '—'}
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Verify the file compiles**

```bash
cd .worktrees/env-compare/web && npx tsc -b --noEmit 2>&1 | head -20
```

Expected: No type errors. Fix any that appear.

- [ ] **Step 3: Commit**

```bash
git -C .worktrees/env-compare add web/src/components/CompareTab.tsx
git -C .worktrees/env-compare commit -m "feat: add CompareTab component with comparison grid (#46)"
```

---

### Task 7: Integrate CompareTab into FlagDetailPage

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx:1-2,37,383-608`

- [ ] **Step 1: Add imports and URL param sync**

At the top of `web/src/pages/FlagDetailPage.tsx`:

1. Modify the existing react-router-dom import (line 2) to add `useSearchParams`:
```typescript
import { useParams, Link, useNavigate, useSearchParams } from 'react-router-dom'
```

2. Add the CompareTab import (after the other component imports, around line 44):
```typescript
import CompareTab from '../components/CompareTab.tsx'
```

Inside the component function, after the existing `useState` declarations (around line 66), add:
```typescript
const [searchParams, setSearchParams] = useSearchParams()
const activeTab = searchParams.get('tab') ?? 'configuration'
```

- [ ] **Step 2: Modify the Tabs component**

Replace the existing `<Tabs defaultValue="configuration" ...>` block (line 383) with a controlled version:

```tsx
<Tabs value={activeTab} onValueChange={(value) => {
  setSearchParams((prev) => {
    const next = new URLSearchParams(prev)
    if (value === 'configuration') next.delete('tab')
    else next.set('tab', value)
    return next
  }, { replace: true })
}} className="w-full">
  <TabsList className="mb-6">
    <TabsTrigger value="configuration">Configuration</TabsTrigger>
    {environments && environments.length >= 2 && (
      <TabsTrigger value="compare">Compare</TabsTrigger>
    )}
    <TabsTrigger value="history">History</TabsTrigger>
  </TabsList>
```

Note: The Compare tab is only shown when `environments.length >= 2`.

- [ ] **Step 3: Add the Compare TabsContent**

Between the existing `</TabsContent>` for configuration (around line 597) and the `<TabsContent value="history">` (line 599), add:

```tsx
<TabsContent value="compare">
  {environments && environments.length >= 2 && data && (
    <CompareTab
      environments={environments}
      environmentConfigs={data.environment_configs}
    />
  )}
</TabsContent>
```

- [ ] **Step 4: Verify the page compiles**

```bash
cd .worktrees/env-compare/web && npx tsc -b --noEmit 2>&1 | head -20
```

Expected: No type errors.

- [ ] **Step 5: Commit**

```bash
git -C .worktrees/env-compare add web/src/pages/FlagDetailPage.tsx
git -C .worktrees/env-compare commit -m "feat: integrate Compare tab into flag detail page (#46)"
```

---

### Task 8: Manual smoke test and lint check

- [ ] **Step 1: Run lint**

```bash
cd .worktrees/env-compare/web && npm run lint
```

Expected: No new lint errors. Fix any that appear.

- [ ] **Step 2: Run all diff tests**

```bash
cd .worktrees/env-compare/web && npm test
```

Expected: All tests pass.

- [ ] **Step 3: Manual smoke test**

Start the dev environment:
```bash
cd .worktrees/env-compare && ./dev.sh
```

In another terminal:
```bash
cd .worktrees/env-compare/web && npm install && npm run dev
```

Verify:
1. Navigate to any flag detail page
2. See three tabs: Configuration, Compare, History
3. Click "Compare" tab — see the comparison grid
4. Toggle "Show differences only" — grid filters correctly
5. Click Variants/Rules rows — expandable panels work
6. URL updates to `?tab=compare` when on Compare tab
7. Refreshing with `?tab=compare` opens the Compare tab directly
8. For a project with only 1 environment, the Compare tab does not appear

- [ ] **Step 4: Fix any issues found**

If any issues are discovered, fix and commit individually.

- [ ] **Step 5: Final commit (if any fixes)**

```bash
cd .worktrees/env-compare && ./dev.sh --down
```

---

## Follow-ups (out of scope for this PR)

- **Component tests for CompareTab**: The spec mentions component tests with mock data, but adding `@testing-library/react` + `jsdom` is a larger change. Tracked as part of #98 (frontend test automation). The diff logic has full unit test coverage which validates the core functionality.
- **CI integration**: The web package now has `npm test`, but CI (`lint-frontend` job) only runs `npm run lint`. Adding `npm test` to CI should be done as part of #98.
- **Multi-flag environment comparison (B)**: Future scope — comparing all flags between two environments to find drift across the project.
