import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { cn } from '@/lib/utils'
import { api, ApiError } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig, User, PaginatedResponse } from '../api/types.ts'
import NotFoundState from '../components/NotFoundState.tsx'
import ConfigEditor from '../components/ConfigEditor.tsx'
import EvaluationFlow from '../components/EvaluationFlow.tsx'
import FlagHistory from '../components/FlagHistory.tsx'
import PendingSchedules from '../components/PendingSchedules.tsx'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Switch } from '@/components/ui/switch'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { gravatarUrl } from '@/lib/gravatar'
import { formatRelativeTime } from '@/lib/date'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useCanWrite, useIsProjectAdmin } from '@/hooks/usePermissions'
import { useEnvironmentWriteAccess } from '@/hooks/useEnvironmentAccess'
import PromoteDialog from '../components/PromoteDialog.tsx'
import { FlagOverrideControl } from '../components/FlagOverrideControl.tsx'
import { Settings, Trash2, Archive, RotateCcw, AlertTriangle, ChevronRight, Play, ArrowRightFromLine, Lock, Unlock } from 'lucide-react'

interface FlagDetailResponse {
  flag: Flag
  environment_configs: FlagEnvironmentConfig[]
}

export default function FlagDetailPage() {
  const { key, flag: flagKey } = useParams<{ key: string; flag: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const canWrite = useCanWrite(key)
  const isProjectAdmin = useIsProjectAdmin(key)
  const { canWriteEnv } = useEnvironmentWriteAccess(key)
  const [expandedEnvs, setExpandedEnvs] = useState<Set<string> | null>(null)
  const [archiveDialogOpen, setArchiveDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [promoteState, setPromoteState] = useState<{ sourceEnvKey: string; targetEnvKey: string } | null>(null)
  const [lockDialogState, setLockDialogState] = useState<{ open: boolean; envKey: string; envName: string } | null>(null)
  const [lockReason, setLockReason] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'flags', flagKey],
    queryFn: () => api.get<FlagDetailResponse>(`/projects/${key}/flags/${flagKey}`),
    enabled: !!key && !!flagKey,
  })

  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const { data: usersResponse } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get<PaginatedResponse<User>>('/management/users'),
  })
  const users = usersResponse?.data

  const { data: overridesData } = useQuery({
    queryKey: ['flag-overrides', key, flagKey],
    queryFn: () => api.overrides.getForFlag(key!, flagKey!),
    enabled: !!key && !!flagKey,
  })

  const archiveMutation = useMutation({
    mutationFn: (archived: boolean) =>
      api.put<Flag>(`/projects/${key}/flags/${flagKey}/archive`, { archived }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      setArchiveDialogOpen(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/projects/${key}/flags/${flagKey}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      navigate(`/projects/${key}`)
    },
  })

  const stalenessMutation = useMutation({
    mutationFn: () => api.put(`/projects/${key}/flags/${flagKey}/staleness`, { status: 'stale' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ envKey, config }: { envKey: string; config: FlagEnvironmentConfig }) =>
      api.put(`/projects/${key}/flags/${flagKey}/environments/${envKey}`, {
        enabled: !config.enabled,
        default_variant: config.default_variant,
        variants: config.variants,
        targeting_rules: config.targeting_rules,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
    },
  })

  const ownerMutation = useMutation({
    mutationFn: (ownerId: string | null) => {
      const f = data!.flag
      return api.put<Flag>(`/projects/${key}/flags/${flagKey}`, {
        name: f.name,
        description: f.description,
        tags: f.tags,
        flag_type: f.flag_type,
        owner_id: ownerId,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
    },
  })

  const promoteMutation = useMutation({
    mutationFn: ({ sourceEnvKey, targetEnvKey }: { sourceEnvKey: string; targetEnvKey: string }) =>
      api.environments.promote(key!, flagKey!, sourceEnvKey, targetEnvKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
      setPromoteState(null)
    },
  })

  const lockMutation = useMutation({
    mutationFn: ({ envKey, reason }: { envKey: string; reason?: string }) =>
      api.flags.lock(key!, flagKey!, envKey, reason),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
    },
    onError: () => {},
  })

  const unlockMutation = useMutation({
    mutationFn: ({ envKey }: { envKey: string }) =>
      api.flags.unlock(key!, flagKey!, envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
    },
    onError: () => {},
  })

  // Sort environments by sort_order for promotion targets
  const sortedEnvironments = environments ? [...environments].sort((a, b) => a.sort_order - b.sort_order) : undefined

  // Derive effective expanded set: null means "not yet interacted, show first env expanded"
  const defaultExpanded = environments && environments.length > 0
    ? new Set([environments[0].key])
    : new Set<string>()
  const effectiveExpandedEnvs = expandedEnvs ?? defaultExpanded

  const setEnvExpanded = (envKey: string, open: boolean) => {
    setExpandedEnvs((prev) => {
      const current = prev ?? defaultExpanded
      const next = new Set(current)
      if (open) next.add(envKey)
      else next.delete(envKey)
      return next
    })
  }

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading flag details...
      </div>
    )
  }

  if (error instanceof ApiError && error.status === 404) {
    return (
      <NotFoundState
        title="Flag not found"
        description={`The flag "${flagKey}" could not be found. It may have been deleted.`}
        backTo={`/projects/${key}`}
        backLabel="Flags"
      />
    )
  }

  if (error || !data) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load flag: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  const flag = data.flag

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      {/* Breadcrumbs */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {key}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">{flag.key}</span>
      </div>

      {/* Header: flag key + actions */}
      <div className="flex items-start justify-between mb-1">
        <div className="flex items-center gap-3">
          <h1 className="text-xl font-mono text-[#d4956a] tracking-wide">{flag.key}</h1>
          <Link
            to={`/projects/${key}/playground?flag=${flag.key}`}
            className="inline-flex items-center gap-1.5 text-[12px] text-muted-foreground hover:text-foreground transition-colors px-2 py-1 rounded-md hover:bg-foreground/[0.05]"
          >
            <Play className="w-3.5 h-3.5" />
            Test in Playground
          </Link>
        </div>

        {canWrite && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                <Settings className="w-4 h-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {flag.lifecycle_status === 'archived' ? (
                <>
                  <DropdownMenuItem onClick={() => archiveMutation.mutate(false)}>
                    <RotateCcw className="w-4 h-4 mr-2" />
                    Unarchive
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onClick={() => setDeleteDialogOpen(true)}
                  >
                    <Trash2 className="w-4 h-4 mr-2" />
                    Delete permanently
                  </DropdownMenuItem>
                </>
              ) : (
                <>
                  {flag.lifecycle_status === 'potentially_stale' && (
                    <>
                      <DropdownMenuItem onClick={() => stalenessMutation.mutate()}>
                        <AlertTriangle className="w-4 h-4 mr-2" />
                        Mark as stale
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                    </>
                  )}
                  <DropdownMenuItem onClick={() => setArchiveDialogOpen(true)}>
                    <Archive className="w-4 h-4 mr-2" />
                    Archive
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>

      {/* Flag name */}
      <div className="text-[15px] text-muted-foreground mb-2">{flag.name}</div>

      {/* Metadata chips */}
      <div className="flex flex-wrap items-center gap-2 text-[13px] text-muted-foreground/60 mb-2">
        <Badge variant="secondary" className="font-mono text-[11px]">{flag.value_type}</Badge>
        <span>&middot;</span>
        <Badge variant="secondary" className="text-[11px] capitalize">{flag.flag_type}</Badge>
        <span>&middot;</span>
        <Badge
          variant="secondary"
          className={cn(
            'text-[11px]',
            flag.lifecycle_status === 'active' && 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
            flag.lifecycle_status === 'potentially_stale' && 'bg-amber-500/10 text-amber-400 border-amber-500/20',
            flag.lifecycle_status === 'stale' && 'bg-red-500/10 text-red-400 border-red-500/20',
            flag.lifecycle_status === 'archived' && 'bg-muted text-muted-foreground',
          )}
        >
          {flag.lifecycle_status.replace(/_/g, ' ')}
        </Badge>
        {flag.tags && flag.tags.length > 0 && (
          <>
            <span>&middot;</span>
            {flag.tags.map((tag) => (
              <Badge key={tag} variant="outline" className="text-[11px]">{tag}</Badge>
            ))}
          </>
        )}
        <span>&middot;</span>
        <Badge
          variant="secondary"
          className={cn(
            'text-[11px]',
            flag.last_evaluated_at
              ? 'bg-muted text-muted-foreground'
              : 'bg-amber-500/10 text-amber-400 border-amber-500/20',
          )}
        >
          {flag.last_evaluated_at
            ? `Last evaluated ${formatRelativeTime(flag.last_evaluated_at)}`
            : 'Never evaluated'}
        </Badge>
      </div>

      {/* Description */}
      {flag.description && (
        <div className="text-[13px] text-muted-foreground/60 leading-relaxed mb-6">
          {flag.description}
        </div>
      )}
      {!flag.description && <div className="mb-6" />}

      {/* Owner */}
      <div className="flex items-center gap-3 mb-6">
        <span className="text-[11px] text-muted-foreground/50 uppercase tracking-wider font-mono">Owner</span>
        <Select
          value={flag.owner_id ?? 'unassigned'}
          onValueChange={(value) => ownerMutation.mutate(value === 'unassigned' ? null : value)}
          disabled={!canWrite}
        >
          <SelectTrigger className="w-[220px] h-8 text-[13px]">
            <SelectValue>
              {flag.owner ? (
                <span className="flex items-center gap-2">
                  <img src={gravatarUrl(flag.owner.email, 20)} alt="" className="w-5 h-5 rounded-full" />
                  {flag.owner.display_name ?? flag.owner.email}
                </span>
              ) : (
                <span className="text-muted-foreground/60">Unassigned</span>
              )}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="unassigned">Unassigned</SelectItem>
            {users?.map((u) => (
              <SelectItem key={u.id} value={u.id}>
                <span className="flex items-center gap-2">
                  <img src={gravatarUrl(u.email, 20)} alt="" className="w-5 h-5 rounded-full" />
                  {u.display_name ?? u.email}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <Tabs defaultValue="configuration" className="w-full">
        <TabsList className="mb-6">
          <TabsTrigger value="configuration">Configuration</TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
        </TabsList>

        <TabsContent value="configuration">
          {/* Mutation error alerts */}
          {archiveMutation.error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>
                Failed to update flag: {archiveMutation.error instanceof Error ? archiveMutation.error.message : 'Unknown error'}
              </AlertDescription>
            </Alert>
          )}
          {deleteMutation.error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>
                Failed to delete flag: {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Unknown error'}
              </AlertDescription>
            </Alert>
          )}
          {lockMutation.error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>
                Failed to lock flag: {lockMutation.error instanceof Error ? lockMutation.error.message : 'Unknown error'}
              </AlertDescription>
            </Alert>
          )}
          {unlockMutation.error && (
            <Alert variant="destructive" className="mb-4">
              <AlertDescription>
                Failed to unlock flag: {unlockMutation.error instanceof Error ? unlockMutation.error.message : 'Unknown error'}
              </AlertDescription>
            </Alert>
          )}

          {/* Environment Configuration section */}
          {environments && environments.length > 0 && (
            <>
              <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-4">
                Environment Configuration
              </div>

              <div className="flex flex-col gap-3">
                {sortedEnvironments!.map((env) => {
                  const config = data.environment_configs.find((c) => c.environment_id === env.id) ?? null
                  const enabled = config?.enabled ?? false
                  const isExpanded = effectiveExpandedEnvs.has(env.key)
                  const envWritable = canWrite && canWriteEnv(env.id)
                  const promoteTargets = sortedEnvironments!.filter((e) => e.sort_order > env.sort_order && canWriteEnv(e.id))

                  return (
                    <Collapsible
                      key={env.id}
                      open={isExpanded}
                      onOpenChange={(open) => setEnvExpanded(env.key, open)}
                    >
                      <div className={cn(
                        'rounded-lg border transition-colors duration-200',
                        isExpanded ? 'border-[#d4956a]/40' : 'border-border',
                      )}>
                        <CollapsibleTrigger className="flex items-center w-full px-4 py-3 cursor-pointer group">
                          <ChevronRight className={cn(
                            'w-4 h-4 text-muted-foreground transition-transform duration-200 mr-3 shrink-0',
                            isExpanded && 'rotate-90',
                          )} />
                          <span className="text-[14px] font-medium text-foreground mr-3">
                            {env.name}
                          </span>
                          {canWrite && !envWritable && (
                            <span className="flex items-center gap-1 text-[11px] text-muted-foreground/50 mr-2" title="You don't have write access to this environment">
                              <Lock className="w-3 h-3" />
                              <span className="hidden sm:inline">Restricted</span>
                            </span>
                          )}
                          {config?.locked && (
                            <Badge variant="destructive" className="text-xs mr-2">
                              <Lock className="h-3 w-3 mr-1" />
                              Locked
                            </Badge>
                          )}
                          <div
                            className="flex items-center gap-2 ml-auto"
                            onClick={(e) => e.stopPropagation()}
                          >
                            {isProjectAdmin && (
                              config?.locked ? (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground"
                                  onClick={() => unlockMutation.mutate({ envKey: env.key })}
                                  disabled={unlockMutation.isPending}
                                >
                                  <Unlock className="h-3.5 w-3.5 mr-1" />
                                  Unlock
                                </Button>
                              ) : (
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground"
                                  onClick={() => {
                                    setLockDialogState({ open: true, envKey: env.key, envName: env.name })
                                    setLockReason('')
                                  }}
                                >
                                  <Lock className="h-3.5 w-3.5 mr-1" />
                                  Lock
                                </Button>
                              )
                            )}
                            {envWritable && promoteTargets.length > 0 && (
                              <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                  <Button variant="ghost" size="sm" className="h-7 px-2 text-[11px] text-muted-foreground hover:text-foreground">
                                    <ArrowRightFromLine className="w-3.5 h-3.5 mr-1" />
                                    Promote
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                  {promoteTargets.map((target) => {
                                    const targetConfig = data.environment_configs.find((c) => c.environment_id === target.id)
                                    const isLocked = targetConfig?.locked ?? false
                                    return (
                                      <DropdownMenuItem
                                        key={target.id}
                                        disabled={isLocked}
                                        onClick={() => setPromoteState({ sourceEnvKey: env.key, targetEnvKey: target.key })}
                                      >
                                        {isLocked && <Lock className="h-3 w-3 mr-1" />}
                                        Promote to {target.name}
                                      </DropdownMenuItem>
                                    )
                                  })}
                                </DropdownMenuContent>
                              </DropdownMenu>
                            )}
                            <span className={cn(
                              'text-[11px] font-mono font-medium',
                              enabled ? 'text-emerald-400' : 'text-muted-foreground/50',
                            )}>
                              {enabled ? 'ON' : 'OFF'}
                            </span>
                            <Switch
                              checked={enabled}
                              disabled={!envWritable || !config || config.locked || toggleMutation.isPending}
                              onCheckedChange={() => {
                                if (config) toggleMutation.mutate({ envKey: env.key, config })
                              }}
                            />
                          </div>
                        </CollapsibleTrigger>

                        <CollapsibleContent>
                          {config?.locked && (
                            <div className="flex items-center gap-2 px-4 py-2.5 bg-amber-950/30 border-t border-amber-900/30">
                              <Lock className="h-4 w-4 text-amber-500 shrink-0" />
                              <span className="text-sm text-amber-500">
                                Locked by {config.locked_by_user?.display_name || config.locked_by_user?.email || 'unknown'}
                                {config.lock_reason && ` — ${config.lock_reason}`}
                              </span>
                              {config.locked_at && (
                                <span className="text-xs text-muted-foreground ml-auto">
                                  {formatRelativeTime(config.locked_at)}
                                </span>
                              )}
                            </div>
                          )}
                          <div className="px-4 pb-4 pt-1 border-t border-border/50">
                            <div className="flex items-center justify-between mb-4 mt-3">
                              <EvaluationFlow config={config} />
                              <FlagOverrideControl
                                projectKey={key!}
                                flagKey={flagKey!}
                                envKey={env.key}
                                valueType={flag.value_type}
                                override={overridesData?.find((o) => o.environment_key === env.key)}
                              />
                            </div>
                            <PendingSchedules
                              projectKey={key!}
                              flagKey={flagKey!}
                              envKey={env.key}
                              flagId={flag.id}
                              environmentId={env.id}
                            />
                            <ConfigEditor
                              key={env.key}
                              config={config}
                              flag={flag}
                              envKey={env.key}
                              projectKey={key!}
                              flagKey={flagKey!}
                              allConfigs={data.environment_configs}
                              environments={environments}
                              readOnly={!envWritable || !!config?.locked}
                            />
                          </div>
                        </CollapsibleContent>
                      </div>
                    </Collapsible>
                  )
                })}
              </div>
            </>
          )}

          {(!environments || environments.length === 0) && (
            <div className="py-8 text-center text-muted-foreground/60 text-[13px]">
              No environments found for this project.
            </div>
          )}
        </TabsContent>

        <TabsContent value="history">
          {environments && environments.length > 0 && (
            <FlagHistory
              projectKey={key!}
              flagKey={flagKey!}
              environments={environments}
            />
          )}
        </TabsContent>
      </Tabs>

      {/* Archive Confirmation Dialog */}
      <Dialog open={archiveDialogOpen} onOpenChange={setArchiveDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Archive {flag.name}?</DialogTitle>
            <DialogDescription>
              Archived flags return default values and are excluded from targeting evaluation.
              You can unarchive it later.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setArchiveDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => archiveMutation.mutate(true)}
              disabled={archiveMutation.isPending}
            >
              {archiveMutation.isPending ? 'Archiving...' : 'Archive'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Permanently delete {flag.name}?</DialogTitle>
            <DialogDescription>
              This will permanently remove the flag and all its environment configurations.
              This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => deleteMutation.mutate()}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete Permanently'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Lock Dialog */}
      <Dialog
        open={lockDialogState?.open ?? false}
        onOpenChange={(open) => !open && setLockDialogState(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Lock Flag in Environment</DialogTitle>
            <DialogDescription>
              Locking prevents any configuration changes to this flag in the selected environment.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label className="text-muted-foreground text-xs">Flag</Label>
              <p className="font-mono text-sm">{flagKey}</p>
            </div>
            <div>
              <Label className="text-muted-foreground text-xs">Environment</Label>
              <p className="font-mono text-sm">{lockDialogState?.envName}</p>
            </div>
            <div>
              <Label htmlFor="lock-reason">Reason (optional)</Label>
              <Input
                id="lock-reason"
                placeholder="e.g. Holiday code freeze"
                value={lockReason}
                onChange={(e) => setLockReason(e.target.value)}
                maxLength={255}
              />
              <p className="text-xs text-muted-foreground mt-1">{lockReason.length}/255</p>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setLockDialogState(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (lockDialogState) {
                  lockMutation.mutate(
                    { envKey: lockDialogState.envKey, reason: lockReason || undefined },
                    { onSuccess: () => setLockDialogState(null) },
                  )
                }
              }}
              disabled={lockMutation.isPending}
            >
              <Lock className="h-3.5 w-3.5 mr-1" />
              Lock Flag
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Promote Dialog */}
      {promoteState && sortedEnvironments && (
        <PromoteDialog
          sourceConfig={data.environment_configs.find((c) => c.environment_id === sortedEnvironments.find((e) => e.key === promoteState.sourceEnvKey)?.id) ?? null}
          targetConfig={data.environment_configs.find((c) => c.environment_id === sortedEnvironments.find((e) => e.key === promoteState.targetEnvKey)?.id) ?? null}
          sourceEnvName={sortedEnvironments.find((e) => e.key === promoteState.sourceEnvKey)?.name ?? promoteState.sourceEnvKey}
          targetEnvName={sortedEnvironments.find((e) => e.key === promoteState.targetEnvKey)?.name ?? promoteState.targetEnvKey}
          open={!!promoteState}
          onOpenChange={(open) => { if (!open) setPromoteState(null) }}
          onConfirm={() => promoteMutation.mutate(promoteState)}
          isLoading={promoteMutation.isPending}
          error={promoteMutation.error instanceof Error ? promoteMutation.error : promoteMutation.error ? new Error('Promotion failed') : null}
        />
      )}
    </div>
  )
}
