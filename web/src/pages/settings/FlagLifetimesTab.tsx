import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'

const FLAG_PURPOSE_LABELS: Record<string, { label: string; description: string }> = {
  'release': { label: 'Release', description: 'Feature rollout flags' },
  'experiment': { label: 'Experiment', description: 'A/B testing flags' },
  'operational': { label: 'Operational', description: 'Technical migration flags' },
  'kill-switch': { label: 'Kill Switch', description: 'Graceful degradation flags' },
  'permission': { label: 'Permission', description: 'Access control flags' },
}

export default function FlagLifetimesTab() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [lifetimes, setLifetimes] = useState<Record<string, number | null>>({})
  const [unevaluatedDays, setUnevaluatedDays] = useState<number | null>(null)
  const [initialized, setInitialized] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['projects', key, 'settings', 'flags'],
    queryFn: () => api.get<{ flag_lifetimes: Record<string, number | null>; unevaluated_stale_after_days?: number | null }>(`/projects/${key}/settings/flags`),
  })

  if (data && !initialized) {
    setLifetimes(data.flag_lifetimes)
    setUnevaluatedDays(data.unevaluated_stale_after_days ?? null)
    setInitialized(true)
  }

  const updateMutation = useMutation({
    mutationFn: (payload: { flag_lifetimes: Record<string, number | null>; unevaluated_stale_after_days: number }) =>
      api.put(`/projects/${key}/settings/flags`, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'flags'] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const handleSave = () => updateMutation.mutate({
    flag_lifetimes: lifetimes,
    unevaluated_stale_after_days: unevaluatedDays ?? 0,
  })

  const handleChange = (purpose: string, value: string) => {
    if (value === '' || value === 'permanent') {
      setLifetimes(prev => ({ ...prev, [purpose]: null }))
    } else {
      const num = parseInt(value, 10)
      if (!isNaN(num) && num > 0) {
        setLifetimes(prev => ({ ...prev, [purpose]: num }))
      }
    }
  }

  if (isLoading) return null

  return (
    <Card>
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-1">
          Flag Lifetimes
        </div>
        <div className="text-xs text-muted-foreground mb-4">
          Expected lifetime per flag type. Flags exceeding their lifetime are marked as potentially stale.
        </div>

        <div className="flex flex-col gap-3">
          {Object.entries(FLAG_PURPOSE_LABELS).map(([purpose, { label, description }]) => (
            <div key={purpose} className="flex flex-col md:flex-row md:items-center gap-2 md:gap-4">
              <div className="md:w-[140px]">
                <div className="text-[13px] font-medium text-foreground">{label}</div>
                <div className="text-[11px] text-muted-foreground">{description}</div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {lifetimes[purpose] === null ? (
                  <Input className="w-full md:w-[120px]" value="Permanent" disabled />
                ) : (
                  <Input
                    className="w-full md:w-[120px]"
                    type="number"
                    min={1}
                    value={lifetimes[purpose] ?? ''}
                    onChange={(e) => handleChange(purpose, e.target.value)}
                  />
                )}
                <span className="text-xs text-muted-foreground">
                  {lifetimes[purpose] === null ? '' : 'days'}
                </span>
                <button
                  type="button"
                  className="text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => handleChange(purpose, lifetimes[purpose] === null ? '40' : 'permanent')}
                >
                  {lifetimes[purpose] === null ? 'Set limit' : 'Make permanent'}
                </button>
              </div>
            </div>
          ))}
        </div>

        <div className="border-t border-border mt-4 pt-4">
          <div className="text-sm font-semibold text-foreground mb-1">
            Evaluation-Based Staleness
          </div>
          <div className="text-xs text-muted-foreground mb-3">
            Optionally mark flags as potentially stale if they haven't been evaluated by any SDK within a time window.
          </div>
          <div className="flex flex-col md:flex-row md:items-center gap-2 md:gap-4">
            <div className="md:w-[200px]">
              <div className="text-[13px] font-medium text-foreground">Unevaluated threshold</div>
              <div className="text-[11px] text-muted-foreground">Flags not evaluated within this window are marked potentially stale</div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {unevaluatedDays === null ? (
                <Input className="w-full md:w-[120px]" value="Disabled" disabled />
              ) : (
                <Input
                  className="w-full md:w-[120px]"
                  type="number"
                  min={1}
                  value={unevaluatedDays}
                  onChange={(e) => {
                    const num = parseInt(e.target.value, 10)
                    if (!isNaN(num) && num > 0) setUnevaluatedDays(num)
                  }}
                />
              )}
              <span className="text-xs text-muted-foreground">
                {unevaluatedDays === null ? '' : 'days'}
              </span>
              <button
                type="button"
                className="text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                onClick={() => setUnevaluatedDays(unevaluatedDays === null ? 30 : null)}
              >
                {unevaluatedDays === null ? 'Enable' : 'Disable'}
              </button>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3 mt-4">
          <Button onClick={handleSave} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving...' : 'Save Settings'}
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
