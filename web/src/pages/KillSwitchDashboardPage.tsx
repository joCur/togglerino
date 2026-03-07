import { useState, useMemo } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Flag, Environment, FlagEnvironmentConfig } from '@/api/types'
import { useCanWrite } from '@/hooks/usePermissions'
import { cn } from '@/lib/utils'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Zap } from 'lucide-react'

interface FlagDetailResponse {
  flag: Flag
  environment_configs: FlagEnvironmentConfig[]
}

interface ToggleTarget {
  flag: Flag
  envKey: string
  envName: string
  config: FlagEnvironmentConfig
}

function relativeTime(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diffMs = now - then
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return 'just now'
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHrs = Math.floor(diffMin / 60)
  if (diffHrs < 24) return `${diffHrs}h ago`
  const diffDays = Math.floor(diffHrs / 24)
  if (diffDays < 30) return `${diffDays}d ago`
  const diffMonths = Math.floor(diffDays / 30)
  return `${diffMonths}mo ago`
}

function userName(user?: { email: string; display_name?: string }): string | null {
  if (!user) return null
  return user.display_name || user.email.split('@')[0]
}

export default function KillSwitchDashboardPage() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const canWrite = useCanWrite(key)
  const [toggleTarget, setToggleTarget] = useState<ToggleTarget | null>(null)
  const [toggleError, setToggleError] = useState<string | null>(null)

  const { data: flags, isLoading: flagsLoading } = useQuery({
    queryKey: ['projects', key, 'flags', { flag_type: 'kill-switch' }],
    queryFn: () => api.flags.list(key!, { flag_type: 'kill-switch' }),
    enabled: !!key,
  })

  const { data: environments, isLoading: envsLoading } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const activeFlags = useMemo(
    () => (flags ?? []).filter((f) => f.lifecycle_status !== 'archived'),
    [flags],
  )

  const activeFlagKeys = useMemo(() => activeFlags.map((f) => f.key), [activeFlags])

  const { data: configsMap } = useQuery({
    queryKey: ['projects', key, 'kill-switch-configs', activeFlagKeys],
    queryFn: async () => {
      const results = await Promise.allSettled(
        activeFlags.map((flag) =>
          api.get<FlagDetailResponse>(`/projects/${key}/flags/${flag.key}`),
        ),
      )
      const map: Record<string, Record<string, FlagEnvironmentConfig>> = {}
      for (const result of results) {
        if (result.status !== 'fulfilled') continue
        const flagConfigs: Record<string, FlagEnvironmentConfig> = {}
        for (const config of result.value.environment_configs) {
          flagConfigs[config.environment_id] = config
        }
        map[result.value.flag.key] = flagConfigs
      }
      return map
    },
    enabled: !!key && activeFlags.length > 0,
  })

  const toggleMutation = useMutation({
    mutationFn: ({ flagKey, envKey, config }: { flagKey: string; envKey: string; config: FlagEnvironmentConfig }) =>
      api.put(`/projects/${key}/flags/${flagKey}/environments/${envKey}`, {
        enabled: !config.enabled,
        default_variant: config.default_variant,
        variants: config.variants,
        targeting_rules: config.targeting_rules,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'kill-switch-configs'] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      setToggleError(null)
      setToggleTarget(null)
    },
    onError: (err) => {
      setToggleError(err instanceof Error ? err.message : 'Failed to toggle kill switch')
    },
  })

  const isLoading = flagsLoading || envsLoading

  // Compute summary counts
  const summary = useMemo(() => {
    if (!activeFlags.length || !environments?.length || !configsMap) {
      return { total: 0, enabled: 0, disabled: 0, envCount: 0 }
    }
    let enabled = 0
    let disabled = 0
    for (const flag of activeFlags) {
      const flagConfigs = configsMap[flag.key]
      if (!flagConfigs) continue
      for (const env of environments) {
        const config = flagConfigs[env.id]
        if (!config) continue
        if (config.enabled) enabled++
        else disabled++
      }
    }
    return { total: activeFlags.length, enabled, disabled, envCount: environments.length }
  }, [activeFlags, environments, configsMap])

  if (isLoading) {
    return (
      <div>
        <h1 className="text-xl font-semibold tracking-tight mb-6">Kill Switches</h1>
        <div className="text-muted-foreground text-sm">Loading...</div>
      </div>
    )
  }

  if (activeFlags.length === 0) {
    return (
      <div>
        <h1 className="text-xl font-semibold tracking-tight mb-6">Kill Switches</h1>
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <Zap className="h-10 w-10 text-muted-foreground mb-4" />
          <p className="text-muted-foreground text-sm">
            No kill switches found. Create a flag with type &quot;kill-switch&quot; to see it here.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-xl font-semibold tracking-tight mb-6">Kill Switches</h1>

      {/* Summary bar */}
      <div className="mb-6 rounded-lg border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
        {summary.total} kill switch{summary.total !== 1 ? 'es' : ''} &mdash;{' '}
        <span className="text-emerald-400">{summary.enabled} enabled</span>,{' '}
        <span className="text-red-400">{summary.disabled} disabled</span>{' '}
        across {summary.envCount} environment{summary.envCount !== 1 ? 's' : ''}
      </div>

      {/* Table */}
      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/50">
              <th className="px-4 py-3 text-left font-medium text-muted-foreground">Flag</th>
              {environments?.map((env) => (
                <th key={env.id} className="px-4 py-3 text-center font-medium text-muted-foreground">
                  {env.name}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {activeFlags.map((flag) => (
              <tr key={flag.id} className="border-b border-border last:border-b-0 hover:bg-muted/30">
                <td className="px-4 py-3">
                  <Link
                    to={`/projects/${key}/flags/${flag.key}`}
                    className="font-medium text-foreground hover:underline"
                  >
                    {flag.name}
                  </Link>
                  <div className="text-xs text-muted-foreground font-mono">{flag.key}</div>
                </td>
                {environments?.map((env) => {
                  const config = configsMap?.[flag.key]?.[env.id]
                  if (!config) {
                    return (
                      <td key={env.id} className="px-4 py-3 text-center">
                        <span className="text-xs text-muted-foreground">--</span>
                      </td>
                    )
                  }
                  const isEnabled = config.enabled
                  const updatedByName = userName(config.updated_by_user)
                  return (
                    <td key={env.id} className="px-4 py-3">
                      <div className="flex flex-col items-center gap-1.5">
                        <div className="flex items-center gap-2">
                          <Switch
                            checked={isEnabled}
                            disabled={!canWrite || toggleMutation.isPending}
                            onCheckedChange={() => {
                              setToggleError(null)
                              setToggleTarget({ flag, envKey: env.key, envName: env.name, config })
                            }}
                            className={cn(
                              'data-[state=checked]:bg-emerald-600 data-[state=unchecked]:bg-red-900/40',
                            )}
                          />
                          <Badge
                            variant="outline"
                            className={cn(
                              'text-[10px] px-1.5 py-0',
                              isEnabled
                                ? 'bg-emerald-600/20 text-emerald-400 border-emerald-600/30'
                                : 'bg-red-900/20 text-red-400 border-red-900/30',
                            )}
                          >
                            {isEnabled ? 'ON' : 'OFF'}
                          </Badge>
                        </div>
                        {config.updated_at && (
                          <div className="text-[10px] text-muted-foreground leading-tight text-center">
                            {relativeTime(config.updated_at)}
                            {updatedByName ? ` by ${updatedByName}` : ''}
                          </div>
                        )}
                      </div>
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Confirmation dialog */}
      <Dialog open={!!toggleTarget} onOpenChange={(open) => !open && setToggleTarget(null)}>
        {toggleTarget && (
          <DialogContent>
            <DialogHeader>
              <DialogTitle>
                {toggleTarget.config.enabled ? 'Disable' : 'Enable'} kill switch
              </DialogTitle>
              <DialogDescription>
                {toggleTarget.config.enabled ? 'Disable' : 'Enable'} &quot;{toggleTarget.flag.name}&quot; in{' '}
                {toggleTarget.envName}?
              </DialogDescription>
            </DialogHeader>
            {toggleError && (
              <Alert variant="destructive">
                <AlertDescription>{toggleError}</AlertDescription>
              </Alert>
            )}
            <DialogFooter>
              <Button variant="outline" onClick={() => setToggleTarget(null)}>
                Cancel
              </Button>
              <Button
                variant={toggleTarget.config.enabled ? 'destructive' : 'default'}
                disabled={toggleMutation.isPending}
                onClick={() =>
                  toggleMutation.mutate({
                    flagKey: toggleTarget.flag.key,
                    envKey: toggleTarget.envKey,
                    config: toggleTarget.config,
                  })
                }
              >
                {toggleMutation.isPending
                  ? 'Saving...'
                  : toggleTarget.config.enabled
                    ? 'Disable'
                    : 'Enable'}
              </Button>
            </DialogFooter>
          </DialogContent>
        )}
      </Dialog>
    </div>
  )
}
