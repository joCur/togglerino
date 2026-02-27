import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag } from '../api/types.ts'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

interface Props {
  open: boolean
  projectKey: string
  onClose: () => void
  onCreated?: () => void
  initialKey?: string
}

const FLAG_PURPOSES = [
  { value: 'release', label: 'Release', description: 'Deploy new features', lifetime: '40 days' },
  { value: 'experiment', label: 'Experiment', description: 'A/B testing', lifetime: '40 days' },
  { value: 'operational', label: 'Operational', description: 'Technical migration', lifetime: '7 days' },
  { value: 'kill-switch', label: 'Kill Switch', description: 'Graceful degradation', lifetime: 'Permanent' },
  { value: 'permission', label: 'Permission', description: 'Access control', lifetime: 'Permanent' },
]

const VALUE_TYPES = [
  { value: 'boolean', label: 'Boolean' },
  { value: 'string', label: 'String' },
  { value: 'number', label: 'Number' },
  { value: 'json', label: 'JSON' },
]

function slugify(text: string): string {
  return text.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

export default function CreateFlagModal({ open, projectKey, onClose, onCreated, initialKey }: Props) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [key, setKey] = useState(initialKey ?? '')
  const [keyManual, setKeyManual] = useState(!!initialKey)
  const [description, setDescription] = useState('')
  const [flagType, setFlagType] = useState('boolean')
  const [flagPurpose, setFlagPurpose] = useState('release')
  const [defaultValue, setDefaultValue] = useState<string>('false')
  const [boolValue, setBoolValue] = useState(false)
  const [tags, setTags] = useState('')

  const mutation = useMutation({
    mutationFn: (data: {
      key: string; name: string; description: string
      value_type: string; flag_type: string; default_value: unknown; tags: string[]
    }) => api.post<Flag>(`/projects/${projectKey}/flags`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags'] })
      onCreated?.()
      resetAndClose()
    },
  })

  const resetAndClose = () => {
    setName(''); setKey(''); setKeyManual(false); setDescription('')
    setFlagType('boolean'); setFlagPurpose('release'); setDefaultValue('false'); setBoolValue(false); setTags('')
    mutation.reset(); onClose()
  }

  const handleNameChange = (val: string) => {
    setName(val)
    if (!keyManual) setKey(slugify(val))
  }

  const handleKeyChange = (val: string) => { setKeyManual(true); setKey(val) }

  const handleTypeChange = (type: string) => {
    setFlagType(type)
    if (type === 'boolean') { setDefaultValue('false'); setBoolValue(false) }
    else if (type === 'number') setDefaultValue('0')
    else if (type === 'json') setDefaultValue('{}')
    else setDefaultValue('')
  }

  const getDefaultValueParsed = (): unknown => {
    if (flagType === 'boolean') return boolValue
    if (flagType === 'number') { const n = Number(defaultValue); return isNaN(n) ? 0 : n }
    if (flagType === 'json') { try { return JSON.parse(defaultValue) } catch { return defaultValue } }
    return defaultValue
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const parsedTags = tags.split(',').map((tag) => tag.trim()).filter(Boolean)
    mutation.mutate({
      key, name, description,
      value_type: flagType,
      flag_type: flagPurpose,
      default_value: getDefaultValueParsed(),
      tags: parsedTags,
    })
  }

  const errorMsg = mutation.error instanceof Error ? mutation.error.message : ''

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) resetAndClose() }}>
      <DialogContent className="sm:max-w-[500px] max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create Flag</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit}>
          {errorMsg && (
            <Alert variant="destructive" className="mb-5">
              <AlertDescription>{errorMsg}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label>Name</Label>
              <Input value={name} onChange={(e) => handleNameChange(e.target.value)} placeholder="Dark Mode" required autoFocus />
            </div>

            <div className="space-y-1.5">
              <Label>Key</Label>
              <Input className="font-mono text-xs" value={key} onChange={(e) => handleKeyChange(e.target.value)} placeholder="dark-mode" required />
            </div>

            <div className="space-y-1.5">
              <Label>Description</Label>
              <Textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional description" className="min-h-[72px] resize-y" />
            </div>

            <div className="space-y-1.5">
              <Label>Flag Purpose</Label>
              <Select value={flagPurpose} onValueChange={setFlagPurpose}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FLAG_PURPOSES.map((fp) => (
                    <SelectItem key={fp.value} value={fp.value}>
                      <span>{fp.label}</span>
                      <span className="ml-2 text-muted-foreground text-xs">{fp.lifetime}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="text-xs text-muted-foreground">
                {FLAG_PURPOSES.find(fp => fp.value === flagPurpose)?.description}
              </div>
            </div>

            <div className="space-y-1.5">
              <Label>Value Type</Label>
              <Select value={flagType} onValueChange={handleTypeChange}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {VALUE_TYPES.map((ft) => (
                    <SelectItem key={ft.value} value={ft.value}>{ft.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>Default Value</Label>
              {flagType === 'boolean' ? (
                <div className="flex items-center gap-2.5">
                  <Switch checked={boolValue} onCheckedChange={setBoolValue} />
                  <span className="text-[13px] font-mono text-foreground">{boolValue ? 'true' : 'false'}</span>
                </div>
              ) : flagType === 'number' ? (
                <Input type="number" value={defaultValue} onChange={(e) => setDefaultValue(e.target.value)} />
              ) : flagType === 'json' ? (
                <Textarea className="font-mono text-xs min-h-[72px]" value={defaultValue} onChange={(e) => setDefaultValue(e.target.value)} placeholder='{"key": "value"}' />
              ) : (
                <Input value={defaultValue} onChange={(e) => setDefaultValue(e.target.value)} placeholder="Default string value" />
              )}
            </div>

            <div className="space-y-1.5">
              <Label>Tags (comma-separated)</Label>
              <Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="ui, experiment, beta" />
            </div>
          </div>

          <div className="flex justify-end gap-2.5 mt-6">
            <Button type="button" variant="outline" onClick={resetAndClose}>Cancel</Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? 'Creating...' : 'Create Flag'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
