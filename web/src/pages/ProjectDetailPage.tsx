import { useState, useMemo } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig, FlagPurpose, LifecycleStatus, User, BulkAction, PaginatedResponse } from '../api/types.ts'
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import { useFlag } from '@togglerino/react'
import FlagCard from '../components/FlagCard.tsx'
import CreateFlagModal from '../components/CreateFlagModal.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Checkbox } from '@/components/ui/checkbox'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { useIsMobile } from '@/hooks/useIsMobile'
import { useCanWrite } from '@/hooks/usePermissions'
import { Plus } from 'lucide-react'
import BulkActionBar from '../components/BulkActionBar.tsx'
import BulkConfirmDialog from '../components/BulkConfirmDialog.tsx'
import { formatRelativeTime } from '@/lib/date'

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
  const [ownerFilter, setOwnerFilter] = useState('')
  const [selectedFlags, setSelectedFlags] = useState<Set<string>>(new Set())
  const [bulkDialogOpen, setBulkDialogOpen] = useState(false)
  const [bulkAction, setBulkAction] = useState<{
    action: BulkAction
    environmentKey?: string
    tags?: string[]
    ownerId?: string | null
  } | null>(null)
  const unknownFlagsEnabled = useFlag('unknown-flags', false)
  const isMobile = useIsMobile()
  const canWrite = useCanWrite(key)

  const PAGE_SIZE = 50

  const {
    data: flagsData,
    isLoading: flagsLoading,
    error: flagsError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: ['projects', key, 'flags', { search, tag: tagFilter, lifecycle_status: statusFilter, flag_type: purposeFilter }],
    queryFn: ({ pageParam }) =>
      api.flags.list(key!, {
        search: search || undefined,
        tag: tagFilter || undefined,
        lifecycle_status: statusFilter || undefined,
        flag_type: purposeFilter || undefined,
        limit: PAGE_SIZE,
        offset: pageParam,
      }),
    initialPageParam: 0,
    getNextPageParam: (lastPage) =>
      lastPage.offset + lastPage.limit < lastPage.total
        ? lastPage.offset + lastPage.limit
        : undefined,
    enabled: !!key,
  })

  const flags = flagsData?.pages.flatMap((page) => page.data)

  const scrollRef = useInfiniteScroll({ hasNextPage, isFetchingNextPage, fetchNextPage })

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
    enabled: !!flags && flags.length > 0 && !isFetchingNextPage,
  })

  const { data: unknownFlags } = useQuery({
    queryKey: ['projects', key, 'unknown-flags'],
    queryFn: async () => {
      const res = await api.unknownFlags.list(key!)
      return res.data
    },
    enabled: !!key && unknownFlagsEnabled,
  })

  const { data: usersResponse } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get<PaginatedResponse<User>>('/management/users'),
  })
  const users = usersResponse?.data

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
    if (!ownerFilter) return flags
    return flags.filter((f) => {
      const matchesOwner = ownerFilter === 'unassigned' ? !f.owner_id : f.owner_id === ownerFilter
      return matchesOwner
    })
  }, [flags, ownerFilter])

  const selectAllChecked: boolean | 'indeterminate' =
    selectedFlags.size === 0
      ? false
      : selectedFlags.size === filtered.length
        ? true
        : 'indeterminate'

  const toggleSelect = (flagKey: string) => {
    setSelectedFlags((prev) => {
      const next = new Set(prev)
      if (next.has(flagKey)) {
        next.delete(flagKey)
      } else {
        next.add(flagKey)
      }
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selectedFlags.size === filtered.length) {
      setSelectedFlags(new Set())
    } else {
      setSelectedFlags(new Set(filtered.map((f) => f.key)))
    }
  }

  const handleBulkExecute = (action: BulkAction, params: {
    environmentKey?: string
    tags?: string[]
    ownerId?: string | null
  }) => {
    setBulkAction({ action, ...params })
    setBulkDialogOpen(true)
  }

  const handleBulkComplete = () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'all-configs'] })
    setSelectedFlags(new Set())
  }

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
        {!isMobile && canWrite && <Button onClick={() => setModalOpen(true)}>Create Flag</Button>}
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
          <div className="flex flex-col md:flex-row gap-2.5 mb-5 mt-5">
            <label
              className="flex items-center gap-2 cursor-pointer whitespace-nowrap select-none"
              onClick={(e) => { e.preventDefault(); toggleSelectAll() }}
            >
              <Checkbox
                checked={selectAllChecked}
                className="cursor-pointer"
              />
              <span className="text-[13px] text-muted-foreground">All</span>
            </label>
            <Input
              className="w-full md:flex-1 md:max-w-[300px]"
              placeholder="Search flags..."
              value={search}
              onChange={(e) => { setSearch(e.target.value); setSelectedFlags(new Set()) }}
            />
            {allTags.length > 0 && (
              <select
                className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer w-full md:w-auto md:min-w-[130px]"
                value={tagFilter}
                onChange={(e) => { setTagFilter(e.target.value); setSelectedFlags(new Set()) }}
              >
                <option value="">All Tags</option>
                {allTags.map((tag) => (
                  <option key={tag} value={tag}>{tag}</option>
                ))}
              </select>
            )}
            <select
              className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer w-full md:w-auto md:min-w-[130px]"
              value={purposeFilter}
              onChange={(e) => { setPurposeFilter(e.target.value as FlagPurpose | ''); setSelectedFlags(new Set()) }}
            >
              <option value="">All Purposes</option>
              <option value="release">Release</option>
              <option value="experiment">Experiment</option>
              <option value="operational">Operational</option>
              <option value="kill-switch">Kill Switch</option>
              <option value="permission">Permission</option>
            </select>
            <select
              className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer w-full md:w-auto md:min-w-[130px]"
              value={statusFilter}
              onChange={(e) => { setStatusFilter(e.target.value as LifecycleStatus | ''); setSelectedFlags(new Set()) }}
            >
              <option value="">All Statuses</option>
              <option value="active">Active</option>
              <option value="potentially_stale">Potentially Stale</option>
              <option value="stale">Stale</option>
              <option value="archived">Archived</option>
            </select>
            <select
              className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer w-full md:w-auto md:min-w-[130px]"
              value={ownerFilter}
              onChange={(e) => { setOwnerFilter(e.target.value); setSelectedFlags(new Set()) }}
            >
              <option value="">All Owners</option>
              <option value="unassigned">Unassigned</option>
              {users?.map((u) => (
                <option key={u.id} value={u.id}>{u.display_name ?? u.email}</option>
              ))}
            </select>
          </div>

          {filtered.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-[15px] font-medium text-foreground mb-1.5">
                {flagsData && flagsData.pages[0]?.total > 0 ? 'No flags match your filters' : 'No flags yet'}
              </div>
              <div className="text-[13px] text-muted-foreground/60">
                {flagsData && flagsData.pages[0]?.total > 0
                  ? 'Try adjusting your search or tag filter.'
                  : 'Create your first feature flag to get started.'}
              </div>
            </div>
          ) : (
            <>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {filtered.map((flag) => (
                <FlagCard
                  key={flag.id}
                  flag={flag}
                  environments={environments ?? []}
                  getEnvStatus={getEnvStatus}
                  onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
                  selected={selectedFlags.has(flag.key)}
                  onSelect={toggleSelect}
                />
              ))}
            </div>
            <div ref={scrollRef} className="h-1" />
            {isFetchingNextPage && (
              <div className="text-center py-4 text-muted-foreground/60 text-[13px] animate-pulse">
                Loading more flags...
              </div>
            )}
            </>
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
            <div className="rounded-lg border overflow-x-auto mt-5">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Flag Key</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Environment</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Requests</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">First Seen</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Last Seen</TableHead>
                    {canWrite && <TableHead className="font-mono text-[11px] uppercase tracking-wider">Actions</TableHead>}
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
                      {canWrite && (
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
                      )}
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

      {isMobile && canWrite && selectedFlags.size === 0 && (
        <button
          onClick={() => setModalOpen(true)}
          className="fixed bottom-6 right-6 z-50 w-14 h-14 rounded-full bg-[#d4956a] text-white shadow-lg flex items-center justify-center hover:bg-[#e0a87a] active:scale-95 transition-all focus-visible:ring-2 focus-visible:ring-[#d4956a] focus-visible:ring-offset-2"
          aria-label="Create Flag"
        >
          <Plus className="w-6 h-6" />
        </button>
      )}

      {selectedFlags.size > 0 && (
        <BulkActionBar
          selectedCount={selectedFlags.size}
          environments={environments ?? []}
          users={users ?? []}
          onExecute={handleBulkExecute}
          onClear={() => setSelectedFlags(new Set())}
        />
      )}

      {bulkAction && (
        <BulkConfirmDialog
          open={bulkDialogOpen}
          onClose={() => { setBulkDialogOpen(false); setBulkAction(null) }}
          projectKey={key!}
          flagKeys={Array.from(selectedFlags)}
          action={bulkAction.action}
          environmentKey={bulkAction.environmentKey}
          tags={bulkAction.tags}
          ownerId={bulkAction.ownerId}
          onComplete={handleBulkComplete}
        />
      )}
    </div>
  )
}
