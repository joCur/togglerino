import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client.ts'
import type { FlagTemplate, FlagPurpose, ValueType } from '@/api/types.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const FLAG_PURPOSES: { value: FlagPurpose; label: string }[] = [
  { value: 'release', label: 'Release' },
  { value: 'experiment', label: 'Experiment' },
  { value: 'operational', label: 'Operational' },
  { value: 'kill-switch', label: 'Kill Switch' },
  { value: 'permission', label: 'Permission' },
]

const VALUE_TYPES: { value: ValueType; label: string }[] = [
  { value: 'boolean', label: 'Boolean' },
  { value: 'string', label: 'String' },
  { value: 'number', label: 'Number' },
  { value: 'json', label: 'JSON' },
]

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function isValidJSON(value: string): boolean {
  if (!value.trim()) return true
  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

interface FormState {
  name: string
  key: string
  keyManual: boolean
  description: string
  flagType: FlagPurpose
  valueType: ValueType
  defaultValue: string
  tags: string
  sortOrder: string
  environmentDefaults: string
  variantConfig: string
}

const defaultFormState = (): FormState => ({
  name: '',
  key: '',
  keyManual: false,
  description: '',
  flagType: 'release',
  valueType: 'boolean',
  defaultValue: 'false',
  tags: '',
  sortOrder: '0',
  environmentDefaults: '',
  variantConfig: '',
})

function templateToFormState(t: FlagTemplate): FormState {
  return {
    name: t.name,
    key: t.key,
    keyManual: true,
    description: t.description ?? '',
    flagType: t.flag_type,
    valueType: t.value_type,
    defaultValue:
      t.default_value === null || t.default_value === undefined
        ? ''
        : typeof t.default_value === 'string'
          ? t.default_value
          : JSON.stringify(t.default_value),
    tags: (t.tags ?? []).join(', '),
    sortOrder: String(t.sort_order ?? 0),
    environmentDefaults:
      t.environment_defaults && Object.keys(t.environment_defaults).length > 0
        ? JSON.stringify(t.environment_defaults, null, 2)
        : '',
    variantConfig:
      t.variant_config && Object.keys(t.variant_config).length > 0
        ? JSON.stringify(t.variant_config, null, 2)
        : '',
  }
}

function buildPayload(form: FormState): Partial<FlagTemplate> {
  let parsedDefaultValue: unknown = form.defaultValue
  if (form.valueType === 'boolean') {
    parsedDefaultValue = form.defaultValue === 'true'
  } else if (form.valueType === 'number') {
    parsedDefaultValue = Number(form.defaultValue)
  } else if (form.valueType === 'json') {
    try {
      parsedDefaultValue = JSON.parse(form.defaultValue)
    } catch {
      parsedDefaultValue = form.defaultValue
    }
  }

  const tags = form.tags
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)

  const environmentDefaults =
    form.environmentDefaults.trim()
      ? (JSON.parse(form.environmentDefaults) as Record<string, { enabled: boolean }>)
      : {}

  const variantConfig =
    form.variantConfig.trim()
      ? (JSON.parse(form.variantConfig) as FlagTemplate['variant_config'])
      : {}

  return {
    name: form.name.trim(),
    key: form.key.trim(),
    description: form.description.trim(),
    flag_type: form.flagType,
    value_type: form.valueType,
    default_value: parsedDefaultValue,
    tags,
    sort_order: Number(form.sortOrder) || 0,
    environment_defaults: environmentDefaults,
    variant_config: variantConfig,
  }
}

interface TemplateDialogProps {
  open: boolean
  editing: FlagTemplate | null
  onClose: () => void
  onSaved: () => void
}

function TemplateDialog({ open, editing, onClose, onSaved }: TemplateDialogProps) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<FormState>(() =>
    editing ? templateToFormState(editing) : defaultFormState(),
  )
  const [error, setError] = useState('')

  // Reset form when dialog opens/closes or editing target changes
  const handleOpenChange = (isOpen: boolean) => {
    if (!isOpen) {
      onClose()
    }
  }

  // Re-initialise form when the dialog opens with a different template
  const [lastEditing, setLastEditing] = useState<FlagTemplate | null>(null)
  if (open && editing !== lastEditing) {
    setLastEditing(editing)
    setForm(editing ? templateToFormState(editing) : defaultFormState())
    setError('')
  }

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  const handleNameChange = (value: string) => {
    set('name', value)
    if (!form.keyManual) {
      set('key', slugify(value))
    }
  }

  const handleKeyChange = (value: string) => {
    set('key', value)
    set('keyManual', true)
  }

  const createMutation = useMutation({
    mutationFn: (payload: Partial<FlagTemplate>) => api.templates.createGlobal(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['templates', 'global'] })
      onSaved()
    },
    onError: (err: Error) => setError(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: ({ key, payload }: { key: string; payload: Partial<FlagTemplate> }) =>
      api.templates.updateGlobal(key, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['templates', 'global'] })
      onSaved()
    },
    onError: (err: Error) => setError(err.message),
  })

  const isPending = createMutation.isPending || updateMutation.isPending

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!form.name.trim()) {
      setError('Name is required')
      return
    }
    if (!form.key.trim()) {
      setError('Key is required')
      return
    }
    if (form.environmentDefaults.trim() && !isValidJSON(form.environmentDefaults)) {
      setError('Environment Defaults must be valid JSON')
      return
    }
    if (form.variantConfig.trim() && !isValidJSON(form.variantConfig)) {
      setError('Variant Config must be valid JSON')
      return
    }

    const payload = buildPayload(form)
    if (editing) {
      updateMutation.mutate({ key: editing.key, payload })
    } else {
      createMutation.mutate(payload)
    }
  }

  const labelClass = 'font-mono text-[10px] uppercase tracking-wider text-muted-foreground'

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{editing ? 'Edit Template' : 'Create Template'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          {/* Name + Key */}
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label className={labelClass}>Name</Label>
              <Input
                value={form.name}
                onChange={(e) => handleNameChange(e.target.value)}
                placeholder="e.g. Feature Flag"
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className={labelClass}>Key</Label>
              <Input
                value={form.key}
                onChange={(e) => handleKeyChange(e.target.value)}
                placeholder="feature-flag"
                className="font-mono text-xs"
              />
            </div>
          </div>

          {/* Description */}
          <div className="flex flex-col gap-1.5">
            <Label className={labelClass}>Description</Label>
            <Textarea
              value={form.description}
              onChange={(e) => set('description', e.target.value)}
              placeholder="Optional description for this template"
              className="min-h-[60px] text-sm"
            />
          </div>

          {/* Flag Type + Value Type */}
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label className={labelClass}>Flag Type</Label>
              <Select value={form.flagType} onValueChange={(v) => set('flagType', v as FlagPurpose)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FLAG_PURPOSES.map((p) => (
                    <SelectItem key={p.value} value={p.value}>
                      {p.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className={labelClass}>Value Type</Label>
              <Select value={form.valueType} onValueChange={(v) => set('valueType', v as ValueType)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VALUE_TYPES.map((t) => (
                    <SelectItem key={t.value} value={t.value}>
                      {t.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Default Value */}
          <div className="flex flex-col gap-1.5">
            <Label className={labelClass}>Default Value</Label>
            {form.valueType === 'boolean' ? (
              <Select value={form.defaultValue} onValueChange={(v) => set('defaultValue', v)}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="false">false</SelectItem>
                  <SelectItem value="true">true</SelectItem>
                </SelectContent>
              </Select>
            ) : (
              <Input
                value={form.defaultValue}
                onChange={(e) => set('defaultValue', e.target.value)}
                placeholder={form.valueType === 'json' ? '{"key": "value"}' : ''}
                className={form.valueType === 'json' ? 'font-mono text-xs' : ''}
              />
            )}
          </div>

          {/* Tags + Sort Order */}
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label className={labelClass}>Tags</Label>
              <Input
                value={form.tags}
                onChange={(e) => set('tags', e.target.value)}
                placeholder="tag1, tag2"
              />
              <p className="text-[10px] text-muted-foreground/60">Comma-separated</p>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className={labelClass}>Sort Order</Label>
              <Input
                type="number"
                value={form.sortOrder}
                onChange={(e) => set('sortOrder', e.target.value)}
                placeholder="0"
              />
            </div>
          </div>

          {/* Environment Defaults */}
          <div className="flex flex-col gap-1.5">
            <Label className={labelClass}>Environment Defaults (JSON)</Label>
            <Textarea
              value={form.environmentDefaults}
              onChange={(e) => set('environmentDefaults', e.target.value)}
              placeholder={'{\n  "production": { "enabled": false }\n}'}
              className="min-h-[80px] font-mono text-xs"
            />
            <p className="text-[10px] text-muted-foreground/60">
              Map of environment key to default enabled state. Leave empty to inherit project defaults.
            </p>
          </div>

          {/* Variant Config */}
          <div className="flex flex-col gap-1.5">
            <Label className={labelClass}>Variant Config (JSON)</Label>
            <Textarea
              value={form.variantConfig}
              onChange={(e) => set('variantConfig', e.target.value)}
              placeholder={'{\n  "variants": [],\n  "default_variant": "on"\n}'}
              className="min-h-[80px] font-mono text-xs"
            />
            <p className="text-[10px] text-muted-foreground/60">
              Pre-configured variants and targeting rules for this template.
            </p>
          </div>

          {error && <p className="text-destructive text-xs">{error}</p>}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button type="submit" disabled={isPending}>
              {isPending ? 'Saving...' : editing ? 'Save Changes' : 'Create Template'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface DeleteDialogProps {
  open: boolean
  template: FlagTemplate | null
  onClose: () => void
}

function DeleteDialog({ open, template, onClose }: DeleteDialogProps) {
  const queryClient = useQueryClient()
  const [error, setError] = useState('')

  const deleteMutation = useMutation({
    mutationFn: (key: string) => api.templates.deleteGlobal(key),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['templates', 'global'] })
      onClose()
    },
    onError: (err: Error) => setError(err.message),
  })

  const handleConfirm = () => {
    if (!template) return
    setError('')
    deleteMutation.mutate(template.key)
  }

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onClose() }}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Delete Template</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-muted-foreground">
          Are you sure you want to delete the template{' '}
          <span className="font-mono text-foreground font-semibold">{template?.key}</span>? This
          cannot be undone.
        </p>
        {error && <p className="text-destructive text-xs">{error}</p>}
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={deleteMutation.isPending}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={handleConfirm}
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function TemplatesSettingsTab() {
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<FlagTemplate | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<FlagTemplate | null>(null)

  const { data: templates, isLoading } = useQuery({
    queryKey: ['templates', 'global'],
    queryFn: () => api.templates.listGlobal(),
  })

  const openCreate = () => {
    setEditing(null)
    setDialogOpen(true)
  }

  const openEdit = (t: FlagTemplate) => {
    setEditing(t)
    setDialogOpen(true)
  }

  const closeDialog = () => {
    setDialogOpen(false)
    setEditing(null)
  }

  return (
    <div>
      <h2 className="text-sm font-medium mb-1">Global Templates</h2>
      <p className="text-xs text-muted-foreground mb-6">
        Manage global flag templates available to all projects.
      </p>

      <div className="flex justify-end mb-4">
        <Button size="sm" onClick={openCreate}>
          Create Template
        </Button>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="text-center py-10 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading templates...
            </div>
          ) : !templates || templates.length === 0 ? (
            <div className="text-center py-10 text-muted-foreground/60 text-[13px]">
              No global templates yet. Create one to get started.
            </div>
          ) : (
            <div className="rounded-md overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Name
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Key
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Flag Type
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Value Type
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      System
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Actions
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {templates.map((t) => (
                    <TableRow key={t.id} className="transition-colors hover:bg-[#d4956a]/8">
                      <TableCell className="text-[13px] text-foreground font-medium">
                        {t.name}
                      </TableCell>
                      <TableCell>
                        <span className="font-mono text-xs text-[#d4956a]">{t.key}</span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="font-mono text-[11px]">
                          {t.flag_type}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="font-mono text-[11px]">
                          {t.value_type}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {t.is_system && (
                          <Badge className="bg-amber-500/10 text-amber-400 border-amber-500/20 font-mono text-[11px]">
                            system
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-xs h-7"
                            onClick={() => openEdit(t)}
                            disabled={t.is_system}
                            title={t.is_system ? 'System templates cannot be edited' : undefined}
                          >
                            Edit
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                            onClick={() => setDeleteTarget(t)}
                            disabled={t.is_system}
                            title={t.is_system ? 'System templates cannot be deleted' : undefined}
                          >
                            Delete
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <TemplateDialog
        open={dialogOpen}
        editing={editing}
        onClose={closeDialog}
        onSaved={closeDialog}
      />

      <DeleteDialog
        open={deleteTarget !== null}
        template={deleteTarget}
        onClose={() => setDeleteTarget(null)}
      />
    </div>
  )
}
