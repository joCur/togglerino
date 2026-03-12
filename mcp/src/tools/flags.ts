import { TogglerinoClient } from '../client.js'

export async function listFlags(
  client: TogglerinoClient,
  projectKey: string,
  params?: { search?: string; tag?: string },
): Promise<unknown> {
  const qs = new URLSearchParams()
  if (params?.search) qs.set('search', params.search)
  if (params?.tag) qs.set('tag', params.tag)
  const query = qs.toString()
  return client.get(`/projects/${projectKey}/flags${query ? `?${query}` : ''}`)
}

export async function getFlag(client: TogglerinoClient, projectKey: string, flagKey: string): Promise<unknown> {
  return client.get(`/projects/${projectKey}/flags/${flagKey}`)
}

export async function createFlag(client: TogglerinoClient, projectKey: string, params: Record<string, unknown>): Promise<unknown> {
  return client.post(`/projects/${projectKey}/flags`, params)
}

export async function updateFlag(
  client: TogglerinoClient, projectKey: string, flagKey: string, params: Record<string, unknown>,
): Promise<unknown> {
  return client.put(`/projects/${projectKey}/flags/${flagKey}`, params)
}

export async function toggleFlag(
  client: TogglerinoClient, projectKey: string, flagKey: string, environmentKey: string, enabled: boolean,
): Promise<unknown> {
  const flag = (await client.get(`/projects/${projectKey}/flags/${flagKey}`)) as {
    environments: Record<string, Record<string, unknown>>
  }
  const envConfig = flag.environments?.[environmentKey] || {}
  return client.put(`/projects/${projectKey}/flags/${flagKey}/environments/${environmentKey}`, {
    ...envConfig,
    enabled,
  })
}

export async function updateFlagConfig(
  client: TogglerinoClient, projectKey: string, flagKey: string, environmentKey: string, updates: Record<string, unknown>,
): Promise<unknown> {
  const flag = (await client.get(`/projects/${projectKey}/flags/${flagKey}`)) as {
    environments: Record<string, Record<string, unknown>>
  }
  const envConfig = flag.environments?.[environmentKey] || {}
  return client.put(`/projects/${projectKey}/flags/${flagKey}/environments/${environmentKey}`, {
    ...envConfig,
    ...updates,
  })
}
