import type { TargetingRule, Variant, Condition } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import AttributeCombobox from './AttributeCombobox.tsx'
import RolloutSlider from './RolloutSlider.tsx'

interface Props {
  rules: TargetingRule[]
  variants: Variant[]
  onChange: (rules: TargetingRule[]) => void
}

const OPERATOR_GROUPS = [
  {
    label: 'Comparison',
    operators: [
      { value: 'equals', label: 'equals' },
      { value: 'not_equals', label: 'not equals' },
    ],
  },
  {
    label: 'String',
    operators: [
      { value: 'contains', label: 'contains' },
      { value: 'not_contains', label: 'not contains' },
      { value: 'starts_with', label: 'starts with' },
      { value: 'ends_with', label: 'ends with' },
    ],
  },
  {
    label: 'List',
    operators: [
      { value: 'in', label: 'in (comma-separated)' },
      { value: 'not_in', label: 'not in (comma-separated)' },
    ],
  },
  {
    label: 'Numeric',
    operators: [
      { value: 'greater_than', label: '> greater than' },
      { value: 'less_than', label: '< less than' },
      { value: 'gte', label: '>= greater or equal' },
      { value: 'lte', label: '<= less or equal' },
    ],
  },
  {
    label: 'Presence',
    operators: [
      { value: 'exists', label: 'exists' },
      { value: 'not_exists', label: 'not exists' },
    ],
  },
  {
    label: 'Pattern',
    operators: [
      { value: 'matches', label: 'matches (regex)' },
    ],
  },
]

export default function RuleBuilder({ rules, variants, onChange }: Props) {
  const updateRule = (index: number, patch: Partial<TargetingRule>) => {
    const updated = [...rules]
    updated[index] = { ...updated[index], ...patch }
    onChange(updated)
  }

  const removeRule = (index: number) => {
    onChange(rules.filter((_, i) => i !== index))
  }

  const addRule = () => {
    onChange([
      ...rules,
      {
        conditions: [{ attribute: '', operator: 'equals', value: '' }],
        variant: variants.length > 0 ? variants[0].key : '',
        percentage_rollout: undefined,
      },
    ])
  }

  const updateCondition = (ruleIdx: number, condIdx: number, patch: Partial<Condition>) => {
    const updated = [...rules]
    const conditions = [...updated[ruleIdx].conditions]
    conditions[condIdx] = { ...conditions[condIdx], ...patch }
    updated[ruleIdx] = { ...updated[ruleIdx], conditions }
    onChange(updated)
  }

  const removeCondition = (ruleIdx: number, condIdx: number) => {
    const updated = [...rules]
    updated[ruleIdx] = {
      ...updated[ruleIdx],
      conditions: updated[ruleIdx].conditions.filter((_, i) => i !== condIdx),
    }
    onChange(updated)
  }

  const addCondition = (ruleIdx: number) => {
    const updated = [...rules]
    updated[ruleIdx] = {
      ...updated[ruleIdx],
      conditions: [...updated[ruleIdx].conditions, { attribute: '', operator: 'equals', value: '' }],
    }
    onChange(updated)
  }

  return (
    <div className="flex flex-col gap-3.5">
      <div className="text-[13px] font-medium text-foreground">Targeting Rules</div>
      <div className="text-xs text-muted-foreground/60 leading-relaxed mb-1">
        Rules are evaluated top to bottom — the first matching rule wins. If no rule matches, the default variant is served.
      </div>

      {rules.length === 0 && (
        <div className="text-xs text-muted-foreground/60 italic">
          No targeting rules. All users will receive the default variant.
        </div>
      )}

      {rules.map((rule, ruleIdx) => (
        <div
          key={ruleIdx}
          className="p-4 rounded-md bg-secondary/50 border"
        >
          <div className="flex justify-between items-center mb-3">
            <span className="text-xs font-medium text-muted-foreground font-mono">
              Rule {ruleIdx + 1}
            </span>
            <Button
              variant="destructive"
              size="sm"
              className="text-[11px] px-2.5 h-6"
              onClick={() => removeRule(ruleIdx)}
            >
              Remove
            </Button>
          </div>

          {/* Conditions */}
          <div className="mb-3">
            <div className="text-[11px] text-muted-foreground/60 mb-1.5 font-mono uppercase tracking-wide">
              Conditions
            </div>
            <div className="text-[11px] text-muted-foreground/60 leading-relaxed mb-1.5 italic">
              All conditions must match (AND logic). Attributes are properties from your SDK's evaluation context.
            </div>
            {rule.conditions.map((cond, condIdx) => (
              <div key={condIdx} className="flex items-center gap-1.5 mb-1.5">
                <AttributeCombobox
                  value={cond.attribute}
                  onChange={(val) => updateCondition(ruleIdx, condIdx, { attribute: val })}
                />
                <select
                  className="w-[170px] px-2.5 py-1.5 text-xs border rounded-md bg-input text-foreground outline-none cursor-pointer"
                  value={cond.operator}
                  onChange={(e) => {
                    const op = e.target.value
                    const patch: Partial<Condition> = { operator: op }
                    if (op === 'exists' || op === 'not_exists') {
                      patch.value = ''
                    }
                    updateCondition(ruleIdx, condIdx, patch)
                  }}
                >
                  {OPERATOR_GROUPS.map((group) => (
                    <optgroup key={group.label} label={group.label}>
                      {group.operators.map((op) => (
                        <option key={op.value} value={op.value}>{op.label}</option>
                      ))}
                    </optgroup>
                  ))}
                </select>
                {cond.operator !== 'exists' && cond.operator !== 'not_exists' && (
                  <Input
                    className="flex-1 text-xs"
                    placeholder={
                      cond.operator === 'in' || cond.operator === 'not_in'
                        ? 'comma-separated values'
                        : 'Value'
                    }
                    value={String(cond.value ?? '')}
                    onChange={(e) => updateCondition(ruleIdx, condIdx, { value: e.target.value })}
                  />
                )}
                {rule.conditions.length > 1 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="shrink-0 text-destructive h-7 px-2 text-[11px]"
                    onClick={() => removeCondition(ruleIdx, condIdx)}
                  >
                    x
                  </Button>
                )}
              </div>
            ))}
            <Button
              variant="outline"
              size="sm"
              className="text-[11px] h-7"
              onClick={() => addCondition(ruleIdx)}
            >
              + Add Condition
            </Button>
          </div>

          {/* Serve variant */}
          <div className="flex flex-col gap-1 mb-3">
            <div className="flex items-center gap-2.5">
              <span className="text-xs text-muted-foreground whitespace-nowrap">Serve variant:</span>
              {variants.length > 0 ? (
                <select
                  className="px-2.5 py-1.5 text-xs border rounded-md bg-input text-foreground outline-none cursor-pointer"
                  value={rule.variant}
                  onChange={(e) => updateRule(ruleIdx, { variant: e.target.value })}
                >
                  {variants.map((v) => (
                    <option key={v.key} value={v.key}>{v.key}</option>
                  ))}
                </select>
              ) : (
                <Input
                  className="flex-none w-[130px] text-xs"
                  placeholder="Variant key"
                  value={rule.variant}
                  onChange={(e) => updateRule(ruleIdx, { variant: e.target.value })}
                />
              )}
            </div>
            <div className="text-[11px] text-muted-foreground/60 italic">
              The variant returned when this rule matches.
            </div>
          </div>

          <RolloutSlider
            value={rule.percentage_rollout}
            onChange={(val) => updateRule(ruleIdx, { percentage_rollout: val })}
          />
        </div>
      ))}

      <Button
        variant="outline"
        size="sm"
        className="self-start"
        onClick={addRule}
      >
        + Add Rule
      </Button>
    </div>
  )
}
