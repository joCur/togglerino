import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { getAuditLog } from '../../src/tools/audit'

describe('getAuditLog', () => {
  it('calls GET /projects/{key}/audit-log without params', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ data: [], total: 0, limit: 50, offset: 0 }),
    } as unknown as TogglerinoClient
    const result = await getAuditLog(mockClient, 'my-project')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/audit-log')
    expect(result).toEqual({ data: [], total: 0, limit: 50, offset: 0 })
  })

  it('appends limit and offset query params', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ data: [], total: 0, limit: 10, offset: 20 }),
    } as unknown as TogglerinoClient
    await getAuditLog(mockClient, 'my-project', { limit: 10, offset: 20 })
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/audit-log?limit=10&offset=20')
  })

  it('appends only limit when offset not provided', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ data: [], total: 0, limit: 10, offset: 0 }),
    } as unknown as TogglerinoClient
    await getAuditLog(mockClient, 'my-project', { limit: 10 })
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/audit-log?limit=10')
  })
})
