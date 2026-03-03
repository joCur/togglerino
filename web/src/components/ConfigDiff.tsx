import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { Variant, TargetingRule } from '../api/types'

interface ConfigDiffProps {
  oldValue: unknown
  newValue: unknown
  entityType: string
}

interface FlagConfigSnapshot {
  enabled?: boolean
  default_variant?: string
  variants?: Variant[]
  targeting_rules?: TargetingRule[]
}

interface FlagSnapshot {
  name?: string
  description?: string
  tags?: string[]
  flag_type?: string
  lifecycle_status?: string
  owner_id?: string
}

type DiffLine = {
  type: 'added' | 'removed' | 'changed' | 'unchanged'
  label: string
  oldVal?: string
  newVal?: string
}

function formatValue(val: unknown): string {
  if (val === null || val === undefined) return 'null'
  if (typeof val === 'boolean') return val ? 'true' : 'false'
  if (typeof val === 'string') return val
  if (typeof val === 'number') return String(val)
  return JSON.stringify(val)
}

function formatVariant(v: Variant): string {
  return `${v.key} = ${formatValue(v.value)}`
}

function formatRule(rule: TargetingRule, index: number): string {
  const conditions = rule.conditions
    .map((c) => `${c.attribute} ${c.operator} ${formatValue(c.value)}`)
    .join(' AND ')
  const rollout =
    rule.percentage_rollout != null ? ` (${rule.percentage_rollout}%)` : ''
  return `Rule ${index + 1}: ${conditions} → ${rule.variant}${rollout}`
}

function diffFlagConfig(
  oldVal: FlagConfigSnapshot,
  newVal: FlagConfigSnapshot,
): DiffLine[] {
  const lines: DiffLine[] = []

  // Enabled
  if (oldVal.enabled !== newVal.enabled) {
    lines.push({
      type: 'changed',
      label: 'Enabled',
      oldVal: formatValue(oldVal.enabled),
      newVal: formatValue(newVal.enabled),
    })
  }

  // Default variant
  if (oldVal.default_variant !== newVal.default_variant) {
    lines.push({
      type: 'changed',
      label: 'Default variant',
      oldVal: oldVal.default_variant ?? 'none',
      newVal: newVal.default_variant ?? 'none',
    })
  }

  // Variants
  const oldVariants = oldVal.variants ?? []
  const newVariants = newVal.variants ?? []
  const oldVarKeys = new Set(oldVariants.map((v) => v.key))
  const newVarKeys = new Set(newVariants.map((v) => v.key))

  for (const v of newVariants) {
    if (!oldVarKeys.has(v.key)) {
      lines.push({ type: 'added', label: `Variant: ${formatVariant(v)}` })
    }
  }
  for (const v of oldVariants) {
    if (!newVarKeys.has(v.key)) {
      lines.push({ type: 'removed', label: `Variant: ${formatVariant(v)}` })
    }
  }
  // Changed variant values
  for (const nv of newVariants) {
    const ov = oldVariants.find((v) => v.key === nv.key)
    if (ov && JSON.stringify(ov.value) !== JSON.stringify(nv.value)) {
      lines.push({
        type: 'changed',
        label: `Variant "${nv.key}"`,
        oldVal: formatValue(ov.value),
        newVal: formatValue(nv.value),
      })
    }
  }

  // Targeting rules — compare by index since rules are ordered
  const oldRules = oldVal.targeting_rules ?? []
  const newRules = newVal.targeting_rules ?? []
  const maxRules = Math.max(oldRules.length, newRules.length)

  for (let i = 0; i < maxRules; i++) {
    const or_ = oldRules[i]
    const nr = newRules[i]
    if (!or_ && nr) {
      lines.push({ type: 'added', label: formatRule(nr, i) })
    } else if (or_ && !nr) {
      lines.push({ type: 'removed', label: formatRule(or_, i) })
    } else if (or_ && nr && JSON.stringify(or_) !== JSON.stringify(nr)) {
      lines.push({
        type: 'changed',
        label: `Rule ${i + 1}`,
        oldVal: formatRule(or_, i),
        newVal: formatRule(nr, i),
      })
    }
  }

  if (lines.length === 0) {
    lines.push({ type: 'unchanged', label: 'No changes detected' })
  }

  return lines
}

function diffFlag(oldVal: FlagSnapshot, newVal: FlagSnapshot): DiffLine[] {
  const lines: DiffLine[] = []

  if (oldVal.name !== newVal.name) {
    lines.push({
      type: 'changed',
      label: 'Name',
      oldVal: oldVal.name,
      newVal: newVal.name,
    })
  }
  if (oldVal.description !== newVal.description) {
    lines.push({
      type: 'changed',
      label: 'Description',
      oldVal: oldVal.description ?? '',
      newVal: newVal.description ?? '',
    })
  }
  if (JSON.stringify(oldVal.tags) !== JSON.stringify(newVal.tags)) {
    lines.push({
      type: 'changed',
      label: 'Tags',
      oldVal: (oldVal.tags ?? []).join(', ') || 'none',
      newVal: (newVal.tags ?? []).join(', ') || 'none',
    })
  }
  if (oldVal.flag_type !== newVal.flag_type) {
    lines.push({
      type: 'changed',
      label: 'Flag type',
      oldVal: oldVal.flag_type,
      newVal: newVal.flag_type,
    })
  }
  if (oldVal.lifecycle_status !== newVal.lifecycle_status) {
    lines.push({
      type: 'changed',
      label: 'Lifecycle',
      oldVal: oldVal.lifecycle_status,
      newVal: newVal.lifecycle_status,
    })
  }
  if (oldVal.owner_id !== newVal.owner_id) {
    lines.push({
      type: 'changed',
      label: 'Owner',
      oldVal: oldVal.owner_id ?? 'unassigned',
      newVal: newVal.owner_id ?? 'unassigned',
    })
  }

  if (lines.length === 0) {
    lines.push({ type: 'unchanged', label: 'No changes detected' })
  }

  return lines
}

export default function ConfigDiff({
  oldValue,
  newValue,
  entityType,
}: ConfigDiffProps) {
  const oldVal = oldValue as Record<string, unknown>
  const newVal = newValue as Record<string, unknown>

  const lines =
    entityType === 'flag_config'
      ? diffFlagConfig(
          oldVal as FlagConfigSnapshot,
          newVal as FlagConfigSnapshot,
        )
      : diffFlag(oldVal as FlagSnapshot, newVal as FlagSnapshot)

  return (
    <div className="space-y-1.5">
      {lines.map((line, i) => (
        <div key={i} className="flex items-start gap-2 text-[13px]">
          {line.type === 'added' && (
            <>
              <Badge
                variant="outline"
                className="text-[10px] bg-emerald-500/10 text-emerald-400 border-emerald-500/20 shrink-0"
              >
                added
              </Badge>
              <span className="text-emerald-400">{line.label}</span>
            </>
          )}
          {line.type === 'removed' && (
            <>
              <Badge
                variant="outline"
                className="text-[10px] bg-red-500/10 text-red-400 border-red-500/20 shrink-0"
              >
                removed
              </Badge>
              <span className="text-red-400 line-through">{line.label}</span>
            </>
          )}
          {line.type === 'changed' && (
            <>
              <Badge
                variant="outline"
                className="text-[10px] bg-amber-500/10 text-amber-400 border-amber-500/20 shrink-0"
              >
                changed
              </Badge>
              <span className="text-muted-foreground">
                {line.label}:{' '}
                <span className={cn('font-mono text-red-400/80 line-through')}>
                  {line.oldVal}
                </span>
                {' → '}
                <span className={cn('font-mono text-emerald-400')}>
                  {line.newVal}
                </span>
              </span>
            </>
          )}
          {line.type === 'unchanged' && (
            <span className="text-muted-foreground/40 italic">
              {line.label}
            </span>
          )}
        </div>
      ))}
    </div>
  )
}
