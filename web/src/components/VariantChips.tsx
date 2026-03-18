import { useState } from 'react'
import type { Variant } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { X, Plus } from 'lucide-react'
import { cn } from '@/lib/utils'

interface VariantChipsProps {
  variants: Variant[]
  valueType: string
  onChange: (variants: Variant[]) => void
  readonly?: boolean
}

function parseValue(raw: string, valueType: string): unknown {
  if (valueType === 'boolean') return raw === 'true'
  if (valueType === 'number') {
    const n = Number(raw)
    return isNaN(n) ? 0 : n
  }
  if (valueType === 'json') {
    try { return JSON.parse(raw) } catch { return raw }
  }
  return raw
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function defaultRawValue(valueType: string): string {
  if (valueType === 'boolean') return 'false'
  if (valueType === 'number') return '0'
  if (valueType === 'json') return '{}'
  return ''
}

/** Popover for editing an existing variant. */
function EditPopover({
  variant,
  valueType,
  onSave,
  children,
}: {
  variant: Variant
  valueType: string
  onSave: (name: string, value: unknown) => void
  children: React.ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [rawValue, setRawValue] = useState('')

  return (
    <Popover open={open} onOpenChange={(v) => {
      if (v) { setName(variant.name); setRawValue(formatValue(variant.value)) }
      setOpen(v)
    }}>
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent className="w-64" align="start" side="bottom"
        onPointerDownOutside={(e) => e.preventDefault()}
      >
        <div className="flex flex-col gap-3" onClick={(e) => e.stopPropagation()}>
          <div className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground/50">Edit Variant</div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Name</Label>
            <Input className="h-8 text-xs font-mono" value={name}
              onChange={(e) => setName(e.target.value)} placeholder="variant-name"
              onKeyDown={(e) => { if (e.key === 'Enter' && name.trim()) { onSave(name.trim(), parseValue(rawValue, valueType)); setOpen(false) } }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Value</Label>
            <Input className="h-8 text-xs font-mono" value={rawValue}
              onChange={(e) => setRawValue(e.target.value)} placeholder="Value"
              onKeyDown={(e) => { if (e.key === 'Enter' && name.trim()) { onSave(name.trim(), parseValue(rawValue, valueType)); setOpen(false) } }}
            />
          </div>
          <Button type="button" size="sm" className="text-xs" onClick={() => {
            if (!name.trim()) return
            onSave(name.trim(), parseValue(rawValue, valueType))
            setOpen(false)
          }}>Save</Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

/** Popover for adding a new variant. */
function AddPopover({
  valueType,
  onAdd,
}: {
  valueType: string
  onAdd: (name: string, value: unknown) => void
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [rawValue, setRawValue] = useState('')

  return (
    <Popover open={open} onOpenChange={(v) => {
      if (v) { setName(''); setRawValue('') }
      setOpen(v)
    }}>
      <PopoverTrigger asChild>
        <button type="button"
          className="inline-flex items-center gap-1 rounded-full border border-dashed border-muted-foreground/30 px-2.5 py-0.5 text-xs text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground transition-colors"
        >
          <Plus className="w-3 h-3" />
          Add
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-64" align="start" side="bottom"
        onPointerDownOutside={(e) => e.preventDefault()}
      >
        <div className="flex flex-col gap-3" onClick={(e) => e.stopPropagation()}>
          <div className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground/50">New Variant</div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Name</Label>
            <Input className="h-8 text-xs font-mono" value={name}
              onChange={(e) => setName(e.target.value)} placeholder="variant-name"
              onKeyDown={(e) => { if (e.key === 'Enter' && name.trim()) { onAdd(name.trim(), parseValue(rawValue || defaultRawValue(valueType), valueType)); setOpen(false) } }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Value</Label>
            <Input className="h-8 text-xs font-mono" value={rawValue}
              onChange={(e) => setRawValue(e.target.value)}
              placeholder={valueType === 'number' ? '0' : valueType === 'json' ? '{}' : 'Value'}
              onKeyDown={(e) => { if (e.key === 'Enter' && name.trim()) { onAdd(name.trim(), parseValue(rawValue || defaultRawValue(valueType), valueType)); setOpen(false) } }}
            />
          </div>
          <Button type="button" size="sm" className="text-xs" onClick={() => {
            if (!name.trim()) return
            onAdd(name.trim(), parseValue(rawValue || defaultRawValue(valueType), valueType))
            setOpen(false)
          }}>Add Variant</Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

export default function VariantChips({
  variants,
  valueType,
  onChange,
  readonly,
}: VariantChipsProps) {
  const isBoolean = valueType === 'boolean'
  const canRemove = variants.length > 2

  // Read-only mode (boolean flags or readonly prop)
  if (isBoolean || readonly) {
    return (
      <div className="flex flex-wrap items-center gap-2">
        {variants.map((v, i) => (
          <Badge key={i} variant="secondary" className="font-mono text-xs px-2.5 py-1 rounded-md">
            {v.name}
          </Badge>
        ))}
        {variants.length === 0 && (
          <span className="text-xs text-muted-foreground/50 italic">No variants</span>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      {variants.map((v, i) => (
        <EditPopover key={i} variant={v} valueType={valueType}
          onSave={(name, value) => {
            const updated = [...variants]
            updated[i] = { name, value }
            onChange(updated)
          }}
        >
          <span className={cn(
            'inline-flex items-center gap-1.5 rounded-md border border-border bg-muted px-2.5 py-1 text-xs font-mono cursor-pointer transition-colors hover:bg-muted/80 hover:border-muted-foreground/30',
          )}>
            {v.name}
            <button type="button"
              className={cn(
                'inline-flex items-center justify-center rounded-sm p-0.5 -mr-1 transition-colors',
                canRemove
                  ? 'hover:bg-destructive/20 hover:text-destructive text-muted-foreground/60'
                  : 'text-muted-foreground/20 cursor-not-allowed',
              )}
              disabled={!canRemove}
              onClick={(e) => {
                e.stopPropagation()
                if (canRemove) onChange(variants.filter((_, idx) => idx !== i))
              }}
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        </EditPopover>
      ))}
      <AddPopover valueType={valueType}
        onAdd={(name, value) => onChange([...variants, { name, value }])}
      />
    </div>
  )
}
