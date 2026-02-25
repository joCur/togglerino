import type { Variant } from '../api/types.ts'

interface Props {
  variants: Variant[]
  flagType: string
  onChange: (variants: Variant[]) => void
}

const styles = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  } as const,
  heading: {
    fontSize: 14,
    fontWeight: 600,
    color: '#e0e0e0',
    marginBottom: 4,
  } as const,
  row: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
  } as const,
  input: {
    padding: '8px 10px',
    fontSize: 13,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    flex: 1,
  } as const,
  keyInput: {
    padding: '8px 10px',
    fontSize: 13,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    width: 120,
    flexShrink: 0,
  } as const,
  removeBtn: {
    padding: '6px 10px',
    fontSize: 12,
    border: '1px solid rgba(233, 69, 96, 0.3)',
    borderRadius: 6,
    backgroundColor: 'rgba(233, 69, 96, 0.1)',
    color: '#e94560',
    cursor: 'pointer',
    flexShrink: 0,
  } as const,
  addBtn: {
    padding: '8px 14px',
    fontSize: 13,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
    alignSelf: 'flex-start',
    marginTop: 4,
  } as const,
  emptyText: {
    fontSize: 13,
    color: '#8892b0',
    fontStyle: 'italic',
  } as const,
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function parseValue(raw: string, flagType: string): unknown {
  if (flagType === 'boolean') return raw === 'true'
  if (flagType === 'number') {
    const n = Number(raw)
    return isNaN(n) ? 0 : n
  }
  if (flagType === 'json') {
    try {
      return JSON.parse(raw)
    } catch {
      return raw
    }
  }
  return raw
}

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
    <div style={styles.container}>
      <div style={styles.heading}>Variants</div>
      {variants.length === 0 && (
        <div style={styles.emptyText}>
          No variants defined.
          {flagType === 'boolean' && (
            <>
              {' '}
              <span
                style={{ color: '#e94560', cursor: 'pointer', fontStyle: 'normal' }}
                onClick={addDefaults}
              >
                Add boolean defaults
              </span>
            </>
          )}
        </div>
      )}
      {variants.map((v, i) => (
        <div key={i} style={styles.row}>
          <input
            style={styles.keyInput}
            placeholder="Key"
            value={v.key}
            onChange={(e) => updateKey(i, e.target.value)}
          />
          {flagType === 'boolean' ? (
            <select
              style={{ ...styles.input, cursor: 'pointer' }}
              value={String(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
            >
              <option value="true">true</option>
              <option value="false">false</option>
            </select>
          ) : flagType === 'json' ? (
            <textarea
              style={{ ...styles.input, minHeight: 32, resize: 'vertical', fontFamily: 'monospace', fontSize: 12 }}
              value={formatValue(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
              placeholder="JSON value"
            />
          ) : (
            <input
              style={styles.input}
              type={flagType === 'number' ? 'number' : 'text'}
              placeholder="Value"
              value={formatValue(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
            />
          )}
          <button style={styles.removeBtn} onClick={() => remove(i)}>
            Remove
          </button>
        </div>
      ))}
      <button style={styles.addBtn} onClick={add}>
        + Add Variant
      </button>
    </div>
  )
}
