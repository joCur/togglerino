import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listSegments, getSegment } from '../../src/tools/segments'

describe('listSegments', () => {
  it('calls GET /projects/{key}/segments', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([{ key: 'beta-users' }]) } as unknown as TogglerinoClient
    const result = await listSegments(mockClient, 'my-project')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/segments')
    expect(result).toEqual([{ key: 'beta-users' }])
  })

  it('returns empty array when no segments exist', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([]) } as unknown as TogglerinoClient
    const result = await listSegments(mockClient, 'my-project')
    expect(result).toHaveLength(0)
  })
})

describe('getSegment', () => {
  it('calls GET /projects/{key}/segments/{segmentKey}', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue({ key: 'beta-users', conditions: [] }) } as unknown as TogglerinoClient
    const result = await getSegment(mockClient, 'my-project', 'beta-users')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/segments/beta-users')
    expect(result).toEqual({ key: 'beta-users', conditions: [] })
  })
})
