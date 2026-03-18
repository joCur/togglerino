import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { FlagTemplate, FlagPurpose, ValueType } from '@/api/types'
import { slugify } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
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
import {
  EnvironmentDefaultsEditor,
  VariantConfigEditor,
} from '@/components/TemplateEditors'
import type { EnvDefault, VariantConfigState } from '@/components/TemplateEditors'
import {
  envDefaultsToRecord,
  recordToEnvDefaults,
  variantConfigToState,
  stateToVariantConfig,
} from '@/lib/templateUtils'

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

interface FormState {
  name: string
  key: string
  keyManual: boolean
  description: string
  flagType: FlagPurpose
  valueType: ValueType
  tags: string
  sortOrder: string
  environmentDefaults: EnvDefault[]
  variantConfig: VariantConfigState
}

function defaultFormState(): FormState {
  return {
    name: '',
    key: '',
    keyManual: false,
    description: '',
    flagType: 'release',
    valueType: 'boolean',
    tags: '',
    sortOrder: '0',
    environmentDefaults: [],
    variantConfig: { variants: [], defaultVariant: '', targetingRules: [] },
  }
}

function templateToFormState(t: FlagTemplate): FormState {
  const vc = variantConfigToState(t.variant_config)
  if (vc.variants.length === 0 && t.default_value !== null && t.default_value !== undefined) {
    if (t.value_type === 'boolean') {
      vc.variants = [
        { name: 'on', value: true },
        { name: 'off', value: false },
      ]
      vc.defaultVariant = t.default_value === true ? 'on' : 'off'
    } else {
      vc.variants = [{ name: 'default', value: t.default_value }]
      vc.defaultVariant = 'default'
    }
  }
  return {
    name: t.name,
    key: t.key,
    keyManual: true,
    description: t.description ?? '',
    flagType: t.flag_type,
    valueType: t.value_type,
    tags: (t.tags ?? []).join(', '),
    sortOrder: String(t.sort_order ?? 0),
    environmentDefaults: recordToEnvDefaults(t.environment_defaults),
    variantConfig: vc,
  }
}

function buildPayload(form: FormState): Partial<FlagTemplate> {
  const { variants, defaultVariant } = form.variantConfig
  const defaultVar = variants.find((v) => v.name === defaultVariant)
  const defaultValue = defaultVar?.value ?? (form.valueType === 'boolean' ? false : form.valueType === 'number' ? 0 : '')

  const tags = form.tags
    .split(',')
    .map((t) => t.trim())
    .filter(Boolean)

  return {
    name: form.name.trim(),
    key: form.key.trim(),
    description: form.description.trim(),
    flag_type: form.flagType,
    value_type: form.valueType,
    default_value: defaultValue,
    tags,
    sort_order: Number(form.sortOrder) || 0,
    environment_defaults: envDefaultsToRecord(form.environmentDefaults),
    variant_config: stateToVariantConfig(form.variantConfig),
  }
}

export interface TemplateFormDialogProps {
  open: boolean
  editing: FlagTemplate | null
  onClose: () => void
  onSaved: () => void
  onDelete?: (template: FlagTemplate) => void
  createFn: (payload: Partial<FlagTemplate>) => Promise<FlagTemplate>
  updateFn: (key: string, payload: Partial<FlagTemplate>) => Promise<FlagTemplate>
  invalidateKey: string[]
}

export function TemplateFormDialog({
  open,
  editing,
  onClose,
  onSaved,
  onDelete,
  createFn,
  updateFn,
  invalidateKey,
}: TemplateFormDialogProps) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<FormState>(() =>
    editing ? templateToFormState(editing) : defaultFormState(),
  )
  const [error, setError] = useState('')
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

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
    mutationFn: (payload: Partial<FlagTemplate>) => createFn(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey })
      onSaved()
    },
    onError: (err: Error) => setError(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: ({ key, payload }: { key: string; payload: Partial<FlagTemplate> }) =>
      updateFn(key, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey })
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
    if (!editing && !form.key.trim()) {
      setError('Key is required')
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
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) onClose() }}>
      <DialogContent className="sm:max-w-3xl max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {editing ? 'Edit Template' : 'Create Template'}
            {editing && (
              <span className="ml-2 font-mono text-sm text-[#d4956a]">{editing.key}</span>
            )}
          </DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
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
                disabled={!!editing}
              />
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className={labelClass}>Description</Label>
            <Textarea
              value={form.description}
              onChange={(e) => set('description', e.target.value)}
              placeholder="Optional description for this template"
              className="min-h-[60px] text-sm"
            />
          </div>

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

          <EnvironmentDefaultsEditor
            value={form.environmentDefaults}
            onChange={(v) => set('environmentDefaults', v)}
          />

          <VariantConfigEditor
            value={form.variantConfig}
            onChange={(v) => set('variantConfig', v)}
            valueType={form.valueType}
          />

          {error && <p className="text-destructive text-xs">{error}</p>}

          <DialogFooter className="flex-col sm:flex-row gap-2">
            {editing && onDelete && !editing.is_system && (
              !showDeleteConfirm ? (
                <Button
                  type="button"
                  variant="outline"
                  className="border-destructive/50 text-destructive hover:bg-destructive/10 mr-auto"
                  onClick={() => setShowDeleteConfirm(true)}
                >
                  Delete
                </Button>
              ) : (
                <div className="flex items-center gap-2 mr-auto animate-[fadeIn_200ms_ease]">
                  <span className="text-xs text-muted-foreground">Are you sure?</span>
                  <Button
                    type="button"
                    variant="destructive"
                    size="sm"
                    onClick={() => onDelete(editing)}
                  >
                    Confirm Delete
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => setShowDeleteConfirm(false)}
                  >
                    Cancel
                  </Button>
                </div>
              )
            )}
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

export interface DeleteTemplateDialogProps {
  open: boolean
  template: FlagTemplate | null
  onClose: () => void
  deleteFn: (key: string) => Promise<void>
  invalidateKey: string[]
}

export function DeleteTemplateDialog({
  open,
  template,
  onClose,
  deleteFn,
  invalidateKey,
}: DeleteTemplateDialogProps) {
  const queryClient = useQueryClient()
  const [error, setError] = useState('')

  const deleteMutation = useMutation({
    mutationFn: (key: string) => deleteFn(key),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey })
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
