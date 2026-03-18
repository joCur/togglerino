import { useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { compareFlag } from '@/lib/flag-diff'
import type { FieldDiff, VariantDiff } from '@/lib/flag-diff'
import type { Environment, FlagEnvironmentConfig, TargetingRule } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { ChevronRight } from 'lucide-react'
import { formatRelativeTime } from '@/lib/date'

interface CompareTabProps {
  environments: Environment[]
  environmentConfigs: FlagEnvironmentConfig[]
}

function DiffBadge({ children, differs }: { children: React.ReactNode; differs: boolean }) {
  if (!differs) {
    return <span className="text-[13px] text-foreground">{children}</span>
  }
  return (
    <Badge variant="outline" className="bg-amber-950/50 text-amber-400 border-amber-800 font-mono text-xs">
      {children}
    </Badge>
  )
}

function EnabledCell({ value, differs }: { value: boolean; differs: boolean }) {
  if (differs) {
    return (
      <Badge
        variant="outline"
        className={cn(
          'font-mono text-xs',
          value
            ? 'bg-green-950/50 text-green-400 border-green-800'
            : 'bg-red-950/50 text-red-400 border-red-800'
        )}
      >
        {value ? 'ON' : 'OFF'}
      </Badge>
    )
  }
  return (
    <span className={cn('text-[13px]', value ? 'text-green-400' : 'text-red-400')}>
      {value ? 'ON' : 'OFF'}
    </span>
  )
}

function NotConfigured() {
  return <span className="text-muted-foreground/50 text-xs italic">Not configured</span>
}

function RuleCard({ rule, index }: { rule: TargetingRule; index: number }) {
  return (
    <div className="rounded border border-border/50 bg-card/50 p-2 border-l-2 border-l-amber-600/60">
      <div className="text-[10px] text-amber-600/80 font-mono mb-1">Rule {index + 1}</div>
      {rule.conditions.map((c, i) => (
        <div key={i} className="text-xs text-foreground">
          <span className="text-foreground">{c.attribute}</span>{' '}
          <span className="text-muted-foreground">{c.operator}</span>{' '}
          <span className="text-blue-400">{typeof c.value === 'string' ? c.value : JSON.stringify(c.value)}</span>
        </div>
      ))}
      <div className="text-[10px] text-muted-foreground mt-1">
        → {rule.variant}
        {rule.percentage_rollout != null && ` (${rule.percentage_rollout}%)`}
      </div>
    </div>
  )
}

export default function CompareTab({ environments, environmentConfigs }: CompareTabProps) {
  const [showDiffsOnly, setShowDiffsOnly] = useState(false)
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set())

  const sortedEnvs = useMemo(() => [...environments].sort((a, b) => a.sort_order - b.sort_order), [environments])
  const envIds = useMemo(() => sortedEnvs.map((e) => e.id), [sortedEnvs])
  const comparison = useMemo(() => compareFlag(environmentConfigs, envIds), [environmentConfigs, envIds])

  const toggleRow = (row: string) => {
    setExpandedRows((prev) => {
      const next = new Set(prev)
      if (next.has(row)) next.delete(row)
      else next.add(row)
      return next
    })
  }

  const getConfig = (envId: string) =>
    environmentConfigs.find((c) => c.environment_id === envId) ?? null

  const allMatch =
    comparison.enabled.status === 'match' &&
    comparison.fallthroughVariant.status === 'match' &&
    comparison.variants.status === 'match' &&
    comparison.rules.status === 'match'

  const gridCols = `160px repeat(${sortedEnvs.length}, minmax(0, 1fr))`

  function shouldShow(field: FieldDiff | VariantDiff) {
    return !showDiffsOnly || field.status === 'differs'
  }

  return (
    <div className="space-y-4">
      {/* Differences toggle */}
      <div className="flex items-center gap-2">
        <Switch
          id="diff-toggle"
          checked={showDiffsOnly}
          onCheckedChange={setShowDiffsOnly}
        />
        <Label htmlFor="diff-toggle" className="text-xs text-muted-foreground cursor-pointer">
          Show differences only
        </Label>
      </div>

      {showDiffsOnly && allMatch ? (
        <div className="py-8 text-center text-muted-foreground/60 text-[13px]">
          All environments have identical configuration.
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-x-auto">
          <div className="min-w-[500px]">
            {/* Header row */}
            <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
              <div className="bg-card p-3 text-xs font-mono text-muted-foreground uppercase tracking-wider" />
              {sortedEnvs.map((env) => (
                <div key={env.id} className="bg-card p-3 text-xs font-medium text-foreground">
                  {env.name}
                </div>
              ))}
            </div>

            {/* Enabled row */}
            {shouldShow(comparison.enabled) && (
              <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                <div className="bg-background p-3 text-xs text-muted-foreground">Enabled</div>
                {sortedEnvs.map((env) => {
                  const config = getConfig(env.id)
                  if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                  return (
                    <div key={env.id} className="bg-background p-3">
                      <EnabledCell
                        value={comparison.enabled.values.get(env.id) as boolean}
                        differs={comparison.enabled.status === 'differs'}
                      />
                    </div>
                  )
                })}
              </div>
            )}

            {/* Default variant row */}
            {shouldShow(comparison.fallthroughVariant) && (
              <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                <div className="bg-background p-3 text-xs text-muted-foreground">Fallthrough variant</div>
                {sortedEnvs.map((env) => {
                  const config = getConfig(env.id)
                  if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                  const value = comparison.fallthroughVariant.values.get(env.id) as string
                  return (
                    <div key={env.id} className="bg-background p-3">
                      <DiffBadge differs={comparison.fallthroughVariant.status === 'differs'}>
                        {value || '—'}
                      </DiffBadge>
                    </div>
                  )
                })}
              </div>
            )}

            {/* Variants row (expandable) */}
            {shouldShow(comparison.variants) && (
              <Collapsible open={expandedRows.has('variants')} onOpenChange={() => toggleRow('variants')}>
                <CollapsibleTrigger asChild>
                  <div className="grid gap-px bg-border cursor-pointer hover:bg-muted/20" style={{ gridTemplateColumns: gridCols }}>
                    <div className="bg-background p-3 text-xs text-muted-foreground flex items-center gap-1">
                      <ChevronRight className={cn('h-3 w-3 transition-transform', expandedRows.has('variants') && 'rotate-90')} />
                      Variants
                    </div>
                    {sortedEnvs.map((env) => {
                      const config = getConfig(env.id)
                      if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                      const count = config.variants.length
                      return (
                        <div key={env.id} className="bg-background p-3">
                          <DiffBadge differs={comparison.variants.status === 'differs'}>
                            {count} variant{count !== 1 ? 's' : ''}
                          </DiffBadge>
                        </div>
                      )
                    })}
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="border-t border-amber-800/30">
                    {[...comparison.variants.perVariant.entries()].map(([variantKey, diff]) => (
                      <div key={variantKey} className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                        <div className="bg-muted/30 p-3 pl-8 text-xs text-muted-foreground font-mono">{variantKey}</div>
                        {sortedEnvs.map((env) => {
                          const value = diff.values.get(env.id)
                          return (
                            <div key={env.id} className="bg-muted/30 p-3">
                              {value != null ? (
                                <DiffBadge differs={diff.status === 'differs'}>
                                  <span className="font-mono text-xs">{typeof value === 'string' ? value : JSON.stringify(value)}</span>
                                </DiffBadge>
                              ) : (
                                <span className="text-muted-foreground/40 text-xs">—</span>
                              )}
                            </div>
                          )
                        })}
                      </div>
                    ))}
                  </div>
                </CollapsibleContent>
              </Collapsible>
            )}

            {/* Rules row (expandable) */}
            {shouldShow(comparison.rules) && (
              <Collapsible open={expandedRows.has('rules')} onOpenChange={() => toggleRow('rules')}>
                <CollapsibleTrigger asChild>
                  <div className="grid gap-px bg-border cursor-pointer hover:bg-muted/20" style={{ gridTemplateColumns: gridCols }}>
                    <div className="bg-background p-3 text-xs text-muted-foreground flex items-center gap-1">
                      <ChevronRight className={cn('h-3 w-3 transition-transform', expandedRows.has('rules') && 'rotate-90')} />
                      Targeting rules
                    </div>
                    {sortedEnvs.map((env) => {
                      const config = getConfig(env.id)
                      if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                      const count = config.targeting_rules.length
                      return (
                        <div key={env.id} className="bg-background p-3">
                          <DiffBadge differs={comparison.rules.status === 'differs'}>
                            {count} rule{count !== 1 ? 's' : ''}
                          </DiffBadge>
                        </div>
                      )
                    })}
                  </div>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className="border-t border-amber-800/30">
                    <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
                      <div className="bg-muted/30 p-3 text-[10px] text-muted-foreground uppercase">Details</div>
                      {sortedEnvs.map((env) => {
                        const config = getConfig(env.id)
                        const rules = config?.targeting_rules ?? []
                        return (
                          <div key={env.id} className="bg-muted/30 p-3 space-y-2">
                            {rules.length === 0 ? (
                              <span className="text-muted-foreground/40 text-xs italic">No targeting rules</span>
                            ) : (
                              rules.map((rule, i) => <RuleCard key={i} rule={rule} index={i} />)
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                </CollapsibleContent>
              </Collapsible>
            )}

            {/* Lock status row (informational, no diff) */}
            <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
              <div className="bg-background p-3 text-xs text-muted-foreground">Lock status</div>
              {sortedEnvs.map((env) => {
                const config = getConfig(env.id)
                if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                return (
                  <div key={env.id} className="bg-background p-3 text-xs">
                    {config.locked ? (
                      <span className="text-amber-400">Locked{config.lock_reason ? ` — ${config.lock_reason}` : ''}</span>
                    ) : (
                      <span className="text-muted-foreground">Unlocked</span>
                    )}
                  </div>
                )
              })}
            </div>

            {/* Last updated row (informational, no diff) */}
            <div className="grid gap-px bg-border" style={{ gridTemplateColumns: gridCols }}>
              <div className="bg-background p-3 text-xs text-muted-foreground">Last updated</div>
              {sortedEnvs.map((env) => {
                const config = getConfig(env.id)
                if (!config) return <div key={env.id} className="bg-background p-3"><NotConfigured /></div>
                return (
                  <div key={env.id} className="bg-background p-3 text-xs text-muted-foreground">
                    {config.updated_by_user
                      ? `${formatRelativeTime(config.updated_at)} by ${config.updated_by_user.display_name ?? config.updated_by_user.email}`
                      : '—'}
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
