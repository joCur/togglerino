import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
import { api } from '../api/client'
import type { Flag } from '../api/types'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCanWrite } from '@/hooks/usePermissions'

const PURPOSE_COLORS: Record<string, string> = {
  'release': 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  'experiment': 'bg-purple-500/10 text-purple-400 border-purple-500/20',
  'operational': 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  'kill-switch': 'bg-red-500/10 text-red-400 border-red-500/20',
  'permission': 'bg-green-500/10 text-green-400 border-green-500/20',
}

const STATUS_COLORS: Record<string, string> = {
  'active': 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  'potentially_stale': 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  'stale': 'bg-red-500/10 text-red-400 border-red-500/20',
  'archived': 'bg-muted text-muted-foreground border-muted',
}

const STATUS_CARDS = [
  { key: 'active' as const, label: 'Active', color: 'text-emerald-400', border: 'border-emerald-500/30' },
  { key: 'potentially_stale' as const, label: 'Potentially Stale', color: 'text-amber-400', border: 'border-amber-500/30' },
  { key: 'stale' as const, label: 'Stale', color: 'text-red-400', border: 'border-red-500/30' },
  { key: 'archived' as const, label: 'Archived', color: 'text-muted-foreground', border: 'border-muted-foreground/30' },
]

function daysAgo(dateStr: string): number {
  return Math.floor((Date.now() - new Date(dateStr).getTime()) / (1000 * 60 * 60 * 24))
}

function HealthBadge({ score }: { score: number }) {
  const rounded = Math.round(score)
  let color = 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
  if (rounded < 50) color = 'bg-red-500/10 text-red-400 border-red-500/20'
  else if (rounded < 80) color = 'bg-amber-500/10 text-amber-400 border-amber-500/20'
  return <Badge variant="outline" className={`text-sm font-semibold ${color}`}>{rounded}%</Badge>
}

export default function LifecycleBoardPage() {
  const { key } = useParams<{ key: string }>()
  const canWrite = useCanWrite(key)
  const queryClient = useQueryClient()
  const navigate = useNavigate()

  const [statusFilter, setStatusFilter] = useState('potentially_stale,stale')
  const [typeFilter, setTypeFilter] = useState('all')
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const { data: summary, isLoading } = useQuery({
    queryKey: ['projects', key, 'lifecycle', 'summary'],
    queryFn: () => api.lifecycle.summary(key!),
    enabled: !!key,
  })

  const { data: trends } = useQuery({
    queryKey: ['projects', key, 'lifecycle', 'trends'],
    queryFn: () => api.lifecycle.trends(key!),
    enabled: !!key,
  })

  const { data: flags } = useQuery({
    queryKey: ['projects', key, 'lifecycle-flags', statusFilter, typeFilter],
    queryFn: () => {
      let path = `/projects/${key}/flags?lifecycle_status=${statusFilter}`
      if (typeFilter !== 'all') path += `&flag_type=${typeFilter}`
      return api.get<Flag[]>(path)
    },
    enabled: !!key,
  })

  const sortedFlags = [...(flags || [])].sort((a, b) =>
    new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  )

  const archiveMutation = useMutation({
    mutationFn: (flagKey: string) => api.put(`/projects/${key}/flags/${flagKey}/archive`, { archived: true }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', key] }),
  })

  const stalenessMutation = useMutation({
    mutationFn: (flagKey: string) => api.put(`/projects/${key}/flags/${flagKey}/staleness`, { status: 'stale' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects', key] }),
  })

  const bulkArchiveMutation = useMutation({
    mutationFn: () => api.flags.bulk(key!, { action: 'archive', flag_keys: [...selected] }),
    onSuccess: () => {
      setSelected(new Set())
      queryClient.invalidateQueries({ queryKey: ['projects', key] })
    },
  })

  function toggleSelect(flagKey: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(flagKey)) next.delete(flagKey)
      else next.add(flagKey)
      return next
    })
  }

  function toggleAll() {
    if (selected.size === sortedFlags.length) setSelected(new Set())
    else setSelected(new Set(sortedFlags.map(f => f.key)))
  }

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading lifecycle dashboard...
      </div>
    )
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">Projects</Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">{key}</Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Lifecycle</span>
      </div>

      <div className="flex items-center gap-3 mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Flag Lifecycle</h1>
        {summary && <HealthBadge score={summary.health_score} />}
      </div>
      <p className="text-[13px] text-muted-foreground/60 mb-6">Track flag health and manage cleanup across lifecycle stages.</p>

      {/* Stat Cards */}
      {summary && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          {STATUS_CARDS.map(card => (
            <Card key={card.key} className={`border-l-2 ${card.border}`}>
              <CardContent className="p-4">
                <div className={`text-[11px] uppercase tracking-wider font-medium ${card.color} mb-1`}>{card.label}</div>
                <div className="text-2xl font-bold text-foreground">{summary[card.key]}</div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Trends Chart */}
      {trends && trends.length > 0 && (
        <Card className="mb-8">
          <CardContent className="p-4">
            <div className="text-[13px] font-medium text-foreground mb-4">Staleness Trends</div>
            <ResponsiveContainer width="100%" height={240}>
              <AreaChart data={trends}>
                <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }}
                  tickFormatter={(v: string) => new Date(v + 'T00:00:00').toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
                />
                <YAxis tick={{ fontSize: 11, fill: 'hsl(var(--muted-foreground))' }} allowDecimals={false} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: 'hsl(var(--card))',
                    border: '1px solid hsl(var(--border))',
                    borderRadius: '8px',
                    fontSize: '12px',
                  }}
                />
                <Area type="monotone" dataKey="active" stackId="1" stroke="#34d399" fill="#34d399" fillOpacity={0.3} name="Active" />
                <Area type="monotone" dataKey="potentially_stale" stackId="1" stroke="#fbbf24" fill="#fbbf24" fillOpacity={0.3} name="Potentially Stale" />
                <Area type="monotone" dataKey="stale" stackId="1" stroke="#f87171" fill="#f87171" fillOpacity={0.3} name="Stale" />
                <Area type="monotone" dataKey="archived" stackId="1" stroke="#6b7280" fill="#6b7280" fillOpacity={0.3} name="Archived" />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      )}

      {/* Action Queue */}
      <Card>
        <CardContent className="p-4">
          <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
            <div className="text-[13px] font-medium text-foreground">Action Queue</div>
            <div className="flex items-center gap-2">
              <Select value={statusFilter} onValueChange={(v) => { setStatusFilter(v); setSelected(new Set()) }}>
                <SelectTrigger className="h-8 text-[12px] w-[180px]">
                  <SelectValue placeholder="Status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="potentially_stale,stale">Needs Attention</SelectItem>
                  <SelectItem value="potentially_stale">Potentially Stale</SelectItem>
                  <SelectItem value="stale">Stale</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="archived">Archived</SelectItem>
                  <SelectItem value="active,potentially_stale,stale,archived">All</SelectItem>
                </SelectContent>
              </Select>
              <Select value={typeFilter} onValueChange={(v) => { setTypeFilter(v); setSelected(new Set()) }}>
                <SelectTrigger className="h-8 text-[12px] w-[140px]">
                  <SelectValue placeholder="Flag type" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All types</SelectItem>
                  <SelectItem value="release">Release</SelectItem>
                  <SelectItem value="experiment">Experiment</SelectItem>
                  <SelectItem value="operational">Operational</SelectItem>
                  <SelectItem value="kill-switch">Kill Switch</SelectItem>
                  <SelectItem value="permission">Permission</SelectItem>
                </SelectContent>
              </Select>
              {canWrite && selected.size > 0 && (
                <Button
                  size="sm"
                  variant="destructive"
                  className="h-8 text-[12px]"
                  onClick={() => bulkArchiveMutation.mutate()}
                  disabled={bulkArchiveMutation.isPending}
                >
                  {bulkArchiveMutation.isPending ? 'Archiving...' : `Archive ${selected.size} selected`}
                </Button>
              )}
            </div>
          </div>

          {sortedFlags.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground/40 text-[13px]">
              No flags match the current filters.
            </div>
          ) : (
            <div className="border rounded-lg overflow-hidden">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="border-b bg-muted/30">
                    {canWrite && (
                      <th className="p-2 w-8">
                        <Checkbox
                          checked={selected.size === sortedFlags.length && sortedFlags.length > 0}
                          onCheckedChange={toggleAll}
                        />
                      </th>
                    )}
                    <th className="p-2 text-left text-muted-foreground font-medium">Flag</th>
                    <th className="p-2 text-left text-muted-foreground font-medium hidden sm:table-cell">Type</th>
                    <th className="p-2 text-left text-muted-foreground font-medium hidden md:table-cell">Status</th>
                    <th className="p-2 text-left text-muted-foreground font-medium hidden md:table-cell">Age</th>
                    {canWrite && <th className="p-2 text-right text-muted-foreground font-medium">Action</th>}
                  </tr>
                </thead>
                <tbody>
                  {sortedFlags.map(flag => (
                    <tr
                      key={flag.id}
                      className="border-b last:border-0 hover:bg-muted/20 cursor-pointer transition-colors"
                      onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
                    >
                      {canWrite && (
                        <td className="p-2" onClick={e => e.stopPropagation()}>
                          <Checkbox
                            checked={selected.has(flag.key)}
                            onCheckedChange={() => toggleSelect(flag.key)}
                          />
                        </td>
                      )}
                      <td className="p-2">
                        <div className="font-medium text-foreground">{flag.name}</div>
                        <div className="font-mono text-[11px] text-[#d4956a]">{flag.key}</div>
                      </td>
                      <td className="p-2 hidden sm:table-cell">
                        <Badge variant="secondary" className={`text-[10px] ${PURPOSE_COLORS[flag.flag_type] || ''}`}>
                          {flag.flag_type}
                        </Badge>
                      </td>
                      <td className="p-2 hidden md:table-cell">
                        <Badge variant="outline" className={`text-[10px] ${STATUS_COLORS[flag.lifecycle_status] || ''}`}>
                          {flag.lifecycle_status.replace('_', ' ')}
                        </Badge>
                      </td>
                      <td className="p-2 hidden md:table-cell text-muted-foreground text-[12px]">
                        {daysAgo(flag.created_at)}d
                        {flag.lifecycle_status_changed_at && (
                          <span className="text-muted-foreground/60"> · {daysAgo(flag.lifecycle_status_changed_at)}d in status</span>
                        )}
                      </td>
                      {canWrite && (
                        <td className="p-2 text-right" onClick={e => e.stopPropagation()}>
                          {flag.lifecycle_status === 'stale' && (
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-[11px] border-destructive/50 text-destructive hover:bg-destructive/10"
                              onClick={() => archiveMutation.mutate(flag.key)}
                              disabled={archiveMutation.isPending}
                            >
                              Archive
                            </Button>
                          )}
                          {flag.lifecycle_status === 'potentially_stale' && (
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-7 text-[11px] border-amber-500/50 text-amber-400 hover:bg-amber-500/10"
                              onClick={() => stalenessMutation.mutate(flag.key)}
                              disabled={stalenessMutation.isPending}
                            >
                              Mark Stale
                            </Button>
                          )}
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
