import { useState, type ReactNode } from 'react'
import type { Variant } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function parseValue(raw: string, valueType: string): unknown {
  if (valueType === 'boolean') return raw === 'true'
  if (valueType === 'number') {
    const n = Number(raw)
    return isNaN(n) ? 0 : n
  }
  if (valueType === 'json') {
    try {
      return JSON.parse(raw)
    } catch {
      return raw
    }
  }
  return raw
}

function VariantEditPopover({
  variant,
  valueType,
  onSave,
  children,
}: {
  variant: Variant
  valueType: string
  onSave: (name: string, value: unknown) => void
  children: ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState(variant.name)
  const [rawValue, setRawValue] = useState(formatValue(variant.value))

  const handleOpen = (isOpen: boolean) => {
    if (isOpen) {
      setName(variant.name)
      setRawValue(formatValue(variant.value))
    }
    setOpen(isOpen)
  }

  const handleSave = () => {
    if (!name.trim()) return
    onSave(name.trim(), parseValue(rawValue, valueType))
    setOpen(false)
  }

  return (
    <Popover open={open} onOpenChange={handleOpen} modal>
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent className="w-64 z-[100]" align="start" onOpenAutoFocus={(e) => e.preventDefault()}>
        <div className="flex flex-col gap-3">
          <div className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground/50">
            Edit Variant
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Name</Label>
            <Input
              className="h-8 text-xs font-mono"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="variant-name"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Value</Label>
            {valueType === 'json' ? (
              <Textarea
                className="min-h-[60px] text-[11px] font-mono resize-y"
                value={rawValue}
                onChange={(e) => setRawValue(e.target.value)}
                placeholder="JSON value"
              />
            ) : (
              <Input
                className="h-8 text-xs font-mono"
                type={valueType === 'number' ? 'number' : 'text'}
                value={rawValue}
                onChange={(e) => setRawValue(e.target.value)}
                placeholder="Value"
              />
            )}
          </div>
          <Button type="button" size="sm" className="text-xs" onClick={handleSave}>
            Save
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function AddVariantPopover({
  valueType,
  onAdd,
}: {
  valueType: string
  onAdd: (name: string, value: unknown) => void
}) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState('')
  const [rawValue, setRawValue] = useState('')

  const handleOpen = (isOpen: boolean) => {
    if (isOpen) {
      setName('')
      setRawValue('')
    }
    setOpen(isOpen)
  }

  const handleAdd = () => {
    if (!name.trim()) return
    onAdd(name.trim(), parseValue(rawValue || defaultRawValue(valueType), valueType))
    setOpen(false)
  }

  return (
    <Popover open={open} onOpenChange={handleOpen} modal>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-full border border-dashed border-muted-foreground/30 px-2.5 py-0.5 text-xs text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground transition-colors"
          onClick={(e) => { e.preventDefault(); e.stopPropagation(); handleOpen(!open) }}
        >
          <Plus className="w-3 h-3" />
          Add
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-64 z-[100]" align="start" onOpenAutoFocus={(e) => e.preventDefault()}>
        <div className="flex flex-col gap-3">
          <div className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground/50">
            New Variant
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Name</Label>
            <Input
              className="h-8 text-xs font-mono"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="variant-name"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">Value</Label>
            {valueType === 'json' ? (
              <Textarea
                className="min-h-[60px] text-[11px] font-mono resize-y"
                value={rawValue}
                onChange={(e) => setRawValue(e.target.value)}
                placeholder="JSON value"
              />
            ) : (
              <Input
                className="h-8 text-xs font-mono"
                type={valueType === 'number' ? 'number' : 'text'}
                value={rawValue}
                onChange={(e) => setRawValue(e.target.value)}
                placeholder="Value"
              />
            )}
          </div>
          <Button type="button" size="sm" className="text-xs" onClick={handleAdd}>
            Add Variant
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function defaultRawValue(valueType: string): string {
  if (valueType === 'boolean') return 'false'
  if (valueType === 'number') return '0'
  if (valueType === 'json') return '{}'
  return ''
}

export default function VariantChips({
  variants,
  valueType,
  onChange,
  readonly,
}: VariantChipsProps) {
  const isBoolean = valueType === 'boolean'
  const canRemove = variants.length > 2

  const handleEdit = (index: number, name: string, value: unknown) => {
    const updated = [...variants]
    updated[index] = { name, value }
    onChange(updated)
  }

  const handleRemove = (index: number) => {
    if (!canRemove) return
    onChange(variants.filter((_, i) => i !== index))
  }

  const handleAdd = (name: string, value: unknown) => {
    onChange([...variants, { name, value }])
  }

  if (isBoolean || readonly) {
    return (
      <div className="flex flex-wrap items-center gap-2">
        {variants.map((v, i) => (
          <Badge
            key={i}
            variant="secondary"
            className="font-mono text-xs px-2.5 py-1 rounded-md"
          >
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
        <VariantEditPopover
          key={i}
          variant={v}
          valueType={valueType}
          onSave={(name, value) => handleEdit(i, name, value)}
        >
          <span
            className={cn(
              'inline-flex items-center gap-1.5 rounded-md border border-border bg-muted px-2.5 py-1 text-xs font-mono cursor-pointer transition-colors hover:bg-muted/80 hover:border-muted-foreground/30',
            )}
          >
            {v.name}
            <button
              type="button"
              className={cn(
                'inline-flex items-center justify-center rounded-sm p-0.5 -mr-1 transition-colors',
                canRemove
                  ? 'hover:bg-destructive/20 hover:text-destructive text-muted-foreground/60'
                  : 'text-muted-foreground/20 cursor-not-allowed',
              )}
              disabled={!canRemove}
              onClick={(e) => {
                e.stopPropagation()
                handleRemove(i)
              }}
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        </VariantEditPopover>
      ))}
      <AddVariantPopover valueType={valueType} onAdd={handleAdd} />
    </div>
  )
}
