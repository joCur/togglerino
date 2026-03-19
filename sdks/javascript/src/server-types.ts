/**
 * Types matching the definitions API response (camelCase JSON).
 * Used by the local evaluation engine for server-side SDKs.
 */

/** A single flag definition as returned by the definitions endpoint. */
export interface FlagDefinition {
  key: string
  valueType: string
  status: string
  defaultValue: unknown
  variants: VariantDefinition[]
  config: FlagDefinitionConfig
}

/** Per-environment configuration for a flag definition. */
export interface FlagDefinitionConfig {
  enabled: boolean
  fallthroughVariant: string
  offVariant: string
  targetingRules: TargetingRuleDefinition[]
}

/** A variant with a name and arbitrary value. */
export interface VariantDefinition {
  name: string
  value: unknown
}

/** A targeting rule with conditions, a variant to serve, and an optional percentage rollout. */
export interface TargetingRuleDefinition {
  variant: string
  percentage: number | null
  conditions: ConditionDefinition[]
}

/** A single condition within a targeting rule. */
export interface ConditionDefinition {
  attribute: string
  operator: string
  value: string
}

/** A reusable segment definition. */
export interface SegmentDefinition {
  key: string
  conditions: ConditionDefinition[]
}

/** Response from the definitions API endpoint. */
export interface DefinitionsResponse {
  flags: FlagDefinition[]
  segments: SegmentDefinition[]
}
