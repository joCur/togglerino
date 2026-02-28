import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export default function EnvironmentsPage() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [envKey, setEnvKey] = useState('')
  const [envName, setEnvName] = useState('')

  const { data: environments, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const createMutation = useMutation({
    mutationFn: (data: { key: string; name: string }) =>
      api.post<Environment>(`/projects/${key}/environments`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments'] })
      setShowForm(false)
      setEnvKey('')
      setEnvName('')
    },
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!envKey.trim() || !envName.trim()) return
    createMutation.mutate({ key: envKey.trim(), name: envName.trim() })
  }

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading environments...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load environments: {error instanceof Error ? error.message : 'Unknown error'}
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
        <span className="text-foreground">Environments</span>
      </div>

      <div className="flex flex-col gap-3 md:flex-row md:justify-between md:items-center mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Environments</h1>
        {!showForm && (
          <Button onClick={() => setShowForm(true)}>Create Environment</Button>
        )}
      </div>

      {showForm && (
        <form
          className="flex flex-col md:flex-row gap-3 mb-6 p-5 rounded-lg bg-card border md:items-end animate-[fadeIn_200ms_ease]"
          onSubmit={handleCreate}
        >
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Key</Label>
            <Input
              className="w-full md:w-auto"
              placeholder="e.g. staging"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              className="w-full md:w-auto"
              placeholder="e.g. Staging"
              value={envName}
              onChange={(e) => setEnvName(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => { setShowForm(false); setEnvKey(''); setEnvName(''); createMutation.reset() }}
          >
            Cancel
          </Button>
        </form>
      )}

      {createMutation.error && (
        <Alert variant="destructive" className="mb-4">
          <AlertDescription>
            {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create environment'}
          </AlertDescription>
        </Alert>
      )}

      {(!environments || environments.length === 0) ? (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No environments yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Create your first environment to start configuring feature flags per environment.
          </div>
        </div>
      ) : (
        <div className="rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Created</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">SDK Keys</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {environments.map((env) => (
                <TableRow key={env.id} className="transition-colors hover:bg-[#d4956a]/8">
                  <TableCell>
                    <span className="font-mono text-xs text-[#d4956a] tracking-wide">{env.key}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">{env.name}</TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">{new Date(env.created_at).toLocaleDateString()}</TableCell>
                  <TableCell>
                    <Link
                      to={`/projects/${key}/environments/${env.key}/sdk-keys`}
                      className="text-[#d4956a] hover:text-[#e0a97e] text-[13px] transition-colors"
                    >
                      Manage SDK Keys
                    </Link>
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
