import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listFlags, getFlag, createFlag, updateFlag, toggleFlag, updateFlagConfig, deleteFlag, archiveFlag } from '../../src/tools/flags'

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
    const existingConfig = { enabled: true, default_variant: 'control', variants: [{ key: 'control', value: false }], targeting_rules: [] }
    const mockFlag = { environments: { production: existingConfig } }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ ...existingConfig, default_variant: 'treatment' }),
    } as unknown as TogglerinoClient

    await updateFlagConfig(mockClient, 'my-project', 'my-flag', 'production', { default_variant: 'treatment' })

    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/flags/my-flag')
    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/production',
      { ...existingConfig, default_variant: 'treatment' },
    )
  })

  it('merges enabled into existing config', async () => {
    const existingConfig = { enabled: false, variants: [], targeting_rules: [] }
    const mockFlag = { environments: { production: existingConfig } }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ ...existingConfig, enabled: true }),
    } as unknown as TogglerinoClient

    await updateFlagConfig(mockClient, 'my-project', 'my-flag', 'production', { enabled: true })

    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/production',
      { ...existingConfig, enabled: true },
    )
  })

  it('merges variants into existing config', async () => {
    const existingConfig = { enabled: true, default_variant: 'control', variants: [{ key: 'control', value: false }], targeting_rules: [] }
    const newVariants = [{ key: 'control', value: false }, { key: 'treatment', value: true }]
    const mockFlag = { environments: { production: existingConfig } }
    const mockClient = {
      get: vi.fn().mockResolvedValue(mockFlag),
      put: vi.fn().mockResolvedValue({ ...existingConfig, variants: newVariants }),
    } as unknown as TogglerinoClient

    await updateFlagConfig(mockClient, 'my-project', 'my-flag', 'production', { variants: newVariants })

    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/my-project/flags/my-flag/environments/production',
      { ...existingConfig, variants: newVariants },
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

describe('deleteFlag', () => {
  it('calls DELETE /projects/{key}/flags/{flagKey}', async () => {
    const mockClient = { del: vi.fn().mockResolvedValue(undefined) } as unknown as TogglerinoClient
    await deleteFlag(mockClient, 'my-project', 'old-flag')
    expect(mockClient.del).toHaveBeenCalledWith('/projects/my-project/flags/old-flag')
  })
})

describe('archiveFlag', () => {
  it('PUTs to /projects/{key}/flags/{flagKey}/archive with archived: true', async () => {
    const mockClient = {
      put: vi.fn().mockResolvedValue({ key: 'my-flag', lifecycle_status: 'archived' }),
    } as unknown as TogglerinoClient
    const result = await archiveFlag(mockClient, 'my-project', 'my-flag', true)
    expect(mockClient.put).toHaveBeenCalledWith('/projects/my-project/flags/my-flag/archive', { archived: true })
    expect(result).toEqual({ key: 'my-flag', lifecycle_status: 'archived' })
  })

  it('PUTs with archived: false to restore', async () => {
    const mockClient = {
      put: vi.fn().mockResolvedValue({ key: 'my-flag', lifecycle_status: 'active' }),
    } as unknown as TogglerinoClient
    await archiveFlag(mockClient, 'my-project', 'my-flag', false)
    expect(mockClient.put).toHaveBeenCalledWith('/projects/my-project/flags/my-flag/archive', { archived: false })
  })
})
