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
import RuleBuilder from './RuleBuilder.tsx'
import ScheduleChangeDialog from './ScheduleChangeDialog.tsx'
import { Button } from '@/components/ui/button'
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
} from '@/components/ui/collapsible'
import { Clock, Ban, CircleCheck } from 'lucide-react'
import { cn } from '@/lib/utils'

interface TargetingConfigProps {
  config: FlagEnvironmentConfig | null
  flag: Flag
  envKey: string
  projectKey: string
  flagKey: string
  allConfigs: FlagEnvironmentConfig[]
  environments: Environment[]
  readOnly?: boolean
}

function variantOptions(flag: Flag, variants: Variant[]) {
  if (flag.value_type === 'boolean') {
    return [
      { value: 'true', label: 'true' },
      { value: 'false', label: 'false' },
    ]
  }
  return variants.map((v) => ({ value: v.name, label: v.name }))
}

function InlineVariantSelect({
  value,
  options,
  onChange,
  disabled,
}: {
  value: string
  options: { value: string; label: string }[]
  onChange: (val: string) => void
  disabled?: boolean
}) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger
        size="sm"
        className="h-7 min-w-0 w-auto gap-1 px-2.5 text-xs font-mono bg-secondary/60 border-muted-foreground/20 hover:bg-secondary"
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((opt) => (
          <SelectItem key={opt.value} value={opt.value} className="text-xs font-mono">
            {opt.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

export default function TargetingConfig({
  config,
  flag,
  envKey,
  projectKey,
  flagKey,
  allConfigs,
  environments,
  readOnly,
}: TargetingConfigProps) {
  const queryClient = useQueryClient()
  const [enabled, setEnabled] = useState(config?.enabled ?? false)
  const [offVariant, setOffVariant] = useState(config?.off_variant ?? (flag.value_type === 'boolean' ? 'false' : ''))
  const [fallthroughVariant, setFallthroughVariant] = useState(config?.fallthrough_variant ?? (flag.value_type === 'boolean' ? 'false' : ''))
  const [variants, setVariants] = useState<Variant[]>(config?.variants ?? [])
  const [rules, setRules] = useState<TargetingRule[]>(config?.targeting_rules ?? [])
  const [saved, setSaved] = useState(false)
  const [copySourceEnv, setCopySourceEnv] = useState<string | null>(null)
  const [copyKey, setCopyKey] = useState(0)
  const [scheduleDialogOpen, setScheduleDialogOpen] = useState(false)

  const otherEnvironments = environments.filter((e) => e.key !== envKey)
  const options = variantOptions(flag, variants)

  const updateConfig = useMutation({
    mutationFn: (data: {
      enabled: boolean
      fallthrough_variant: string
      off_variant: string
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
      fallthrough_variant: fallthroughVariant,
      off_variant: offVariant,
      variants,
      targeting_rules: rules,
    })
  }

  return (
    <div className="p-6 rounded-lg bg-card border">
      {readOnly && (
        <div className="flex items-center gap-2 text-xs text-amber-400/80 mb-4 px-3 py-2 rounded-md bg-amber-500/5 border border-amber-500/10">
          <svg className="w-3.5 h-3.5 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
          You do not have write access to this environment.
        </div>
      )}

      {/* Sentence line */}
      <div className={cn('flex flex-wrap items-center gap-2 text-sm text-muted-foreground', readOnly && 'pointer-events-none')}>
        <span>Targeting is</span>
        <button
          type="button"
          onClick={() => setEnabled(!enabled)}
          disabled={readOnly || !config}
          className={cn(
            'inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-colors',
            enabled
              ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 hover:bg-emerald-500/20'
              : 'bg-muted text-muted-foreground border border-muted-foreground/20 hover:bg-muted/80',
            (readOnly || !config) && 'opacity-50 cursor-not-allowed',
          )}
        >
          {enabled ? (
            <CircleCheck className="w-3.5 h-3.5" />
          ) : (
            <Ban className="w-3.5 h-3.5" />
          )}
          {enabled ? 'ON' : 'OFF'}
        </button>

        {!enabled ? (
          <>
            <span>, serving</span>
            <InlineVariantSelect
              value={offVariant}
              options={options}
              onChange={setOffVariant}
              disabled={readOnly || options.length === 0}
            />
            <span>to all traffic</span>
          </>
        ) : (
          <span>, serving based on rules below</span>
        )}
      </div>

      {/* Rules section (when ON) */}
      {enabled && (
        <div className={cn('mt-6', readOnly && 'pointer-events-none opacity-50')}>
          {/* Rules with left-border timeline */}
          <div className="border-l-2 border-[#d4956a]/30 pl-5 ml-1">
            <RuleBuilder
              rules={rules}
              variants={variants}
              valueType={flag.value_type}
              onChange={setRules}
              projectKey={projectKey}
            />
          </div>

          {/* Fallthrough / default rule */}
          <div className="border-l-2 border-muted-foreground/20 pl-5 ml-1 mt-0">
            <div className="border-t border-dashed border-muted-foreground/20 pt-4 mt-4">
              <div className="text-[11px] font-mono uppercase tracking-wider text-muted-foreground/50 mb-2">
                Default rule
              </div>
              <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
                <span>Serve</span>
                <InlineVariantSelect
                  value={fallthroughVariant}
                  options={options}
                  onChange={setFallthroughVariant}
                  disabled={readOnly || options.length === 0}
                />
                <span>to all remaining traffic</span>
              </div>
            </div>
          </div>
        </div>
      )}


      {/* Copy from environment */}
      {!readOnly && otherEnvironments.length > 0 && (
        <div className="flex flex-col md:flex-row md:items-center gap-3 mt-6 p-3 rounded-md bg-secondary/30 border border-dashed">
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
        <div className="flex items-center gap-3 mt-6">
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
              Saved
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
        valueType={flag.value_type}
        currentConfig={{
          enabled,
          fallthrough_variant: fallthroughVariant,
          off_variant: offVariant,
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
              This will replace the current variants, targeting rules, and fallthrough variant
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
              setFallthroughVariant(sourceConfig.fallthrough_variant ?? '')
              setOffVariant(sourceConfig.off_variant ?? '')
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
