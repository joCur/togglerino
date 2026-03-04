import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import type { FlagTemplate, FlagPurpose, ValueType } from '../../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const FLAG_TYPES: FlagPurpose[] = ['release', 'experiment', 'operational', 'kill-switch', 'permission']
const VALUE_TYPES: ValueType[] = ['boolean', 'string', 'number', 'json']

const FLAG_TYPE_LABELS: Record<FlagPurpose, string> = {
  'release': 'Release',
  'experiment': 'Experiment',
  'operational': 'Operational',
  'kill-switch': 'Kill Switch',
  'permission': 'Permission',
}

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

function getDefaultValueForType(valueType: ValueType): unknown {
  switch (valueType) {
    case 'boolean': return false
    case 'number': return 0
    case 'json': return {}
    default: return ''
  }
}

function formatDefaultValue(value: unknown, valueType: ValueType): string {
  if (valueType === 'boolean') return String(value)
  if (valueType === 'json') return JSON.stringify(value)
  return String(value ?? '')
}

interface TemplateFormState {
  name: string
  key: string
  description: string
  flag_type: FlagPurpose
  value_type: ValueType
  default_value_str: string
  tags_str: string
}

function defaultFormState(): TemplateFormState {
  return {
    name: '',
    key: '',
    description: '',
    flag_type: 'release',
    value_type: 'boolean',
    default_value_str: 'false',
    tags_str: '',
  }
}

function parseDefaultValue(str: string, valueType: ValueType): unknown {
  if (valueType === 'boolean') return str === 'true'
  if (valueType === 'number') {
    const n = Number(str)
    return isNaN(n) ? 0 : n
  }
  if (valueType === 'json') {
    try { return JSON.parse(str) } catch { return {} }
  }
  return str
}

interface CreateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectKey: string
}

function CreateTemplateDialog({ open, onOpenChange, projectKey }: CreateDialogProps) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<TemplateFormState>(defaultFormState())
  const [keyTouched, setKeyTouched] = useState(false)

  const createMutation = useMutation({
    mutationFn: (body: Partial<FlagTemplate>) => api.templates.createForProject(projectKey, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'templates'] })
      resetForm()
      onOpenChange(false)
    },
  })

  const resetForm = () => {
    setForm(defaultFormState())
    setKeyTouched(false)
    createMutation.reset()
  }

  const handleNameChange = (value: string) => {
    setForm(prev => ({
      ...prev,
      name: value,
      key: keyTouched ? prev.key : slugify(value),
    }))
  }

  const handleValueTypeChange = (vt: ValueType) => {
    setForm(prev => ({
      ...prev,
      value_type: vt,
      default_value_str: formatDefaultValue(getDefaultValueForType(vt), vt),
    }))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.key.trim() || !form.name.trim()) return
    const tags = form.tags_str
      .split(',')
      .map(t => t.trim())
      .filter(Boolean)
    createMutation.mutate({
      key: form.key.trim(),
      name: form.name.trim(),
      description: form.description.trim(),
      flag_type: form.flag_type,
      value_type: form.value_type,
      default_value: parseDefaultValue(form.default_value_str, form.value_type),
      tags,
    })
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) resetForm()
        onOpenChange(v)
      }}
    >
      <DialogContent className="max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Create Template</DialogTitle>
          <DialogDescription>
            Create a project-specific template for quickly creating flags with pre-filled settings.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              placeholder="e.g. Percentage Rollout"
              value={form.name}
              onChange={(e) => handleNameChange(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Key</Label>
            <Input
              className="font-mono"
              placeholder="e.g. percentage-rollout"
              value={form.key}
              onChange={(e) => {
                setForm(prev => ({ ...prev, key: e.target.value }))
                setKeyTouched(true)
              }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
            <Textarea
              className="min-h-[60px]"
              placeholder="Optional description"
              value={form.description}
              onChange={(e) => setForm(prev => ({ ...prev, description: e.target.value }))}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Flag Type</Label>
              <Select
                value={form.flag_type}
                onValueChange={(v) => setForm(prev => ({ ...prev, flag_type: v as FlagPurpose }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FLAG_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>{FLAG_TYPE_LABELS[t]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Value Type</Label>
              <Select
                value={form.value_type}
                onValueChange={(v) => handleValueTypeChange(v as ValueType)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VALUE_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>{t}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Default Value</Label>
            {form.value_type === 'boolean' ? (
              <Select
                value={form.default_value_str}
                onValueChange={(v) => setForm(prev => ({ ...prev, default_value_str: v }))}
              >
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
                className={form.value_type === 'json' ? 'font-mono text-xs' : ''}
                placeholder={form.value_type === 'json' ? '{"key": "value"}' : form.value_type === 'number' ? '0' : ''}
                value={form.default_value_str}
                onChange={(e) => setForm(prev => ({ ...prev, default_value_str: e.target.value }))}
              />
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Tags</Label>
            <Input
              placeholder="comma-separated, e.g. backend, infra"
              value={form.tags_str}
              onChange={(e) => setForm(prev => ({ ...prev, tags_str: e.target.value }))}
            />
          </div>

          {createMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create template'}
              </AlertDescription>
            </Alert>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => { resetForm(); onOpenChange(false) }}>
              Cancel
            </Button>
            <Button type="submit" disabled={createMutation.isPending || !form.key.trim() || !form.name.trim()}>
              {createMutation.isPending ? 'Creating...' : 'Create Template'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface EditDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  template: FlagTemplate
  projectKey: string
}

function EditTemplateDialog({ open, onOpenChange, template, projectKey }: EditDialogProps) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<TemplateFormState>({
    name: template.name,
    key: template.key,
    description: template.description,
    flag_type: template.flag_type,
    value_type: template.value_type,
    default_value_str: formatDefaultValue(template.default_value, template.value_type),
    tags_str: (template.tags ?? []).join(', '),
  })
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const handleValueTypeChange = (vt: ValueType) => {
    setForm(prev => ({
      ...prev,
      value_type: vt,
      default_value_str: formatDefaultValue(getDefaultValueForType(vt), vt),
    }))
  }

  const updateMutation = useMutation({
    mutationFn: (body: Partial<FlagTemplate>) => api.templates.updateForProject(projectKey, template.key, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'templates'] })
      onOpenChange(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.templates.deleteForProject(projectKey, template.key),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'templates'] })
      onOpenChange(false)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name.trim()) return
    const tags = form.tags_str
      .split(',')
      .map(t => t.trim())
      .filter(Boolean)
    updateMutation.mutate({
      name: form.name.trim(),
      description: form.description.trim(),
      flag_type: form.flag_type,
      value_type: form.value_type,
      default_value: parseDefaultValue(form.default_value_str, form.value_type),
      tags,
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[520px]">
        <DialogHeader>
          <DialogTitle>
            Edit Template
            <span className="ml-2 font-mono text-sm text-[#d4956a]">{template.key}</span>
          </DialogTitle>
          <DialogDescription>
            Update this project template's settings.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              value={form.name}
              onChange={(e) => setForm(prev => ({ ...prev, name: e.target.value }))}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
            <Textarea
              className="min-h-[60px]"
              value={form.description}
              onChange={(e) => setForm(prev => ({ ...prev, description: e.target.value }))}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Flag Type</Label>
              <Select
                value={form.flag_type}
                onValueChange={(v) => setForm(prev => ({ ...prev, flag_type: v as FlagPurpose }))}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FLAG_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>{FLAG_TYPE_LABELS[t]}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Value Type</Label>
              <Select
                value={form.value_type}
                onValueChange={(v) => handleValueTypeChange(v as ValueType)}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VALUE_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>{t}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Default Value</Label>
            {form.value_type === 'boolean' ? (
              <Select
                value={form.default_value_str}
                onValueChange={(v) => setForm(prev => ({ ...prev, default_value_str: v }))}
              >
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
                className={form.value_type === 'json' ? 'font-mono text-xs' : ''}
                placeholder={form.value_type === 'json' ? '{"key": "value"}' : form.value_type === 'number' ? '0' : ''}
                value={form.default_value_str}
                onChange={(e) => setForm(prev => ({ ...prev, default_value_str: e.target.value }))}
              />
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Tags</Label>
            <Input
              placeholder="comma-separated, e.g. backend, infra"
              value={form.tags_str}
              onChange={(e) => setForm(prev => ({ ...prev, tags_str: e.target.value }))}
            />
          </div>

          {updateMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {updateMutation.error instanceof Error ? updateMutation.error.message : 'Failed to update template'}
              </AlertDescription>
            </Alert>
          )}

          {deleteMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete template'}
              </AlertDescription>
            </Alert>
          )}

          <DialogFooter className="flex-col sm:flex-row gap-2">
            {!showDeleteConfirm ? (
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
                  disabled={deleteMutation.isPending}
                  onClick={() => deleteMutation.mutate()}
                >
                  {deleteMutation.isPending ? 'Deleting...' : 'Confirm Delete'}
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
            )}
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={updateMutation.isPending || !form.name.trim()}>
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export default function TemplatesTab() {
  const { key: projectKey } = useParams<{ key: string }>()
  const [createOpen, setCreateOpen] = useState(false)
  const [editTemplate, setEditTemplate] = useState<FlagTemplate | null>(null)

  const { data, isLoading, error } = useQuery({
    queryKey: ['projects', projectKey, 'templates'],
    queryFn: () => api.templates.listForProject(projectKey!),
    enabled: !!projectKey,
  })

  const templates = data?.project ?? []

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading templates...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load templates: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <div className="text-sm font-semibold text-foreground">Project Templates</div>
          <div className="text-[13px] text-muted-foreground/60 mt-0.5">
            Templates specific to this project for quickly creating flags with pre-filled settings.
          </div>
        </div>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          Create Template
        </Button>
      </div>

      {templates.length === 0 ? (
        <div className="text-center py-12 border rounded-lg">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No project templates yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Create templates to help your team create flags consistently.
          </div>
        </div>
      ) : (
        <div className="rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Type</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Value</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Tags</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {templates.map((template) => (
                <TableRow
                  key={template.id}
                  className="transition-colors hover:bg-[#d4956a]/8 cursor-pointer"
                  onClick={() => setEditTemplate(template)}
                >
                  <TableCell>
                    <span className="font-mono text-xs text-[#d4956a] tracking-wide">{template.key}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">{template.name}</TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">
                    <span className="font-mono text-xs">{template.flag_type}</span>
                    {' / '}
                    <span className="font-mono text-xs">{template.value_type}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">
                    <span className="font-mono text-xs">
                      {formatDefaultValue(template.default_value, template.value_type)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {(template.tags ?? []).map((tag) => (
                        <Badge key={tag} variant="secondary" className="text-[10px] font-mono px-1.5 py-0">
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CreateTemplateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        projectKey={projectKey!}
      />

      {editTemplate && (
        <EditTemplateDialog
          key={editTemplate.id}
          open={!!editTemplate}
          onOpenChange={(open) => { if (!open) setEditTemplate(null) }}
          template={editTemplate}
          projectKey={projectKey!}
        />
      )}
    </div>
  )
}
