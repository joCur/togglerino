import { useId } from 'react'

interface Props {
  value: number | undefined
  onChange: (value: number | undefined) => void
}

export default function RolloutSlider({ value, onChange }: Props) {
  const enabled = value !== undefined
  const checkboxId = useId()

  return (
    <div>
      <div className="flex items-center gap-2 mb-2">
        <input
          type="checkbox"
          id={checkboxId}
          className="cursor-pointer"
          checked={enabled}
          onChange={(e) => onChange(e.target.checked ? 100 : undefined)}
        />
        <label htmlFor={checkboxId} className="text-xs text-muted-foreground cursor-pointer">
          Percentage rollout
        </label>
      </div>
      {enabled && (
        <>
          <div className="text-xs text-muted-foreground/60 leading-relaxed mb-2">
            Gradually roll out this variant to a percentage of users. Uses consistent hashing — the same user always gets the same result.
          </div>
          <div className="flex items-center gap-3">
            <input
              type="range"
              min={0}
              max={100}
              step={1}
              value={value}
              onChange={(e) => onChange(Number(e.target.value))}
              className="flex-1 cursor-pointer"
            />
            <span className="text-[13px] font-semibold text-[#d4956a] min-w-[40px] text-right font-mono">
              {value}%
            </span>
          </div>
        </>
      )}
    </div>
  )
}
