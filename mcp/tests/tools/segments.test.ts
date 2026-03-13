import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listSegments, getSegment, createSegment, updateSegment } from '../../src/tools/segments'

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

describe('updateSegment', () => {
  it('does GET then PUT with updates merged into existing segment', async () => {
    const existing = {
      key: 'beta-users',
      name: 'Beta Users',
      description: 'Old desc',
      conditions: [{ attribute: 'plan', operator: 'equals', value: 'beta' }],
    }
    const mockClient = {
      get: vi.fn().mockResolvedValue(existing),
      put: vi.fn().mockResolvedValue({ ...existing, name: 'Updated Name' }),
    } as unknown as TogglerinoClient

    await updateSegment(mockClient, 'my-project', 'beta-users', { name: 'Updated Name' })

    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/segments/beta-users')
    expect(mockClient.put).toHaveBeenCalledWith('/projects/my-project/segments/beta-users', {
      name: 'Updated Name',
      description: 'Old desc',
      conditions: [{ attribute: 'plan', operator: 'equals', value: 'beta' }],
    })
  })

  it('merges only provided fields', async () => {
    const existing = {
      key: 'beta-users',
      name: 'Beta Users',
      description: 'Desc',
      conditions: [{ attribute: 'plan', operator: 'equals', value: 'beta' }],
    }
    const newConditions = [{ attribute: 'plan', operator: 'in', value: ['beta', 'alpha'] }]
    const mockClient = {
      get: vi.fn().mockResolvedValue(existing),
      put: vi.fn().mockResolvedValue({ ...existing, conditions: newConditions }),
    } as unknown as TogglerinoClient

    await updateSegment(mockClient, 'my-project', 'beta-users', { conditions: newConditions })

    expect(mockClient.put).toHaveBeenCalledWith('/projects/my-project/segments/beta-users', {
      name: 'Beta Users',
      description: 'Desc',
      conditions: newConditions,
    })
  })
})

describe('createSegment', () => {
  it('POSTs to /projects/{key}/segments', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ key: 'beta-users', name: 'Beta Users', conditions: [] }),
    } as unknown as TogglerinoClient
    const params = {
      key: 'beta-users',
      name: 'Beta Users',
      description: 'Users in beta program',
      conditions: [{ attribute: 'plan', operator: 'equals', value: 'beta' }],
    }
    const result = await createSegment(mockClient, 'my-project', params)
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/segments', params)
    expect(result).toEqual({ key: 'beta-users', name: 'Beta Users', conditions: [] })
  })
})
