import { useState } from 'react'
import { useRoles, useCreateRole, useUpdateRole, useDeleteRole } from '@/hooks/useRoles'
import type { RoleDefinition } from '@/hooks/useRoles'
import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { useIsOrgAdmin } from '@/hooks/usePermissions'
import { Navigate } from 'react-router-dom'

const permissionLabels: Record<string, string> = {
  'flags:read': 'View flags',
  'flags:write': 'Create & edit flags',
  'environments:read': 'View environments',
  'environments:write': 'Create environments',
  'sdk_keys:manage': 'Manage SDK keys',
  'segments:write': 'Create & edit segments',
  'templates:manage': 'Manage templates',
  'project:settings': 'Project settings',
}

const allPermissions = Object.keys(permissionLabels)

interface RoleFormState {
  name: string
  description: string
  permissions: string[]
}

const emptyForm: RoleFormState = { name: '', description: '', permissions: [] }

export default function RolesPage() {
  const isAdmin = useIsOrgAdmin()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingRole, setEditingRole] = useState<RoleDefinition | null>(null)
  const [form, setForm] = useState<RoleFormState>(emptyForm)
  const [error, setError] = useState('')
  const [deleteError, setDeleteError] = useState('')

  const { data: roles, isLoading } = useRoles()
  const createRole = useCreateRole()
  const updateRole = useUpdateRole()
  const deleteRole = useDeleteRole()

  const openCreate = () => {
    setEditingRole(null)
    setForm(emptyForm)
    setError('')
    setDialogOpen(true)
  }

  const openEdit = (role: RoleDefinition) => {
    setEditingRole(role)
    setForm({
      name: role.name,
      description: role.description,
      permissions: [...role.permissions],
    })
    setError('')
    setDialogOpen(true)
  }

  const togglePermission = (perm: string) => {
    setForm((prev) => ({
      ...prev,
      permissions: prev.permissions.includes(perm)
        ? prev.permissions.filter((p) => p !== perm)
        : [...prev.permissions, perm],
    }))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (!form.name.trim()) {
      setError('Name is required')
      return
    }
    if (form.permissions.length === 0) {
      setError('At least one permission is required')
      return
    }

    const payload = {
      name: form.name.trim(),
      description: form.description.trim(),
      permissions: form.permissions,
    }

    if (editingRole) {
      updateRole.mutate(payload, {
        onSuccess: () => setDialogOpen(false),
        onError: (err: Error) => setError(err.message),
      })
    } else {
      createRole.mutate(payload, {
        onSuccess: () => setDialogOpen(false),
        onError: (err: Error) => setError(err.message),
      })
    }
  }

  const handleDelete = (role: RoleDefinition) => {
    if (!window.confirm(`Are you sure you want to delete the "${role.name}" role? This action cannot be undone.`)) {
      return
    }
    setDeleteError('')
    deleteRole.mutate(role.name, {
      onError: (err: Error) => {
        if (err instanceof ApiError && err.status === 409) {
          setDeleteError(`Cannot delete "${role.name}": role is currently in use.`)
        } else {
          setDeleteError(err.message)
        }
      },
    })
  }

  const isSaving = createRole.isPending || updateRole.isPending

  if (!isAdmin) return <Navigate to="/projects" replace />

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-lg font-semibold">Roles</h1>
          <p className="text-[13px] text-muted-foreground/60">
            Manage project roles and their permissions.
          </p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger asChild>
            <Button size="sm" onClick={openCreate}>Create Role</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{editingRole ? 'Edit Role' : 'Create Role'}</DialogTitle>
              <DialogDescription>
                {editingRole
                  ? 'Update the role name, description, and permissions.'
                  : 'Define a new project role with specific permissions.'}
              </DialogDescription>
            </DialogHeader>
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
                <Input
                  value={form.name}
                  onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                  placeholder="e.g. reviewer"
                  disabled={!!editingRole}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
                <Input
                  value={form.description}
                  onChange={(e) => setForm((prev) => ({ ...prev, description: e.target.value }))}
                  placeholder="A brief description of this role"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Permissions</Label>
                <div className="grid grid-cols-2 gap-3">
                  {allPermissions.map((perm) => (
                    <label
                      key={perm}
                      className="flex items-center gap-2 cursor-pointer text-[13px] text-foreground hover:bg-[#d4956a]/8 rounded px-2 py-1.5 transition-colors"
                    >
                      <input
                        type="checkbox"
                        checked={form.permissions.includes(perm)}
                        onChange={() => togglePermission(perm)}
                        className="h-4 w-4 rounded border border-border bg-input accent-[#d4956a]"
                      />
                      {permissionLabels[perm]}
                    </label>
                  ))}
                </div>
              </div>
              {error && (
                <Alert variant="destructive">
                  <AlertDescription>{error}</AlertDescription>
                </Alert>
              )}
              <DialogFooter>
                <Button type="submit" size="sm" disabled={isSaving}>
                  {isSaving ? 'Saving...' : editingRole ? 'Update Role' : 'Create Role'}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {deleteError && (
        <Alert variant="destructive" className="mb-4">
          <AlertDescription>{deleteError}</AlertDescription>
        </Alert>
      )}

      <Card>
        <CardContent className="p-6">
          {isLoading ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading roles...
            </div>
          ) : !roles || roles.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
              No roles found.
            </div>
          ) : (
            <div className="rounded-md border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Description</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Permissions</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {roles.map((role) => (
                    <TableRow key={role.id} className="transition-colors hover:bg-[#d4956a]/8">
                      <TableCell className="text-[13px] text-foreground">
                        <div className="flex items-center gap-2">
                          <span className="font-medium">{role.name}</span>
                          {role.is_built_in && (
                            <Badge variant="secondary" className="font-mono text-[10px]">
                              built-in
                            </Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        {role.description || '\u2014'}
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        <div className="flex flex-wrap gap-1">
                          {role.permissions.map((perm) => (
                            <Badge key={perm} variant="outline" className="font-mono text-[10px]">
                              {permissionLabels[perm] ?? perm}
                            </Badge>
                          ))}
                        </div>
                      </TableCell>
                      <TableCell>
                        {!role.is_built_in && (
                          <div className="flex items-center gap-2">
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-xs h-7"
                              onClick={() => openEdit(role)}
                            >
                              Edit
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                              onClick={() => handleDelete(role)}
                              disabled={deleteRole.isPending}
                            >
                              Delete
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
        </CardContent>
      </Card>
    </div>
  )
}
