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

export interface Flag {
  id: string
  project_id: string
  key: string
  name: string
  description: string
  flag_type: 'boolean' | 'string' | 'number' | 'json'
  default_value: unknown
  tags: string[]
  archived: boolean
  created_at: string
  updated_at: string
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
