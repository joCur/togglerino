import { TogglerinoClient } from '../client.js'

export async function listSegments(client: TogglerinoClient, projectKey: string): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/segments`)
}

export async function getSegment(client: TogglerinoClient, projectKey: string, segmentKey: string): Promise<unknown> {
  return client.get(`/projects/${projectKey}/segments/${segmentKey}`)
}
