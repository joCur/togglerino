import { TogglerinoClient } from '../client.js'

export async function listSegments(client: TogglerinoClient, projectKey: string): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/segments`)
}

export async function getSegment(client: TogglerinoClient, projectKey: string, segmentKey: string): Promise<unknown> {
  return client.get(`/projects/${projectKey}/segments/${segmentKey}`)
}

export async function updateSegment(
  client: TogglerinoClient,
  projectKey: string,
  segmentKey: string,
  updates: { name?: string; description?: string; conditions?: unknown[] },
): Promise<unknown> {
  const existing = (await client.get(`/projects/${projectKey}/segments/${segmentKey}`)) as {
    name: string
    description?: string
    conditions: unknown[]
  }
  const merged = {
    name: updates.name !== undefined ? updates.name : existing.name,
    description: updates.description !== undefined ? updates.description : existing.description,
    conditions: updates.conditions !== undefined ? updates.conditions : existing.conditions,
  }
  return client.put(`/projects/${projectKey}/segments/${segmentKey}`, merged)
}

export async function createSegment(
  client: TogglerinoClient,
  projectKey: string,
  params: { key: string; name: string; description?: string; conditions: unknown[] },
): Promise<unknown> {
  return client.post(`/projects/${projectKey}/segments`, params)
}
