export interface User {
  id: string
  email: string
  role: 'admin' | 'member'
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
  created_at: string
  updated_at: string
}

export interface ProjectFlagSettings {
  flag_lifetimes: Record<FlagPurpose, number | null>
}

export interface Variant {
  key: string
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
  default_variant: string
  variants: Variant[]
  targeting_rules: TargetingRule[]
  updated_at: string
}

export interface AuditEntry {
  id: string
  project_id?: string
  user_id?: string
  action: string
  entity_type: string
  entity_id: string
  old_value?: unknown
  new_value?: unknown
  created_at: string
}
