import { useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type {
  Flag,
  Environment,
  FlagEnvironmentConfig,
  Variant,
  TargetingRule,
} from '../api/types.ts'
import VariantEditor from '../components/VariantEditor.tsx'
import RuleBuilder from '../components/RuleBuilder.tsx'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface FlagDetailResponse {
  flag: Flag
  environment_configs: FlagEnvironmentConfig[]
}

function ConfigEditor({
  config,
  flag,
  envKey,
  projectKey,
  flagKey,
  allConfigs,
  environments,
}: {
  config: FlagEnvironmentConfig | null
  flag: Flag
  envKey: string
  projectKey: string
  flagKey: string
  allConfigs: FlagEnvironmentConfig[]
  environments: Environment[]
}) {
  const queryClient = useQueryClient()
  const [enabled, setEnabled] = useState(config?.enabled ?? false)
  const [defaultVariant, setDefaultVariant] = useState(config?.default_variant ?? '')
  const [variants, setVariants] = useState<Variant[]>(config?.variants ?? [])
  const [rules, setRules] = useState<TargetingRule[]>(config?.targeting_rules ?? [])
  const [saved, setSaved] = useState(false)
  const [copySourceEnv, setCopySourceEnv] = useState<string | null>(null)

  const otherEnvironments = environments.filter((e) => e.key !== envKey)

  const updateConfig = useMutation({
    mutationFn: (data: {
      enabled: boolean
      default_variant: string
      variants: Variant[]
      targeting_rules: TargetingRule[]
    }) => api.put(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey] })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const handleSave = () => {
    updateConfig.mutate({
      enabled,
      default_variant: defaultVariant,
      variants,
      targeting_rules: rules,
    })
  }

  return (
    <div className="p-6 rounded-lg bg-card border">
      {/* Copy from environment */}
      {otherEnvironments.length > 0 && (
        <div className="flex items-center gap-3 mb-6 p-3 rounded-md bg-secondary/30 border border-dashed">
          <div className="text-[13px] text-muted-foreground whitespace-nowrap">Copy from</div>
          <Select onValueChange={(value) => setCopySourceEnv(value)}>
            <SelectTrigger className="w-[180px]" size="sm">
              <SelectValue placeholder="Select environment" />
            </SelectTrigger>
            <SelectContent>
              {otherEnvironments.map((env) => (
                <SelectItem key={env.key} value={env.key}>
                  {env.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* Toggle */}
      <div className="flex items-start gap-4 mb-6 p-4 rounded-md bg-secondary/50 border">
        <Switch checked={enabled} onCheckedChange={setEnabled} />
        <div>
          <span className="text-sm font-medium text-foreground">
            {enabled ? 'Enabled' : 'Disabled'} in {envKey}
          </span>
          <div className="text-xs text-muted-foreground leading-relaxed mt-1">
            {enabled
              ? 'Targeting rules and variants are active. Users are evaluated against rules below.'
              : 'All SDK evaluations return the default variant. Targeting rules are ignored.'}
          </div>
        </div>
      </div>

      {/* Default Variant */}
      <div className="mb-6">
        <div className="text-[13px] font-medium text-foreground mb-1">
          Default Variant
        </div>
        <div className="text-xs text-muted-foreground leading-relaxed mb-2.5">
          {flag.flag_type === 'boolean'
            ? "The value returned when no targeting rule matches. For boolean flags, this is typically 'on' or 'off'."
            : flag.flag_type === 'string'
            ? 'The string value returned when no targeting rule matches.'
            : flag.flag_type === 'number'
            ? 'The numeric value returned when no targeting rule matches.'
            : 'The JSON payload returned when no targeting rule matches.'}
        </div>
        {variants.length > 0 ? (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[160px]"
            value={defaultVariant}
            onChange={(e) => setDefaultVariant(e.target.value)}
          >
            <option value="">-- Select --</option>
            {variants.map((v) => (
              <option key={v.key} value={v.key}>
                {v.key}
              </option>
            ))}
          </select>
        ) : (
          <Input
            className="min-w-[200px] max-w-[300px]"
            placeholder="Variant key"
            value={defaultVariant}
            onChange={(e) => setDefaultVariant(e.target.value)}
          />
        )}
      </div>

      {/* Variants */}
      <div className="mb-6">
        <VariantEditor
          variants={variants}
          flagType={flag.flag_type}
          onChange={setVariants}
        />
      </div>

      {/* Targeting Rules */}
      <div className="mb-6">
        <RuleBuilder
          rules={rules}
          variants={variants}
          onChange={setRules}
        />
      </div>

      {/* Save */}
      <Button onClick={handleSave} disabled={updateConfig.isPending}>
        {updateConfig.isPending ? 'Saving...' : 'Save Configuration'}
      </Button>

      {saved && (
        <Alert className="mt-3 bg-emerald-500/10 border-emerald-500/20 text-emerald-400 animate-[fadeIn_200ms_ease]">
          <AlertDescription>Configuration saved successfully.</AlertDescription>
        </Alert>
      )}
      {updateConfig.error && (
        <Alert variant="destructive" className="mt-3">
          <AlertDescription>
            Failed to save: {updateConfig.error instanceof Error ? updateConfig.error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {/* Copy Config Confirmation Dialog */}
      <Dialog open={copySourceEnv !== null} onOpenChange={(open) => { if (!open) setCopySourceEnv(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Copy configuration?</DialogTitle>
            <DialogDescription>
              This will replace the current variants, targeting rules, and default variant
              in <span className="font-semibold text-foreground">{envKey}</span> with
              the configuration from <span className="font-semibold text-foreground">{copySourceEnv}</span>.
              The enabled/disabled state will not change.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCopySourceEnv(null)}>
              Cancel
            </Button>
            <Button onClick={() => {
              if (!copySourceEnv) return
              const sourceEnv = environments.find((e) => e.key === copySourceEnv)
              if (!sourceEnv) return
              const sourceConfig = allConfigs.find((c) => c.environment_id === sourceEnv.id)
              if (!sourceConfig) return
              setVariants(sourceConfig.variants ?? [])
              setRules(sourceConfig.targeting_rules ?? [])
              setDefaultVariant(sourceConfig.default_variant ?? '')
              setCopySourceEnv(null)
            }}>
              Copy Configuration
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default function FlagDetailPage() {
  const { key, flag: flagKey } = useParams<{ key: string; flag: string }>()

  const [selectedEnvKey, setSelectedEnvKey] = useState<string>('')

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

  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [archiveDialogOpen, setArchiveDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)

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

  const effectiveEnvKey = selectedEnvKey || (environments?.[0]?.key ?? '')

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
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {key}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground font-mono text-xs">{flagKey}</span>
      </div>

      {/* Flag Metadata Card */}
      <div className="p-6 rounded-lg bg-card border mb-6">
        <div className="text-xl font-semibold text-foreground mb-1 tracking-tight">
          {flag.name}
          {flag.archived && (
            <Badge variant="secondary" className="ml-2 text-xs align-middle">Archived</Badge>
          )}
        </div>
        <div className="text-[13px] font-mono text-[#d4956a] mb-3.5 tracking-wide">
          {flag.key}
        </div>
        <div className="flex gap-6 flex-wrap mb-2">
          <div>
            <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-1">
              Type
            </div>
            <Badge variant="secondary" className="font-mono text-xs">{flag.flag_type}</Badge>
          </div>
          {flag.tags && flag.tags.length > 0 && (
            <div>
              <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-1">
                Tags
              </div>
              <div className="flex gap-1">
                {flag.tags.map((tag) => (
                  <Badge key={tag} variant="outline" className="text-[11px]">{tag}</Badge>
                ))}
              </div>
            </div>
          )}
        </div>
        {flag.description && (
          <div className="text-[13px] text-muted-foreground leading-relaxed mt-2">
            {flag.description}
          </div>
        )}
      </div>

      {/* Danger Zone */}
      <div className="p-6 rounded-lg border border-destructive/30 mb-6">
        <div className="font-mono text-[10px] font-medium text-destructive uppercase tracking-wider mb-3">
          Danger Zone
        </div>
        {flag.archived ? (
          <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <div>
                <div className="text-[13px] font-medium text-foreground">Unarchive this flag</div>
                <div className="text-xs text-muted-foreground">Restore this flag so it can be evaluated again.</div>
              </div>
              <Button
                variant="outline"
                onClick={() => archiveMutation.mutate(false)}
                disabled={archiveMutation.isPending}
              >
                {archiveMutation.isPending ? 'Unarchiving...' : 'Unarchive'}
              </Button>
            </div>
            <div className="border-t border-destructive/20" />
            <div className="flex items-center justify-between">
              <div>
                <div className="text-[13px] font-medium text-foreground">Delete this flag permanently</div>
                <div className="text-xs text-muted-foreground">This action cannot be undone.</div>
              </div>
              <Button variant="destructive" onClick={() => setDeleteDialogOpen(true)}>
                Delete Flag
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-between">
            <div>
              <div className="text-[13px] font-medium text-foreground">Archive this flag</div>
              <div className="text-xs text-muted-foreground">
                Archived flags return default values and are excluded from evaluation.
              </div>
            </div>
            <Button
              variant="outline"
              className="border-destructive/50 text-destructive hover:bg-destructive/10"
              onClick={() => setArchiveDialogOpen(true)}
            >
              Archive Flag
            </Button>
          </div>
        )}
      </div>

      {archiveMutation.error && (
        <Alert variant="destructive" className="mb-6">
          <AlertDescription>
            Failed to update flag: {archiveMutation.error instanceof Error ? archiveMutation.error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}
      {deleteMutation.error && (
        <Alert variant="destructive" className="mb-6">
          <AlertDescription>
            Failed to delete flag: {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {/* Environment Tabs */}
      {environments && environments.length > 0 && (
        <Tabs value={effectiveEnvKey} onValueChange={setSelectedEnvKey}>
          <TabsList className="mb-6">
            {environments.map((env) => (
              <TabsTrigger key={env.key} value={env.key}>{env.name}</TabsTrigger>
            ))}
          </TabsList>
          {environments.map((env) => {
            const envConfig = data.environment_configs.find((c) => c.environment_id === env.id) ?? null
            return (
              <TabsContent key={env.key} value={env.key}>
                <ConfigEditor
                  config={envConfig}
                  flag={flag}
                  envKey={env.key}
                  projectKey={key!}
                  flagKey={flagKey!}
                  allConfigs={data.environment_configs}
                  environments={environments}
                />
              </TabsContent>
            )
          })}
        </Tabs>
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
