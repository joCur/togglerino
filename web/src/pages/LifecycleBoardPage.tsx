import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, LifecycleStatus } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

const COLUMNS: { status: LifecycleStatus; label: string; color: string; bgColor: string }[] = [
  { status: 'active', label: 'Active', color: 'text-emerald-400', bgColor: 'border-emerald-500/30' },
  { status: 'potentially_stale', label: 'Potentially Stale', color: 'text-amber-400', bgColor: 'border-amber-500/30' },
  { status: 'stale', label: 'Stale', color: 'text-red-400', bgColor: 'border-red-500/30' },
  { status: 'archived', label: 'Archived', color: 'text-muted-foreground', bgColor: 'border-muted-foreground/30' },
]

const PURPOSE_COLORS: Record<string, string> = {
  'release': 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  'experiment': 'bg-purple-500/10 text-purple-400 border-purple-500/20',
  'operational': 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  'kill-switch': 'bg-red-500/10 text-red-400 border-red-500/20',
  'permission': 'bg-green-500/10 text-green-400 border-green-500/20',
}

function daysAgo(dateStr: string): number {
  return Math.floor((Date.now() - new Date(dateStr).getTime()) / (1000 * 60 * 60 * 24))
}

function FlagCard({ flag, projectKey }: { flag: Flag; projectKey: string }) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const stalenessMutation = useMutation({
    mutationFn: () => api.put(`/projects/${projectKey}/flags/${flag.key}/staleness`, { status: 'stale' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags'] }),
  })

  const archiveMutation = useMutation({
    mutationFn: () => api.put(`/projects/${projectKey}/flags/${flag.key}/archive`, { archived: true }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags'] }),
  })

  return (
    <Card
      className="cursor-pointer hover:border-[#d4956a]/40 transition-all duration-200"
      onClick={() => navigate(`/projects/${projectKey}/flags/${flag.key}`)}
    >
      <CardContent className="p-3">
        <div className="text-[13px] font-medium text-foreground mb-0.5 truncate">{flag.name}</div>
        <div className="font-mono text-[11px] text-[#d4956a] mb-2 truncate">{flag.key}</div>
        <div className="flex flex-wrap gap-1 mb-2">
          <Badge variant="secondary" className={`text-[10px] ${PURPOSE_COLORS[flag.flag_type] || ''}`}>
            {flag.flag_type}
          </Badge>
          <Badge variant="secondary" className="font-mono text-[10px]">{flag.value_type}</Badge>
        </div>
        {flag.tags && flag.tags.length > 0 && (
          <div className="flex flex-wrap gap-1 mb-2">
            {flag.tags.map(tag => (
              <Badge key={tag} variant="outline" className="text-[10px]">{tag}</Badge>
            ))}
          </div>
        )}
        <div className="text-[11px] text-muted-foreground mb-2">
          {daysAgo(flag.created_at)} days old
          {flag.lifecycle_status_changed_at && flag.lifecycle_status !== 'active' && (
            <span> · status changed {daysAgo(flag.lifecycle_status_changed_at)}d ago</span>
          )}
        </div>
        {flag.lifecycle_status === 'potentially_stale' && (
          <Button
            size="sm"
            variant="outline"
            className="text-[11px] h-7 border-amber-500/50 text-amber-400 hover:bg-amber-500/10"
            onClick={(e) => { e.stopPropagation(); stalenessMutation.mutate() }}
            disabled={stalenessMutation.isPending}
          >
            {stalenessMutation.isPending ? 'Marking...' : 'Mark as Stale'}
          </Button>
        )}
        {flag.lifecycle_status === 'stale' && (
          <Button
            size="sm"
            variant="outline"
            className="text-[11px] h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
            onClick={(e) => { e.stopPropagation(); archiveMutation.mutate() }}
            disabled={archiveMutation.isPending}
          >
            {archiveMutation.isPending ? 'Archiving...' : 'Archive'}
          </Button>
        )}
      </CardContent>
    </Card>
  )
}

export default function LifecycleBoardPage() {
  const { key } = useParams<{ key: string }>()

  const { data: flags, isLoading } = useQuery({
    queryKey: ['projects', key, 'flags'],
    queryFn: () => api.get<Flag[]>(`/projects/${key}/flags`),
    enabled: !!key,
  })

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading lifecycle board...
      </div>
    )
  }

  const grouped = COLUMNS.map(col => ({
    ...col,
    flags: (flags || []).filter(f => f.lifecycle_status === col.status),
  }))

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
        <span className="text-foreground">Lifecycle</span>
      </div>

      <div className="mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Flag Lifecycle</h1>
        <div className="text-[13px] text-muted-foreground/60 mt-1">
          Track flag health and manage cleanup across lifecycle stages.
        </div>
      </div>

      <div className="grid grid-cols-4 gap-4">
        {grouped.map(col => (
          <div key={col.status}>
            <div className={`flex items-center gap-2 mb-3 pb-2 border-b-2 ${col.bgColor}`}>
              <span className={`text-[13px] font-semibold ${col.color}`}>{col.label}</span>
              <span className="text-[11px] text-muted-foreground/60 bg-secondary px-1.5 py-0.5 rounded-full">
                {col.flags.length}
              </span>
            </div>
            <div className="flex flex-col gap-2">
              {col.flags.length === 0 ? (
                <div className="text-[12px] text-muted-foreground/40 text-center py-6">
                  No flags
                </div>
              ) : (
                col.flags.map(flag => (
                  <FlagCard key={flag.id} flag={flag} projectKey={key!} />
                ))
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
