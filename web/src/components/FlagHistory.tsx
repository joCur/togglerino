import { useState } from 'react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { api } from '../api/client'
import type { AuditEntry, Environment, PaginatedResponse } from '../api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import ConfigDiff from './ConfigDiff'
import { ChevronDown, ChevronRight } from 'lucide-react'

const PAGE_SIZE = 50

interface FlagHistoryProps {
  projectKey: string
  flagKey: string
  environments: Environment[]
}

function formatRelativeTime(dateStr: string): string {
  const now = Date.now()
  const date = new Date(dateStr).getTime()
  const diff = now - date
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  return new Date(dateStr).toLocaleDateString()
}

function formatAction(action: string): string {
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function formatPromotedDescription(entry: AuditEntry, envNameMap: Map<string, string>): string | null {
  if (entry.action !== 'promoted') return null
  const newVal = entry.new_value as Record<string, unknown> | undefined
  const promotedFromEnv = newVal?.promoted_from_env as string | undefined
  if (!promotedFromEnv) return null
  const targetEnvName = entry.environment_id ? (envNameMap.get(entry.environment_id) ?? 'unknown') : 'unknown'
  return `Promoted config from ${promotedFromEnv} to ${targetEnvName}`
}

export default function FlagHistory({ projectKey, flagKey, environments }: FlagHistoryProps) {
  const [envFilter, setEnvFilter] = useState<string>('all')
  const [expandedEntries, setExpandedEntries] = useState<Set<string>>(new Set())

  const envParam = envFilter === 'all' ? '' : `&env=${envFilter}`

  const {
    data,
    isLoading,
    error,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: ['projects', projectKey, 'flags', flagKey, 'history', envFilter],
    queryFn: ({ pageParam = 0 }) =>
      api.get<PaginatedResponse<AuditEntry>>(
        `/projects/${projectKey}/flags/${flagKey}/history?limit=${PAGE_SIZE}&offset=${pageParam}${envParam}`
      ),
    initialPageParam: 0,
    getNextPageParam: (lastPage) =>
      lastPage.offset + lastPage.limit < lastPage.total
        ? lastPage.offset + lastPage.limit
        : undefined,
    enabled: !!projectKey && !!flagKey,
  })

  const allEntries = data?.pages.flatMap((page) => page.data) ?? []

  const handleEnvChange = (value: string) => {
    setEnvFilter(value)
  }

  const toggleExpanded = (id: string) => {
    setExpandedEntries((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const envNameMap = new Map(environments.map((e) => [e.id, e.name]))

  return (
    <div>
      {/* Environment filter */}
      <div className="flex items-center gap-3 mb-6">
        <span className="text-[11px] text-muted-foreground/50 uppercase tracking-wider font-mono">Environment</span>
        <Select value={envFilter} onValueChange={handleEnvChange}>
          <SelectTrigger className="w-[200px] h-8 text-[13px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All environments</SelectItem>
            {environments.map((env) => (
              <SelectItem key={env.id} value={env.key}>{env.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Error state */}
      {error && allEntries.length === 0 && (
        <Alert variant="destructive">
          <AlertDescription>
            Failed to load history: {error instanceof Error ? error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {/* Loading state */}
      {isLoading && allEntries.length === 0 && (
        <div className="text-center py-12 text-muted-foreground/60 text-[13px] animate-pulse">
          Loading history...
        </div>
      )}

      {/* Empty state */}
      {!isLoading && allEntries.length === 0 && !error && (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No history yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Changes to this flag will appear here.
          </div>
        </div>
      )}

      {/* Timeline */}
      {allEntries.length > 0 && (
        <div className="space-y-2">
          {allEntries.map((entry) => {
            const isExpanded = expandedEntries.has(entry.id)
            const hasDiff = entry.old_value != null && entry.new_value != null
            const hasSnapshot = entry.old_value != null || entry.new_value != null

            return (
              <div
                key={entry.id}
                className="rounded-lg border border-border hover:border-[#d4956a]/20 transition-colors"
              >
                <button
                  className="flex items-center w-full px-4 py-3 text-left cursor-pointer"
                  onClick={() => hasSnapshot && toggleExpanded(entry.id)}
                  disabled={!hasSnapshot}
                >
                  {hasSnapshot ? (
                    isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-muted-foreground mr-3 shrink-0" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-muted-foreground mr-3 shrink-0" />
                    )
                  ) : (
                    <div className="w-4 h-4 mr-3 shrink-0" />
                  )}

                  <div className="flex flex-wrap items-center gap-2 min-w-0 flex-1">
                    <span
                      className="text-xs text-muted-foreground font-mono whitespace-nowrap"
                      title={new Date(entry.created_at).toISOString()}
                    >
                      {formatRelativeTime(entry.created_at)}
                    </span>

                    <Badge variant="secondary" className="font-mono text-[11px]">
                      {formatAction(entry.action)}
                    </Badge>

                    {entry.environment_id && (
                      <Badge variant="outline" className="text-[11px]">
                        {envNameMap.get(entry.environment_id) ?? 'unknown'}
                      </Badge>
                    )}

                    {formatPromotedDescription(entry, envNameMap) && (
                      <span className="text-xs text-muted-foreground/80">
                        {formatPromotedDescription(entry, envNameMap)}
                      </span>
                    )}

                    <span className="text-xs text-muted-foreground/60 ml-auto whitespace-nowrap">
                      {entry.user_email ?? (entry.user_id ? entry.user_id.slice(0, 8) + '...' : 'system')}
                    </span>
                  </div>
                </button>

                {/* Expanded diff content */}
                {isExpanded && hasSnapshot && (
                  <div className="px-4 pb-4 border-t border-border/50">
                    <div className="mt-3">
                      {hasDiff ? (
                        <ConfigDiff
                          oldValue={entry.old_value}
                          newValue={entry.new_value}
                          entityType={entry.entity_type}
                        />
                      ) : (
                        <div className="text-[13px] text-muted-foreground/60 italic">
                          Snapshot available (no previous version for comparison)
                        </div>
                      )}
                    </div>

                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Load more */}
      {hasNextPage && allEntries.length > 0 && (
        <div className="text-center mt-6">
          <Button
            variant="outline"
            onClick={() => fetchNextPage()}
            disabled={isFetchingNextPage}
          >
            {isFetchingNextPage ? 'Loading...' : 'Load More'}
          </Button>
        </div>
      )}

    </div>
  )
}
