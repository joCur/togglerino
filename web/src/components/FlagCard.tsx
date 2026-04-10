import { useState } from 'react'
import type { Flag, Environment } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { cn } from '@/lib/utils'
import { gravatarUrl } from '@/lib/gravatar'
import { formatRelativeTime } from '@/lib/date'
import { Copy, Check } from 'lucide-react'

interface Props {
  flag: Flag
  environments: Environment[]
  getEnvStatus: (flagKey: string, envId: string) => boolean
  onClick: () => void
  selected?: boolean
  onSelect?: (flagKey: string) => void
}

export default function FlagCard({ flag, environments, getEnvStatus, onClick, selected, onSelect }: Props) {
  const isArchived = flag.lifecycle_status === 'archived'
  const isSelectable = !!onSelect

  const [copied, setCopied] = useState(false)

  const handleCopyKey = async (e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await navigator.clipboard.writeText(flag.key)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard API may not be available
    }
  }

  return (
    <div
      onClick={onClick}
      className={cn(
        'relative rounded-lg border bg-card cursor-pointer transition-all duration-200',
        'hover:border-[#d4956a]/40 hover:shadow-[0_0_12px_rgba(212,149,106,0.06)]',
        isSelectable ? 'pl-11 pr-4 py-4' : 'p-4',
        isArchived && 'opacity-60',
        selected && 'border-[#d4956a]/50 bg-[#d4956a]/[0.04] shadow-[0_0_16px_rgba(212,149,106,0.08)]',
      )}
    >
      {/* Checkbox gutter */}
      {isSelectable && (
        <div
          className="absolute left-0 top-0 bottom-0 w-11 flex items-center justify-center"
          onClick={(e) => e.stopPropagation()}
        >
          <Checkbox
            checked={selected ?? false}
            onCheckedChange={() => onSelect(flag.key)}
            className="cursor-pointer"
          />
        </div>
      )}

      {/* Row 1: Name + Type */}
      <div className="flex items-center justify-between mb-1">
        <span className="text-sm font-medium text-foreground">{flag.name}</span>
        <Badge variant="secondary" className="font-mono text-[11px]">{flag.value_type}</Badge>
      </div>

      {/* Row 2: Key (with copy) + lifecycle badge */}
      <div className="flex items-center gap-2 mb-3">
        <span className="font-mono text-[11px] text-[#d4956a]/70 tracking-wide">{flag.key}</span>
        <button
          onClick={handleCopyKey}
          className="text-muted-foreground/40 hover:text-muted-foreground transition-colors"
          title="Copy flag key"
        >
          {copied
            ? <Check className="w-3 h-3 text-emerald-400" />
            : <Copy className="w-3 h-3" />}
        </button>
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

      {/* Row 4: Owner + Evaluated + Purpose */}
      <div className="grid grid-cols-3 items-center min-w-0">
        {flag.owner ? (
          <div className="flex items-center gap-1.5 min-w-0">
            <img
              src={gravatarUrl(flag.owner.email, 20)}
              alt=""
              className="w-5 h-5 rounded-full shrink-0"
            />
            <span className="text-[11px] text-muted-foreground/60 truncate">
              {flag.owner.display_name ?? flag.owner.email}
            </span>
          </div>
        ) : (
          <span />
        )}
        {!isArchived ? (
          <span className={cn(
            'text-[11px] truncate text-center',
            flag.last_evaluated_at
              ? 'text-muted-foreground/50'
              : 'text-amber-400/70',
          )}>
            {flag.last_evaluated_at
              ? `Evaluated ${formatRelativeTime(flag.last_evaluated_at)}`
              : 'Never evaluated'}
          </span>
        ) : (
          <span />
        )}
        <span className="text-[11px] text-muted-foreground/50 capitalize text-right">{flag.flag_type}</span>
      </div>
    </div>
  )
}
