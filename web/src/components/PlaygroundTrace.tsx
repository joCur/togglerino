import type { EvaluationTrace, TraceStep, ConditionTrace } from '../api/types.ts'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { Check, X, Minus } from 'lucide-react'

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

function TraceStepRow({ step, index, isSelected }: { step: TraceStep; index: number; isSelected: boolean }) {
  const label = step.type === 'lifecycle_check'
    ? 'Lifecycle Check'
    : step.type === 'enabled_check'
      ? 'Enabled Check'
      : `Rule ${(step.rule_index ?? index - 1) + 1}`

  const statusBadge = () => {
    if (step.type === 'lifecycle_check') {
      return (
        <Badge
          variant="secondary"
          className={cn(
            'text-[10px]',
            step.passed
              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
              : 'bg-red-500/10 text-red-400 border-red-500/20',
          )}
        >
          {step.status ?? (step.passed ? 'active' : 'archived')}
        </Badge>
      )
    }
    if (step.type === 'enabled_check') {
      return (
        <Badge
          variant="secondary"
          className={cn(
            'text-[10px]',
            step.enabled
              ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
              : 'bg-muted text-muted-foreground',
          )}
        >
          {step.enabled ? 'Enabled' : 'Disabled'}
        </Badge>
      )
    }
    if (step.skipped) {
      return <Badge variant="secondary" className="text-[10px] bg-muted text-muted-foreground">Skipped</Badge>
    }
    if (step.matched) {
      return <Badge variant="secondary" className="text-[10px] bg-emerald-500/10 text-emerald-400 border-emerald-500/20">Matched</Badge>
    }
    return <Badge variant="secondary" className="text-[10px]">Not matched</Badge>
  }

  return (
    <div className="flex gap-3">
      {/* Vertical line + icon */}
      <div className="flex flex-col items-center">
        <StepIcon passed={step.passed} skipped={step.skipped} />
        <div className="w-px flex-1 bg-border/50" />
      </div>

      {/* Content */}
      <div className={cn(
        'flex-1 pb-4 -mt-0.5',
      )}>
        <div className={cn(
          'rounded-lg border p-3',
          isSelected && 'border-[#d4956a]/50 bg-[#d4956a]/5',
          step.skipped && 'opacity-50',
        )}>
          <div className="flex items-center gap-2 mb-1">
            <span className="text-[13px] font-medium text-foreground">{label}</span>
            {statusBadge()}
            {step.variant && (
              <Badge variant="outline" className="text-[10px] font-mono">{step.variant}</Badge>
            )}
          </div>

          {/* Conditions */}
          {step.conditions && step.conditions.length > 0 && (
            <div className="mt-2 flex flex-col gap-0.5">
              {step.conditions.map((cond, i) => (
                <ConditionRow key={i} condition={cond} />
              ))}
            </div>
          )}

          {/* Rollout */}
          <RolloutBar step={step} />
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
        />
      ))}
    </div>
  )
}
