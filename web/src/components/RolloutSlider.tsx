import { t } from '../theme.ts'

interface Props {
  value: number | undefined
  onChange: (value: number | undefined) => void
}

export default function RolloutSlider({ value, onChange }: Props) {
  const enabled = value !== undefined

  return (
    <div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
        <input
          type="checkbox"
          id="rollout-toggle"
          style={{ cursor: 'pointer' }}
          checked={enabled}
          onChange={(e) => onChange(e.target.checked ? 100 : undefined)}
        />
        <label
          htmlFor="rollout-toggle"
          style={{ fontSize: 12, color: t.textSecondary, cursor: 'pointer' }}
        >
          Percentage rollout
        </label>
      </div>
      {enabled && (
        <>
          <div style={{ fontSize: 12, color: t.textMuted, lineHeight: 1.5, marginBottom: 8 }}>
            Gradually roll out this variant to a percentage of users. Uses consistent hashing — the same user always gets the same result.
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <input
              type="range"
              min={0}
              max={100}
              step={1}
              value={value}
              onChange={(e) => onChange(Number(e.target.value))}
              style={{ flex: 1, cursor: 'pointer' }}
            />
            <span
              style={{
                fontSize: 13,
                fontWeight: 600,
                color: t.accent,
                minWidth: 40,
                textAlign: 'right',
                fontFamily: t.fontMono,
              }}
            >
              {value}%
            </span>
          </div>
        </>
      )}
    </div>
  )
}
