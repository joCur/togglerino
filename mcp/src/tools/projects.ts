import { TogglerinoClient } from '../client.js'

export async function listProjects(client: TogglerinoClient): Promise<unknown[]> {
  return client.get<unknown[]>('/projects')
}
