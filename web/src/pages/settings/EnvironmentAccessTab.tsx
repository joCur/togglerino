import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useRoles } from '@/hooks/useRoles'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import type { EnvironmentAccessRestriction, EnvironmentAccessResponse } from '@/api/types'
import type { RoleDefinition } from '@/hooks/useRoles'

function EnvironmentAccessGrid({ data, writableRoles }: { data: EnvironmentAccessResponse; writableRoles: RoleDefinition[] }) {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()

  // Build initial map from server data - this is the "reset" baseline
  const initialMap: Record<string, string[] | null> = {}
  for (const r of data.restrictions) {
    initialMap[r.role_name] = r.environment_ids
  }

  // Use React state seeded from server data; re-mount via key resets it
  const [accessMap, setAccessMap] = useState<Record<string, string[] | null>>(initialMap)
  const [isDirty, setIsDirty] = useState(false)
  const [saved, setSaved] = useState(false)

  const saveMutation = useMutation({
    mutationFn: () => {
      const restrictions: EnvironmentAccessRestriction[] = []
      for (const [roleName, envIds] of Object.entries(accessMap)) {
        if (envIds !== null) {
          restrictions.push({ role_name: roleName, environment_ids: envIds })
        }
      }
      return api.environmentAccess.update(key!, restrictions)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environment-access'] })
      setIsDirty(false)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const handleToggle = (roleName: string, envId: string, checked: boolean) => {
    const allEnvIds = data.environments.map(e => e.id)
    const current = accessMap[roleName] ?? allEnvIds

    let next: string[]
    if (checked) {
      next = [...current, envId]
    } else {
      next = current.filter(id => id !== envId)
    }

    const isUnrestricted = allEnvIds.every(id => next.includes(id))

    setAccessMap(prev => ({
      ...prev,
      [roleName]: isUnrestricted ? null : next,
    }))
    setIsDirty(true)
  }

  const isChecked = (roleName: string, envId: string): boolean => {
    const allowed = accessMap[roleName]
    if (allowed === null || allowed === undefined) return true
    return allowed.includes(envId)
  }

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-sm font-medium mb-1">Environment Write Access</h3>
        <p className="text-xs text-muted-foreground/60">
          Control which roles can update flag configurations per environment.
          Unrestricted roles can write to all environments.
        </p>
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border/40">
                  <th className="text-left p-3 font-medium text-muted-foreground">Environment</th>
                  {writableRoles.map(role => (
                    <th key={role.name} className="text-center p-3 font-medium text-muted-foreground">
                      <div className="flex items-center justify-center gap-1.5">
                        {role.name}
                        {(accessMap[role.name] === undefined || accessMap[role.name] === null) && (
                          <Badge variant="outline" className="text-[10px] px-1 py-0">all</Badge>
                        )}
                      </div>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {data.environments.map(env => (
                  <tr key={env.id} className="border-b border-border/20 last:border-0">
                    <td className="p-3 font-mono text-xs">{env.key}</td>
                    {writableRoles.map(role => (
                      <td key={role.name} className="text-center p-3">
                        <Switch
                          checked={isChecked(role.name, env.id)}
                          onCheckedChange={(checked) => handleToggle(role.name, env.id, checked)}
                        />
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <div className="flex items-center justify-end gap-3">
        {saved && (
          <span className="text-[13px] text-emerald-400 animate-[fadeIn_200ms_ease]">
            Saved
          </span>
        )}
        {saveMutation.error && (
          <span className="text-[13px] text-destructive">
            Failed to save: {saveMutation.error instanceof Error ? saveMutation.error.message : 'Unknown error'}
          </span>
        )}
        <Button
          onClick={() => saveMutation.mutate()}
          disabled={!isDirty || saveMutation.isPending}
          size="sm"
        >
          {saveMutation.isPending ? 'Saving...' : 'Save changes'}
        </Button>
      </div>
    </div>
  )
}

export default function EnvironmentAccessTab() {
  const { key } = useParams<{ key: string }>()
  const { data: roles } = useRoles()

  const { data, isLoading, dataUpdatedAt } = useQuery({
    queryKey: ['projects', key, 'environment-access'],
    queryFn: () => api.environmentAccess.get(key!),
  })

  if (isLoading || !data) return <div className="text-sm text-muted-foreground">Loading...</div>

  // Only show roles that have flags:write permission
  const writableRoles = roles?.filter(r => r.permissions?.includes('flags:write')) ?? []

  if (writableRoles.length === 0) {
    return (
      <div className="text-sm text-muted-foreground">
        No roles with flag write permissions found.
      </div>
    )
  }

  // Key on dataUpdatedAt so the grid remounts (resetting local state) when server data changes
  return <EnvironmentAccessGrid key={dataUpdatedAt} data={data} writableRoles={writableRoles} />
}
