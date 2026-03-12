import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listEnvironments } from '../../src/tools/environments'

describe('listEnvironments', () => {
  it('calls GET /projects/{key}/environments', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([{ key: 'dev' }]) } as unknown as TogglerinoClient
    const result = await listEnvironments(mockClient, 'my-project')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/environments')
    expect(result).toHaveLength(1)
  })
})
