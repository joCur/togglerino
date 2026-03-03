import type { Condition, Segment, BulkActionRequest, BulkActionResponse } from './types'

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
    bulk: (projectKey: string, body: BulkActionRequest) =>
      request<BulkActionResponse>(`/projects/${projectKey}/flags/bulk`, {
        method: 'POST',
        body: JSON.stringify(body),
      }),
  },

  segments: {
    list: (projectKey: string) => request<Segment[]>(`/projects/${projectKey}/segments`),
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
}
