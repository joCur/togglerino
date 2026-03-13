import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { createSdkKey, listSdkKeys, deleteSdkKey } from '../../src/tools/sdk-keys'

describe('createSdkKey', () => {
  it('POSTs to /projects/{key}/environments/{env}/sdk-keys', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ id: 'key-1', key: 'sdk_abc123', name: 'My Key' }),
    } as unknown as TogglerinoClient
    const result = await createSdkKey(mockClient, 'my-project', 'production', 'My Key')
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/environments/production/sdk-keys', { name: 'My Key' })
    expect(result).toEqual({ id: 'key-1', key: 'sdk_abc123', name: 'My Key' })
  })
})

describe('listSdkKeys', () => {
  it('calls GET /projects/{key}/environments/{env}/sdk-keys', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue([{ id: 'key-1', name: 'My Key' }]),
    } as unknown as TogglerinoClient
    const result = await listSdkKeys(mockClient, 'my-project', 'production')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/environments/production/sdk-keys')
    expect(result).toEqual([{ id: 'key-1', name: 'My Key' }])
  })
})

describe('deleteSdkKey', () => {
  it('calls DELETE /projects/{key}/environments/{env}/sdk-keys/{id}', async () => {
    const mockClient = { del: vi.fn().mockResolvedValue(undefined) } as unknown as TogglerinoClient
    await deleteSdkKey(mockClient, 'my-project', 'production', 'key-1')
    expect(mockClient.del).toHaveBeenCalledWith('/projects/my-project/environments/production/sdk-keys/key-1')
  })
})
