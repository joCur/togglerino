import { TogglerinoClient } from '../client.js'

export async function evaluateFlags(
  client: TogglerinoClient,
  projectKey: string,
  params: {
    environmentKey: string
    flagKey?: string
    userId?: string
    attributes?: Record<string, unknown>
  },
): Promise<unknown> {
  const body: Record<string, unknown> = {
    environment_key: params.environmentKey,
  }
  if (params.flagKey !== undefined) {
    body.flag_key = params.flagKey
  }
  if (params.userId !== undefined || params.attributes !== undefined) {
    const context: Record<string, unknown> = {}
    if (params.userId !== undefined) context.user_id = params.userId
    if (params.attributes !== undefined) context.attributes = params.attributes
    body.context = context
  }
  return client.post(`/projects/${projectKey}/playground`, body)
}
