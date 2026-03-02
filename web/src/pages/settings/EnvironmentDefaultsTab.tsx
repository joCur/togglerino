import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'

export default function EnvironmentDefaultsTab() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [defaults, setDefaults] = useState<{ key: string; name: string; enabled: boolean }[]>([])
  const [initialized, setInitialized] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['projects', key, 'settings', 'environments'],
    queryFn: () => api.get<{ environment_defaults: { key: string; name: string; enabled: boolean }[] }>(
      `/projects/${key}/settings/environments`
    ),
  })

  if (data && !initialized) {
    setDefaults(data.environment_defaults)
    setInitialized(true)
  }

  const updateMutation = useMutation({
    mutationFn: (envDefaults: Record<string, { enabled: boolean }>) =>
      api.put(`/projects/${key}/settings/environments`, { environment_defaults: envDefaults }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'environments'] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const handleToggle = (envKey: string) => {
    setDefaults(prev => prev.map(d => d.key === envKey ? { ...d, enabled: !d.enabled } : d))
  }

  const handleSave = () => {
    const payload: Record<string, { enabled: boolean }> = {}
    for (const d of defaults) {
      payload[d.key] = { enabled: d.enabled }
    }
    updateMutation.mutate(payload)
  }

  if (isLoading) return null

  return (
    <Card>
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-1">
          Environment Defaults
        </div>
        <div className="text-xs text-muted-foreground mb-4">
          Default enabled state for new flags per environment.
        </div>

        <div className="flex flex-col gap-3">
          {defaults.map((env) => (
            <div key={env.key} className="flex items-center justify-between gap-4">
              <div>
                <div className="text-[13px] font-medium text-foreground">{env.name}</div>
                <div className="text-[11px] text-muted-foreground font-mono">{env.key}</div>
              </div>
              <div className="flex items-center gap-2">
                <Switch checked={env.enabled} onCheckedChange={() => handleToggle(env.key)} />
                <span className="text-xs text-muted-foreground w-[52px]">
                  {env.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
            </div>
          ))}
        </div>

        <div className="flex items-center gap-3 mt-4">
          <Button onClick={handleSave} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving...' : 'Save Defaults'}
          </Button>
          {saveSuccess && (
            <span className="text-[13px] text-emerald-400 animate-[fadeIn_200ms_ease]">Saved</span>
          )}
          {updateMutation.error && (
            <span className="text-[13px] text-destructive">
              {updateMutation.error instanceof Error ? updateMutation.error.message : 'Failed to save'}
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
