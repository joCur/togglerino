import type { Variant, TargetingRule } from '@/api/types'
import type { EnvDefault, VariantConfigState } from '@/components/TemplateEditors'

export function envDefaultsToRecord(envDefaults: EnvDefault[]): Record<string, { enabled: boolean }> {
  const result: Record<string, { enabled: boolean }> = {}
  for (const e of envDefaults) {
    result[e.envKey] = { enabled: e.enabled }
  }
  return result
}

export function recordToEnvDefaults(record: Record<string, { enabled: boolean }> | undefined): EnvDefault[] {
  if (!record) return []
  return Object.entries(record).map(([envKey, { enabled }]) => ({ envKey, enabled }))
}

export function variantConfigToState(
  config: { variants?: Variant[]; fallthrough_variant?: string; off_variant?: string; targeting_rules?: TargetingRule[] } | undefined,
): VariantConfigState {
  if (!config || Object.keys(config).length === 0) {
    return { variants: [], defaultVariant: '', targetingRules: [] }
  }
  return {
    variants: config.variants ?? [],
    defaultVariant: config.fallthrough_variant ?? '',
    targetingRules: config.targeting_rules ?? [],
  }
}

export function stateToVariantConfig(
  state: VariantConfigState,
): { variants?: Variant[]; fallthrough_variant?: string; off_variant?: string; targeting_rules?: TargetingRule[] } {
  if (state.variants.length === 0 && state.targetingRules.length === 0) return {}
  return {
    variants: state.variants,
    fallthrough_variant: state.defaultVariant || undefined,
    off_variant: state.defaultVariant || undefined,
    targeting_rules: state.targetingRules.length > 0 ? state.targetingRules : undefined,
  }
}
