import { describe, it, expect, vi, beforeEach } from 'vitest'
import { TogglerinoClient } from '../src/client'

describe('TogglerinoClient', () => {
  let client: TogglerinoClient

  beforeEach(() => {
    client = new TogglerinoClient('http://localhost:8080', 'pat_testtoken123')
  })

  it('sends authorization header', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
    })
    vi.stubGlobal('fetch', mockFetch)

    await client.get('/projects')

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/projects',
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer pat_testtoken123',
        }),
      }),
    )

    vi.unstubAllGlobals()
  })

  it('throws on non-ok response with error message', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: 'forbidden' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    await expect(client.get('/projects')).rejects.toThrow('forbidden')

    vi.unstubAllGlobals()
  })

  it('constructs POST requests with JSON body', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve({ id: '1' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    await client.post('/projects/test/flags', { name: 'my-flag', key: 'my-flag' })

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/projects/test/flags',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'my-flag', key: 'my-flag' }),
      }),
    )

    vi.unstubAllGlobals()
  })

  it('constructs DELETE requests', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
    })
    vi.stubGlobal('fetch', mockFetch)

    await client.del('/projects/test/flags/my-flag')

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/projects/test/flags/my-flag',
      expect.objectContaining({
        method: 'DELETE',
      }),
    )

    vi.unstubAllGlobals()
  })
})
