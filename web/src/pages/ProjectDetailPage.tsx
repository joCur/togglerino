import { useState, useMemo } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig, UnknownFlag, FlagPurpose, LifecycleStatus } from '../api/types.ts'
import { useFlag } from '@togglerino/react'
import CreateFlagModal from '../components/CreateFlagModal.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)

  if (diffSecs < 60) return 'just now'
  if (diffMins < 60) return `${diffMins}m ago`
  if (diffHours < 24) return `${diffHours}h ago`
  if (diffDays < 30) return `${diffDays}d ago`
  return date.toLocaleDateString()
}

export default function ProjectDetailPage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [search, setSearch] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [purposeFilter, setPurposeFilter] = useState<FlagPurpose | ''>('')
  const [statusFilter, setStatusFilter] = useState<LifecycleStatus | ''>('')
  const [modalOpen, setModalOpen] = useState(false)
  const [createFromKey, setCreateFromKey] = useState('')
  const unknownFlagsEnabled = useFlag('unknown-flags', false)

  const { data: flags, isLoading: flagsLoading, error: flagsError } = useQuery({
    queryKey: ['projects', key, 'flags'],
    queryFn: () => api.get<Flag[]>(`/projects/${key}/flags`),
    enabled: !!key,
  })

  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const { data: allConfigs } = useQuery({
    queryKey: ['projects', key, 'all-configs'],
    queryFn: async () => {
      if (!flags || flags.length === 0) return {}
      const configMap: Record<string, FlagEnvironmentConfig[]> = {}
      await Promise.all(
        flags.map(async (flag) => {
          try {
            const resp = await api.get<{ flag: Flag; environment_configs: FlagEnvironmentConfig[] }>(
              `/projects/${key}/flags/${flag.key}`
            )
            configMap[flag.key] = resp.environment_configs
          } catch {
            configMap[flag.key] = []
          }
        })
      )
      return configMap
    },
    enabled: !!flags && flags.length > 0,
  })

  const { data: unknownFlags } = useQuery({
    queryKey: ['projects', key, 'unknown-flags'],
    queryFn: () => api.get<UnknownFlag[]>(`/projects/${key}/unknown-flags`),
    enabled: !!key && unknownFlagsEnabled,
  })

  const dismissMutation = useMutation({
    mutationFn: (id: string) => api.delete(`/projects/${key}/unknown-flags/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'unknown-flags'] })
    },
  })

  const allTags = useMemo(() => {
    if (!flags) return []
    const tagSet = new Set<string>()
    flags.forEach((f) => f.tags?.forEach((tag) => tagSet.add(tag)))
    return Array.from(tagSet).sort()
  }, [flags])

  const filtered = useMemo(() => {
    if (!flags) return []
    return flags.filter((f) => {
      const matchesSearch =
        !search ||
        f.key.toLowerCase().includes(search.toLowerCase()) ||
        f.name.toLowerCase().includes(search.toLowerCase())
      const matchesTag = !tagFilter || (f.tags && f.tags.includes(tagFilter))
      const matchesPurpose = !purposeFilter || f.flag_type === purposeFilter
      const matchesStatus = !statusFilter || f.lifecycle_status === statusFilter
      return matchesSearch && matchesTag && matchesPurpose && matchesStatus
    })
  }, [flags, search, tagFilter, purposeFilter, statusFilter])

  if (flagsLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading flags...
      </div>
    )
  }

  if (flagsError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load flags: {flagsError instanceof Error ? flagsError.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  const getEnvStatus = (flagKey: string, envId: string): boolean => {
    if (!allConfigs || !allConfigs[flagKey]) return false
    const cfg = allConfigs[flagKey].find((c) => c.environment_id === envId)
    return cfg?.enabled ?? false
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground font-mono text-xs">{key}</span>
      </div>

      <div className="flex items-center justify-between mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">{key}</h1>
        <Button onClick={() => setModalOpen(true)}>Create Flag</Button>
      </div>

      <Tabs defaultValue="flags">
        <TabsList variant="line">
          <TabsTrigger value="flags">Flags</TabsTrigger>
          {unknownFlagsEnabled && (
            <TabsTrigger value="unknown">
              Unknown Flags
              {unknownFlags && unknownFlags.length > 0 && (
                <Badge variant="secondary" className="ml-1.5 text-[10px] px-1.5 py-0">
                  {unknownFlags.length}
                </Badge>
              )}
            </TabsTrigger>
          )}
        </TabsList>

        <TabsContent value="flags">
          {/* Filters */}
          <div className="flex gap-2.5 mb-5 mt-5">
            <Input
              className="flex-1 max-w-[300px]"
              placeholder="Search flags..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            {allTags.length > 0 && (
              <select
                className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[130px]"
                value={tagFilter}
                onChange={(e) => setTagFilter(e.target.value)}
              >
                <option value="">All Tags</option>
                {allTags.map((tag) => (
                  <option key={tag} value={tag}>{tag}</option>
                ))}
              </select>
            )}
            <select
              className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[130px]"
              value={purposeFilter}
              onChange={(e) => setPurposeFilter(e.target.value as FlagPurpose | '')}
            >
              <option value="">All Purposes</option>
              <option value="release">Release</option>
              <option value="experiment">Experiment</option>
              <option value="operational">Operational</option>
              <option value="kill-switch">Kill Switch</option>
              <option value="permission">Permission</option>
            </select>
            <select
              className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[130px]"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as LifecycleStatus | '')}
            >
              <option value="">All Statuses</option>
              <option value="active">Active</option>
              <option value="potentially_stale">Potentially Stale</option>
              <option value="stale">Stale</option>
              <option value="archived">Archived</option>
            </select>
          </div>

          {filtered.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-[15px] font-medium text-foreground mb-1.5">
                {flags && flags.length > 0 ? 'No flags match your filters' : 'No flags yet'}
              </div>
              <div className="text-[13px] text-muted-foreground/60">
                {flags && flags.length > 0
                  ? 'Try adjusting your search or tag filter.'
                  : 'Create your first feature flag to get started.'}
              </div>
            </div>
          ) : (
            <div className="rounded-lg border overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Type</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Purpose</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Tags</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Environments</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.map((flag) => (
                    <TableRow
                      key={flag.id}
                      className="cursor-pointer transition-colors hover:bg-[#d4956a]/8"
                      onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
                    >
                      <TableCell>
                        <span className={`font-mono text-xs text-[#d4956a] tracking-wide ${flag.lifecycle_status === 'archived' ? 'opacity-50' : ''}`}>{flag.key}</span>
                      </TableCell>
                      <TableCell className="text-[13px] text-foreground">
                        <span className={flag.lifecycle_status === 'archived' ? 'opacity-50' : ''}>
                          {flag.name}
                        </span>
                        {flag.lifecycle_status !== 'active' && (
                          <Badge
                            variant="secondary"
                            className={`ml-2 text-[10px] ${
                              flag.lifecycle_status === 'stale' ? 'bg-red-500/10 text-red-400 border-red-500/20' :
                              flag.lifecycle_status === 'potentially_stale' ? 'bg-amber-500/10 text-amber-400 border-amber-500/20' :
                              ''
                            }`}
                          >
                            {flag.lifecycle_status === 'archived' ? 'Archived' :
                             flag.lifecycle_status === 'stale' ? 'Stale' :
                             flag.lifecycle_status === 'potentially_stale' ? 'Potentially Stale' : ''}
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="font-mono text-[11px]">{flag.value_type}</Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="text-[11px]">{flag.flag_type}</Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {flag.tags?.map((tag) => (
                            <Badge key={tag} variant="outline" className="text-[11px]">{tag}</Badge>
                          ))}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          {environments?.map((env) => {
                            const enabled = getEnvStatus(flag.key, env.id)
                            return (
                              <span key={env.id} className="inline-flex items-center gap-1 whitespace-nowrap">
                                <span
                                  className={`inline-block w-[7px] h-[7px] rounded-full transition-all duration-300 ${
                                    enabled
                                      ? 'bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.4)]'
                                      : 'bg-muted-foreground/60'
                                  }`}
                                />
                                <span className="text-[11px] text-muted-foreground/60">{env.name}</span>
                              </span>
                            )
                          })}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>

        {unknownFlagsEnabled && <TabsContent value="unknown">
          {!unknownFlags || unknownFlags.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-[15px] font-medium text-foreground mb-1.5">No unknown flags detected</div>
              <div className="text-[13px] text-muted-foreground/60">
                Unknown flags appear here when your SDKs try to evaluate flags that don't exist in this project.
              </div>
            </div>
          ) : (
            <div className="rounded-lg border overflow-hidden mt-5">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Flag Key</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Environment</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Requests</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">First Seen</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Last Seen</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {unknownFlags.map((uf) => (
                    <TableRow key={uf.id}>
                      <TableCell>
                        <span className="font-mono text-xs text-[#d4956a] tracking-wide">{uf.flag_key}</span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="text-[11px]">{uf.environment_name}</Badge>
                      </TableCell>
                      <TableCell className="text-[13px] text-foreground tabular-nums">
                        {uf.request_count.toLocaleString()}
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground/60">
                        {formatRelativeTime(uf.first_seen_at)}
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground/60">
                        {formatRelativeTime(uf.last_seen_at)}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-[11px] h-7"
                            onClick={() => { setCreateFromKey(uf.flag_key); setModalOpen(true) }}
                          >
                            Create Flag
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-[11px] h-7 text-muted-foreground"
                            onClick={() => dismissMutation.mutate(uf.id)}
                            disabled={dismissMutation.isPending && dismissMutation.variables === uf.id}
                          >
                            Dismiss
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>}
      </Tabs>

      <CreateFlagModal
        key={createFromKey}
        open={modalOpen}
        projectKey={key!}
        initialKey={createFromKey}
        onClose={() => { setModalOpen(false); setCreateFromKey('') }}
        onCreated={() => queryClient.invalidateQueries({ queryKey: ['projects', key, 'unknown-flags'] })}
      />
    </div>
  )
}
