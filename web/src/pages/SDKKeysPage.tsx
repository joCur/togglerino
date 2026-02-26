import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { SDKKey } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

function maskKey(key: string): string {
  if (key.length <= 12) return key
  return key.slice(0, 8) + '...' + key.slice(-4)
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

export default function SDKKeysPage() {
  const { key, env } = useParams<{ key: string; env: string }>()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [keyName, setKeyName] = useState('')
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const { data: sdkKeys, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'environments', env, 'sdk-keys'],
    queryFn: () => api.get<SDKKey[]>(`/projects/${key}/environments/${env}/sdk-keys`),
    enabled: !!key && !!env,
  })

  const createMutation = useMutation({
    mutationFn: (data: { name: string }) =>
      api.post<SDKKey>(`/projects/${key}/environments/${env}/sdk-keys`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments', env, 'sdk-keys'] })
      setShowForm(false)
      setKeyName('')
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) =>
      api.delete(`/projects/${key}/environments/${env}/sdk-keys/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments', env, 'sdk-keys'] })
    },
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!keyName.trim()) return
    createMutation.mutate({ name: keyName.trim() })
  }

  const handleRevoke = (sdkKey: SDKKey) => {
    if (window.confirm(`Are you sure you want to revoke the SDK key "${sdkKey.name}"? This action cannot be undone.`)) {
      revokeMutation.mutate(sdkKey.id)
    }
  }

  const handleCopy = async (sdkKey: SDKKey) => {
    try {
      await navigator.clipboard.writeText(sdkKey.key)
      setCopiedId(sdkKey.id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch {
      // Clipboard API may not be available
    }
  }

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading SDK keys...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load SDK keys: {error instanceof Error ? error.message : 'Unknown error'}
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
        <Link to={`/projects/${key}/environments`} className="text-muted-foreground hover:text-foreground transition-colors">
          Environments
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground font-mono text-xs">{env}</span>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">SDK Keys</span>
      </div>

      <div className="flex justify-between items-center mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">SDK Keys</h1>
        {!showForm && (
          <Button onClick={() => setShowForm(true)}>Generate New Key</Button>
        )}
      </div>

      {showForm && (
        <form
          className="flex gap-3 mb-6 p-5 rounded-lg bg-card border items-end animate-[fadeIn_200ms_ease]"
          onSubmit={handleCreate}
        >
          <div className="flex flex-col gap-1.5 flex-1">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              placeholder="e.g. Backend Service Key"
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              autoFocus
            />
          </div>
          <Button type="submit" disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Generating...' : 'Generate'}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => { setShowForm(false); setKeyName(''); createMutation.reset() }}
          >
            Cancel
          </Button>
        </form>
      )}

      {createMutation.error && (
        <Alert variant="destructive" className="mb-4">
          <AlertDescription>
            {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to generate key'}
          </AlertDescription>
        </Alert>
      )}

      {(!sdkKeys || sdkKeys.length === 0) ? (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No SDK keys yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Generate an SDK key to connect your application to this environment.
          </div>
        </div>
      ) : (
        <div className="rounded-lg border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Status</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Created</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sdkKeys.map((sdkKey) => (
                <TableRow key={sdkKey.id} className="transition-colors hover:bg-[#d4956a]/8">
                  <TableCell>
                    <span className="font-mono text-xs text-[#d4956a] tracking-wide">{maskKey(sdkKey.key)}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">{sdkKey.name}</TableCell>
                  <TableCell>
                    {sdkKey.revoked ? (
                      <Badge variant="destructive" className="text-[11px]">Revoked</Badge>
                    ) : (
                      <Badge className="bg-emerald-500/10 text-emerald-400 border-emerald-500/20 text-[11px]">Active</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">{formatDate(sdkKey.created_at)}</TableCell>
                  <TableCell>
                    {!sdkKey.revoked && (
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-xs h-7"
                          onClick={() => handleCopy(sdkKey)}
                        >
                          {copiedId === sdkKey.id ? 'Copied!' : 'Copy'}
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                          onClick={() => handleRevoke(sdkKey)}
                          disabled={revokeMutation.isPending}
                        >
                          Revoke
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}
