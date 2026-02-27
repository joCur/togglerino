import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment, FlagEnvironmentConfig } from '../api/types.ts'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

interface Props {
  environments: Environment[]
  configs: FlagEnvironmentConfig[]
  selectedEnvKey: string
  onSelectEnv: (envKey: string) => void
  projectKey: string
  flagKey: string
}

export default function EnvironmentSelector({
  environments,
  configs,
  selectedEnvKey,
  onSelectEnv,
  projectKey,
  flagKey,
}: Props) {
  const queryClient = useQueryClient()

  const toggleMutation = useMutation({
    mutationFn: ({ envKey, config }: { envKey: string; config: FlagEnvironmentConfig }) =>
      api.put(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}`, {
        enabled: !config.enabled,
        default_variant: config.default_variant,
        variants: config.variants,
        targeting_rules: config.targeting_rules,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey] })
    },
  })

  return (
    <div className="flex flex-wrap gap-3">
      {environments.map((env) => {
        const config = configs.find((c) => c.environment_id === env.id)
        const enabled = config?.enabled ?? false
        const isSelected = env.key === selectedEnvKey

        return (
          <div
            key={env.id}
            onClick={() => onSelectEnv(env.key)}
            className={cn(
              'flex flex-col items-center gap-2 px-5 py-3 rounded-lg border cursor-pointer transition-all duration-200 min-w-[120px]',
              isSelected
                ? 'border-[#d4956a]/60 bg-[#d4956a]/5 shadow-[0_0_12px_rgba(212,149,106,0.08)]'
                : 'border-border hover:border-muted-foreground/30',
            )}
          >
            <span className={cn(
              'text-[13px] font-medium',
              isSelected ? 'text-foreground' : 'text-muted-foreground',
            )}>
              {env.name}
            </span>
            <div
              onClick={(e) => {
                e.stopPropagation()
                if (config) toggleMutation.mutate({ envKey: env.key, config })
              }}
            >
              <Switch
                checked={enabled}
                disabled={!config || toggleMutation.isPending}
              />
            </div>
            <span className={cn(
              'text-[10px] font-mono font-medium',
              enabled ? 'text-emerald-400' : 'text-muted-foreground/50',
            )}>
              {enabled ? 'ON' : 'OFF'}
            </span>
          </div>
        )
      })}
    </div>
  )
}
