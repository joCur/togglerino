import type { Variant } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

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
    <div className="flex flex-col gap-2">
      <div className="text-[13px] font-medium text-foreground">Variants</div>
      <div className="text-xs text-muted-foreground/60 leading-relaxed mb-1">
        {flagType === 'boolean'
          ? 'Define the on/off states for this flag. Each variant has a key (referenced in targeting rules) and a boolean value.'
          : flagType === 'string'
          ? 'Define the possible string values this flag can return. Each variant has a key (referenced in targeting rules) and a string value.'
          : flagType === 'number'
          ? 'Define the possible numeric values this flag can return. Each variant has a key (referenced in targeting rules) and a number value.'
          : 'Define the possible JSON payloads this flag can return. Each variant has a key (referenced in targeting rules) and a JSON value.'}
      </div>

      {variants.length === 0 && (
        <div className="text-xs text-muted-foreground/60 italic">
          {flagType === 'boolean' ? (
            <>
              No variants defined.{' '}
              <span
                className="text-[#d4956a] cursor-pointer not-italic hover:text-[#e0a87a] transition-colors duration-200"
                onClick={addDefaults}
              >
                Add boolean defaults
              </span>
            </>
          ) : (
            'No variants defined. Add variants to use in targeting rules and as the default value.'
          )}
        </div>
      )}

      {variants.map((v, i) => (
        <div key={i} className="flex items-center gap-2">
          <Input
            className="flex-none w-[110px] font-mono text-xs"
            placeholder="Key"
            value={v.key}
            onChange={(e) => updateKey(i, e.target.value)}
          />
          {flagType === 'boolean' ? (
            <select
              className="flex-1 px-2.5 py-1.5 text-xs border rounded-md bg-input text-foreground outline-none cursor-pointer"
              value={String(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
            >
              <option value="true">true</option>
              <option value="false">false</option>
            </select>
          ) : flagType === 'json' ? (
            <Textarea
              className="flex-1 min-h-[32px] resize-y font-mono text-[11px]"
              value={formatValue(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
              placeholder="JSON value"
            />
          ) : (
            <Input
              className="flex-1 text-xs"
              type={flagType === 'number' ? 'number' : 'text'}
              placeholder="Value"
              value={formatValue(v.value)}
              onChange={(e) => updateValue(i, e.target.value)}
            />
          )}
          <Button
            variant="destructive"
            size="sm"
            className="shrink-0 text-[11px] px-2.5 h-7"
            onClick={() => remove(i)}
          >
            Remove
          </Button>
        </div>
      ))}

      <Button
        variant="outline"
        size="sm"
        className="self-start mt-0.5"
        onClick={add}
      >
        + Add Variant
      </Button>
    </div>
  )
}
