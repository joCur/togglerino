import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Link } from 'react-router-dom'

function AppIdentitySection() {
  const queryClient = useQueryClient()
  const [editingProject, setEditingProject] = useState<string | null>(null)
  const [editValue, setEditValue] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const identityQuery = useQuery({
    queryKey: ['app-identities-mine'],
    queryFn: () => api.appIdentity.listMine(),
  })

  const identities = identityQuery.data ?? []
  const hasIdentities = identities.length > 0

  const setMutation = useMutation({
    mutationFn: ({ projectKey, appUserID }: { projectKey: string; appUserID: string }) =>
      api.appIdentity.set(projectKey, appUserID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app-identities-mine'] })
      setEditingProject(null)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (projectKey: string) => api.appIdentity.delete(projectKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app-identities-mine'] })
      queryClient.invalidateQueries({ queryKey: ['my-overrides'] })
      setConfirmDelete(null)
    },
  })

  const handleEdit = (projectKey: string, currentValue: string) => {
    setEditValue(currentValue)
    setEditingProject(projectKey)
  }

  if (!hasIdentities && !identityQuery.isLoading) return null

  return (
    <div className="space-y-3">
      <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">App Identities</h2>
      {identityQuery.isLoading ? (
        <p className="text-muted-foreground/60 text-[13px] animate-pulse">Loading identities...</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Project</TableHead>
              <TableHead>App User ID</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {identities.map((identity) => (
              <TableRow key={identity.project_key ?? identity.project_id}>
                <TableCell>
                  <Link to={`/projects/${identity.project_key}`} className="text-[#d4956a] hover:underline">
                    {identity.project_key}
                  </Link>
                </TableCell>
                <TableCell className="font-mono text-sm">{identity.app_user_id}</TableCell>
                <TableCell className="text-right space-x-1">
                  <Button variant="ghost" size="sm" onClick={() => handleEdit(identity.project_key!, identity.app_user_id)}>
                    Edit
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setConfirmDelete(identity.project_key!)}>
                    Remove
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {/* Edit dialog */}
      <Dialog open={editingProject !== null} onOpenChange={(open) => !open && setEditingProject(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              Edit app identity for <span className="font-mono text-[#d4956a]">{editingProject}</span>
            </DialogTitle>
          </DialogHeader>
          <Input
            placeholder="Your app user ID"
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
          />
          {setMutation.error && (
            <p className="text-sm text-destructive">
              {setMutation.error instanceof Error ? setMutation.error.message : 'Failed to update'}
            </p>
          )}
          <DialogFooter>
            <Button
              onClick={() => editingProject && setMutation.mutate({ projectKey: editingProject, appUserID: editValue })}
              disabled={!editValue || setMutation.isPending}
            >
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation dialog */}
      <Dialog open={confirmDelete !== null} onOpenChange={(open) => !open && setConfirmDelete(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove app identity?</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            This will remove your app identity for <span className="font-mono text-[#d4956a]">{confirmDelete}</span> and
            delete all your overrides in that project.
          </p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmDelete(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => confirmDelete && deleteMutation.mutate(confirmDelete)}
              disabled={deleteMutation.isPending}
            >
              Remove
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function MyOverridesPage() {
  const queryClient = useQueryClient()

  const { data: overrides, isLoading, error } = useQuery({
    queryKey: ['my-overrides'],
    queryFn: () => api.overrides.listMine(),
  })

  const deleteMutation = useMutation({
    mutationFn: (o: { projectKey: string; flagKey: string; envKey: string }) =>
      api.overrides.delete(o.projectKey, o.flagKey, o.envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['my-overrides'] })
    },
  })

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading overrides...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load overrides: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-8">
      <h1 className="text-xl font-mono text-[#d4956a] tracking-wide">My Overrides</h1>

      <AppIdentitySection />

      <div className="space-y-3">
        <h2 className="text-sm font-medium text-muted-foreground uppercase tracking-wider">Active Overrides</h2>
        {overrides?.length === 0 ? (
          <p className="text-muted-foreground/60 text-[13px]">No active overrides.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Project</TableHead>
                <TableHead>Flag</TableHead>
                <TableHead>Environment</TableHead>
                <TableHead>Value</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {overrides?.filter((o) => o.project_key && o.flag_key && o.environment_key).map((o) => (
                <TableRow key={o.id}>
                  <TableCell>
                    <Link to={`/projects/${o.project_key}`} className="text-[#d4956a] hover:underline">
                      {o.project_key}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Link
                      to={`/projects/${o.project_key}/flags/${o.flag_key}`}
                      className="font-mono text-sm text-[#d4956a] hover:underline"
                    >
                      {o.flag_key}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">{o.environment_key}</Badge>
                  </TableCell>
                  <TableCell className="font-mono text-sm">{JSON.stringify(o.value)}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {o.expires_at ? new Date(o.expires_at).toLocaleString() : 'Never'}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() =>
                        deleteMutation.mutate({
                          projectKey: o.project_key as string,
                          flagKey: o.flag_key as string,
                          envKey: o.environment_key as string,
                        })
                      }
                    >
                      Remove
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>
    </div>
  )
}
