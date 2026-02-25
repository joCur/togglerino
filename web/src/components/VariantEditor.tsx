import type { Variant } from '../api/types.ts'
import { t } from '../theme.ts'

interface Props {
  variants: Variant[]
  flagType: string
  onChange: (variants: Variant[]) => void
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function parseValue(raw: string, flagType: string): unknown {
  if (flagType === 'boolean') return raw === 'true'
  if (flagType === 'number') { const n = Number(raw); return isNaN(n) ? 0 : n }
  if (flagType === 'json') { try { return JSON.parse(raw) } catch { return raw } }
  return raw
}

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

export default function VariantEditor({ variants, flagType, onChange }: Props) {
  const updateKey = (index: number, newKey: string) => {
    const updated = [...variants]
    updated[index] = { ...updated[index], key: newKey }
    onChange(updated)
  }

  const updateValue = (index: number, raw: string) => {
    const updated = [...variants]
    updated[index] = { ...updated[index], value: parseValue(raw, flagType) }
    onChange(updated)
  }

  const remove = (index: number) => {
    onChange(variants.filter((_, i) => i !== index))
  }

  const add = () => {
    let defaultVal: unknown = ''
    if (flagType === 'boolean') defaultVal = false
    else if (flagType === 'number') defaultVal = 0
    else if (flagType === 'json') defaultVal = {}
    onChange([...variants, { key: '', value: defaultVal }])
  }

  const addDefaults = () => {
    if (flagType === 'boolean') {
      onChange([
        { key: 'on', value: true },
        { key: 'off', value: false },
      ])
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ fontSize: 13, fontWeight: 500, color: t.textPrimary }}>Variants</div>

      {variants.length === 0 && (
        <div style={{ fontSize: 12, color: t.textMuted, fontStyle: 'italic' }}>
          No variants defined.
          {flagType === 'boolean' && (
            <>
              {' '}
              <span
                style={{ color: t.accent, cursor: 'pointer', fontStyle: 'normal', transition: 'color 200ms ease' }}
                onClick={addDefaults}
                onMouseEnter={(e) => { e.currentTarget.style.color = t.accentLight }}
                onMouseLeave={(e) => { e.currentTarget.style.color = t.accent }}
              >
                Add boolean defaults
              </span>
            </>
          )}
        </div>
      )}

      {variants.map((v, i) => (
        <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <input
            style={{ ...inputStyle, flex: 'none', width: 110, fontFamily: t.fontMono }}
            placeholder="Key"
            value={v.key}
            onChange={(e) => updateKey(i, e.target.value)}
            onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
            onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
          />
          {flagType === 'boolean' ? (
            <select
              style={{ ...inputStyle, cursor: 'pointer' }}
              value={String(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
            >
              <option value="true">true</option>
              <option value="false">false</option>
            </select>
          ) : flagType === 'json' ? (
            <textarea
              style={{ ...inputStyle, minHeight: 32, resize: 'vertical', fontFamily: t.fontMono, fontSize: 11 }}
              value={formatValue(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
              placeholder="JSON value"
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
            />
          ) : (
            <input
              style={inputStyle}
              type={flagType === 'number' ? 'number' : 'text'}
              placeholder="Value"
              value={formatValue(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
            />
          )}
          <button
            style={{
              padding: '5px 10px', fontSize: 11, fontWeight: 500,
              border: `1px solid ${t.dangerBorder}`, borderRadius: t.radiusSm,
              backgroundColor: t.dangerSubtle, color: t.danger,
              cursor: 'pointer', flexShrink: 0, fontFamily: t.fontSans,
              transition: 'all 200ms ease',
            }}
            onClick={() => remove(i)}
            onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = 'rgba(242,116,116,0.15)' }}
            onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = t.dangerSubtle }}
          >
            Remove
          </button>
        </div>
      ))}

      <button
        style={{
          padding: '7px 14px', fontSize: 12, fontWeight: 500,
          border: `1px solid ${t.border}`, borderRadius: t.radiusMd,
          backgroundColor: 'transparent', color: t.textSecondary,
          cursor: 'pointer', alignSelf: 'flex-start', marginTop: 2,
          fontFamily: t.fontSans, transition: 'all 200ms ease',
        }}
        onClick={add}
        onMouseEnter={(e) => { e.currentTarget.style.borderColor = t.accentBorder; e.currentTarget.style.color = t.accent }}
        onMouseLeave={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.color = t.textSecondary }}
      >
        + Add Variant
      </button>
    </div>
  )
}
