import { TogglerinoClient } from '../client.js'

export async function getAuditLog(
  client: TogglerinoClient,
  projectKey: string,
  params?: { limit?: number; offset?: number },
): Promise<unknown> {
  const qs = new URLSearchParams()
  if (params?.limit !== undefined) qs.set('limit', String(params.limit))
  if (params?.offset !== undefined) qs.set('offset', String(params.offset))
  const query = qs.toString()
  return client.get(`/projects/${projectKey}/audit-log${query ? `?${query}` : ''}`)
}
