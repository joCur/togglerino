import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listProjects } from '../../src/tools/projects'

describe('listProjects', () => {
  it('calls GET /projects', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([{ key: 'proj-1' }]) } as unknown as TogglerinoClient
    const result = await listProjects(mockClient)
    expect(mockClient.get).toHaveBeenCalledWith('/projects')
    expect(result).toEqual([{ key: 'proj-1' }])
  })
})
