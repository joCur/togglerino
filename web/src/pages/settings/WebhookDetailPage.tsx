import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Webhook, WebhookTestResult } from '@/api/types'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
} from '@/components/ui/dialog'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

const EVENT_TYPES = [
  { value: 'flag.created', label: 'Flag Created' },
  { value: 'flag.updated', label: 'Flag Updated' },
  { value: 'flag.deleted', label: 'Flag Deleted' },
  { value: 'flag.archived', label: 'Flag Archived' },
  { value: 'flag_config.updated', label: 'Flag Config Updated' },
  { value: 'segment.created', label: 'Segment Created' },
  { value: 'segment.updated', label: 'Segment Updated' },
  { value: 'segment.deleted', label: 'Segment Deleted' },
  { value: 'environment.created', label: 'Environment Created' },
]

function formatTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  if (diffDay < 7) return `${diffDay}d ago`
  return date.toLocaleDateString()
}

export default function WebhookDetailPage() {
  const { key, id } = useParams<{ key: string; id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [editOpen, setEditOpen] = useState(false)
  const [editName, setEditName] = useState('')
  const [editUrl, setEditUrl] = useState('')
  const [editEvents, setEditEvents] = useState<string[]>([])
  const [editError, setEditError] = useState('')
  const [testResult, setTestResult] = useState<WebhookTestResult | null>(null)
  const [testLoading, setTestLoading] = useState(false)

  const { data: webhook, isLoading: webhookLoading } = useQuery({
    queryKey: ['webhooks', key, id],
    queryFn: () => api.webhooks.get(key!, id!),
    enabled: !!key && !!id,
  })

  const { data: deliveriesData, isLoading: deliveriesLoading } = useQuery({
    queryKey: ['webhook-deliveries', key, id],
    queryFn: () => api.webhooks.deliveries(key!, id!),
    enabled: !!key && !!id,
  })

  const deliveries = deliveriesData?.data ?? []

  const updateMutation = useMutation({
    mutationFn: (body: { name?: string; url?: string; event_types?: string[] }) =>
      api.webhooks.update(key!, id!, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks', key, id] })
      queryClient.invalidateQueries({ queryKey: ['webhooks', key] })
      setEditOpen(false)
      setEditError('')
    },
    onError: (err: Error) => {
      setEditError(err.message)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.webhooks.delete(key!, id!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['webhooks', key] })
      navigate(`/projects/${key}/settings/webhooks`)
    },
  })

  const handleEdit = (e: React.FormEvent) => {
    e.preventDefault()
    setEditError('')
    if (!editName.trim() || !editUrl.trim() || editEvents.length === 0) {
      setEditError('Name, URL, and at least one event type are required.')
      return
    }
    updateMutation.mutate({
      name: editName.trim(),
      url: editUrl.trim(),
      event_types: editEvents,
    })
  }

  const openEditDialog = (wh: Webhook) => {
    setEditName(wh.name)
    setEditUrl(wh.url)
    setEditEvents([...wh.event_types])
    setEditError('')
    setEditOpen(true)
  }

  const toggleEditEvent = (value: string) => {
    setEditEvents((prev) =>
      prev.includes(value) ? prev.filter((v) => v !== value) : [...prev, value],
    )
  }

  const handleDelete = () => {
    if (window.confirm(`Delete webhook "${webhook?.name}"? This cannot be undone.`)) {
      deleteMutation.mutate()
    }
  }

  const handleTest = async () => {
    setTestResult(null)
    setTestLoading(true)
    try {
      const result = await api.webhooks.test(key!, id!)
      setTestResult(result)
    } catch (err) {
      setTestResult({
        success: false,
        error: err instanceof Error ? err.message : 'Unknown error',
        duration_ms: 0,
      })
    } finally {
      setTestLoading(false)
    }
  }

  if (webhookLoading) {
    return (
      <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading webhook...
      </div>
    )
  }

  if (!webhook) {
    return (
      <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
        Webhook not found.
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      {/* Back link */}
      <div>
        <Link
          to={`/projects/${key}/settings/webhooks`}
          className="text-[13px] text-muted-foreground/60 hover:text-foreground transition-colors"
        >
          &larr; Back to webhooks
        </Link>
      </div>

      {/* Header */}
      <Card>
        <CardContent className="p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <div className="flex items-center gap-2 mb-1">
                <span className="text-sm font-semibold text-foreground">{webhook.name}</span>
                <Badge variant={webhook.enabled ? 'secondary' : 'outline'} className="text-[11px]">
                  {webhook.enabled ? 'Enabled' : 'Disabled'}
                </Badge>
              </div>
              <div className="text-[13px] text-muted-foreground/60 font-mono break-all">
                {webhook.url}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" variant="outline" onClick={() => openEditDialog(webhook)}>
                Edit
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="border-destructive/50 text-destructive hover:bg-destructive/10"
                onClick={handleDelete}
                disabled={deleteMutation.isPending}
              >
                Delete
              </Button>
            </div>
          </div>

          {/* Event types */}
          <div className="flex flex-wrap gap-1.5 mb-4">
            {webhook.event_types.map((evt) => (
              <Badge key={evt} variant="outline" className="font-mono text-[11px]">
                {evt}
              </Badge>
            ))}
          </div>

          {/* Test button */}
          <div className="flex items-center gap-3">
            <Button size="sm" onClick={handleTest} disabled={testLoading}>
              {testLoading ? 'Sending...' : 'Send Test'}
            </Button>
            {testResult && (
              <span className={`text-[13px] ${testResult.success ? 'text-green-500' : 'text-destructive'}`}>
                {testResult.success ? 'Success' : 'Failed'}
                {testResult.status_code != null && ` (${testResult.status_code})`}
                {testResult.error && ` - ${testResult.error}`}
                {' '}&middot; {testResult.duration_ms}ms
              </span>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Delivery log */}
      <Card>
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-foreground mb-1">
            Delivery Log
          </div>
          <div className="text-[13px] text-muted-foreground/60 mb-4">
            Recent webhook delivery attempts.
          </div>

          {deliveriesLoading ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading deliveries...
            </div>
          ) : deliveries.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
              No deliveries yet.
            </div>
          ) : (
            <div className="rounded-md border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Time
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Event
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Status
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Duration
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Details
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {deliveries.map((d) => (
                    <Collapsible key={d.id} asChild>
                      <>
                        <TableRow className="transition-colors hover:bg-[#d4956a]/8">
                          <TableCell className="text-[13px] text-muted-foreground">
                            {formatTime(d.created_at)}
                          </TableCell>
                          <TableCell className="text-[13px] text-foreground font-mono">
                            {d.event_type}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={d.success ? 'secondary' : 'destructive'}
                              className="font-mono text-[11px]"
                            >
                              {d.success ? `${d.status_code ?? 'OK'}` : `${d.status_code ?? 'ERR'}`}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-[13px] text-muted-foreground">
                            {d.duration_ms != null ? `${d.duration_ms}ms` : '-'}
                          </TableCell>
                          <TableCell>
                            <CollapsibleTrigger asChild>
                              <Button variant="outline" size="sm" className="text-xs h-7">
                                Expand
                              </Button>
                            </CollapsibleTrigger>
                          </TableCell>
                        </TableRow>
                        <CollapsibleContent asChild>
                          <tr>
                            <td colSpan={5} className="p-4 bg-muted/30">
                              <div className="flex flex-col gap-2">
                                {d.error && (
                                  <div className="text-[13px] text-destructive">
                                    Error: {d.error}
                                  </div>
                                )}
                                <div className="text-[11px] font-mono text-muted-foreground uppercase tracking-wider mb-1">
                                  Payload
                                </div>
                                <pre className="text-[12px] font-mono bg-muted rounded p-3 overflow-x-auto max-h-64">
                                  {JSON.stringify(d.payload, null, 2)}
                                </pre>
                                {d.response_body && (
                                  <>
                                    <div className="text-[11px] font-mono text-muted-foreground uppercase tracking-wider mb-1 mt-2">
                                      Response
                                    </div>
                                    <pre className="text-[12px] font-mono bg-muted rounded p-3 overflow-x-auto max-h-32">
                                      {d.response_body}
                                    </pre>
                                  </>
                                )}
                              </div>
                            </td>
                          </tr>
                        </CollapsibleContent>
                      </>
                    </Collapsible>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Edit dialog */}
      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Webhook</DialogTitle>
            <DialogDescription>
              Update the webhook endpoint configuration.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={handleEdit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">
                Name
              </Label>
              <Input
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">
                URL
              </Label>
              <Input
                value={editUrl}
                onChange={(e) => setEditUrl(e.target.value)}
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
                      checked={editEvents.includes(evt.value)}
                      onChange={() => toggleEditEvent(evt.value)}
                      className="accent-[#d4956a] rounded"
                    />
                    {evt.label}
                  </label>
                ))}
              </div>
            </div>
            {editError && (
              <div className="text-[13px] text-destructive">{editError}</div>
            )}
            <DialogFooter>
              <Button
                type="submit"
                size="sm"
                disabled={updateMutation.isPending}
              >
                {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
