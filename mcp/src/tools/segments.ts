import { TogglerinoClient } from '../client.js'

export async function listSegments(client: TogglerinoClient, projectKey: string): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/segments`)
}

export async function getSegment(client: TogglerinoClient, projectKey: string, segmentKey: string): Promise<unknown> {
  return client.get(`/projects/${projectKey}/segments/${segmentKey}`)
}

export async function createSegment(
  client: TogglerinoClient,
  projectKey: string,
  params: { key: string; name: string; description?: string; conditions: unknown[] },
): Promise<unknown> {
  return client.post(`/projects/${projectKey}/segments`, params)
}
