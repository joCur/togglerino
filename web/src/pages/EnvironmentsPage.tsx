import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment, SDKKey } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useEnvironmentsWrite, useSdkKeysManage, useIsProjectAdmin } from '@/hooks/usePermissions'
import { ArrowUp, ArrowDown, Trash2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

export default function EnvironmentsPage() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const canWrite = useEnvironmentsWrite(key)
  const canManageKeys = useSdkKeysManage(key)
  const canDelete = useIsProjectAdmin(key)
  const [showForm, setShowForm] = useState(false)
  const [envKey, setEnvKey] = useState('')
  const [envName, setEnvName] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null)
  const [sdkKeyCount, setSdkKeyCount] = useState<number | null>(null)

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

  const reorderMutation = useMutation({
    mutationFn: (environmentIds: string[]) =>
      api.environments.reorder(key!, environmentIds),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (envKey: string) => api.environments.delete(key!, envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments'] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      setDeleteTarget(null)
      setSdkKeyCount(null)
    },
  })

  const sortedEnvironments = environments ? [...environments].sort((a, b) => a.sort_order - b.sort_order) : undefined

  const handleMoveUp = (index: number) => {
    if (!sortedEnvironments || index <= 0) return
    const reordered = [...sortedEnvironments]
    ;[reordered[index - 1], reordered[index]] = [reordered[index], reordered[index - 1]]
    reorderMutation.mutate(reordered.map((e) => e.id))
  }

  const handleMoveDown = (index: number) => {
    if (!sortedEnvironments || index >= sortedEnvironments.length - 1) return
    const reordered = [...sortedEnvironments]
    ;[reordered[index], reordered[index + 1]] = [reordered[index + 1], reordered[index]]
    reorderMutation.mutate(reordered.map((e) => e.id))
  }

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!envKey.trim() || !envName.trim()) return
    createMutation.mutate({ key: envKey.trim(), name: envName.trim() })
  }

  const handleDeleteClick = async (env: Environment) => {
    setDeleteTarget(env)
    deleteMutation.reset()
    try {
      const keys = await api.get<SDKKey[]>(`/projects/${key}/environments/${env.key}/sdk-keys`)
      setSdkKeyCount(keys.length)
    } catch {
      setSdkKeyCount(0)
    }
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
        {canWrite && !showForm && (
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

      {reorderMutation.error && (
        <Alert variant="destructive" className="mb-4">
          <AlertDescription>
            {reorderMutation.error instanceof Error ? reorderMutation.error.message : 'Failed to reorder environments'}
          </AlertDescription>
        </Alert>
      )}

      {(!sortedEnvironments || sortedEnvironments.length === 0) ? (
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
                {canWrite && (
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider w-[80px]">Order</TableHead>
                )}
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Created</TableHead>
                {canManageKeys && <TableHead className="font-mono text-[11px] uppercase tracking-wider">SDK Keys</TableHead>}
                {canDelete && <TableHead className="font-mono text-[11px] uppercase tracking-wider w-[60px]" />}
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedEnvironments.map((env, index) => (
                <TableRow key={env.id} className="transition-colors hover:bg-[#d4956a]/8">
                  {canWrite && (
                    <TableCell>
                      <div className="flex items-center gap-0.5">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0"
                          disabled={index === 0 || reorderMutation.isPending}
                          onClick={() => handleMoveUp(index)}
                        >
                          <ArrowUp className="w-3.5 h-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0"
                          disabled={index === sortedEnvironments.length - 1 || reorderMutation.isPending}
                          onClick={() => handleMoveDown(index)}
                        >
                          <ArrowDown className="w-3.5 h-3.5" />
                        </Button>
                      </div>
                    </TableCell>
                  )}
                  <TableCell>
                    <span className="font-mono text-xs text-[#d4956a] tracking-wide">{env.key}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">{env.name}</TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">{new Date(env.created_at).toLocaleDateString()}</TableCell>
                  {canManageKeys && (
                    <TableCell>
                      <Link
                        to={`/projects/${key}/environments/${env.key}/sdk-keys`}
                        className="text-[#d4956a] hover:text-[#e0a97e] text-[13px] transition-colors"
                      >
                        Manage SDK Keys
                      </Link>
                    </TableCell>
                  )}
                  {canDelete && (
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                        onClick={() => handleDeleteClick(env)}
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <Dialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) { setDeleteTarget(null); setSdkKeyCount(null); deleteMutation.reset() } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete environment</DialogTitle>
            <DialogDescription>
              {sdkKeyCount && sdkKeyCount > 0
                ? `This environment has ${sdkKeyCount} active SDK key${sdkKeyCount === 1 ? '' : 's'}. Deleting it will revoke all keys and remove all flag configurations for this environment. This cannot be undone.`
                : `All flag configurations for the "${deleteTarget?.name}" environment will be removed. This cannot be undone.`}
            </DialogDescription>
          </DialogHeader>
          {deleteMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete environment'}
              </AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => { setDeleteTarget(null); setSdkKeyCount(null); deleteMutation.reset() }}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => deleteTarget && deleteMutation.mutate(deleteTarget.key)}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
