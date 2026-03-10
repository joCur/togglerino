import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Webhook } from '@/api/types'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { EVENT_TYPES } from './webhook-constants'

export default function WebhooksTab() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [createOpen, setCreateOpen] = useState(false)
  const [secretDialogOpen, setSecretDialogOpen] = useState(false)
  const [createdSecret, setCreatedSecret] = useState('')
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [selectedEvents, setSelectedEvents] = useState<string[]>([])
  const [error, setError] = useState('')
  const [mutationError, setMutationError] = useState('')
  const [copied, setCopied] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['webhooks', key],
    queryFn: () => api.webhooks.list(key!),
    enabled: !!key,
  })

  const webhooks = data?.data ?? []

  const createMutation = useMutation({
    mutationFn: (body: { name: string; url: string; event_types: string[] }) =>
      api.webhooks.create(key!, body),
    onSuccess: (webhook: Webhook) => {
      queryClient.invalidateQueries({ queryKey: ['webhooks', key] })
      setCreateOpen(false)
      setCreatedSecret(webhook.secret)
      setSecretDialogOpen(true)
      setName('')
      setUrl('')
      setSelectedEvents([])
      setError('')
    },
    onError: (err: Error) => {
      setError(err.message)
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      api.webhooks.update(key!, id, { enabled }),
    onSuccess: () => {
      setMutationError('')
      queryClient.invalidateQueries({ queryKey: ['webhooks', key] })
    },
    onError: (err: Error) => setMutationError(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.webhooks.delete(key!, id),
    onSuccess: () => {
      setMutationError('')
      queryClient.invalidateQueries({ queryKey: ['webhooks', key] })
    },
    onError: (err: Error) => setMutationError(err.message),
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    if (!name.trim() || !url.trim() || selectedEvents.length === 0) {
      setError('Name, URL, and at least one event type are required.')
      return
    }
    createMutation.mutate({ name: name.trim(), url: url.trim(), event_types: selectedEvents })
  }

  const handleCreateOpenChange = (open: boolean) => {
    setCreateOpen(open)
    if (!open) {
      setName('')
      setUrl('')
      setSelectedEvents([])
      setError('')
    }
  }

  const toggleEvent = (value: string) => {
    setSelectedEvents((prev) =>
      prev.includes(value) ? prev.filter((v) => v !== value) : [...prev, value],
    )
  }

  const handleDelete = (id: string, webhookName: string) => {
    if (window.confirm(`Delete webhook "${webhookName}"? This cannot be undone.`)) {
      deleteMutation.mutate(id)
    }
  }

  const handleCopySecret = async () => {
    await navigator.clipboard.writeText(createdSecret)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex flex-col gap-5">
      <Card>
        <CardContent className="p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <div className="text-sm font-semibold text-foreground mb-1">
                Webhooks
              </div>
              <div className="text-[13px] text-muted-foreground/60">
                Manage webhook endpoints for this project.
              </div>
            </div>
            <Dialog open={createOpen} onOpenChange={handleCreateOpenChange}>
              <DialogTrigger asChild>
                <Button size="sm">Create Webhook</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Create Webhook</DialogTitle>
                  <DialogDescription>
                    Add a new webhook endpoint to receive event notifications.
                  </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleCreate} className="flex flex-col gap-4">
                  <div className="flex flex-col gap-1.5">
                    <Label className="font-mono text-[10px] uppercase tracking-wider">
                      Name
                    </Label>
                    <Input
                      value={name}
                      onChange={(e) => setName(e.target.value)}
                      placeholder="My Webhook"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label className="font-mono text-[10px] uppercase tracking-wider">
                      URL
                    </Label>
                    <Input
                      value={url}
                      onChange={(e) => setUrl(e.target.value)}
                      placeholder="https://example.com/webhook"
                      type="url"
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label className="font-mono text-[10px] uppercase tracking-wider">
                      Event Types
                    </Label>
                    <div className="grid grid-cols-2 gap-2">
                      {EVENT_TYPES.map((evt) => (
                        <label
                          key={evt.value}
                          className="flex items-center gap-2 text-[13px] cursor-pointer"
                        >
                          <input
                            type="checkbox"
                            checked={selectedEvents.includes(evt.value)}
                            onChange={() => toggleEvent(evt.value)}
                            className="accent-[#d4956a] rounded"
                          />
                          {evt.label}
                        </label>
                      ))}
                    </div>
                  </div>
                  {error && (
                    <div className="text-[13px] text-destructive">{error}</div>
                  )}
                  <DialogFooter>
                    <Button
                      type="submit"
                      size="sm"
                      disabled={createMutation.isPending}
                    >
                      {createMutation.isPending ? 'Creating...' : 'Create Webhook'}
                    </Button>
                  </DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
          </div>

          {isLoading ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading webhooks...
            </div>
          ) : webhooks.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
              No webhooks configured for this project.
            </div>
          ) : (
            <div className="rounded-md border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Name
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      URL
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Events
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Enabled
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Actions
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {webhooks.map((wh) => (
                    <TableRow
                      key={wh.id}
                      className="transition-colors hover:bg-[#d4956a]/8"
                    >
                      <TableCell className="text-[13px] text-foreground">
                        <button
                          type="button"
                          className="hover:underline text-left cursor-pointer"
                          onClick={() => navigate(`webhooks/${wh.id}`)}
                        >
                          {wh.name}
                        </button>
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        <span className="truncate max-w-[200px] block">
                          {wh.url}
                        </span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="font-mono text-[11px]">
                          {wh.event_types.length} event{wh.event_types.length !== 1 ? 's' : ''}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Switch
                          checked={wh.enabled}
                          onCheckedChange={(checked) =>
                            toggleMutation.mutate({ id: wh.id, enabled: checked })
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                          onClick={() => handleDelete(wh.id, wh.name)}
                          disabled={deleteMutation.isPending}
                        >
                          Delete
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
          {mutationError && (
            <div className="text-[13px] text-destructive mt-4">{mutationError}</div>
          )}
        </CardContent>
      </Card>

      {/* Secret dialog shown after successful creation */}
      <Dialog open={secretDialogOpen} onOpenChange={setSecretDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Webhook Secret</DialogTitle>
            <DialogDescription>
              Save this secret now. It will not be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded bg-muted px-3 py-2 text-[13px] font-mono break-all">
                {createdSecret}
              </code>
              <Button size="sm" variant="outline" onClick={handleCopySecret}>
                {copied ? 'Copied!' : 'Copy'}
              </Button>
            </div>
            <div className="text-[13px] text-destructive">
              This secret is used to verify webhook payloads. Store it securely.
            </div>
          </div>
          <DialogFooter>
            <Button size="sm" onClick={() => setSecretDialogOpen(false)}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
