import type { TargetingRule, Variant, Condition } from '../api/types.ts'
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

const styles = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: 16,
  } as const,
  heading: {
    fontSize: 14,
    fontWeight: 600,
    color: '#e0e0e0',
    marginBottom: 4,
  } as const,
  ruleCard: {
    padding: 16,
    borderRadius: 8,
    backgroundColor: 'rgba(15, 52, 96, 0.3)',
    border: '1px solid #2a2a4a',
  } as const,
  ruleHeader: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 12,
  } as const,
  ruleTitle: {
    fontSize: 13,
    fontWeight: 600,
    color: '#8892b0',
  } as const,
  removeBtn: {
    padding: '4px 10px',
    fontSize: 12,
    border: '1px solid rgba(233, 69, 96, 0.3)',
    borderRadius: 6,
    backgroundColor: 'rgba(233, 69, 96, 0.1)',
    color: '#e94560',
    cursor: 'pointer',
  } as const,
  conditionsSection: {
    marginBottom: 12,
  } as const,
  conditionRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    marginBottom: 8,
  } as const,
  input: {
    padding: '7px 10px',
    fontSize: 13,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    flex: 1,
  } as const,
  select: {
    padding: '7px 10px',
    fontSize: 13,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    cursor: 'pointer',
  } as const,
  smallBtn: {
    padding: '4px 8px',
    fontSize: 11,
    border: '1px solid rgba(233, 69, 96, 0.3)',
    borderRadius: 4,
    backgroundColor: 'transparent',
    color: '#e94560',
    cursor: 'pointer',
    flexShrink: 0,
  } as const,
  addConditionBtn: {
    padding: '6px 12px',
    fontSize: 12,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
  } as const,
  variantRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    marginBottom: 12,
  } as const,
  label: {
    fontSize: 13,
    color: '#8892b0',
    whiteSpace: 'nowrap',
  } as const,
  addRuleBtn: {
    padding: '8px 14px',
    fontSize: 13,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
    alignSelf: 'flex-start',
  } as const,
  emptyText: {
    fontSize: 13,
    color: '#8892b0',
    fontStyle: 'italic',
  } as const,
}

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
    <div style={styles.container}>
      <div style={styles.heading}>Targeting Rules</div>
      {rules.length === 0 && (
        <div style={styles.emptyText}>No targeting rules. All users will receive the default variant.</div>
      )}
      {rules.map((rule, ruleIdx) => (
        <div key={ruleIdx} style={styles.ruleCard}>
          <div style={styles.ruleHeader}>
            <span style={styles.ruleTitle}>Rule {ruleIdx + 1}</span>
            <button style={styles.removeBtn} onClick={() => removeRule(ruleIdx)}>
              Remove
            </button>
          </div>

          <div style={styles.conditionsSection}>
            <div style={{ ...styles.label, marginBottom: 6 }}>Conditions</div>
            {rule.conditions.map((cond, condIdx) => (
              <div key={condIdx} style={styles.conditionRow}>
                <input
                  style={styles.input}
                  placeholder="Attribute (e.g. country)"
                  value={cond.attribute}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { attribute: e.target.value })}
                />
                <select
                  style={{ ...styles.select, width: 140 }}
                  value={cond.operator}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { operator: e.target.value })}
                >
                  {OPERATORS.map((op) => (
                    <option key={op.value} value={op.value}>
                      {op.label}
                    </option>
                  ))}
                </select>
                <input
                  style={styles.input}
                  placeholder="Value"
                  value={String(cond.value ?? '')}
                  onChange={(e) => updateCondition(ruleIdx, condIdx, { value: e.target.value })}
                />
                {rule.conditions.length > 1 && (
                  <button style={styles.smallBtn} onClick={() => removeCondition(ruleIdx, condIdx)}>
                    x
                  </button>
                )}
              </div>
            ))}
            <button style={styles.addConditionBtn} onClick={() => addCondition(ruleIdx)}>
              + Add Condition
            </button>
          </div>

          <div style={styles.variantRow}>
            <span style={styles.label}>Serve variant:</span>
            {variants.length > 0 ? (
              <select
                style={styles.select}
                value={rule.variant}
                onChange={(e) => updateRule(ruleIdx, { variant: e.target.value })}
              >
                {variants.map((v) => (
                  <option key={v.key} value={v.key}>
                    {v.key}
                  </option>
                ))}
              </select>
            ) : (
              <input
                style={{ ...styles.input, flex: 'none', width: 140 }}
                placeholder="Variant key"
                value={rule.variant}
                onChange={(e) => updateRule(ruleIdx, { variant: e.target.value })}
              />
            )}
          </div>

          <RolloutSlider
            value={rule.percentage_rollout}
            onChange={(val) => updateRule(ruleIdx, { percentage_rollout: val })}
          />
        </div>
      ))}
      <button style={styles.addRuleBtn} onClick={addRule}>
        + Add Rule
      </button>
    </div>
  )
}
