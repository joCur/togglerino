import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { AuditEntry, Environment } from '../api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import ConfigDiff from './ConfigDiff'
import { RotateCcw, ChevronDown, ChevronRight } from 'lucide-react'

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

export default function FlagHistory({ projectKey, flagKey, environments }: FlagHistoryProps) {
  const queryClient = useQueryClient()
  const [envFilter, setEnvFilter] = useState<string>('all')
  const [offset, setOffset] = useState(0)
  const [allEntries, setAllEntries] = useState<AuditEntry[]>([])
  const [hasMore, setHasMore] = useState(true)
  const [expandedEntries, setExpandedEntries] = useState<Set<string>>(new Set())
  const [restoreEntry, setRestoreEntry] = useState<AuditEntry | null>(null)

  const envParam = envFilter === 'all' ? '' : `&env=${envFilter}`

  const { isLoading, error } = useQuery({
    queryKey: ['projects', projectKey, 'flags', flagKey, 'history', envFilter, offset],
    queryFn: async () => {
      const entries = await api.get<AuditEntry[]>(
        `/projects/${projectKey}/flags/${flagKey}/history?limit=${PAGE_SIZE}&offset=${offset}${envParam}`
      )
      if (offset === 0) {
        setAllEntries(entries)
      } else {
        setAllEntries((prev) => [...prev, ...entries])
      }
      setHasMore(entries.length === PAGE_SIZE)
      return entries
    },
    enabled: !!projectKey && !!flagKey,
  })

  const restoreMutation = useMutation({
    mutationFn: (entryId: string) =>
      api.post(`/projects/${projectKey}/flags/${flagKey}/history/${entryId}/restore`),
    onSuccess: () => {
      setRestoreEntry(null)
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey] })
      // Reset history to refetch from scratch
      setOffset(0)
      setAllEntries([])
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey, 'history'] })
    },
  })

  const handleEnvChange = (value: string) => {
    setEnvFilter(value)
    setOffset(0)
    setAllEntries([])
    setHasMore(true)
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

  const canRestore = (entry: AuditEntry): boolean => {
    return entry.entity_type === 'flag_config' &&
      entry.environment_id != null &&
      (entry.old_value != null || entry.new_value != null)
  }

  const restoreEnvName = restoreEntry?.environment_id
    ? envNameMap.get(restoreEntry.environment_id) ?? 'unknown'
    : 'unknown'

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

                    {canRestore(entry) && (
                      <div className="mt-3 flex justify-end">
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-[12px]"
                          onClick={(e) => {
                            e.stopPropagation()
                            setRestoreEntry(entry)
                          }}
                        >
                          <RotateCcw className="w-3 h-3 mr-1.5" />
                          Restore this version
                        </Button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Load more */}
      {hasMore && allEntries.length > 0 && (
        <div className="text-center mt-6">
          <Button
            variant="outline"
            onClick={() => setOffset((prev) => prev + PAGE_SIZE)}
            disabled={isLoading}
          >
            {isLoading ? 'Loading...' : 'Load More'}
          </Button>
        </div>
      )}

      {/* Restore confirmation dialog */}
      <Dialog open={restoreEntry !== null} onOpenChange={(open) => !open && setRestoreEntry(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Restore configuration?</DialogTitle>
            <DialogDescription>
              This will apply the configuration from{' '}
              <span className="font-mono text-foreground">
                {restoreEntry && new Date(restoreEntry.created_at).toLocaleString()}
              </span>{' '}
              to <span className="font-medium text-foreground">{restoreEnvName}</span>.
              This creates a new change entry and does not delete any history.
            </DialogDescription>
          </DialogHeader>
          {restoreMutation.error && (
            <Alert variant="destructive">
              <AlertDescription>
                {restoreMutation.error instanceof Error ? restoreMutation.error.message : 'Failed to restore'}
              </AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setRestoreEntry(null)}>
              Cancel
            </Button>
            <Button
              onClick={() => restoreEntry && restoreMutation.mutate(restoreEntry.id)}
              disabled={restoreMutation.isPending}
            >
              {restoreMutation.isPending ? 'Restoring...' : 'Restore'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
