import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { evaluateFlags } from '../../src/tools/playground'

describe('evaluateFlags', () => {
  it('POSTs to /projects/{key}/playground with environment_key only', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ results: [] }),
    } as unknown as TogglerinoClient
    const result = await evaluateFlags(mockClient, 'my-project', { environmentKey: 'production' })
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/playground', {
      environment_key: 'production',
    })
    expect(result).toEqual({ results: [] })
  })

  it('includes flag_key when provided', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ results: [{ flag_key: 'my-flag', value: true }] }),
    } as unknown as TogglerinoClient
    await evaluateFlags(mockClient, 'my-project', { environmentKey: 'production', flagKey: 'my-flag' })
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/playground', {
      environment_key: 'production',
      flag_key: 'my-flag',
    })
  })

  it('includes context with user_id and attributes', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ results: [] }),
    } as unknown as TogglerinoClient
    await evaluateFlags(mockClient, 'my-project', {
      environmentKey: 'production',
      userId: 'user-123',
      attributes: { plan: 'enterprise', country: 'US' },
    })
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/playground', {
      environment_key: 'production',
      context: {
        user_id: 'user-123',
        attributes: { plan: 'enterprise', country: 'US' },
      },
    })
  })

  it('includes context with only user_id', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ results: [] }),
    } as unknown as TogglerinoClient
    await evaluateFlags(mockClient, 'my-project', {
      environmentKey: 'production',
      userId: 'user-123',
    })
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/playground', {
      environment_key: 'production',
      context: {
        user_id: 'user-123',
      },
    })
  })
})
