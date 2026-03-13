import { TogglerinoClient } from '../client.js'

export async function createSdkKey(
  client: TogglerinoClient,
  projectKey: string,
  environmentKey: string,
  name: string,
): Promise<unknown> {
  return client.post(`/projects/${projectKey}/environments/${environmentKey}/sdk-keys`, { name })
}

export async function listSdkKeys(
  client: TogglerinoClient,
  projectKey: string,
  environmentKey: string,
): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/environments/${environmentKey}/sdk-keys`)
}

export async function deleteSdkKey(
  client: TogglerinoClient,
  projectKey: string,
  environmentKey: string,
  sdkKeyId: string,
): Promise<void> {
  await client.del(`/projects/${projectKey}/environments/${environmentKey}/sdk-keys/${sdkKeyId}`)
}
