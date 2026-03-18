import { useState } from 'react'
import type { Variant } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { X, Plus, Check } from 'lucide-react'
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

export default function VariantChips({
  variants,
  valueType,
  onChange,
  readonly,
}: VariantChipsProps) {
  const isBoolean = valueType === 'boolean'
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [editName, setEditName] = useState('')
  const [editValue, setEditValue] = useState('')
  const [adding, setAdding] = useState(false)
  const [addName, setAddName] = useState('')
  const [addValue, setAddValue] = useState('')

  const canRemove = variants.length > 2

  const startEdit = (index: number) => {
    if (isBoolean || readonly) return
    setEditingIndex(index)
    setEditName(variants[index].name)
    setEditValue(formatValue(variants[index].value))
    setAdding(false)
  }

  const saveEdit = () => {
    if (editingIndex === null || !editName.trim()) return
    const updated = [...variants]
    updated[editingIndex] = { name: editName.trim(), value: parseValue(editValue, valueType) }
    onChange(updated)
    setEditingIndex(null)
  }

  const cancelEdit = () => setEditingIndex(null)

  const startAdd = () => {
    setAdding(true)
    setAddName('')
    setAddValue('')
    setEditingIndex(null)
  }

  const confirmAdd = () => {
    if (!addName.trim()) return
    onChange([...variants, { name: addName.trim(), value: parseValue(addValue || '', valueType) }])
    setAdding(false)
  }

  const cancelAdd = () => setAdding(false)

  const handleRemove = (index: number, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!canRemove) return
    onChange(variants.filter((_, i) => i !== index))
    if (editingIndex === index) setEditingIndex(null)
  }

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
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {variants.map((v, i) => (
          <span
            key={i}
            onClick={() => startEdit(i)}
            className={cn(
              'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-mono cursor-pointer transition-colors',
              editingIndex === i
                ? 'border-[#d4956a]/50 bg-[#d4956a]/10 text-foreground'
                : 'border-border bg-muted hover:bg-muted/80 hover:border-muted-foreground/30',
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
              onClick={(e) => handleRemove(i, e)}
            >
              <X className="w-3 h-3" />
            </button>
          </span>
        ))}
        <button
          type="button"
          onClick={startAdd}
          className="inline-flex items-center gap-1 rounded-full border border-dashed border-muted-foreground/30 px-2.5 py-0.5 text-xs text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground transition-colors"
        >
          <Plus className="w-3 h-3" />
          Add
        </button>
      </div>

      {/* Inline edit form */}
      {editingIndex !== null && (
        <div className="flex items-center gap-2 p-2 rounded-md border border-border bg-card">
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={editName}
            onChange={(e) => setEditName(e.target.value)}
            placeholder="Name"
            autoFocus
            onKeyDown={(e) => { if (e.key === 'Enter') saveEdit(); if (e.key === 'Escape') cancelEdit() }}
          />
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            placeholder="Value"
            onKeyDown={(e) => { if (e.key === 'Enter') saveEdit(); if (e.key === 'Escape') cancelEdit() }}
          />
          <Button type="button" variant="ghost" size="sm" className="h-7 px-2" onClick={saveEdit}>
            <Check className="w-3.5 h-3.5" />
          </Button>
          <Button type="button" variant="ghost" size="sm" className="h-7 px-2" onClick={cancelEdit}>
            <X className="w-3.5 h-3.5" />
          </Button>
        </div>
      )}

      {/* Inline add form */}
      {adding && (
        <div className="flex items-center gap-2 p-2 rounded-md border border-dashed border-[#d4956a]/30 bg-card">
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={addName}
            onChange={(e) => setAddName(e.target.value)}
            placeholder="Variant name"
            autoFocus
            onKeyDown={(e) => { if (e.key === 'Enter') confirmAdd(); if (e.key === 'Escape') cancelAdd() }}
          />
          <Input
            className="h-7 text-xs font-mono flex-1"
            value={addValue}
            onChange={(e) => setAddValue(e.target.value)}
            placeholder={valueType === 'number' ? '0' : valueType === 'json' ? '{}' : 'Value'}
            onKeyDown={(e) => { if (e.key === 'Enter') confirmAdd(); if (e.key === 'Escape') cancelAdd() }}
          />
          <Button type="button" variant="ghost" size="sm" className="h-7 px-2" onClick={confirmAdd}>
            <Check className="w-3.5 h-3.5" />
          </Button>
          <Button type="button" variant="ghost" size="sm" className="h-7 px-2" onClick={cancelAdd}>
            <X className="w-3.5 h-3.5" />
          </Button>
        </div>
      )}
    </div>
  )
}
