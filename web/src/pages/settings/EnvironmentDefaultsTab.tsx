import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import type { Environment, SDKKey } from '../../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
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

export default function EnvironmentDefaultsTab() {
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

  // Defaults state
  const [defaults, setDefaults] = useState<{ key: string; name: string; enabled: boolean }[]>([])
  const [defaultsInitialized, setDefaultsInitialized] = useState(false)

  // Environments list
  const { data: environments, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  // Environment defaults
  const { data: defaultsData } = useQuery({
    queryKey: ['projects', key, 'settings', 'environments'],
    queryFn: () => api.get<{ environment_defaults: { key: string; name: string; enabled: boolean }[] }>(
      `/projects/${key}/settings/environments`
    ),
  })

  if (defaultsData && !defaultsInitialized) {
    setDefaults(defaultsData.environment_defaults)
    setDefaultsInitialized(true)
  }

  const createMutation = useMutation({
    mutationFn: (data: { key: string; name: string }) =>
      api.post<Environment>(`/projects/${key}/environments`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments'] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'environments'] })
      setShowForm(false)
      setEnvKey('')
      setEnvName('')
      setDefaultsInitialized(false)
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
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'environments'] })
      setDeleteTarget(null)
      setSdkKeyCount(null)
      setDefaultsInitialized(false)
    },
  })

  const updateDefaultsMutation = useMutation({
    mutationFn: (envDefaults: Record<string, { enabled: boolean }>) =>
      api.put(`/projects/${key}/settings/environments`, { environment_defaults: envDefaults }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'environments'] })
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

  const handleToggleDefault = (toggledKey: string) => {
    const updated = defaults.map(d => d.key === toggledKey ? { ...d, enabled: !d.enabled } : d)
    setDefaults(updated)
    const payload: Record<string, { enabled: boolean }> = {}
    for (const d of updated) {
      payload[d.key] = { enabled: d.enabled }
    }
    updateDefaultsMutation.mutate(payload)
  }

  const getDefault = (envKey: string) => defaults.find(d => d.key === envKey)

  if (isLoading) return null

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div>
          <div className="text-xs text-muted-foreground">
            Manage environments, display order, and default flag state.
          </div>
        </div>
        {canWrite && !showForm && (
          <Button size="sm" onClick={() => setShowForm(true)}>Add Environment</Button>
        )}
      </div>

      {showForm && (
        <form
          className="flex flex-col sm:flex-row gap-3 mb-4 p-4 rounded-lg bg-muted/30 border sm:items-end animate-[fadeIn_200ms_ease]"
          onSubmit={handleCreate}
        >
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Key</Label>
            <Input
              className="w-full sm:w-auto"
              placeholder="e.g. staging"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              className="w-full sm:w-auto"
              placeholder="e.g. Staging"
              value={envName}
              onChange={(e) => setEnvName(e.target.value)}
            />
          </div>
          <Button type="submit" size="sm" disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
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

      {error && (
        <Alert variant="destructive">
          <AlertDescription>
            Failed to load environments: {error instanceof Error ? error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {sortedEnvironments && sortedEnvironments.length > 0 && (
        <>
          <div className="rounded-lg border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  {canWrite && (
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider w-[80px]">Order</TableHead>
                  )}
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Default</TableHead>
                  {canManageKeys && <TableHead className="font-mono text-[11px] uppercase tracking-wider">SDK Keys</TableHead>}
                  {canDelete && <TableHead className="font-mono text-[11px] uppercase tracking-wider w-[60px]" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {sortedEnvironments.map((env, index) => {
                  const envDefault = getDefault(env.key)
                  return (
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
                      <TableCell>
                        {envDefault && (
                          <div className="flex items-center gap-2">
                            <Switch checked={envDefault.enabled} onCheckedChange={() => handleToggleDefault(env.key)} />
                            <span className="text-xs text-muted-foreground">
                              {envDefault.enabled ? 'On' : 'Off'}
                            </span>
                          </div>
                        )}
                      </TableCell>
                      {canManageKeys && (
                        <TableCell>
                          <Link
                            to={`/projects/${key}/environments/${env.key}/sdk-keys`}
                            className="text-[#d4956a] hover:text-[#e0a97e] text-[13px] transition-colors"
                          >
                            SDK Keys
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
                  )
                })}
              </TableBody>
            </Table>
          </div>

          {updateDefaultsMutation.error && (
            <div className="mt-3 text-[13px] text-destructive">
              {updateDefaultsMutation.error instanceof Error ? updateDefaultsMutation.error.message : 'Failed to save default'}
            </div>
          )}
        </>
      )}

      {sortedEnvironments && sortedEnvironments.length === 0 && (
        <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
          No environments yet. Create your first environment to get started.
        </div>
      )}

      {/* Delete confirmation dialog */}
      <Dialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) { setDeleteTarget(null); setSdkKeyCount(null); deleteMutation.reset() } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete environment</DialogTitle>
            <DialogDescription>
              {sdkKeyCount !== null && sdkKeyCount > 0
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
