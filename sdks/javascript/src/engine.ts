/**
 * Local flag evaluation engine for server-side SDKs.
 * Ports the Go backend's evaluation logic (internal/evaluation/engine.go).
 */

import type { EvaluationContext, EvaluationResult } from './types'
import type {
  FlagDefinition,
  SegmentDefinition,
  ConditionDefinition,
} from './server-types'
import { consistentHash } from './hash'
import { evaluateCondition } from './operators'

/**
 * Evaluates a flag for a given context.
 *
 * @param flag - The flag definition (from the definitions API).
 * @param context - The evaluation context (userId + attributes).
 * @param segments - Array of segment definitions for segment_match resolution.
 * @returns The evaluation result with value, variant, and reason.
 */
export function evaluate(
  flag: FlagDefinition,
  context: EvaluationContext | undefined,
  segments: SegmentDefinition[] | undefined,
): EvaluationResult {
  const ctx = context ?? {}
  const segMap = buildSegmentMap(segments)

  const config = flag.config

  // 1. If flag is archived, return fallthrough variant value with reason "archived".
  if (flag.status === 'archived') {
    return {
      value: lookupVariantValue(flag.variants, config.fallthroughVariant, flag.defaultValue),
      variant: '',
      reason: 'archived',
    }
  }

  // 2. If config is disabled, return off variant value with reason "disabled".
  if (!config.enabled) {
    return {
      value: lookupVariantValue(flag.variants, config.offVariant, flag.defaultValue),
      variant: config.offVariant,
      reason: 'disabled',
    }
  }

  // 3. Evaluate targeting rules in order (first match wins).
  for (const rule of config.targetingRules) {
    if (matchesAllConditions(rule.conditions, ctx, segMap)) {
      // Check percentage rollout.
      if (rule.percentage != null) {
        const bucket = consistentHash(flag.key, ctx.userId ?? '')
        if (bucket >= rule.percentage) {
          // User is outside the rollout percentage; continue to next rule.
          continue
        }
      }
      // Rule matched — find the variant value.
      const value = lookupVariantValue(flag.variants, rule.variant, flag.defaultValue)
      return { value, variant: rule.variant, reason: 'rule_match' }
    }
  }

  // 4. Return fallthrough variant.
  const value = lookupVariantValue(flag.variants, config.fallthroughVariant, flag.defaultValue)
  return { value, variant: config.fallthroughVariant, reason: 'default' }
}

/**
 * Checks if all conditions in a rule match the evaluation context.
 * segment_match conditions look up the segment by key and evaluate its conditions.
 * Passing null for segments in the recursive call prevents nesting.
 */
function matchesAllConditions(
  conditions: ConditionDefinition[],
  ctx: EvaluationContext,
  segments: Map<string, SegmentDefinition> | null,
): boolean {
  for (const cond of conditions) {
    if (cond.operator === 'segment_match') {
      const segKey = cond.value
      if (typeof segKey !== 'string') return false
      if (segments == null) return false
      const seg = segments.get(segKey)
      if (!seg) return false
      // Evaluate segment conditions (pass null for segments to prevent nesting).
      if (!matchesAllConditions(seg.conditions, ctx, null)) return false
      continue
    }

    const attrValue = getContextValue(ctx, cond.attribute)
    const condValue = parseConditionValue(cond.operator, cond.value)
    if (!evaluateCondition(attrValue, cond.operator, condValue)) {
      return false
    }
  }
  return true
}

/**
 * Gets a context value for the given attribute.
 * Maps "user_id" to context.userId; other attributes come from context.attributes.
 */
function getContextValue(ctx: EvaluationContext, attribute: string): unknown {
  if (attribute === 'user_id') {
    return ctx.userId ?? undefined
  }
  return ctx.attributes?.[attribute] ?? undefined
}

/**
 * Parses a condition value for operators that require special handling.
 * For in/not_in operators, the value is a JSON-encoded array string.
 */
function parseConditionValue(operator: string, value: string): unknown {
  if (operator === 'in' || operator === 'not_in') {
    try {
      const parsed = JSON.parse(value)
      if (Array.isArray(parsed)) return parsed
    } catch {
      // Fall through to return raw value.
    }
  }
  return value
}

/**
 * Finds the value for a variant name in the variants list.
 * If the variant is not found, returns the defaultValue.
 */
function lookupVariantValue(
  variants: { name: string; value: unknown }[],
  variantName: string,
  defaultValue: unknown,
): unknown {
  for (const v of variants) {
    if (v.name === variantName) return v.value
  }
  return defaultValue
}

/**
 * Builds a Map<string, SegmentDefinition> from an array for efficient lookup.
 */
function buildSegmentMap(
  segments: SegmentDefinition[] | undefined,
): Map<string, SegmentDefinition> {
  const map = new Map<string, SegmentDefinition>()
  if (segments) {
    for (const s of segments) {
      map.set(s.key, s)
    }
  }
  return map
}
