import { useState } from 'react'
import { Navigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '@/hooks/useAuth'
import { useBaseProjectRole } from '@/hooks/usePermissions'
import { useRoles } from '@/hooks/useRoles'
import { api } from '@/api/client'
import OIDCSettingsTab from './settings/OIDCSettingsTab'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

const noneOption = { value: 'none', label: 'None', description: 'No access unless explicitly assigned' }

function BaseProjectRoleCard() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useBaseProjectRole()
  const { data: roles } = useRoles()
  const baseRoleOptions = [
    ...(roles ?? []).map(r => ({ value: r.name, label: r.name, description: r.description })),
    noneOption,
  ]
  const [overrideRole, setOverrideRole] = useState<string | null>(null)
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')

  const selectedRole = overrideRole ?? data?.base_project_role ?? ''

  const saveMutation = useMutation({
    mutationFn: (role: string) =>
      api.put('/settings/base-project-role', { base_project_role: role }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['base-project-role'] })
      setOverrideRole(null)
      setSuccess('Base project role updated')
      setError('')
    },
    onError: (err: Error) => {
      setError(err.message)
      setSuccess('')
    },
  })

  const handleSave = () => {
    setSuccess('')
    setError('')
    saveMutation.mutate(selectedRole)
  }

  const hasChanged = data?.base_project_role !== selectedRole

  if (isLoading) {
    return (
      <Card className="mb-8">
        <CardContent className="p-6">
          <div className="text-center py-4 text-muted-foreground/60 text-[13px] animate-pulse">
            Loading...
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className="mb-8">
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-1">
          Base Project Role
        </div>
        <div className="text-[13px] text-muted-foreground/60 mb-4">
          Default project access level for all members. Set to &quot;None&quot; to require
          explicit project membership.
        </div>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">
              Default Role
            </Label>
            <Select value={selectedRole} onValueChange={setOverrideRole}>
              <SelectTrigger className="w-[200px]">
                <SelectValue placeholder="Select a role" />
              </SelectTrigger>
              <SelectContent>
                {baseRoleOptions.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {selectedRole && (
              <div className="text-[12px] text-muted-foreground/60">
                {baseRoleOptions.find((o) => o.value === selectedRole)?.description}
              </div>
            )}
          </div>

          {error && <div className="text-[13px] text-destructive">{error}</div>}
          {success && <div className="text-[13px] text-emerald-500">{success}</div>}

          <div>
            <Button
              size="sm"
              onClick={handleSave}
              disabled={saveMutation.isPending || !hasChanged}
            >
              {saveMutation.isPending ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export default function SettingsPage() {
  const { user } = useAuth()

  if (user?.role !== 'admin') {
    return <Navigate to="/projects" replace />
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-lg font-semibold mb-1">Settings</h1>
      <p className="text-sm text-muted-foreground mb-8">Organization settings.</p>

      <BaseProjectRoleCard />
      <OIDCSettingsTab />
    </div>
  )
}
