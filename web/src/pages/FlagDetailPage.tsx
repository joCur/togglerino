import { useState, useEffect } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { cn } from '@/lib/utils'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig } from '../api/types.ts'
import ConfigEditor from '../components/ConfigEditor.tsx'
import EvaluationFlow from '../components/EvaluationFlow.tsx'
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
import { Settings, Trash2, Archive, RotateCcw, AlertTriangle, ChevronRight } from 'lucide-react'

interface FlagDetailResponse {
  flag: Flag
  environment_configs: FlagEnvironmentConfig[]
}

export default function FlagDetailPage() {
  const { key, flag: flagKey } = useParams<{ key: string; flag: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [expandedEnvs, setExpandedEnvs] = useState<Set<string>>(new Set())
  const [archiveDialogOpen, setArchiveDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)

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

  // Auto-expand first environment on initial load
  useEffect(() => {
    if (environments && environments.length > 0) {
      setExpandedEnvs((prev) => {
        if (prev.size > 0) return prev
        return new Set([environments[0].key])
      })
    }
  }, [environments])

  const setEnvExpanded = (envKey: string, open: boolean) => {
    setExpandedEnvs((prev) => {
      const next = new Set(prev)
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
      {/* Back link */}
      <Link
        to={`/projects/${key}`}
        className="inline-flex items-center gap-1 text-[13px] text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        &larr; Back to flags
      </Link>

      {/* Header: flag key + settings dropdown */}
      <div className="flex items-start justify-between mb-1">
        <h1 className="text-xl font-mono text-[#d4956a] tracking-wide">{flag.key}</h1>

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
      </div>

      {/* Flag name */}
      <div className="text-[15px] text-muted-foreground mb-2">{flag.name}</div>

      {/* Metadata chips */}
      <div className="flex items-center gap-2 text-[13px] text-muted-foreground/60 mb-2">
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
      </div>

      {/* Description */}
      {flag.description && (
        <div className="text-[13px] text-muted-foreground/60 leading-relaxed mb-6">
          {flag.description}
        </div>
      )}
      {!flag.description && <div className="mb-6" />}

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

      {/* Environment Configuration section */}
      {environments && environments.length > 0 && (
        <>
          <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-4">
            Environment Configuration
          </div>

          <div className="flex flex-col gap-3">
            {environments.map((env) => {
              const config = data.environment_configs.find((c) => c.environment_id === env.id) ?? null
              const enabled = config?.enabled ?? false
              const isExpanded = expandedEnvs.has(env.key)

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
                      <div
                        className="flex items-center gap-2 ml-auto"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <span className={cn(
                          'text-[11px] font-mono font-medium',
                          enabled ? 'text-emerald-400' : 'text-muted-foreground/50',
                        )}>
                          {enabled ? 'ON' : 'OFF'}
                        </span>
                        <Switch
                          checked={enabled}
                          disabled={!config || toggleMutation.isPending}
                          onCheckedChange={() => {
                            if (config) toggleMutation.mutate({ envKey: env.key, config })
                          }}
                        />
                      </div>
                    </CollapsibleTrigger>

                    <CollapsibleContent>
                      <div className="px-4 pb-4 pt-1 border-t border-border/50">
                        <div className="mb-4 mt-3">
                          <EvaluationFlow config={config} />
                        </div>
                        <ConfigEditor
                          key={env.key}
                          config={config}
                          flag={flag}
                          envKey={env.key}
                          projectKey={key!}
                          flagKey={flagKey!}
                          allConfigs={data.environment_configs}
                          environments={environments}
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
    </div>
  )
}
