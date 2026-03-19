import type { FlagEnvironmentConfig } from '@/api/types'

/**
 * Recursively serializes a value with sorted object keys.
 * Ensures {a:1, b:2} and {b:2, a:1} produce identical strings.
 */
export function canonicalize(value: unknown): string {
  if (value === null || value === undefined) {
    return JSON.stringify(value ?? null)
  }
  if (Array.isArray(value)) {
    return '[' + value.map(canonicalize).join(',') + ']'
  }
  if (typeof value === 'object') {
    const obj = value as Record<string, unknown>
    const keys = Object.keys(obj).sort()
    const entries = keys
      .filter((k) => obj[k] !== undefined)
      .map((k) => JSON.stringify(k) + ':' + canonicalize(obj[k]))
    return '{' + entries.join(',') + '}'
  }
  return JSON.stringify(value)
}

export type DiffStatus = 'match' | 'differs'

export type FieldDiff = {
  status: DiffStatus
  values: Map<string, unknown>
}

export type ComparisonResult = {
  enabled: FieldDiff
  fallthroughVariant: FieldDiff
  offVariant: FieldDiff
  rules: FieldDiff
}

function getConfig(configs: FlagEnvironmentConfig[], envId: string): FlagEnvironmentConfig | null {
  return configs.find((c) => c.environment_id === envId) ?? null
}

export function compareEnabled(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    values.set(envId, config?.enabled ?? false)
  }
  const allValues = [...values.values()]
  const status: DiffStatus = allValues.every((v) => v === allValues[0]) ? 'match' : 'differs'
  return { status, values }
}

export function compareFallthroughVariant(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    values.set(envId, config?.fallthrough_variant ?? '')
  }
  const allValues = [...values.values()]
  const status: DiffStatus = allValues.every((v) => v === allValues[0]) ? 'match' : 'differs'
  return { status, values }
}

export function compareOffVariant(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    values.set(envId, config?.off_variant ?? '')
  }
  const allValues = [...values.values()]
  const status: DiffStatus = allValues.every((v) => v === allValues[0]) ? 'match' : 'differs'
  return { status, values }
}

// Rule order is intentionally preserved (first-match-wins); only conditions within each rule are sorted.
function canonicalizeRules(rules: FlagEnvironmentConfig['targeting_rules']): string {
  const normalized = rules.map((rule) => ({
    ...rule,
    conditions: [...rule.conditions].sort((a, b) =>
      a.attribute.localeCompare(b.attribute) ||
      a.operator.localeCompare(b.operator) ||
      canonicalize(a.value).localeCompare(canonicalize(b.value))
    ),
  }))
  return canonicalize(normalized)
}

export function compareRules(configs: FlagEnvironmentConfig[], environmentIds: string[]): FieldDiff {
  const values = new Map<string, unknown>()
  const serialized: string[] = []

  for (const envId of environmentIds) {
    const config = getConfig(configs, envId)
    const rules = config?.targeting_rules ?? []
    values.set(envId, rules)
    serialized.push(canonicalizeRules(rules))
  }

  const status: DiffStatus = serialized.every((s) => s === serialized[0]) ? 'match' : 'differs'
  return { status, values }
}

export function compareFlag(configs: FlagEnvironmentConfig[], environmentIds: string[]): ComparisonResult {
  return {
    enabled: compareEnabled(configs, environmentIds),
    fallthroughVariant: compareFallthroughVariant(configs, environmentIds),
    offVariant: compareOffVariant(configs, environmentIds),
    rules: compareRules(configs, environmentIds),
  }
}
