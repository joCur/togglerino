import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listFlags, getFlag, createFlag, updateFlag, toggleFlag, updateFlagConfig } from '../../src/tools/flags'

describe('listFlags', () => {
  it('calls GET /projects/{key}/flags without params', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([]) } as unknown as TogglerinoClient
    await listFlags(mockClient, 'my-project')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags')
  })

  it('appends search query param', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([]) } as unknown as TogglerinoClient
    await listFlags(mockClient, 'my-project', { search: 'login' })
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags?search=login')
  })

  it('appends tag query param', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([]) } as unknown as TogglerinoClient
    await listFlags(mockClient, 'my-project', { tag: 'beta' })
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags?tag=beta')
  })

  it('appends both search and tag query params', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue([]) } as unknown as TogglerinoClient
    await listFlags(mockClient, 'my-project', { search: 'login', tag: 'beta' })
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags?search=login&tag=beta')
  })
})

describe('getFlag', () => {
  it('calls GET /projects/{key}/flags/{flagKey}', async () => {
    const mockClient = { get: vi.fn().mockResolvedValue({ key: 'my-flag' }) } as unknown as TogglerinoClient
    const result = await getFlag(mockClient, 'my-project', 'my-flag')
    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags/my-flag')
    expect(result).toEqual({ key: 'my-flag' })
  })
})

describe('createFlag', () => {
  it('POSTs to /projects/{key}/flags', async () => {
    const mockClient = { post: vi.fn().mockResolvedValue({ key: 'new-flag' }) } as unknown as TogglerinoClient
    const params = { name: 'New Flag', key: 'new-flag', type: 'release' }
    const result = await createFlag(mockClient, 'my-project', params)
    expect(mockClient.post).toHaveBeenCalledWith('/projects/my-project/flags', params)
    expect(result).toEqual({ key: 'new-flag' })
  })
})

describe('updateFlag', () => {
  it('PUTs to /projects/{key}/flags/{flagKey}', async () => {
    const mockClient = { put: vi.fn().mockResolvedValue({ key: 'my-flag', name: 'Updated' }) } as unknown as TogglerinoClient
    const params = { name: 'Updated' }
    const result = await updateFlag(mockClient, 'my-project', 'my-flag', params)
    expect(mockClient.put).toHaveBeenCalledWith('/projects/my-project/flags/my-flag', params)
    expect(result).toEqual({ key: 'my-flag', name: 'Updated' })
  })
})

describe('toggleFlag', () => {
  it('does GET then PUT with enabled merged into existing config', async () => {
    const existingConfig = { rollout_percentage: 50, targeting_rules: [] }
    const mockFlag = {
      environments: {
        production: existingConfig,
      },
    }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ ...existingConfig, enabled: true }),
    } as unknown as TogglerinoClient

    await toggleFlag(mockClient, 'my-project', 'my-flag', 'production', true)

    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags/my-flag')
    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/production',
      { ...existingConfig, enabled: true },
    )
  })

  it('uses empty object when environment config is missing', async () => {
    const mockFlag = { environments: {} }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ enabled: false }),
    } as unknown as TogglerinoClient

    await toggleFlag(mockClient, 'my-project', 'my-flag', 'staging', false)

    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/staging',
      { enabled: false },
    )
  })
})

describe('updateFlagConfig', () => {
  it('does GET then PUT with updates merged into existing config', async () => {
    const existingConfig = { enabled: true, rollout_percentage: 50 }
    const mockFlag = {
      environments: {
        production: existingConfig,
      },
    }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ ...existingConfig, rollout_percentage: 75 }),
    } as unknown as TogglerinoClient

    await updateFlagConfig(mockClient, 'my-project', 'my-flag', 'production', { rollout_percentage: 75 })

    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags/my-flag')
    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/production',
      { enabled: true, rollout_percentage: 75 },
    )
  })

  it('uses empty object when environment config is missing', async () => {
    const mockFlag = { environments: {} }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ targeting_rules: [] }),
    } as unknown as TogglerinoClient

    await updateFlagConfig(mockClient, 'my-project', 'my-flag', 'dev', { targeting_rules: [] })

    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/dev',
      { targeting_rules: [] },
    )
  })
})
