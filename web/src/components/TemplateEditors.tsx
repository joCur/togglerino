import { useState } from 'react'
import type { Variant, TargetingRule, Condition, ValueType } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { X, Plus } from 'lucide-react'

const labelClass = 'font-mono text-[10px] uppercase tracking-wider text-muted-foreground'

// --- Environment Defaults Editor ---

export interface EnvDefault {
  envKey: string
  enabled: boolean
}

interface EnvironmentDefaultsEditorProps {
  value: EnvDefault[]
  onChange: (value: EnvDefault[]) => void
}

export function EnvironmentDefaultsEditor({ value, onChange }: EnvironmentDefaultsEditorProps) {
  const [newEnvKey, setNewEnvKey] = useState('')

  const addEnv = () => {
    const key = newEnvKey.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-')
    if (!key || value.some((e) => e.envKey === key)) return
    onChange([...value, { envKey: key, enabled: false }])
    setNewEnvKey('')
  }

  const removeEnv = (envKey: string) => {
    onChange(value.filter((e) => e.envKey !== envKey))
  }

  const toggleEnv = (envKey: string) => {
    onChange(value.map((e) => (e.envKey === envKey ? { ...e, enabled: !e.enabled } : e)))
  }

  return (
    <div className="flex flex-col gap-2">
      <Label className={labelClass}>Environment Defaults</Label>
      <p className="text-[10px] text-muted-foreground/60 -mt-1">
        Set which environments should be enabled or disabled by default when a flag is created from this template.
      </p>

      {value.length > 0 && (
        <div className="rounded-md border divide-y">
          {value.map((env) => (
            <div key={env.envKey} className="flex items-center justify-between px-3 py-2">
              <span className="font-mono text-xs text-foreground">{env.envKey}</span>
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-2">
                  <span className="text-[11px] text-muted-foreground">
                    {env.enabled ? 'Enabled' : 'Disabled'}
                  </span>
                  <Switch
                    checked={env.enabled}
                    onCheckedChange={() => toggleEnv(env.envKey)}
                  />
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                  onClick={() => removeEnv(env.envKey)}
                >
                  <X className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="flex gap-2">
        <Input
          value={newEnvKey}
          onChange={(e) => setNewEnvKey(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              addEnv()
            }
          }}
          placeholder="e.g. production"
          className="font-mono text-xs flex-1"
        />
        <Button type="button" variant="outline" size="sm" onClick={addEnv} disabled={!newEnvKey.trim()}>
          <Plus className="h-3.5 w-3.5 mr-1" />
          Add
        </Button>
      </div>
    </div>
  )
}

// --- Variant Config Editor ---

export interface VariantConfigState {
  variants: Variant[]
  defaultVariant: string
  targetingRules: TargetingRule[]
}

interface VariantConfigEditorProps {
  value: VariantConfigState
  onChange: (value: VariantConfigState) => void
  valueType: ValueType
}

const CONDITION_OPERATORS = [
  'equals', 'not_equals', 'contains', 'not_contains',
  'starts_with', 'ends_with', 'in', 'not_in',
  'greater_than', 'less_than', 'gte', 'lte',
  'exists', 'not_exists', 'matches', 'segment_match',
]

export function VariantConfigEditor({ value, onChange, valueType }: VariantConfigEditorProps) {
  const { variants, defaultVariant, targetingRules } = value

  const addVariant = () => {
    const newKey = `variant-${variants.length + 1}`
    const newValue = valueType === 'boolean' ? true : valueType === 'number' ? 0 : ''
    onChange({
      ...value,
      variants: [...variants, { name: newKey, value: newValue }],
      defaultVariant: defaultVariant || newKey,
    })
  }

  const removeVariant = (idx: number) => {
    const updated = variants.filter((_, i) => i !== idx)
    const removedKey = variants[idx].name
    onChange({
      ...value,
      variants: updated,
      defaultVariant: defaultVariant === removedKey ? (updated[0]?.name ?? '') : defaultVariant,
      targetingRules: targetingRules.map((r) =>
        r.variant === removedKey ? { ...r, variant: updated[0]?.name ?? '' } : r
      ),
    })
  }

  const updateVariantKey = (idx: number, key: string) => {
    const oldKey = variants[idx].name
    const updated = variants.map((v, i) => (i === idx ? { ...v, name: key } : v))
    onChange({
      ...value,
      variants: updated,
      defaultVariant: defaultVariant === oldKey ? key : defaultVariant,
      targetingRules: targetingRules.map((r) =>
        r.variant === oldKey ? { ...r, variant: key } : r
      ),
    })
  }

  const updateVariantValue = (idx: number, rawValue: string) => {
    let parsed: unknown = rawValue
    if (valueType === 'boolean') parsed = rawValue === 'true'
    else if (valueType === 'number') parsed = Number(rawValue) || 0
    else if (valueType === 'json') {
      try { parsed = JSON.parse(rawValue) } catch { parsed = rawValue }
    }
    const updated = variants.map((v, i) => (i === idx ? { ...v, value: parsed } : v))
    onChange({ ...value, variants: updated })
  }

  const addRule = () => {
    onChange({
      ...value,
      targetingRules: [
        ...targetingRules,
        {
          conditions: [{ attribute: '', operator: 'equals', value: '' }],
          variant: variants[0]?.name ?? '',
          percentage_rollout: undefined,
        },
      ],
    })
  }

  const removeRule = (idx: number) => {
    onChange({
      ...value,
      targetingRules: targetingRules.filter((_, i) => i !== idx),
    })
  }

  const updateRule = (idx: number, updates: Partial<TargetingRule>) => {
    onChange({
      ...value,
      targetingRules: targetingRules.map((r, i) => (i === idx ? { ...r, ...updates } : r)),
    })
  }

  const addCondition = (ruleIdx: number) => {
    const rule = targetingRules[ruleIdx]
    updateRule(ruleIdx, {
      conditions: [...rule.conditions, { attribute: '', operator: 'equals', value: '' }],
    })
  }

  const removeCondition = (ruleIdx: number, condIdx: number) => {
    const rule = targetingRules[ruleIdx]
    updateRule(ruleIdx, {
      conditions: rule.conditions.filter((_, i) => i !== condIdx),
    })
  }

  const updateCondition = (ruleIdx: number, condIdx: number, updates: Partial<Condition>) => {
    const rule = targetingRules[ruleIdx]
    updateRule(ruleIdx, {
      conditions: rule.conditions.map((c, i) => (i === condIdx ? { ...c, ...updates } : c)),
    })
  }

  const formatVariantValue = (v: unknown): string => {
    if (typeof v === 'object' && v !== null) return JSON.stringify(v)
    return String(v ?? '')
  }

  return (
    <div className="flex flex-col gap-3">
      <Label className={labelClass}>Variants</Label>
      <p className="text-[10px] text-muted-foreground/60 -mt-2">
        Define pre-configured variants for this template. Each variant has a key and a value.
      </p>

      {variants.length > 0 && (
        <div className="rounded-md border divide-y">
          {variants.map((variant, idx) => (
            <div key={idx} className="flex items-center gap-2 px-3 py-2">
              <Input
                value={variant.name}
                onChange={(e) => updateVariantKey(idx, e.target.value)}
                placeholder="Key"
                className="font-mono text-xs flex-1 h-8"
              />
              {valueType === 'boolean' ? (
                <Select
                  value={String(variant.value)}
                  onValueChange={(v) => updateVariantValue(idx, v)}
                >
                  <SelectTrigger className="w-[100px] h-8 text-xs">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">true</SelectItem>
                    <SelectItem value="false">false</SelectItem>
                  </SelectContent>
                </Select>
              ) : (
                <Input
                  value={formatVariantValue(variant.value)}
                  onChange={(e) => updateVariantValue(idx, e.target.value)}
                  placeholder="Value"
                  className={`text-xs flex-1 h-8 ${valueType === 'json' || valueType === 'number' ? 'font-mono' : ''}`}
                />
              )}
              {defaultVariant === variant.name ? (
                <span className="text-[10px] text-[#d4956a] font-mono whitespace-nowrap px-1">default</span>
              ) : (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 text-[10px] text-muted-foreground hover:text-foreground px-1.5"
                  onClick={() => onChange({ ...value, defaultVariant: variant.name })}
                >
                  set default
                </Button>
              )}
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive shrink-0"
                onClick={() => removeVariant(idx)}
              >
                <X className="h-3.5 w-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}

      <Button type="button" variant="outline" size="sm" className="w-fit" onClick={addVariant}>
        <Plus className="h-3.5 w-3.5 mr-1" />
        Add Variant
      </Button>

      {/* Targeting Rules */}
      <Label className={`${labelClass} mt-2`}>Targeting Rules</Label>
      <p className="text-[10px] text-muted-foreground/60 -mt-2">
        Optional rules to pre-configure targeting when creating a flag from this template.
      </p>

      {targetingRules.map((rule, ruleIdx) => (
        <div key={ruleIdx} className="rounded-md border p-3 flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <span className="text-[11px] font-medium text-muted-foreground">Rule {ruleIdx + 1}</span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
              onClick={() => removeRule(ruleIdx)}
            >
              <X className="h-3.5 w-3.5" />
            </Button>
          </div>

          {/* Conditions */}
          {rule.conditions.map((cond, condIdx) => (
            <div key={condIdx} className="flex items-center gap-1.5">
              {condIdx > 0 && (
                <span className="text-[10px] text-muted-foreground font-mono w-7 text-center shrink-0">AND</span>
              )}
              <Input
                value={cond.attribute}
                onChange={(e) => updateCondition(ruleIdx, condIdx, { attribute: e.target.value })}
                placeholder="attribute"
                className="font-mono text-xs h-7 flex-1"
              />
              <Select
                value={cond.operator}
                onValueChange={(v) => updateCondition(ruleIdx, condIdx, { operator: v })}
              >
                <SelectTrigger className="w-[130px] h-7 text-xs font-mono">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CONDITION_OPERATORS.map((op) => (
                    <SelectItem key={op} value={op} className="font-mono text-xs">
                      {op}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {!['exists', 'not_exists'].includes(cond.operator) && (
                <Input
                  value={typeof cond.value === 'string' ? cond.value : JSON.stringify(cond.value)}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { value: e.target.value })}
                  placeholder="value"
                  className="font-mono text-xs h-7 flex-1"
                />
              )}
              {rule.conditions.length > 1 && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive shrink-0"
                  onClick={() => removeCondition(ruleIdx, condIdx)}
                >
                  <X className="h-3 w-3" />
                </Button>
              )}
            </div>
          ))}

          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="w-fit text-[11px] h-6 text-muted-foreground"
            onClick={() => addCondition(ruleIdx)}
          >
            <Plus className="h-3 w-3 mr-1" />
            Add Condition
          </Button>

          {/* Serve variant + rollout */}
          <div className="flex items-center gap-2 mt-1 pt-2 border-t">
            <span className="text-[11px] text-muted-foreground whitespace-nowrap">Serve</span>
            <Select
              value={rule.variant}
              onValueChange={(v) => updateRule(ruleIdx, { variant: v })}
            >
              <SelectTrigger className="w-[140px] h-7 text-xs font-mono">
                <SelectValue placeholder="variant" />
              </SelectTrigger>
              <SelectContent>
                {variants.map((v) => (
                  <SelectItem key={v.name} value={v.name} className="font-mono text-xs">
                    {v.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span className="text-[11px] text-muted-foreground whitespace-nowrap">at</span>
            <Input
              type="number"
              min={0}
              max={100}
              value={rule.percentage_rollout ?? 100}
              onChange={(e) =>
                updateRule(ruleIdx, {
                  percentage_rollout: e.target.value === '' ? undefined : Number(e.target.value),
                })
              }
              className="w-[70px] h-7 text-xs font-mono"
            />
            <span className="text-[11px] text-muted-foreground">%</span>
          </div>
        </div>
      ))}

      {variants.length > 0 && (
        <Button type="button" variant="outline" size="sm" className="w-fit" onClick={addRule}>
          <Plus className="h-3.5 w-3.5 mr-1" />
          Add Rule
        </Button>
      )}
    </div>
  )
}

