import type { TargetingRule, Variant, Condition } from '../api/types.ts'
import { t } from '../theme.ts'
import RolloutSlider from './RolloutSlider.tsx'

interface Props {
  rules: TargetingRule[]
  variants: Variant[]
  onChange: (rules: TargetingRule[]) => void
}

const OPERATORS = [
  { value: 'equals', label: 'equals' },
  { value: 'not_equals', label: 'not equals' },
  { value: 'contains', label: 'contains' },
  { value: 'not_contains', label: 'not contains' },
  { value: 'in', label: 'in' },
  { value: 'not_in', label: 'not in' },
  { value: 'greater_than', label: 'greater than' },
  { value: 'less_than', label: 'less than' },
  { value: 'matches', label: 'matches (regex)' },
]

const inputStyle = {
  padding: '7px 10px',
  fontSize: 12,
  border: `1px solid ${t.border}`,
  borderRadius: t.radiusSm,
  backgroundColor: t.bgInput,
  color: t.textPrimary,
  outline: 'none',
  flex: 1,
  fontFamily: t.fontSans,
  transition: 'border-color 200ms ease',
} as const

const selectStyle = {
  padding: '7px 10px',
  fontSize: 12,
  border: `1px solid ${t.border}`,
  borderRadius: t.radiusSm,
  backgroundColor: t.bgInput,
  color: t.textPrimary,
  outline: 'none',
  cursor: 'pointer',
  fontFamily: t.fontSans,
} as const

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
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{ fontSize: 13, fontWeight: 500, color: t.textPrimary }}>Targeting Rules</div>

      {rules.length === 0 && (
        <div style={{ fontSize: 12, color: t.textMuted, fontStyle: 'italic' }}>
          No targeting rules. All users will receive the default variant.
        </div>
      )}

      {rules.map((rule, ruleIdx) => (
        <div
          key={ruleIdx}
          style={{
            padding: 16,
            borderRadius: t.radiusMd,
            backgroundColor: t.bgElevated,
            border: `1px solid ${t.border}`,
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <span style={{ fontSize: 12, fontWeight: 500, color: t.textSecondary, fontFamily: t.fontMono }}>
              Rule {ruleIdx + 1}
            </span>
            <button
              style={{
                padding: '3px 10px', fontSize: 11, fontWeight: 500,
                border: `1px solid ${t.dangerBorder}`, borderRadius: t.radiusSm,
                backgroundColor: t.dangerSubtle, color: t.danger,
                cursor: 'pointer', fontFamily: t.fontSans, transition: 'all 200ms ease',
              }}
              onClick={() => removeRule(ruleIdx)}
              onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = 'rgba(242,116,116,0.15)' }}
              onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = t.dangerSubtle }}
            >
              Remove
            </button>
          </div>

          {/* Conditions */}
          <div style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 11, color: t.textMuted, marginBottom: 6, fontFamily: t.fontMono, textTransform: 'uppercase', letterSpacing: '0.5px' }}>
              Conditions
            </div>
            {rule.conditions.map((cond, condIdx) => (
              <div key={condIdx} style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 6 }}>
                <input
                  style={inputStyle}
                  placeholder="Attribute"
                  value={cond.attribute}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { attribute: e.target.value })}
                  onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
                  onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
                />
                <select
                  style={{ ...selectStyle, width: 130 }}
                  value={cond.operator}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { operator: e.target.value })}
                >
                  {OPERATORS.map((op) => (
                    <option key={op.value} value={op.value}>{op.label}</option>
                  ))}
                </select>
                <input
                  style={inputStyle}
                  placeholder="Value"
                  value={String(cond.value ?? '')}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { value: e.target.value })}
                  onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
                  onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
                />
                {rule.conditions.length > 1 && (
                  <button
                    style={{
                      padding: '3px 8px', fontSize: 11,
                      border: `1px solid ${t.dangerBorder}`, borderRadius: 4,
                      backgroundColor: 'transparent', color: t.danger,
                      cursor: 'pointer', flexShrink: 0, fontFamily: t.fontSans,
                    }}
                    onClick={() => removeCondition(ruleIdx, condIdx)}
                  >
                    x
                  </button>
                )}
              </div>
            ))}
            <button
              style={{
                padding: '5px 12px', fontSize: 11, fontWeight: 500,
                border: `1px solid ${t.border}`, borderRadius: t.radiusSm,
                backgroundColor: 'transparent', color: t.textSecondary,
                cursor: 'pointer', fontFamily: t.fontSans, transition: 'all 200ms ease',
              }}
              onClick={() => addCondition(ruleIdx)}
              onMouseEnter={(e) => { e.currentTarget.style.borderColor = t.borderHover; e.currentTarget.style.color = t.textPrimary }}
              onMouseLeave={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.color = t.textSecondary }}
            >
              + Add Condition
            </button>
          </div>

          {/* Serve variant */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
            <span style={{ fontSize: 12, color: t.textSecondary, whiteSpace: 'nowrap' }}>Serve variant:</span>
            {variants.length > 0 ? (
              <select
                style={selectStyle}
                value={rule.variant}
                onChange={(e) => updateRule(ruleIdx, { variant: e.target.value })}
              >
                {variants.map((v) => (
                  <option key={v.key} value={v.key}>{v.key}</option>
                ))}
              </select>
            ) : (
              <input
                style={{ ...inputStyle, flex: 'none', width: 130 }}
                placeholder="Variant key"
                value={rule.variant}
                onChange={(e) => updateRule(ruleIdx, { variant: e.target.value })}
                onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
                onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
              />
            )}
          </div>

          <RolloutSlider
            value={rule.percentage_rollout}
            onChange={(val) => updateRule(ruleIdx, { percentage_rollout: val })}
          />
        </div>
      ))}

      <button
        style={{
          padding: '7px 14px', fontSize: 12, fontWeight: 500,
          border: `1px solid ${t.border}`, borderRadius: t.radiusMd,
          backgroundColor: 'transparent', color: t.textSecondary,
          cursor: 'pointer', alignSelf: 'flex-start', fontFamily: t.fontSans,
          transition: 'all 200ms ease',
        }}
        onClick={addRule}
        onMouseEnter={(e) => { e.currentTarget.style.borderColor = t.accentBorder; e.currentTarget.style.color = t.accent }}
        onMouseLeave={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.color = t.textSecondary }}
      >
        + Add Rule
      </button>
    </div>
  )
}
