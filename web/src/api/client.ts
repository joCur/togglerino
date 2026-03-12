import type { Condition, Segment, Flag, FlagEnvironmentConfig, BulkActionRequest, BulkActionResponse, FlagTemplate, TemplatesForProject, PlaygroundRequest, PlaygroundResponse, LifecycleSummary, LifecycleSnapshot, PaginatedResponse, AuditEntry, UnknownFlag, Project, User, AppIdentity, FlagOverrideEntry, EnvironmentAccessResponse, EnvironmentAccessRestriction, Webhook, WebhookDelivery, WebhookTestResult, PersonalAccessToken, PersonalAccessTokenWithValue } from './types'

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

const API_BASE = '/api/v1'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
    ...options,
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }))
    throw new ApiError(res.status, error.error || res.statusText)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json()
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),

  flags: {
    list: (projectKey: string, params?: { search?: string; tag?: string; lifecycle_status?: string; flag_type?: string; include?: string; limit?: number; offset?: number; unevaluated_days?: string }) => {
      const search = new URLSearchParams()
      if (params?.search) search.set('search', params.search)
      if (params?.tag) search.set('tag', params.tag)
      if (params?.lifecycle_status) search.set('lifecycle_status', params.lifecycle_status)
      if (params?.flag_type) search.set('flag_type', params.flag_type)
      if (params?.include) search.set('include', params.include)
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      if (params?.unevaluated_days) search.set('unevaluated_days', params.unevaluated_days)
      const qs = search.toString()
      return request<PaginatedResponse<Flag>>(`/projects/${projectKey}/flags${qs ? `?${qs}` : ''}`)
    },
    bulk: (projectKey: string, body: BulkActionRequest) =>
      request<BulkActionResponse>(`/projects/${projectKey}/flags/bulk`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
    lock: (projectKey: string, flagKey: string, envKey: string, reason?: string) =>
      request<FlagEnvironmentConfig>(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/lock`, {
        method: 'POST',
        body: JSON.stringify({ reason }),
      }),
    unlock: (projectKey: string, flagKey: string, envKey: string) =>
      request<FlagEnvironmentConfig>(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/lock`, {
        method: 'DELETE',
      }),
    bulkLock: (projectKey: string, flagKeys: string[], environmentKey: string, reason?: string) =>
      request<{ locked: number; already_locked: number; errors: string[] }>(`/projects/${projectKey}/flags/bulk-lock`, {
        method: 'POST',
        body: JSON.stringify({ flag_keys: flagKeys, environment_key: environmentKey, reason }),
      }),
    bulkUnlock: (projectKey: string, flagKeys: string[], environmentKey: string) =>
      request<{ unlocked: number; already_unlocked: number; errors: string[] }>(`/projects/${projectKey}/flags/bulk-unlock`, {
        method: 'POST',
        body: JSON.stringify({ flag_keys: flagKeys, environment_key: environmentKey }),
      }),
  },

  environments: {
    reorder: (projectKey: string, environmentIds: string[]) =>
      request<void>(`/projects/${projectKey}/environments/order`, {
        method: 'PUT',
        body: JSON.stringify({ environment_ids: environmentIds }),
      }),
    promote: (projectKey: string, flagKey: string, sourceEnvKey: string, targetEnvKey: string) =>
      request<FlagEnvironmentConfig>(`/projects/${projectKey}/flags/${flagKey}/environments/${targetEnvKey}/promote`, {
        method: 'POST',
        body: JSON.stringify({ source_environment_key: sourceEnvKey }),
      }),
    delete: (projectKey: string, envKey: string) =>
      request<void>(`/projects/${projectKey}/environments/${envKey}`, { method: 'DELETE' }),
  },

  environmentAccess: {
    get: (projectKey: string) =>
      request<EnvironmentAccessResponse>(`/projects/${projectKey}/environment-access`),
    update: (projectKey: string, restrictions: EnvironmentAccessRestriction[]) =>
      request<{ status: string }>(`/projects/${projectKey}/environment-access`, {
        method: 'PUT',
        body: JSON.stringify({ restrictions }),
      }),
  },

  segments: {
    list: (projectKey: string, params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<Segment>>(`/projects/${projectKey}/segments${qs ? `?${qs}` : ''}`)
    },
    get: (projectKey: string, segmentKey: string) =>
      request<Segment>(`/projects/${projectKey}/segments/${segmentKey}`),
    create: (
      projectKey: string,
      body: { key: string; name: string; description: string; conditions: Condition[] },
    ) => request<Segment>(`/projects/${projectKey}/segments`, { method: 'POST', body: JSON.stringify(body) }),
    update: (
      projectKey: string,
      segmentKey: string,
      body: { name: string; description: string; conditions: Condition[] },
    ) =>
      request<Segment>(`/projects/${projectKey}/segments/${segmentKey}`, {
        method: 'PUT',
        body: JSON.stringify(body),
      }),
    delete: (projectKey: string, segmentKey: string) =>
      request<void>(`/projects/${projectKey}/segments/${segmentKey}`, { method: 'DELETE' }),
    usage: (projectKey: string, segmentKey: string) =>
      request<{ referencing_flags: string[] }>(
        `/projects/${projectKey}/segments/${segmentKey}/usage`,
      ),
  },

  playground: {
    evaluate: (projectKey: string, body: PlaygroundRequest) =>
      request<PlaygroundResponse>(`/projects/${projectKey}/playground`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
  },

  lifecycle: {
    summary: (projectKey: string) =>
      request<LifecycleSummary>(`/projects/${projectKey}/lifecycle/summary`),
    trends: (projectKey: string, days = 30) =>
      request<LifecycleSnapshot[]>(`/projects/${projectKey}/lifecycle/trends?days=${days}`),
  },

  projects: {
    list: (params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<Project>>(`/projects${qs ? `?${qs}` : ''}`)
    },
  },

  users: {
    list: (params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<User>>(`/management/users${qs ? `?${qs}` : ''}`)
    },
  },

  unknownFlags: {
    list: (projectKey: string, params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<UnknownFlag>>(`/projects/${projectKey}/unknown-flags${qs ? `?${qs}` : ''}`)
    },
  },

  auditLog: {
    list: (projectKey: string, params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<AuditEntry>>(`/projects/${projectKey}/audit-log${qs ? `?${qs}` : ''}`)
    },
  },

  appIdentity: {
    get: (projectKey: string) =>
      request<AppIdentity>(`/projects/${projectKey}/app-identity`),
    set: (projectKey: string, appUserID: string) =>
      request<AppIdentity>(`/projects/${projectKey}/app-identity`, {
        method: 'PUT',
        body: JSON.stringify({ app_user_id: appUserID }),
      }),
    delete: (projectKey: string) =>
      request<void>(`/projects/${projectKey}/app-identity`, { method: 'DELETE' }),
    listMine: () => request<AppIdentity[]>('/app-identities/me'),
  },

  overrides: {
    listMine: () => request<FlagOverrideEntry[]>('/overrides/me'),
    getForFlag: (projectKey: string, flagKey: string) =>
      request<FlagOverrideEntry[]>(`/projects/${projectKey}/flags/${flagKey}/overrides/me`),
    set: (projectKey: string, flagKey: string, envKey: string, value: unknown, duration?: string | null) =>
      request<FlagOverrideEntry>(
        `/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/override`,
        { method: 'PUT', body: JSON.stringify({ value, duration }) },
      ),
    delete: (projectKey: string, flagKey: string, envKey: string) =>
      request<void>(
        `/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/override`,
        { method: 'DELETE' },
      ),
    setAll: (projectKey: string, flagKey: string, value: unknown, duration?: string | null) =>
      request<void>(`/projects/${projectKey}/flags/${flagKey}/override`, {
        method: 'PUT',
        body: JSON.stringify({ value, duration }),
      }),
    deleteAll: (projectKey: string, flagKey: string) =>
      request<void>(`/projects/${projectKey}/flags/${flagKey}/override`, { method: 'DELETE' }),
  },

  templates: {
    listGlobal: () => request<FlagTemplate[]>('/templates'),
    createGlobal: (body: Partial<FlagTemplate>) =>
      request<FlagTemplate>('/templates', { method: 'POST', body: JSON.stringify(body) }),
    updateGlobal: (key: string, body: Partial<FlagTemplate>) =>
      request<FlagTemplate>(`/templates/${key}`, { method: 'PUT', body: JSON.stringify(body) }),
    deleteGlobal: (key: string) =>
      request<void>(`/templates/${key}`, { method: 'DELETE' }),
    listForProject: (projectKey: string) =>
      request<TemplatesForProject>(`/projects/${projectKey}/templates`),
    createForProject: (projectKey: string, body: Partial<FlagTemplate>) =>
      request<FlagTemplate>(`/projects/${projectKey}/templates`, { method: 'POST', body: JSON.stringify(body) }),
    updateForProject: (projectKey: string, templateKey: string, body: Partial<FlagTemplate>) =>
      request<FlagTemplate>(`/projects/${projectKey}/templates/${templateKey}`, { method: 'PUT', body: JSON.stringify(body) }),
    deleteForProject: (projectKey: string, templateKey: string) =>
      request<void>(`/projects/${projectKey}/templates/${templateKey}`, { method: 'DELETE' }),
  },

  tokens: {
    list: () => request<PersonalAccessToken[]>('/auth/tokens'),
    create: (data: { name: string; expires_at?: string }) =>
      request<PersonalAccessTokenWithValue>('/auth/tokens', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    delete: (id: string) =>
      request<void>(`/auth/tokens/${id}`, { method: 'DELETE' }),
  },

  webhooks: {
    list: (projectKey: string, params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<Webhook>>(`/projects/${projectKey}/webhooks${qs ? `?${qs}` : ''}`)
    },
    get: (projectKey: string, id: string) =>
      request<Webhook>(`/projects/${projectKey}/webhooks/${id}`),
    create: (projectKey: string, body: { name: string; url: string; event_types: string[] }) =>
      request<Webhook>(`/projects/${projectKey}/webhooks`, { method: 'POST', body: JSON.stringify(body) }),
    update: (projectKey: string, id: string, body: { name?: string; url?: string; event_types?: string[]; enabled?: boolean }) =>
      request<Webhook>(`/projects/${projectKey}/webhooks/${id}`, { method: 'PUT', body: JSON.stringify(body) }),
    delete: (projectKey: string, id: string) =>
      request<void>(`/projects/${projectKey}/webhooks/${id}`, { method: 'DELETE' }),
    test: (projectKey: string, id: string) =>
      request<WebhookTestResult>(`/projects/${projectKey}/webhooks/${id}/test`, { method: 'POST' }),
    deliveries: (projectKey: string, id: string, params?: { limit?: number; offset?: number }) => {
      const search = new URLSearchParams()
      if (params?.limit !== undefined) search.set('limit', String(params.limit))
      if (params?.offset !== undefined) search.set('offset', String(params.offset))
      const qs = search.toString()
      return request<PaginatedResponse<WebhookDelivery>>(`/projects/${projectKey}/webhooks/${id}/deliveries${qs ? `?${qs}` : ''}`)
    },
  },
}
