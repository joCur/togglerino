import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useFlag } from '@togglerino/react'
import { api } from '../api/client.ts'
import type { Segment, Condition } from '../api/types.ts'
import AttributeCombobox from '../components/AttributeCombobox.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { slugify } from '@/lib/utils'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const OPERATOR_GROUPS = [
  {
    label: 'Comparison',
    operators: [
      { value: 'equals', label: 'equals' },
      { value: 'not_equals', label: 'not equals' },
    ],
  },
  {
    label: 'String',
    operators: [
      { value: 'contains', label: 'contains' },
      { value: 'not_contains', label: 'not contains' },
      { value: 'starts_with', label: 'starts with' },
      { value: 'ends_with', label: 'ends with' },
    ],
  },
  {
    label: 'List',
    operators: [
      { value: 'in', label: 'in (comma-separated)' },
      { value: 'not_in', label: 'not in (comma-separated)' },
    ],
  },
  {
    label: 'Numeric',
    operators: [
      { value: 'greater_than', label: '> greater than' },
      { value: 'less_than', label: '< less than' },
      { value: 'gte', label: '>= greater or equal' },
      { value: 'lte', label: '<= less or equal' },
    ],
  },
  {
    label: 'Presence',
    operators: [
      { value: 'exists', label: 'exists' },
      { value: 'not_exists', label: 'not exists' },
    ],
  },
  {
    label: 'Pattern',
    operators: [
      { value: 'matches', label: 'matches (regex)' },
    ],
  },
]

interface ConditionBuilderProps {
  conditions: Condition[]
  onChange: (conditions: Condition[]) => void
  autocompleteEnabled: boolean
}

function ConditionBuilder({ conditions, onChange, autocompleteEnabled }: ConditionBuilderProps) {
  const updateCondition = (index: number, patch: Partial<Condition>) => {
    const updated = [...conditions]
    updated[index] = { ...updated[index], ...patch }
    onChange(updated)
  }

  const removeCondition = (index: number) => {
    onChange(conditions.filter((_, i) => i !== index))
  }

  const addCondition = () => {
    onChange([...conditions, { attribute: '', operator: 'equals', value: '' }])
  }

  return (
    <div className="flex flex-col gap-1.5">
      <div className="text-[11px] text-muted-foreground/60 leading-relaxed mb-1 italic">
        All conditions must match (AND logic).
      </div>
      {conditions.map((cond, idx) => (
        <div key={idx} className="flex flex-col md:flex-row md:items-center gap-1.5">
          {autocompleteEnabled ? (
            <AttributeCombobox
              value={cond.attribute}
              onChange={(val) => updateCondition(idx, { attribute: val })}
            />
          ) : (
            <Input
              className="w-full md:w-[180px] text-xs"
              placeholder="Attribute"
              value={cond.attribute}
              onChange={(e) => updateCondition(idx, { attribute: e.target.value })}
            />
          )}
          <select
            className="w-full md:w-[170px] px-2.5 py-1.5 text-xs border rounded-md bg-input text-foreground outline-none cursor-pointer"
            value={cond.operator}
            onChange={(e) => {
              const op = e.target.value
              const patch: Partial<Condition> = { operator: op }
              if (op === 'exists' || op === 'not_exists') {
                patch.value = ''
              }
              updateCondition(idx, patch)
            }}
          >
            {OPERATOR_GROUPS.map((group) => (
              <optgroup key={group.label} label={group.label}>
                {group.operators.map((op) => (
                  <option key={op.value} value={op.value}>{op.label}</option>
                ))}
              </optgroup>
            ))}
          </select>
          {cond.operator !== 'exists' && cond.operator !== 'not_exists' && (
            <Input
              className="flex-1 text-xs"
              placeholder={
                cond.operator === 'in' || cond.operator === 'not_in'
                  ? 'comma-separated values'
                  : 'Value'
              }
              value={String(cond.value ?? '')}
              onChange={(e) => updateCondition(idx, { value: e.target.value })}
            />
          )}
          {conditions.length > 1 && (
            <Button
              variant="ghost"
              size="sm"
              className="shrink-0 text-destructive h-7 px-2 text-[11px]"
              onClick={() => removeCondition(idx)}
            >
              x
            </Button>
          )}
        </div>
      ))}
      <Button
        variant="outline"
        size="sm"
        className="text-[11px] h-7 self-start"
        onClick={addCondition}
      >
        + Add Condition
      </Button>
    </div>
  )
}

interface CreateDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  projectKey: string
  autocompleteEnabled: boolean
}

function CreateSegmentDialog({ open, onOpenChange, projectKey, autocompleteEnabled }: CreateDialogProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [segmentKey, setSegmentKey] = useState('')
  const [keyTouched, setKeyTouched] = useState(false)
  const [description, setDescription] = useState('')
  const [conditions, setConditions] = useState<Condition[]>([
    { attribute: '', operator: 'equals', value: '' },
  ])

  const createMutation = useMutation({
    mutationFn: (body: { key: string; name: string; description: string; conditions: Condition[] }) =>
      api.segments.create(projectKey, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['segments', projectKey] })
      resetForm()
      onOpenChange(false)
    },
  })

  const resetForm = () => {
    setName('')
    setSegmentKey('')
    setKeyTouched(false)
    setDescription('')
    setConditions([{ attribute: '', operator: 'equals', value: '' }])
    createMutation.reset()
  }

  const handleNameChange = (value: string) => {
    setName(value)
    if (!keyTouched) {
      setSegmentKey(slugify(value))
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!segmentKey.trim() || !name.trim()) return
    createMutation.mutate({
      key: segmentKey.trim(),
      name: name.trim(),
      description: description.trim(),
      conditions: conditions.filter((c) => c.attribute.trim() !== ''),
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
      <DialogContent className="max-w-[560px]">
        <DialogHeader>
          <DialogTitle>Create Segment</DialogTitle>
          <DialogDescription>
            Segments are reusable groups of conditions you can reference in targeting rules.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              placeholder="e.g. Beta Users"
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Key</Label>
            <Input
              className="font-mono"
              placeholder="e.g. beta-users"
              value={segmentKey}
              onChange={(e) => {
                setSegmentKey(e.target.value)
                setKeyTouched(true)
              }}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
            <Textarea
              className="min-h-[60px]"
              placeholder="Optional description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Conditions</Label>
            <ConditionBuilder
              conditions={conditions}
              onChange={setConditions}
              autocompleteEnabled={autocompleteEnabled}
            />
          </div>

          {createMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create segment'}
              </AlertDescription>
            </Alert>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => { resetForm(); onOpenChange(false) }}>
              Cancel
            </Button>
            <Button type="submit" disabled={createMutation.isPending || !segmentKey.trim() || !name.trim()}>
              {createMutation.isPending ? 'Creating...' : 'Create Segment'}
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
  segment: Segment
  projectKey: string
  autocompleteEnabled: boolean
}

function EditSegmentDialog({ open, onOpenChange, segment, projectKey, autocompleteEnabled }: EditDialogProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(segment.name)
  const [description, setDescription] = useState(segment.description)
  const [conditions, setConditions] = useState<Condition[]>(
    segment.conditions.length > 0 ? segment.conditions : [{ attribute: '', operator: 'equals', value: '' }],
  )
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  const { data: usageData } = useQuery({
    queryKey: ['segments', projectKey, segment.key, 'usage'],
    queryFn: () => api.segments.usage(projectKey, segment.key),
    enabled: open,
  })

  const updateMutation = useMutation({
    mutationFn: (body: { name: string; description: string; conditions: Condition[] }) =>
      api.segments.update(projectKey, segment.key, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['segments', projectKey] })
      onOpenChange(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.segments.delete(projectKey, segment.key),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['segments', projectKey] })
      onOpenChange(false)
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return
    updateMutation.mutate({
      name: name.trim(),
      description: description.trim(),
      conditions: conditions.filter((c) => c.attribute.trim() !== ''),
    })
  }

  const referencingFlags = usageData?.referencing_flags ?? []

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[560px]">
        <DialogHeader>
          <DialogTitle>
            Edit Segment
            <span className="ml-2 font-mono text-sm text-[#d4956a]">{segment.key}</span>
          </DialogTitle>
          <DialogDescription>
            Update this segment's name, description, and conditions.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4 mt-2">
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
            <Textarea
              className="min-h-[60px]"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Conditions</Label>
            <ConditionBuilder
              conditions={conditions}
              onChange={setConditions}
              autocompleteEnabled={autocompleteEnabled}
            />
          </div>

          {referencingFlags.length > 0 && (
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Used by</Label>
              <div className="flex flex-wrap gap-1.5">
                {referencingFlags.map((flagKey) => (
                  <Link
                    key={flagKey}
                    to={`/projects/${projectKey}/flags/${flagKey}`}
                    className="inline-block font-mono text-xs text-[#d4956a] hover:text-[#e0a97e] transition-colors bg-[#d4956a]/10 px-2 py-0.5 rounded"
                    onClick={() => onOpenChange(false)}
                  >
                    {flagKey}
                  </Link>
                ))}
              </div>
            </div>
          )}

          {updateMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {updateMutation.error instanceof Error ? updateMutation.error.message : 'Failed to update segment'}
              </AlertDescription>
            </Alert>
          )}

          {deleteMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete segment'}
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
            <Button type="submit" disabled={updateMutation.isPending || !name.trim()}>
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export default function SegmentsPage() {
  const { key } = useParams<{ key: string }>()
  const [createOpen, setCreateOpen] = useState(false)
  const [editSegment, setEditSegment] = useState<Segment | null>(null)
  const autocompleteEnabled = useFlag('context-attribute-autocomplete', false)

  const { data: segments, isLoading, error } = useQuery({
    queryKey: ['segments', key],
    queryFn: () => api.segments.list(key!),
    enabled: !!key,
  })

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading segments...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load segments: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {key}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Segments</span>
      </div>

      <div className="flex flex-col gap-3 md:flex-row md:justify-between md:items-center mb-6">
        <div>
          <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Segments</h1>
          <p className="text-[13px] text-muted-foreground/60 mt-0.5">
            Reusable groups of conditions for targeting rules.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>Create Segment</Button>
      </div>

      {(!segments || segments.length === 0) ? (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No segments yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Create a segment to define reusable targeting conditions across your flags.
          </div>
        </div>
      ) : (
        <div className="rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Conditions</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Last Updated</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {segments.map((segment) => (
                <TableRow
                  key={segment.id}
                  className="transition-colors hover:bg-[#d4956a]/8 cursor-pointer"
                  onClick={() => setEditSegment(segment)}
                >
                  <TableCell>
                    <span className="font-mono text-xs text-[#d4956a] tracking-wide">{segment.key}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">{segment.name}</TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">
                    {segment.conditions.length} condition{segment.conditions.length !== 1 ? 's' : ''}
                  </TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">
                    {new Date(segment.updated_at).toLocaleDateString()}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CreateSegmentDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        projectKey={key!}
        autocompleteEnabled={autocompleteEnabled}
      />

      {editSegment && (
        <EditSegmentDialog
          key={editSegment.id}
          open={!!editSegment}
          onOpenChange={(open) => { if (!open) setEditSegment(null) }}
          segment={editSegment}
          projectKey={key!}
          autocompleteEnabled={autocompleteEnabled}
        />
      )}
    </div>
  )
}
