import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { TogglerioServer } from '../server'
import type { DefinitionsResponse } from '../server-types'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const mockFetch = vi.fn()
// @ts-expect-error – replace global fetch for testing
globalThis.fetch = mockFetch

const baseConfig = {
  serverUrl: 'http://localhost:8080',
  sdkKey: 'sdk_test123',
  streaming: false,
  pollingInterval: 60_000,
}

/** Build a successful definitions response. */
function definitionsResponse(data: DefinitionsResponse) {
  return {
    ok: true,
    json: () => Promise.resolve(data),
  }
}

/** A minimal boolean flag definition. */
function booleanFlag(key: string, enabled: boolean, status = 'active') {
  return {
    key,
    valueType: 'boolean',
    status,
    defaultValue: false,
    variants: [
      { name: 'true', value: true },
      { name: 'false', value: false },
    ],
    config: {
      enabled,
      fallthroughVariant: enabled ? 'true' : 'false',
      offVariant: 'false',
      targetingRules: [],
    },
  }
}

/** A multi-variant string flag definition. */
function stringFlag(
  key: string,
  fallthroughVariant: string,
  variants: { name: string; value: unknown }[],
  targetingRules: { variant: string; percentage: number | null; conditions: { attribute: string; operator: string; value: string }[] }[] = [],
) {
  return {
    key,
    valueType: 'string',
    status: 'active',
    defaultValue: null,
    variants,
    config: {
      enabled: true,
      fallthroughVariant,
      offVariant: '',
      targetingRules,
    },
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('TogglerioServer', () => {
  beforeEach(() => {
    mockFetch.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // -------------------------------------------------------------------------
  // Initialization
  // -------------------------------------------------------------------------

  it('should initialize and fetch definitions from the server', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('dark-mode', true)],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    expect(mockFetch).toHaveBeenCalledOnce()
    const [url, options] = mockFetch.mock.calls[0]
    expect(url).toBe('http://localhost:8080/api/v1/definitions')
    expect(options.headers['Authorization']).toBe('Bearer sdk_test123')

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getBool('dark-mode')).toBe(true)

    server.close()
  })

  it('should strip trailing slashes from serverUrl', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({ flags: [], segments: [] })
    )

    const server = new TogglerioServer({
      ...baseConfig,
      serverUrl: 'http://localhost:8080///',
    })
    await server.initialize()

    const [url] = mockFetch.mock.calls[0]
    expect(url).toBe('http://localhost:8080/api/v1/definitions')

    server.close()
  })

  it('should throw on fetch failure', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 })

    const server = new TogglerioServer(baseConfig)

    await expect(server.initialize()).rejects.toThrow(
      'TogglerinoServer: definitions fetch failed with status 401'
    )

    server.close()
  })

  it('should throw on network failure', async () => {
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    const server = new TogglerioServer(baseConfig)

    await expect(server.initialize()).rejects.toThrow('Failed to fetch')

    server.close()
  })

  // -------------------------------------------------------------------------
  // evaluate() — typed getters
  // -------------------------------------------------------------------------

  it('should evaluate boolean flags and return correct values via getBool', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          booleanFlag('enabled-flag', true),
          booleanFlag('disabled-flag', false),
        ],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getBool('enabled-flag')).toBe(true)
    expect(flags.getBool('disabled-flag')).toBe(false)

    server.close()
  })

  it('should evaluate string flags and return correct values via getString', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          stringFlag('color', 'blue', [
            { name: 'blue', value: 'blue' },
            { name: 'red', value: 'red' },
          ]),
        ],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getString('color')).toBe('blue')

    server.close()
  })

  it('should evaluate number flags and return correct values via getNumber', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          {
            key: 'max-items',
            valueType: 'number',
            status: 'active',
            defaultValue: 0,
            variants: [{ name: 'ten', value: 10 }],
            config: {
              enabled: true,
              fallthroughVariant: 'ten',
              offVariant: '',
              targetingRules: [],
            },
          },
        ],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getNumber('max-items')).toBe(10)

    server.close()
  })

  it('should evaluate JSON flags and return correct values via getJson', async () => {
    const jsonValue = { theme: 'dark', sidebar: true }
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          {
            key: 'ui-config',
            valueType: 'json',
            status: 'active',
            defaultValue: null,
            variants: [{ name: 'default', value: jsonValue }],
            config: {
              enabled: true,
              fallthroughVariant: 'default',
              offVariant: '',
              targetingRules: [],
            },
          },
        ],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getJson('ui-config')).toEqual(jsonValue)

    server.close()
  })

  it('should return default values for unknown flags', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({ flags: [], segments: [] })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getBool('unknown')).toBe(false)
    expect(flags.getBool('unknown', true)).toBe(true)
    expect(flags.getString('unknown')).toBe('')
    expect(flags.getString('unknown', 'fallback')).toBe('fallback')
    expect(flags.getNumber('unknown')).toBe(0)
    expect(flags.getNumber('unknown', 42)).toBe(42)
    expect(flags.getJson('unknown')).toBeUndefined()
    expect(flags.getJson('unknown', { x: 1 })).toEqual({ x: 1 })
    expect(flags.getDetail('unknown')).toBeUndefined()

    server.close()
  })

  it('should return default when flag type does not match getter', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          stringFlag('str-flag', 'greeting', [
            { name: 'greeting', value: 'hello' },
          ]),
        ],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    // Asking for bool on a string flag should return the default
    expect(flags.getBool('str-flag')).toBe(false)
    expect(flags.getNumber('str-flag')).toBe(0)
    // getJson does not check type, returns the value
    expect(flags.getJson('str-flag')).toBe('hello')

    server.close()
  })

  it('should return full evaluation detail via getDetail', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('dark-mode', true)],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    const detail = flags.getDetail('dark-mode')
    expect(detail).toEqual({
      value: true,
      variant: 'true',
      reason: 'default',
    })

    server.close()
  })

  // -------------------------------------------------------------------------
  // Different contexts produce different results (core server-side use case)
  // -------------------------------------------------------------------------

  it('should produce different results for different contexts', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          stringFlag(
            'banner',
            'default',
            [
              { name: 'default', value: 'Welcome!' },
              { name: 'pro', value: 'Welcome, Pro user!' },
            ],
            [
              {
                variant: 'pro',
                percentage: null,
                conditions: [
                  { attribute: 'plan', operator: 'equals', value: 'pro' },
                ],
              },
            ]
          ),
        ],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    // Pro user sees the pro banner
    const proFlags = server.evaluate({
      userId: 'user-1',
      attributes: { plan: 'pro' },
    })
    expect(proFlags.getString('banner')).toBe('Welcome, Pro user!')
    expect(proFlags.getDetail('banner')?.reason).toBe('rule_match')

    // Free user sees the default banner
    const freeFlags = server.evaluate({
      userId: 'user-2',
      attributes: { plan: 'free' },
    })
    expect(freeFlags.getString('banner')).toBe('Welcome!')
    expect(freeFlags.getDetail('banner')?.reason).toBe('default')

    // Both evaluated from a single fetch (definitions were fetched once)
    expect(mockFetch).toHaveBeenCalledOnce()

    server.close()
  })

  it('should resolve segment_match conditions across different contexts', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          stringFlag(
            'feature',
            'off',
            [
              { name: 'off', value: 'disabled' },
              { name: 'on', value: 'enabled' },
            ],
            [
              {
                variant: 'on',
                percentage: null,
                conditions: [
                  { attribute: '', operator: 'segment_match', value: 'beta-users' },
                ],
              },
            ]
          ),
        ],
        segments: [
          {
            key: 'beta-users',
            conditions: [
              { attribute: 'beta', operator: 'equals', value: 'true' },
            ],
          },
        ],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const betaFlags = server.evaluate({
      userId: 'user-1',
      attributes: { beta: 'true' },
    })
    expect(betaFlags.getString('feature')).toBe('enabled')

    const nonBetaFlags = server.evaluate({
      userId: 'user-2',
      attributes: { beta: 'false' },
    })
    expect(nonBetaFlags.getString('feature')).toBe('disabled')

    server.close()
  })

  it('should evaluate without a context', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('feature', true)],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    // No context passed at all
    const flags = server.evaluate()
    expect(flags.getBool('feature')).toBe(true)

    server.close()
  })

  // -------------------------------------------------------------------------
  // Polling
  // -------------------------------------------------------------------------

  it('should poll on the configured interval when streaming is disabled', async () => {
    vi.useFakeTimers()

    mockFetch.mockResolvedValue(
      definitionsResponse({ flags: [], segments: [] })
    )

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: false,
      pollingInterval: 5_000,
    })
    await server.initialize()

    expect(mockFetch).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(5_000)
    expect(mockFetch).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(5_000)
    expect(mockFetch).toHaveBeenCalledTimes(3)

    server.close()

    await vi.advanceTimersByTimeAsync(5_000)
    expect(mockFetch).toHaveBeenCalledTimes(3)

    vi.useRealTimers()
  })

  it('should update cached definitions when polling fetches new data', async () => {
    vi.useFakeTimers()

    // First fetch: flag is disabled
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('feature', false)],
        segments: [],
      })
    )

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: false,
      pollingInterval: 5_000,
    })
    await server.initialize()

    expect(server.evaluate().getBool('feature')).toBe(false)

    // Second fetch: flag is now enabled
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('feature', true)],
        segments: [],
      })
    )

    await vi.advanceTimersByTimeAsync(5_000)

    expect(server.evaluate().getBool('feature')).toBe(true)

    server.close()
    vi.useRealTimers()
  })

  // -------------------------------------------------------------------------
  // SSE streaming
  // -------------------------------------------------------------------------

  it('should connect to SSE and re-fetch definitions on flag_update event', async () => {
    // Initial definitions fetch
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('dark-mode', false)],
        segments: [],
      })
    )

    // SSE stream that emits one flag_update event then closes
    const sseData =
      'event: flag_update\ndata: {"flagKey":"dark-mode"}\n\n'
    const encoder = new TextEncoder()
    let readerDone = false

    const mockReader = {
      read: vi.fn().mockImplementation(() => {
        if (!readerDone) {
          readerDone = true
          return Promise.resolve({ done: false, value: encoder.encode(sseData) })
        }
        return Promise.resolve({ done: true, value: undefined })
      }),
    }

    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: { getReader: () => mockReader },
    })

    // Re-fetch after SSE event — dark-mode is now enabled
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('dark-mode', true)],
        segments: [],
      })
    )

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: true,
    })
    await server.initialize()

    // Give async SSE processing a tick
    await new Promise((r) => setTimeout(r, 10))

    expect(server.evaluate().getBool('dark-mode')).toBe(true)

    // Verify: initial definitions + SSE connection + re-fetch = 3 calls
    expect(mockFetch).toHaveBeenCalledTimes(3)

    server.close()
  })

  it('should re-fetch definitions on flag_deleted event', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [
          booleanFlag('keep-me', true),
          booleanFlag('delete-me', true),
        ],
        segments: [],
      })
    )

    const sseData = 'event: flag_deleted\ndata: {"flagKey":"delete-me"}\n\n'
    const encoder = new TextEncoder()
    let readerDone = false

    const mockReader = {
      read: vi.fn().mockImplementation(() => {
        if (!readerDone) {
          readerDone = true
          return Promise.resolve({ done: false, value: encoder.encode(sseData) })
        }
        return Promise.resolve({ done: true, value: undefined })
      }),
    }

    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: { getReader: () => mockReader },
    })

    // Re-fetch after delete event — delete-me is gone
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('keep-me', true)],
        segments: [],
      })
    )

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: true,
    })
    await server.initialize()

    await new Promise((r) => setTimeout(r, 10))

    expect(server.evaluate().getBool('keep-me')).toBe(true)
    expect(server.evaluate().getDetail('delete-me')).toBeUndefined()

    server.close()
  })

  it('should fall back to polling when SSE fails and schedule reconnection', async () => {
    vi.useFakeTimers()

    mockFetch.mockResolvedValueOnce(
      definitionsResponse({ flags: [], segments: [] })
    )

    // SSE connection fails
    mockFetch.mockRejectedValueOnce(new Error('SSE connection refused'))

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: true,
      pollingInterval: 5_000,
    })
    await server.initialize()

    // SSE retry at 1s — fail again
    mockFetch.mockRejectedValueOnce(new Error('SSE still down'))
    // Provide definitions responses for polling
    mockFetch.mockResolvedValue(
      definitionsResponse({ flags: [], segments: [] })
    )

    // Advance to 5s — polling at 5s fires
    await vi.advanceTimersByTimeAsync(5_000)

    // Verify polling is running (initial + at least one poll)
    const definitionsCalls = mockFetch.mock.calls.filter(
      ([url]: [string]) => url.includes('/definitions')
    )
    expect(definitionsCalls.length).toBeGreaterThanOrEqual(2)

    server.close()
    vi.useRealTimers()
  })

  it('should use exponential backoff for SSE reconnection', async () => {
    vi.useFakeTimers()

    mockFetch.mockResolvedValueOnce(
      definitionsResponse({ flags: [], segments: [] })
    )
    // SSE fails
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: true,
      pollingInterval: 60_000,
    })
    await server.initialize()

    // First SSE failure scheduled a retry at 1s delay
    // Track SSE connection attempts via /stream calls
    const sseCallsBefore = mockFetch.mock.calls.filter(
      ([url]: [string]) => url.includes('/stream')
    ).length

    // At 1s: SSE retry fires, fail again -> next retry at 2s
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))
    await vi.advanceTimersByTimeAsync(1_000)

    const sseCallsAfter1s = mockFetch.mock.calls.filter(
      ([url]: [string]) => url.includes('/stream')
    ).length
    expect(sseCallsAfter1s).toBe(sseCallsBefore + 1)

    // At 3s (1+2): SSE retry fires, fail again -> next retry at 4s
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))
    await vi.advanceTimersByTimeAsync(2_000)

    const sseCallsAfter3s = mockFetch.mock.calls.filter(
      ([url]: [string]) => url.includes('/stream')
    ).length
    expect(sseCallsAfter3s).toBe(sseCallsAfter1s + 1)

    server.close()
    vi.useRealTimers()
  })

  // -------------------------------------------------------------------------
  // close()
  // -------------------------------------------------------------------------

  it('should cancel pending reconnection timeout on close', async () => {
    vi.useFakeTimers()

    mockFetch.mockResolvedValueOnce(
      definitionsResponse({ flags: [], segments: [] })
    )
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: true,
      pollingInterval: 60_000,
    })
    await server.initialize()

    const callCountBeforeClose = mockFetch.mock.calls.length

    server.close()

    await vi.advanceTimersByTimeAsync(2_000)

    // No new fetch calls should have been made
    expect(mockFetch).toHaveBeenCalledTimes(callCountBeforeClose)

    vi.useRealTimers()
  })

  it('should stop polling on close', async () => {
    vi.useFakeTimers()

    mockFetch.mockResolvedValue(
      definitionsResponse({ flags: [], segments: [] })
    )

    const server = new TogglerioServer({
      ...baseConfig,
      streaming: false,
      pollingInterval: 5_000,
    })
    await server.initialize()

    expect(mockFetch).toHaveBeenCalledTimes(1)

    server.close()

    await vi.advanceTimersByTimeAsync(10_000)

    // No additional calls after close
    expect(mockFetch).toHaveBeenCalledTimes(1)

    vi.useRealTimers()
  })

  // -------------------------------------------------------------------------
  // Archived / disabled flags
  // -------------------------------------------------------------------------

  it('should return the fallthrough variant value for archived boolean flags', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('old-feature', false, 'archived')],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getBool('old-feature')).toBe(false)
    expect(flags.getDetail('old-feature')?.reason).toBe('archived')

    server.close()
  })

  it('should return false for disabled boolean flags', async () => {
    mockFetch.mockResolvedValueOnce(
      definitionsResponse({
        flags: [booleanFlag('off-feature', false)],
        segments: [],
      })
    )

    const server = new TogglerioServer(baseConfig)
    await server.initialize()

    const flags = server.evaluate({ userId: 'user-1' })
    expect(flags.getBool('off-feature')).toBe(false)
    expect(flags.getDetail('off-feature')?.reason).toBe('disabled')

    server.close()
  })
})
