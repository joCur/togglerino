import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { Togglerino } from '../client'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const mockFetch = vi.fn()
// @ts-expect-error – replace global fetch for testing
globalThis.fetch = mockFetch

const baseConfig = {
  serverUrl: 'http://localhost:8080',
  sdkKey: 'sdk_test123',
  streaming: false, // disable SSE for most tests; use polling
  pollingInterval: 60_000, // long interval so polling doesn't fire during tests
  context: { userId: 'user-1', attributes: { plan: 'pro' } },
}

/** Build a successful evaluate response. */
function evaluateResponse(flags: Record<string, { value: unknown; variant: string; reason: string }>) {
  return {
    ok: true,
    json: () => Promise.resolve({ flags }),
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('Togglerino', () => {
  beforeEach(() => {
    mockFetch.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // -------------------------------------------------------------------------
  // Initialization & flag fetching
  // -------------------------------------------------------------------------

  it('should initialize and fetch flags from the server', async () => {
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: true, variant: 'on', reason: 'rule_match' },
        'max-uploads': { value: 10, variant: 'ten', reason: 'default' },
        'welcome-msg': { value: 'Hello!', variant: 'greeting', reason: 'default' },
      })
    )

    const client = new Togglerino(baseConfig)
    await client.initialize()

    // Verify the fetch was called with the correct URL, headers, and body
    expect(mockFetch).toHaveBeenCalledOnce()
    const [url, options] = mockFetch.mock.calls[0]
    expect(url).toBe('http://localhost:8080/api/v1/evaluate')
    expect(options.method).toBe('POST')
    expect(options.headers['Authorization']).toBe('Bearer sdk_test123')
    expect(options.headers['Content-Type']).toBe('application/json')

    const body = JSON.parse(options.body)
    expect(body.context.user_id).toBe('user-1')
    expect(body.context.attributes.plan).toBe('pro')

    // Verify flag values
    expect(client.getBool('dark-mode')).toBe(true)
    expect(client.getNumber('max-uploads')).toBe(10)
    expect(client.getString('welcome-msg')).toBe('Hello!')

    client.close()
  })

  it('should strip trailing slashes from serverUrl', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino({
      ...baseConfig,
      serverUrl: 'http://localhost:8080///',
    })
    await client.initialize()

    const [url] = mockFetch.mock.calls[0]
    expect(url).toBe('http://localhost:8080/api/v1/evaluate')

    client.close()
  })

  it('should emit ready event after initialization', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    const readyFn = vi.fn()
    client.on('ready', readyFn)

    await client.initialize()

    expect(readyFn).toHaveBeenCalledOnce()

    client.close()
  })

  it('should throw and emit error on fetch failure', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401 })

    const client = new Togglerino(baseConfig)
    const errorFn = vi.fn()
    client.on('error', errorFn)

    await expect(client.initialize()).rejects.toThrow(
      'Togglerino: flag evaluation failed with status 401'
    )

    expect(errorFn).toHaveBeenCalledOnce()
    expect(errorFn.mock.calls[0][0]).toBeInstanceOf(Error)

    client.close()
  })

  it('should throw and emit error on network failure', async () => {
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    const client = new Togglerino(baseConfig)
    const errorFn = vi.fn()
    client.on('error', errorFn)

    await expect(client.initialize()).rejects.toThrow('Failed to fetch')
    expect(errorFn).toHaveBeenCalledOnce()

    client.close()
  })

  // -------------------------------------------------------------------------
  // Default values for unknown flags
  // -------------------------------------------------------------------------

  it('should return default values for unknown flags', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    await client.initialize()

    expect(client.getBool('unknown')).toBe(false)
    expect(client.getBool('unknown', true)).toBe(true)
    expect(client.getString('unknown')).toBe('')
    expect(client.getString('unknown', 'fallback')).toBe('fallback')
    expect(client.getNumber('unknown')).toBe(0)
    expect(client.getNumber('unknown', 42)).toBe(42)
    expect(client.getJson('unknown')).toBeUndefined()
    expect(client.getJson('unknown', { x: 1 })).toEqual({ x: 1 })

    client.close()
  })

  it('should return default when flag type does not match getter', async () => {
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'str-flag': { value: 'hello', variant: 'a', reason: 'default' },
      })
    )

    const client = new Togglerino(baseConfig)
    await client.initialize()

    // Asking for bool on a string flag should return the default
    expect(client.getBool('str-flag')).toBe(false)
    expect(client.getNumber('str-flag')).toBe(0)
    // getJson does not check type, returns the value
    expect(client.getJson('str-flag')).toBe('hello')

    client.close()
  })

  // -------------------------------------------------------------------------
  // getDetail
  // -------------------------------------------------------------------------

  it('should return full evaluation detail', async () => {
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: true, variant: 'on', reason: 'rule_match' },
      })
    )

    const client = new Togglerino(baseConfig)
    await client.initialize()

    const detail = client.getDetail('dark-mode')
    expect(detail).toEqual({
      value: true,
      variant: 'on',
      reason: 'rule_match',
    })

    expect(client.getDetail('nonexistent')).toBeUndefined()

    client.close()
  })

  // -------------------------------------------------------------------------
  // Context update
  // -------------------------------------------------------------------------

  it('should update context and re-fetch flags', async () => {
    // First fetch (initialize)
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: false, variant: 'off', reason: 'default' },
      })
    )
    // Second fetch (after context update)
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: true, variant: 'on', reason: 'rule_match' },
      })
    )

    const client = new Togglerino(baseConfig)
    await client.initialize()

    expect(client.getBool('dark-mode')).toBe(false)

    const changeFn = vi.fn()
    client.on('change', changeFn)

    await client.updateContext({ userId: 'user-2', attributes: { plan: 'enterprise' } })

    expect(client.getBool('dark-mode')).toBe(true)

    // Verify second fetch used updated context
    const [, options] = mockFetch.mock.calls[1]
    const body = JSON.parse(options.body)
    expect(body.context.user_id).toBe('user-2')
    expect(body.context.attributes.plan).toBe('enterprise')

    // Change event should have been emitted
    expect(changeFn).toHaveBeenCalledWith({
      flagKey: 'dark-mode',
      value: true,
      variant: 'on',
    })

    client.close()
  })

  it('should merge context on update (not replace)', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    await client.initialize()

    // Update only attributes, userId should persist from original config
    await client.updateContext({ attributes: { tier: 'gold' } })

    const [, options] = mockFetch.mock.calls[1]
    const body = JSON.parse(options.body)
    // userId from original config should still be there
    expect(body.context.user_id).toBe('user-1')

    client.close()
  })

  // -------------------------------------------------------------------------
  // getContext
  // -------------------------------------------------------------------------

  it('should return the current context via getContext()', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    await client.initialize()

    expect(client.getContext()).toEqual({ userId: 'user-1', attributes: { plan: 'pro' } })

    client.close()
  })

  it('should return updated context after updateContext()', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    await client.initialize()

    await client.updateContext({ userId: 'user-2' })

    expect(client.getContext()).toEqual({ userId: 'user-2', attributes: { plan: 'pro' } })

    client.close()
  })

  // -------------------------------------------------------------------------
  // context_change event
  // -------------------------------------------------------------------------

  it('should emit context_change event when context is updated', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    await client.initialize()

    const contextChangeFn = vi.fn()
    client.on('context_change', contextChangeFn)

    await client.updateContext({ userId: 'user-2', attributes: { plan: 'enterprise' } })

    expect(contextChangeFn).toHaveBeenCalledWith({
      userId: 'user-2',
      attributes: { plan: 'enterprise' },
    })

    client.close()
  })

  // -------------------------------------------------------------------------
  // Event emitter
  // -------------------------------------------------------------------------

  it('should allow unsubscribing from events', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    const fn = vi.fn()
    const unsub = client.on('ready', fn)

    // Unsubscribe before initialize
    unsub()

    await client.initialize()

    // Listener should not have been called
    expect(fn).not.toHaveBeenCalled()

    client.close()
  })

  it('should handle multiple listeners on the same event', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    const fn1 = vi.fn()
    const fn2 = vi.fn()
    client.on('ready', fn1)
    client.on('ready', fn2)

    await client.initialize()

    expect(fn1).toHaveBeenCalledOnce()
    expect(fn2).toHaveBeenCalledOnce()

    client.close()
  })

  it('should not break if a listener throws', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    const throwingFn = vi.fn(() => {
      throw new Error('listener error')
    })
    const normalFn = vi.fn()

    client.on('ready', throwingFn)
    client.on('ready', normalFn)

    await client.initialize()

    // Both should have been called, even though the first throws
    expect(throwingFn).toHaveBeenCalledOnce()
    expect(normalFn).toHaveBeenCalledOnce()

    client.close()
  })

  // -------------------------------------------------------------------------
  // Polling
  // -------------------------------------------------------------------------

  it('should poll on the configured interval when streaming is disabled', async () => {
    vi.useFakeTimers()

    mockFetch.mockResolvedValue(evaluateResponse({}))

    const client = new Togglerino({
      ...baseConfig,
      streaming: false,
      pollingInterval: 5_000,
    })
    await client.initialize()

    expect(mockFetch).toHaveBeenCalledTimes(1)

    // Advance past one polling interval
    await vi.advanceTimersByTimeAsync(5_000)
    expect(mockFetch).toHaveBeenCalledTimes(2)

    // Advance past another
    await vi.advanceTimersByTimeAsync(5_000)
    expect(mockFetch).toHaveBeenCalledTimes(3)

    client.close()

    // After close, no more polling
    await vi.advanceTimersByTimeAsync(5_000)
    expect(mockFetch).toHaveBeenCalledTimes(3)

    vi.useRealTimers()
  })

  it('should emit change events when polling detects a flag change', async () => {
    vi.useFakeTimers()

    // First fetch: dark-mode is false
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: false, variant: 'off', reason: 'default' },
      })
    )

    const client = new Togglerino({
      ...baseConfig,
      streaming: false,
      pollingInterval: 5_000,
    })
    await client.initialize()

    const changeFn = vi.fn()
    client.on('change', changeFn)

    // Second fetch: dark-mode changed to true
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: true, variant: 'on', reason: 'rule_match' },
      })
    )

    await vi.advanceTimersByTimeAsync(5_000)

    expect(changeFn).toHaveBeenCalledWith({
      flagKey: 'dark-mode',
      value: true,
      variant: 'on',
    })

    client.close()
    vi.useRealTimers()
  })

  // -------------------------------------------------------------------------
  // SSE streaming
  // -------------------------------------------------------------------------

  it('should connect to SSE and process flag_update events', async () => {
    // Initial fetch
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'dark-mode': { value: false, variant: 'off', reason: 'default' },
      })
    )

    // Create a mock ReadableStream that emits one SSE event then closes
    const sseData =
      'event: flag_update\ndata: {"flagKey":"dark-mode","value":true,"variant":"on"}\n\n'
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

    // SSE fetch
    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: {
        getReader: () => mockReader,
      },
    })

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
    })

    const changeFn = vi.fn()
    client.on('change', changeFn)

    await client.initialize()

    // Give the async SSE processing a tick to complete
    await new Promise((r) => setTimeout(r, 10))

    expect(client.getBool('dark-mode')).toBe(true)
    expect(changeFn).toHaveBeenCalledWith({
      flagKey: 'dark-mode',
      value: true,
      variant: 'on',
    })

    client.close()
  })

  it('should fall back to polling when SSE fetch fails and schedule reconnection', async () => {
    vi.useFakeTimers()

    // Initial fetch
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    // SSE fetch fails — triggers polling fallback + reconnection schedule (1s delay)
    mockFetch.mockRejectedValueOnce(new Error('SSE connection refused'))

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
      pollingInterval: 5_000,
    })

    const reconnectingFn = vi.fn()
    client.on('reconnecting', reconnectingFn)

    await client.initialize()

    // Should have emitted reconnecting event
    expect(reconnectingFn).toHaveBeenCalledWith({ attempt: 1, delay: 1000 })

    // At 1s: SSE retry fires — make it fail again (schedules next at 2s)
    mockFetch.mockRejectedValueOnce(new Error('SSE still down'))
    await vi.advanceTimersByTimeAsync(1_000)

    expect(reconnectingFn).toHaveBeenCalledTimes(2)

    // Provide mocks for subsequent SSE retries and polling within the window
    mockFetch.mockRejectedValueOnce(new Error('SSE still down'))
    mockFetch.mockResolvedValue(evaluateResponse({}))

    // Advance to 5s total — polling at 5s fires
    await vi.advanceTimersByTimeAsync(4_000)

    // Verify polling is running by checking for an evaluate call
    const evaluateCalls = mockFetch.mock.calls.filter(
      ([url]: [string]) => url.includes('/evaluate')
    )
    expect(evaluateCalls.length).toBeGreaterThanOrEqual(2) // initial + at least one poll

    client.close()
    vi.useRealTimers()
  })

  it('should fall back to polling when SSE response has no body and schedule reconnection', async () => {
    vi.useFakeTimers()

    // Initial fetch
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    // SSE fetch returns ok but no body — triggers polling + reconnect schedule
    mockFetch.mockResolvedValueOnce({ ok: true, body: null })

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
      pollingInterval: 5_000,
    })

    const reconnectingFn = vi.fn()
    client.on('reconnecting', reconnectingFn)

    await client.initialize()

    // Should have emitted reconnecting event (1s delay)
    expect(reconnectingFn).toHaveBeenCalledWith({ attempt: 1, delay: 1000 })

    // Provide mocks for SSE retries and polling
    mockFetch.mockResolvedValueOnce({ ok: true, body: null }) // SSE retry at 1s
    mockFetch.mockResolvedValue(evaluateResponse({})) // polling + further retries

    // Advance past one polling interval
    await vi.advanceTimersByTimeAsync(5_000)

    // Verify polling is running (at least initial + poll evaluate calls)
    const evaluateCalls = mockFetch.mock.calls.filter(
      ([url]: [string]) => url.includes('/evaluate')
    )
    expect(evaluateCalls.length).toBeGreaterThanOrEqual(2)

    client.close()
    vi.useRealTimers()
  })

  // -------------------------------------------------------------------------
  // SSE reconnection with exponential backoff
  // -------------------------------------------------------------------------

  it('should use exponential backoff delays: 1s, 2s, 4s, 8s', async () => {
    vi.useFakeTimers()

    // Initial fetch
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    // SSE fails
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
      pollingInterval: 60_000, // long interval so polling doesn't interfere
    })

    const reconnectingFn = vi.fn()
    client.on('reconnecting', reconnectingFn)

    await client.initialize()

    // First reconnect scheduled: attempt 1, delay 1000ms
    expect(reconnectingFn).toHaveBeenCalledWith({ attempt: 1, delay: 1000 })

    // At 1s: SSE retry fires, fails again -> schedules at 2s delay
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))
    await vi.advanceTimersByTimeAsync(1_000)
    expect(reconnectingFn).toHaveBeenCalledWith({ attempt: 2, delay: 2000 })

    // At 3s: SSE retry fires, fails again -> schedules at 4s delay
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))
    await vi.advanceTimersByTimeAsync(2_000)
    expect(reconnectingFn).toHaveBeenCalledWith({ attempt: 3, delay: 4000 })

    // At 7s: SSE retry fires, fails again -> schedules at 8s delay
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))
    await vi.advanceTimersByTimeAsync(4_000)
    expect(reconnectingFn).toHaveBeenCalledWith({ attempt: 4, delay: 8000 })

    expect(reconnectingFn).toHaveBeenCalledTimes(4)

    client.close()
    vi.useRealTimers()
  })

  it('should cap retry delay at 30 seconds', async () => {
    vi.useFakeTimers()

    // Initial fetch
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    // SSE fails
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
      pollingInterval: 60_000,
    })

    const reconnectingFn = vi.fn()
    client.on('reconnecting', reconnectingFn)

    await client.initialize()

    // Advance through retries: 1s, 2s, 4s, 8s, 16s — total 31s elapsed
    // After 5 failures, next delay would be 2^5 * 1000 = 32000 but capped at 30000
    for (let i = 0; i < 5; i++) {
      mockFetch.mockRejectedValueOnce(new Error('SSE failed'))
      const delay = Math.min(1000 * Math.pow(2, i), 30000)
      await vi.advanceTimersByTimeAsync(delay)
    }

    // 6th reconnect attempt: delay should be capped at 30000
    expect(reconnectingFn).toHaveBeenLastCalledWith({ attempt: 6, delay: 30000 })

    client.close()
    vi.useRealTimers()
  })

  it('should emit reconnected and stop polling on successful SSE reconnect', async () => {
    vi.useFakeTimers()

    // Initial fetch
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    // SSE fails — triggers reconnect schedule
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
      pollingInterval: 60_000,
    })

    const reconnectingFn = vi.fn()
    const reconnectedFn = vi.fn()
    client.on('reconnecting', reconnectingFn)
    client.on('reconnected', reconnectedFn)

    await client.initialize()

    expect(reconnectingFn).toHaveBeenCalledTimes(1)

    // At 1s: SSE retry succeeds
    const encoder = new TextEncoder()
    let readerDone = false
    const mockReader = {
      read: vi.fn().mockImplementation(() => {
        if (!readerDone) {
          readerDone = true
          return Promise.resolve({
            done: false,
            value: encoder.encode('event: flag_update\ndata: {"flagKey":"x","value":true,"variant":"on"}\n\n'),
          })
        }
        // Keep stream open by returning a pending promise
        return new Promise(() => {})
      }),
    }
    mockFetch.mockResolvedValueOnce({
      ok: true,
      body: { getReader: () => mockReader },
    })

    await vi.advanceTimersByTimeAsync(1_000)

    // Give async processing a tick
    await vi.advanceTimersByTimeAsync(0)

    expect(reconnectedFn).toHaveBeenCalledOnce()

    client.close()
    vi.useRealTimers()
  })

  it('should cancel pending reconnection timeout on close', async () => {
    vi.useFakeTimers()

    // Initial fetch
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    // SSE fails
    mockFetch.mockRejectedValueOnce(new Error('SSE failed'))

    const client = new Togglerino({
      ...baseConfig,
      streaming: true,
      pollingInterval: 60_000,
    })

    await client.initialize()

    const callCountBeforeClose = mockFetch.mock.calls.length

    // Close before the 1s retry timeout fires
    client.close()

    // Advance past the retry timeout
    await vi.advanceTimersByTimeAsync(2_000)

    // No new fetch calls should have been made
    expect(mockFetch).toHaveBeenCalledTimes(callCountBeforeClose)

    vi.useRealTimers()
  })

  // -------------------------------------------------------------------------
  // close()
  // -------------------------------------------------------------------------

  it('should clear all listeners on close', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino(baseConfig)
    const changeFn = vi.fn()
    client.on('change', changeFn)

    await client.initialize()
    client.close()

    // After close, updateContext should still work but change events should not fire
    // (listeners were cleared)
    mockFetch.mockResolvedValueOnce(
      evaluateResponse({
        'new-flag': { value: true, variant: 'on', reason: 'default' },
      })
    )

    // updateContext will fetchFlags, which may emit change, but listeners are cleared
    await client.updateContext({ userId: 'user-3' })
    expect(changeFn).not.toHaveBeenCalled()
  })

  // -------------------------------------------------------------------------
  // Default config values
  // -------------------------------------------------------------------------

  it('should use default context when none provided', async () => {
    mockFetch.mockResolvedValueOnce(evaluateResponse({}))

    const client = new Togglerino({
      serverUrl: 'http://localhost:8080',
      sdkKey: 'sdk_test',
      streaming: false,
    })

    await client.initialize()

    const [, options] = mockFetch.mock.calls[0]
    const body = JSON.parse(options.body)
    expect(body.context.user_id).toBe('')
    expect(body.context.attributes).toEqual({})

    client.close()
  })
})
