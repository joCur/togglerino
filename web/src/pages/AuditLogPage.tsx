import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { AuditEntry } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const PAGE_SIZE = 50

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

export default function AuditLogPage() {
  const { key } = useParams<{ key: string }>()
  const [offset, setOffset] = useState(0)
  const [allEntries, setAllEntries] = useState<AuditEntry[]>([])
  const [hasMore, setHasMore] = useState(true)

  const { isLoading, error } = useQuery({
    queryKey: ['projects', key, 'audit-log', offset],
    queryFn: async () => {
      const entries = await api.get<AuditEntry[]>(
        `/projects/${key}/audit-log?limit=${PAGE_SIZE}&offset=${offset}`
      )
      if (offset === 0) {
        setAllEntries(entries)
      } else {
        setAllEntries((prev) => [...prev, ...entries])
      }
      setHasMore(entries.length === PAGE_SIZE)
      return entries
    },
    enabled: !!key,
  })

  const handleLoadMore = () => {
    setOffset((prev) => prev + PAGE_SIZE)
  }

  if (isLoading && allEntries.length === 0) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading audit log...
      </div>
    )
  }

  if (error && allEntries.length === 0) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load audit log: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {key}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Audit Log</span>
      </div>

      <div className="mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Audit Log</h1>
      </div>

      {allEntries.length === 0 ? (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No audit log entries</div>
          <div className="text-[13px] text-muted-foreground/60">
            Activity in this project will be recorded here.
          </div>
        </div>
      ) : (
        <>
          <div className="rounded-lg border overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Time</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">User</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Action</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Entity Type</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Entity</TableHead>
                  <TableHead className="font-mono text-[11px] uppercase tracking-wider">Details</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {allEntries.map((entry) => (
                  <TableRow key={entry.id} className="transition-colors hover:bg-[#d4956a]/8">
                    <TableCell>
                      <span className="text-xs text-muted-foreground whitespace-nowrap font-mono" title={new Date(entry.created_at).toISOString()}>
                        {formatRelativeTime(entry.created_at)}
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs text-muted-foreground font-mono">
                        {entry.user_id ? entry.user_id.slice(0, 8) + '...' : '--'}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary" className="font-mono text-[11px]">
                        {formatAction(entry.action)}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <span className="text-[11px] text-muted-foreground uppercase tracking-wide font-mono">{entry.entity_type}</span>
                    </TableCell>
                    <TableCell>
                      <span className="font-mono text-xs text-[#d4956a]">{entry.entity_id.slice(0, 12)}</span>
                    </TableCell>
                    <TableCell>
                      <span
                        className="text-xs text-muted-foreground max-w-[200px] overflow-hidden text-ellipsis whitespace-nowrap block"
                        title={entry.new_value ? JSON.stringify(entry.new_value) : undefined}
                      >
                        {entry.new_value ? JSON.stringify(entry.new_value).slice(0, 50) : '--'}
                      </span>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className="text-center text-xs text-muted-foreground mt-4">
            Showing {allEntries.length} entries
          </div>

          {hasMore && (
            <div className="text-center mt-4">
              <Button
                variant="outline"
                onClick={handleLoadMore}
                disabled={isLoading}
              >
                {isLoading ? 'Loading...' : 'Load More'}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
