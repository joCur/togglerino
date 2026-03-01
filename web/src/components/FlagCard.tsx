import type { Flag, Environment } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Props {
  flag: Flag
  environments: Environment[]
  getEnvStatus: (flagKey: string, envId: string) => boolean
  onClick: () => void
}

export default function FlagCard({ flag, environments, getEnvStatus, onClick }: Props) {
  const isArchived = flag.lifecycle_status === 'archived'

  return (
    <div
      onClick={onClick}
      className={cn(
        'p-4 rounded-lg border bg-card cursor-pointer transition-all duration-200',
        'hover:border-[#d4956a]/40 hover:shadow-[0_0_12px_rgba(212,149,106,0.06)]',
        isArchived && 'opacity-60',
      )}
    >
      {/* Row 1: Key + Type */}
      <div className="flex items-center justify-between mb-1">
        <span className="font-mono text-sm text-[#d4956a] tracking-wide">{flag.key}</span>
        <Badge variant="secondary" className="font-mono text-[11px]">{flag.value_type}</Badge>
      </div>

      {/* Row 2: Name + lifecycle badge */}
      <div className="flex items-center gap-2 mb-3">
        <span className="text-[13px] text-muted-foreground">{flag.name}</span>
        {flag.lifecycle_status !== 'active' && flag.lifecycle_status !== 'archived' && (
          <Badge
            variant="secondary"
            className={cn(
              'text-[10px]',
              flag.lifecycle_status === 'stale' && 'bg-red-500/10 text-red-400 border-red-500/20',
              flag.lifecycle_status === 'potentially_stale' && 'bg-amber-500/10 text-amber-400 border-amber-500/20',
            )}
          >
            {flag.lifecycle_status === 'stale' ? 'Stale' : 'Potentially Stale'}
          </Badge>
        )}
        {isArchived && (
          <Badge variant="secondary" className="text-[10px]">Archived</Badge>
        )}
      </div>

      {/* Row 3: Environment status pills */}
      <div className="flex flex-wrap gap-2 mb-2">
        {environments?.map((env) => {
          const enabled = getEnvStatus(flag.key, env.id)
          return (
            <span
              key={env.id}
              className={cn(
                'inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-medium',
                enabled
                  ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                  : 'bg-muted/50 text-muted-foreground/60 border border-transparent',
              )}
            >
              <span
                className={cn(
                  'w-1.5 h-1.5 rounded-full',
                  enabled ? 'bg-emerald-400' : 'bg-muted-foreground/40',
                )}
              />
              {env.name}
              <span className="font-mono text-[10px] ml-0.5">
                {enabled ? 'ON' : 'OFF'}
              </span>
            </span>
          )
        })}
      </div>

      {/* Row 4: Purpose */}
      <div className="flex justify-end">
        <span className="text-[11px] text-muted-foreground/50 capitalize">{flag.flag_type}</span>
      </div>
    </div>
  )
}
