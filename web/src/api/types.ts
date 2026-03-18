export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export interface User {
  id: string
  email: string
  display_name?: string
  role: 'admin' | 'member'
  permissions: string[]
  created_at: string
  updated_at: string
}

export interface Project {
  id: string
  key: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface Environment {
  id: string
  project_id: string
  key: string
  name: string
  sort_order: number
  created_at: string
}

export interface SDKKey {
  id: string
  key: string
  environment_id: string
  name: string
  revoked: boolean
  created_at: string
}

export type ValueType = 'boolean' | 'string' | 'number' | 'json'
export type FlagPurpose = 'release' | 'experiment' | 'operational' | 'kill-switch' | 'permission'
export type LifecycleStatus = 'active' | 'potentially_stale' | 'stale' | 'archived'

export interface FlagOwner {
  id: string
  email: string
  display_name?: string
}

export interface Flag {
  id: string
  project_id: string
  key: string
  name: string
  description: string
  value_type: ValueType
  flag_type: FlagPurpose
  default_value: unknown
  tags: string[]
  lifecycle_status: LifecycleStatus
  lifecycle_status_changed_at: string | null
  last_evaluated_at: string | null
  created_at: string
  updated_at: string
  owner_id?: string
  owner?: FlagOwner
  environment_configs?: FlagEnvironmentConfig[]
}

export interface ProjectFlagSettings {
  flag_lifetimes: Record<FlagPurpose, number | null>
  unevaluated_stale_after_days?: number | null
}

export interface Variant {
  name: string
  value: unknown
}

export interface Condition {
  attribute: string
  operator: string
  value: unknown
}

export interface TargetingRule {
  conditions: Condition[]
  variant: string
  percentage_rollout?: number
}

export interface FlagEnvironmentConfig {
  id: string
  flag_id: string
  environment_id: string
  enabled: boolean
  fallthrough_variant: string
  off_variant: string
  variants: Variant[]
  targeting_rules: TargetingRule[]
  updated_at: string
  updated_by?: string
  updated_by_user?: FlagOwner
  locked: boolean
  locked_by?: string
  locked_by_user?: FlagOwner
  locked_at?: string
  lock_reason?: string
}

export interface AuditEntry {
  id: string
  project_id?: string
  user_id?: string
  user_email?: string
  environment_id?: string
  batch_id?: string
  action: string
  entity_type: string
  entity_id: string
  old_value?: unknown
  new_value?: unknown
  created_at: string
}

export interface UnknownFlag {
  id: string
  project_id: string
  environment_id: string
  flag_key: string
  request_count: number
  first_seen_at: string
  last_seen_at: string
  environment_key: string
  environment_name: string
}

export interface ContextAttribute {
  id: string
  project_id: string
  name: string
  last_seen_at: string
}

export interface Segment {
  id: string
  project_id: string
  key: string
  name: string
  description: string
  conditions: Condition[]
  created_at: string
  updated_at: string
}

export type ScheduleStatus = 'pending' | 'executed' | 'cancelled' | 'failed'

export interface ScheduledFlagChange {
  id: string
  flag_id: string
  environment_id: string
  scheduled_at: string
  status: ScheduleStatus
  config_snapshot: {
    enabled: boolean
    fallthrough_variant: string
    off_variant: string
    variants: Variant[]
    targeting_rules: TargetingRule[]
  }
  created_by?: string
  created_at: string
  executed_at?: string
  cancelled_at?: string
  cancel_reason?: string
}

export type BulkAction = 'enable' | 'disable' | 'archive' | 'add_tags' | 'remove_tags' | 'set_owner'

export interface BulkActionRequest {
  action: BulkAction
  flag_keys: string[]
  environment_key?: string
  tags?: string[]
  owner_id?: string | null
}

export interface BulkActionResult {
  flag_key: string
  success: boolean
  error?: string
}

export interface BulkActionResponse {
  batch_id: string
  results: BulkActionResult[]
}

export interface OIDCProvider {
  id: string
  name: string
  issuer_url: string
  client_id: string
  scopes: string
  default_role: 'admin' | 'member'
  enabled: boolean
  skip_email_verification: boolean
  created_at: string
  updated_at: string
}

export interface OIDCIdentity {
  id: string
  user_id: string
  provider_id: string
  subject: string
  email?: string
  created_at: string
}

export interface FlagTemplate {
    id: string
    project_id: string | null
    key: string
    name: string
    description: string
    flag_type: FlagPurpose
    value_type: ValueType
    default_value: unknown
    tags: string[]
    environment_defaults: Record<string, { enabled: boolean }>
    variant_config: {
        variants?: Variant[]
        fallthrough_variant?: string
        off_variant?: string
        targeting_rules?: TargetingRule[]
    }
    is_system: boolean
    sort_order: number
    created_at: string
    updated_at: string
}

export interface TemplatesForProject {
    global: FlagTemplate[]
    project: FlagTemplate[]
}

// Playground types
export interface ConditionTrace {
  attribute: string
  operator: string
  condition_value: unknown
  actual_value?: unknown
  passed: boolean
  segment_trace?: ConditionTrace[]
}

export interface TraceStep {
  type: 'lifecycle_check' | 'enabled_check' | 'rule'
  passed: boolean
  status?: string
  enabled?: boolean
  rule_index?: number
  variant?: string
  percentage_rollout?: number
  hash_bucket?: number
  in_rollout?: boolean
  matched?: boolean
  skipped?: boolean
  conditions?: ConditionTrace[]
}

export interface EvaluationTrace {
  flag_key: string
  value: unknown
  variant: string
  reason: 'archived' | 'disabled' | 'rule_match' | 'default'
  steps: TraceStep[]
  fallthrough_variant: string
  selected_step: number
}

export interface PlaygroundRequest {
  environment_key: string
  flag_key?: string
  context?: {
    user_id: string
    attributes: Record<string, unknown>
  }
}

export interface PlaygroundResponse {
  results: EvaluationTrace[]
}

export interface LifecycleSummary {
  active: number
  potentially_stale: number
  stale: number
  archived: number
  health_score: number
}

export interface LifecycleSnapshot {
  date: string
  active: number
  potentially_stale: number
  stale: number
  archived: number
}

export interface AppIdentity {
  user_id: string
  project_id: string
  project_key?: string
  app_user_id: string
  created_at: string
  updated_at: string
}

export interface FlagOverrideEntry {
  id: string
  user_id: string
  flag_id: string
  flag_key?: string
  environment_id: string
  environment_key?: string
  project_key?: string
  value: unknown
  expires_at: string | null
  created_at: string
}

export interface EnvironmentAccessRestriction {
  role_name: string
  environment_ids: string[]
}

export interface EnvironmentSummary {
  id: string
  key: string
  name: string
}

export interface EnvironmentAccessResponse {
  restrictions: EnvironmentAccessRestriction[]
  environments: EnvironmentSummary[]
}

export interface Webhook {
  id: string
  project_id: string
  name: string
  url: string
  secret: string
  event_types: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface WebhookDelivery {
  id: string
  webhook_id: string
  event_type: string
  payload: unknown
  status_code?: number
  response_body?: string
  error?: string
  attempt: number
  success: boolean
  duration_ms?: number
  created_at: string
}

export interface WebhookTestResult {
  success: boolean
  status_code?: number
  error?: string
  duration_ms: number
}

export interface PersonalAccessToken {
  id: string
  name: string
  token_prefix: string
  expires_at: string | null
  last_used_at: string | null
  created_at: string
}

export interface PersonalAccessTokenWithValue extends PersonalAccessToken {
  token: string
}
