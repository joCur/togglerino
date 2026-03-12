import { TogglerinoClient } from '../client.js'

export async function listEnvironments(client: TogglerinoClient, projectKey: string): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/environments`)
}
