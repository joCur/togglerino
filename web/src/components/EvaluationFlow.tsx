import type { FlagEnvironmentConfig } from '../api/types.ts'
import { cn } from '@/lib/utils'

interface Props {
  config: FlagEnvironmentConfig | null
}

export default function EvaluationFlow({ config }: Props) {
  const enabled = config?.enabled ?? false
  const ruleCount = config?.targeting_rules?.length ?? 0
  const defaultVariant = config?.default_variant ?? '—'
  const hasRules = ruleCount > 0

  return (
    <div className="flex items-center gap-0 px-4 py-3 rounded-lg bg-secondary/30 border border-dashed text-[11px] font-mono overflow-x-auto">
      {/* Request node */}
      <span className="text-muted-foreground whitespace-nowrap">Request</span>
      <Arrow />

      {/* Enabled check */}
      <span
        className={cn(
          'px-2 py-1 rounded border whitespace-nowrap',
          enabled
            ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
            : 'bg-red-500/10 text-red-400 border-red-500/20',
        )}
      >
        {enabled ? 'Enabled ✓' : 'Disabled ✗'}
      </span>
      <Arrow />

      {/* Targeting rules */}
      <span
        className={cn(
          'px-2 py-1 rounded border whitespace-nowrap',
          !enabled && 'opacity-30',
          enabled && hasRules
            ? 'bg-[#d4956a]/10 text-[#d4956a] border-[#d4956a]/20'
            : 'bg-muted/50 text-muted-foreground/40 border-muted-foreground/10',
        )}
      >
        {hasRules ? `${ruleCount} rule${ruleCount > 1 ? 's' : ''}` : 'No rules'}
      </span>
      <Arrow />

      {/* Default variant */}
      <span className="px-2 py-1 rounded border bg-muted/50 text-foreground border-muted-foreground/10 whitespace-nowrap">
        Default: <span className="text-[#d4956a]">{defaultVariant || '—'}</span>
      </span>
    </div>
  )
}

function Arrow() {
  return (
    <span className="text-muted-foreground/30 mx-1.5 select-none shrink-0">→</span>
  )
}
