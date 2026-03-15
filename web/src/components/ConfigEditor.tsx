import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type {
  Flag,
  Environment,
  FlagEnvironmentConfig,
  Variant,
  TargetingRule,
} from '../api/types.ts'
import VariantEditor from './VariantEditor.tsx'
import RuleBuilder from './RuleBuilder.tsx'
import ScheduleChangeDialog from './ScheduleChangeDialog.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { ChevronRight, Clock } from 'lucide-react'
import { cn } from '@/lib/utils'

interface Props {
  config: FlagEnvironmentConfig | null
  flag: Flag
  envKey: string
  projectKey: string
  flagKey: string
  allConfigs: FlagEnvironmentConfig[]
  environments: Environment[]
  readOnly?: boolean
}

export default function ConfigEditor({
  config,
  flag,
  envKey,
  projectKey,
  flagKey,
  allConfigs,
  environments,
  readOnly,
}: Props) {
  const queryClient = useQueryClient()
  const [defaultVariant, setDefaultVariant] = useState(config?.default_variant ?? '')
  const [variants, setVariants] = useState<Variant[]>(config?.variants ?? [])
  const [rules, setRules] = useState<TargetingRule[]>(config?.targeting_rules ?? [])
  const [saved, setSaved] = useState(false)
  const [copySourceEnv, setCopySourceEnv] = useState<string | null>(null)
  const [copyKey, setCopyKey] = useState(0)
  const [variantsOpen, setVariantsOpen] = useState((config?.variants ?? []).length > 0)
  const [rulesOpen, setRulesOpen] = useState((config?.targeting_rules ?? []).length > 0)
  const [scheduleDialogOpen, setScheduleDialogOpen] = useState(false)

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
      enabled: config?.enabled ?? false,
      default_variant: flag.value_type === 'boolean' ? '' : defaultVariant,
      variants: flag.value_type === 'boolean' ? [] : variants,
      targeting_rules: rules,
    })
  }

  return (
    <div className="p-6 rounded-lg bg-card border">
      <div className="text-[13px] font-medium text-muted-foreground mb-4">
        Configuration: <span className="text-foreground">{envKey}</span>
      </div>

      {readOnly && (
        <div className="flex items-center gap-2 text-xs text-amber-400/80 mb-4 px-3 py-2 rounded-md bg-amber-500/5 border border-amber-500/10">
          <svg className="w-3.5 h-3.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          You do not have write access to this environment.
        </div>
      )}
      <div className={readOnly ? 'pointer-events-none opacity-50' : ''}>
      {/* Default Variant — hidden for boolean flags */}
      {flag.value_type !== 'boolean' && (
      <div className="mb-6">
        <div className="text-[13px] font-medium text-foreground mb-1">Default Variant</div>
        <div className="text-xs text-muted-foreground leading-relaxed mb-2.5">
          {`The ${flag.value_type} value returned when no targeting rule matches.`}
        </div>
        {variants.length > 0 ? (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer w-full md:min-w-[160px] md:w-auto"
            value={defaultVariant}
            onChange={(e) => setDefaultVariant(e.target.value)}
          >
            <option value="">-- Select --</option>
            {variants.map((v) => (
              <option key={v.key} value={v.key}>{v.key}</option>
            ))}
          </select>
        ) : (
          <Input
            className="w-full md:min-w-[200px] md:max-w-[300px]"
            placeholder="Variant key"
            value={defaultVariant}
            onChange={(e) => setDefaultVariant(e.target.value)}
          />
        )}
      </div>
      )}

      {/* Variants (collapsible) — hidden for boolean flags */}
      {flag.value_type !== 'boolean' && (
      <Collapsible open={variantsOpen} onOpenChange={setVariantsOpen} className="mb-6">
        <CollapsibleTrigger className="flex items-center gap-2 w-full text-left group">
          <ChevronRight className={cn(
            'w-4 h-4 text-muted-foreground transition-transform duration-200',
            variantsOpen && 'rotate-90',
          )} />
          <span className="text-[13px] font-medium text-foreground">
            Variants
            <span className="text-muted-foreground/60 font-normal ml-1.5">
              ({variants.length})
            </span>
          </span>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-3 pl-6">
          <VariantEditor
            variants={variants}
            valueType={flag.value_type}
            onChange={setVariants}
          />
        </CollapsibleContent>
      </Collapsible>
      )}

      {/* Targeting Rules (collapsible) */}
      <Collapsible open={rulesOpen} onOpenChange={setRulesOpen} className="mb-6">
        <CollapsibleTrigger className="flex items-center gap-2 w-full text-left group">
          <ChevronRight className={cn(
            'w-4 h-4 text-muted-foreground transition-transform duration-200',
            rulesOpen && 'rotate-90',
          )} />
          <span className="text-[13px] font-medium text-foreground">
            Targeting Rules
            <span className="text-muted-foreground/60 font-normal ml-1.5">
              ({rules.length})
            </span>
          </span>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-3 pl-6">
          <RuleBuilder
            rules={rules}
            variants={variants}
            onChange={setRules}
            projectKey={projectKey}
          />
        </CollapsibleContent>
      </Collapsible>
      </div>

      {/* Copy from environment */}
      {!readOnly && otherEnvironments.length > 0 && (
        <div className="flex flex-col md:flex-row md:items-center gap-3 mb-6 p-3 rounded-md bg-secondary/30 border border-dashed">
          <div className="text-[13px] text-muted-foreground whitespace-nowrap">Copy from</div>
          <Select key={copyKey} onValueChange={(value) => setCopySourceEnv(value)}>
            <SelectTrigger className="w-full md:w-[180px]" size="sm">
              <SelectValue placeholder="Select environment" />
            </SelectTrigger>
            <SelectContent>
              {otherEnvironments.map((env) => (
                <SelectItem key={env.key} value={env.key}>{env.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* Save + Schedule */}
      {!readOnly && (
        <div className="flex items-center gap-3">
          <Button onClick={handleSave} disabled={updateConfig.isPending}>
            {updateConfig.isPending ? 'Saving...' : 'Save Configuration'}
          </Button>
          <Button
            variant="outline"
            onClick={() => setScheduleDialogOpen(true)}
            disabled={flag.lifecycle_status === 'archived'}
          >
            <Clock className="w-3.5 h-3.5 mr-1.5" />
            Schedule
          </Button>
          {saved && (
            <span className="text-[13px] text-emerald-400 animate-[fadeIn_200ms_ease]">
              Saved ✓
            </span>
          )}
        </div>
      )}

      {updateConfig.error && (
        <Alert variant="destructive" className="mt-3">
          <AlertDescription>
            Failed to save: {updateConfig.error instanceof Error ? updateConfig.error.message : 'Unknown error'}
          </AlertDescription>
        </Alert>
      )}

      {/* Schedule Change Dialog */}
      <ScheduleChangeDialog
        open={scheduleDialogOpen}
        onClose={() => setScheduleDialogOpen(false)}
        projectKey={projectKey}
        flagKey={flagKey}
        envKey={envKey}
        currentConfig={{
          enabled: config?.enabled ?? false,
          default_variant: defaultVariant,
          variants,
          targeting_rules: rules,
        }}
      />

      {/* Copy Config Confirmation Dialog */}
      <Dialog open={copySourceEnv !== null} onOpenChange={(open) => { if (!open) { setCopySourceEnv(null); setCopyKey((k) => k + 1) } }}>
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
            <Button variant="outline" onClick={() => { setCopySourceEnv(null); setCopyKey((k) => k + 1) }}>
              Cancel
            </Button>
            <Button onClick={() => {
              if (!copySourceEnv) return
              const sourceEnv = environments.find((e) => e.key === copySourceEnv)
              if (!sourceEnv) return
              const sourceConfig = allConfigs.find((c) => c.environment_id === sourceEnv.id)
              if (!sourceConfig) return
              setVariants(structuredClone(sourceConfig.variants ?? []))
              setRules(structuredClone(sourceConfig.targeting_rules ?? []))
              setDefaultVariant(sourceConfig.default_variant ?? '')
              setCopySourceEnv(null)
              setCopyKey((k) => k + 1)
            }}>
              Copy Configuration
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
