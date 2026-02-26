import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface Props {
  open: boolean
  onClose: () => void
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

export default function CreateProjectModal({ open, onClose }: Props) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const [keyManual, setKeyManual] = useState(false)
  const [description, setDescription] = useState('')

  const mutation = useMutation({
    mutationFn: (data: { key: string; name: string; description: string }) =>
      api.post<Project>('/projects', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      resetAndClose()
    },
  })

  const resetAndClose = () => {
    setName('')
    setKey('')
    setKeyManual(false)
    setDescription('')
    mutation.reset()
    onClose()
  }

  const handleNameChange = (val: string) => {
    setName(val)
    if (!keyManual) {
      setKey(slugify(val))
    }
  }

  const handleKeyChange = (val: string) => {
    setKeyManual(true)
    setKey(val)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    mutation.mutate({ key, name, description })
  }

  const errorMsg = mutation.error instanceof Error ? mutation.error.message : ''

  return (
    <Dialog open={open} onOpenChange={(isOpen) => { if (!isOpen) resetAndClose() }}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>Create Project</DialogTitle>
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
              <Input value={name} onChange={(e) => handleNameChange(e.target.value)} placeholder="My Project" required autoFocus />
            </div>

            <div className="space-y-1.5">
              <Label>Key</Label>
              <Input className="font-mono text-xs" value={key} onChange={(e) => handleKeyChange(e.target.value)} placeholder="my-project" required />
            </div>

            <div className="space-y-1.5">
              <Label>Description</Label>
              <Textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional description" className="min-h-[80px] resize-y" />
            </div>
          </div>

          <div className="flex justify-end gap-2.5 mt-6">
            <Button type="button" variant="outline" onClick={resetAndClose}>Cancel</Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? 'Creating...' : 'Create Project'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
