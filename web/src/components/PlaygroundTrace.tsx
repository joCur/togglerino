import type { EvaluationTrace, TraceStep, ConditionTrace } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { Check, X, Minus, ArrowRight } from 'lucide-react'

function StepIcon({ passed, skipped }: { passed: boolean; skipped?: boolean }) {
  if (skipped) {
    return (
      <div className="w-5 h-5 rounded-full bg-muted flex items-center justify-center shrink-0">
        <Minus className="w-3 h-3 text-muted-foreground" />
      </div>
    )
  }
  if (passed) {
    return (
      <div className="w-5 h-5 rounded-full bg-emerald-500/20 flex items-center justify-center shrink-0">
        <Check className="w-3 h-3 text-emerald-400" />
      </div>
    )
  }
  return (
    <div className="w-5 h-5 rounded-full bg-red-500/20 flex items-center justify-center shrink-0">
      <X className="w-3 h-3 text-red-400" />
    </div>
  )
}

function ConditionRow({ condition, indent = false }: { condition: ConditionTrace; indent?: boolean }) {
  return (
    <>
      <div className={cn('flex items-center gap-2 text-[12px] py-1', indent && 'ml-4')}>
        <StepIcon passed={condition.passed} />
        <span className="font-mono text-muted-foreground">{condition.attribute}</span>
        <span className="text-muted-foreground/60">{condition.operator}</span>
        {condition.operator !== 'segment_match' && (
          <>
            <span className="font-mono text-foreground">{formatValue(condition.condition_value)}</span>
            <span className="text-muted-foreground/40">|</span>
            <span className="text-[11px] text-muted-foreground/60">actual:</span>
            <span className="font-mono text-foreground">
              {condition.actual_value !== undefined ? formatValue(condition.actual_value) : <span className="text-muted-foreground/40 italic">undefined</span>}
            </span>
          </>
        )}
        {condition.operator === 'segment_match' && (
          <span className="font-mono text-[#d4956a]">{String(condition.condition_value)}</span>
        )}
      </div>
      {condition.segment_trace && condition.segment_trace.length > 0 && (
        <div className="border-l border-border/50 ml-2.5 pl-2">
          {condition.segment_trace.map((sub, i) => (
            <ConditionRow key={i} condition={sub} indent />
          ))}
        </div>
      )}
    </>
  )
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function RolloutBar({ step }: { step: TraceStep }) {
  if (step.percentage_rollout === undefined || step.percentage_rollout === null) return null

  const pct = step.percentage_rollout
  const bucket = step.hash_bucket ?? 0
  const inRollout = step.in_rollout ?? false

  return (
    <div className="mt-2 flex flex-col gap-1">
      <div className="flex items-center gap-2 text-[11px] text-muted-foreground/60">
        <span>Rollout: <span className="font-mono text-foreground">{pct}%</span></span>
        <span className="text-muted-foreground/40">|</span>
        <span>Bucket: <span className="font-mono text-foreground">{bucket}</span></span>
        <span className="text-muted-foreground/40">|</span>
        <Badge
          variant="secondary"
          className={cn(
            'text-[10px]',
            inRollout
              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
              : 'bg-red-500/10 text-red-400 border-red-500/20',
          )}
        >
          {inRollout ? 'In rollout' : 'Outside rollout'}
        </Badge>
      </div>
      <div className="h-1.5 bg-muted rounded-full overflow-hidden w-full max-w-[200px] relative">
        <div
          className="h-full bg-[#d4956a] rounded-full"
          style={{ width: `${pct}%` }}
        />
        <div
          className="absolute top-0 h-full w-0.5 bg-foreground"
          style={{ left: `${bucket}%` }}
        />
      </div>
    </div>
  )
}

function LifecycleStepContent({ step }: { step: TraceStep }) {
  const status = step.status ?? (step.passed ? 'active' : 'archived')
  if (step.passed) {
    return (
      <span className="text-[13px] text-muted-foreground">
        Flag is <span className="text-emerald-400 font-medium">{status}</span>
      </span>
    )
  }
  return (
    <span className="text-[13px] text-muted-foreground">
      Flag is <span className="text-red-400 font-medium">{status}</span>, returning default value
    </span>
  )
}

function EnabledStepContent({ step, offVariant }: { step: TraceStep; offVariant?: string }) {
  const enabled = step.enabled ?? false
  if (enabled) {
    return (
      <span className="text-[13px] text-muted-foreground">
        Targeting is{' '}
        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
          ON
        </span>
        , evaluating rules
      </span>
    )
  }
  return (
    <span className="text-[13px] text-muted-foreground">
      Targeting is{' '}
      <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-medium bg-muted text-muted-foreground border border-muted-foreground/20">
        OFF
      </span>
      {offVariant && (
        <>
          , serving <span className="font-mono text-[#d4956a]">{offVariant}</span> to all traffic
        </>
      )}
    </span>
  )
}

function RuleStepContent({ step, index }: { step: TraceStep; index: number }) {
  // Fallback: subtract 2 to account for lifecycle_check + enabled_check steps before rules
  const ruleNum = (step.rule_index ?? index - 2) + 1
  const matched = step.matched ?? false

  if (step.skipped) {
    return (
      <span className="text-[13px] text-muted-foreground">
        <span className="font-medium text-foreground">Rule {ruleNum}</span> skipped
      </span>
    )
  }

  const variantFragment = step.variant && (
    <>, serving <span className="font-mono text-[#d4956a]">{step.variant}</span></>
  )

  const rolloutFragment = step.percentage_rollout != null && (
    <> to <span className="font-medium text-foreground">{step.percentage_rollout}%</span> of traffic</>
  )

  return (
    <div>
      <span className="text-[13px] text-muted-foreground">
        <span className="font-medium text-foreground">Rule {ruleNum}</span>
        {matched ? (
          <> matched{variantFragment}{rolloutFragment}</>
        ) : (
          <> did not match</>
        )}
      </span>

      {step.conditions && step.conditions.length > 0 && (
        <div className="mt-2 border-l-2 border-border/50 pl-3 flex flex-col gap-0.5">
          {step.conditions.map((cond, i) => (
            <ConditionRow key={i} condition={cond} />
          ))}
        </div>
      )}

      <RolloutBar step={step} />
    </div>
  )
}

function TraceStepRow({ step, index, isSelected, offVariant }: { step: TraceStep; index: number; isSelected: boolean; offVariant?: string }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <StepIcon passed={step.passed} skipped={step.skipped} />
        <div className="w-px flex-1 bg-border/50" />
      </div>
      <div className="flex-1 pb-4 -mt-0.5">
        <div className={cn(
          'rounded-lg border p-3',
          isSelected && 'border-[#d4956a]/50 bg-[#d4956a]/5',
          step.skipped && 'opacity-50',
        )}>
          {step.type === 'lifecycle_check' && <LifecycleStepContent step={step} />}
          {step.type === 'enabled_check' && (
            <EnabledStepContent step={step} offVariant={offVariant} />
          )}
          {step.type === 'rule' && <RuleStepContent step={step} index={index} />}
        </div>
      </div>
    </div>
  )
}

function FallthroughStep({ variant }: { variant: string }) {
  return (
    <div className="flex gap-3">
      <div className="flex flex-col items-center">
        <div className="w-5 h-5 rounded-full bg-[#d4956a]/20 flex items-center justify-center shrink-0">
          <ArrowRight className="w-3 h-3 text-[#d4956a]" />
        </div>
      </div>
      <div className="flex-1 -mt-0.5">
        <div className="rounded-lg border border-[#d4956a]/50 bg-[#d4956a]/5 p-3">
          <span className="text-[13px] text-muted-foreground">
            No rules matched, serving default variant <span className="font-mono text-[#d4956a]">{variant}</span>
          </span>
        </div>
      </div>
    </div>
  )
}

export default function PlaygroundTrace({ trace }: { trace: EvaluationTrace }) {
  return (
    <div className="flex flex-col">
      {trace.steps.map((step, i) => (
        <TraceStepRow
          key={i}
          step={step}
          index={i}
          isSelected={i === trace.selected_step}
          offVariant={trace.reason === 'disabled' ? trace.variant : undefined}
        />
      ))}
      {trace.reason === 'default' && trace.selected_step === -1 && (
        <FallthroughStep variant={trace.fallthrough_variant} />
      )}
    </div>
  )
}
