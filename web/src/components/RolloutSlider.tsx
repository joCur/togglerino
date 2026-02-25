interface Props {
  value: number | undefined
  onChange: (value: number | undefined) => void
}

const styles = {
  wrapper: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
  } as const,
  label: {
    fontSize: 13,
    color: '#8892b0',
    whiteSpace: 'nowrap',
  } as const,
  slider: {
    flex: 1,
    height: 6,
    appearance: 'none' as const,
    WebkitAppearance: 'none' as const,
    background: '#0f3460',
    borderRadius: 3,
    outline: 'none',
    cursor: 'pointer',
    accentColor: '#e94560',
  },
  valueDisplay: {
    fontSize: 14,
    fontWeight: 600,
    color: '#e0e0e0',
    minWidth: 44,
    textAlign: 'right' as const,
  },
  toggle: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    marginBottom: 8,
  } as const,
  checkbox: {
    accentColor: '#e94560',
    cursor: 'pointer',
  } as const,
  checkboxLabel: {
    fontSize: 13,
    color: '#8892b0',
    cursor: 'pointer',
  } as const,
}

export default function RolloutSlider({ value, onChange }: Props) {
  const enabled = value !== undefined

  return (
    <div>
      <div style={styles.toggle}>
        <input
          type="checkbox"
          id="rollout-toggle"
          style={styles.checkbox}
          checked={enabled}
          onChange={(e) => onChange(e.target.checked ? 100 : undefined)}
        />
        <label htmlFor="rollout-toggle" style={styles.checkboxLabel}>
          Percentage rollout
        </label>
      </div>
      {enabled && (
        <div style={styles.wrapper}>
          <input
            type="range"
            min={0}
            max={100}
            step={1}
            value={value}
            onChange={(e) => onChange(Number(e.target.value))}
            style={styles.slider}
          />
          <span style={styles.valueDisplay}>{value}%</span>
        </div>
      )}
    </div>
  )
}
