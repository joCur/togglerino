import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { type ReactNode } from 'react'
import { TogglerioContext } from '../context'
import { useFlag, useTogglerinoContext } from '../hooks'
import { useTogglerino } from '../context'
import type { EvaluationContext, FlagChangeEvent } from '@togglerino/sdk'

// ---------------------------------------------------------------------------
// Mock client factory
// ---------------------------------------------------------------------------

type Listener = (...args: unknown[]) => void

function createMockClient() {
  const listeners = new Map<string, Set<Listener>>()
  return {
    getBool: vi.fn((key: string, def: boolean) => def),
    getString: vi.fn((key: string, def: string) => def),
    getNumber: vi.fn((key: string, def: number) => def),
    getJson: vi.fn((key: string, def: unknown) => def),
    getDetail: vi.fn(),
    on: vi.fn((event: string, listener: Listener) => {
      if (!listeners.has(event)) listeners.set(event, new Set())
      listeners.get(event)!.add(listener)
      return () => {
        listeners.get(event)?.delete(listener)
      }
    }),
    initialize: vi.fn(() => Promise.resolve()),
    close: vi.fn(),
    updateContext: vi.fn(() => Promise.resolve()),
    getContext: vi.fn(() => ({ userId: 'user-1', attributes: { plan: 'pro' } })),
    // Test helper: emit an event to all registered listeners
    emit(event: string, ...args: unknown[]) {
      listeners.get(event)?.forEach((fn) => fn(...args))
    },
    _listeners: listeners,
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type MockClient = ReturnType<typeof createMockClient>

// ---------------------------------------------------------------------------
// Helper: wrap hook in provider with mock client
// ---------------------------------------------------------------------------

function createWrapper(client: MockClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TogglerioContext.Provider value={client as never}>
        {children}
      </TogglerioContext.Provider>
    )
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useTogglerino', () => {
  it('throws when used outside a provider', () => {
    // Suppress React error boundary console.error noise
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(() => {
      renderHook(() => useTogglerino())
    }).toThrow('useTogglerino must be used within a <TogglerioProvider>')

    spy.mockRestore()
  })

  it('returns the client when inside a provider', () => {
    const client = createMockClient()
    const wrapper = createWrapper(client)

    const { result } = renderHook(() => useTogglerino(), { wrapper })
    expect(result.current).toBe(client)
  })
})

describe('useFlag', () => {
  let client: MockClient

  beforeEach(() => {
    client = createMockClient()
  })

  // -------------------------------------------------------------------------
  // Boolean flags
  // -------------------------------------------------------------------------

  it('returns the default boolean value', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('dark-mode', false), { wrapper })

    expect(result.current).toBe(false)
    expect(client.getBool).toHaveBeenCalledWith('dark-mode', false)
  })

  it('returns true when getBool returns true', () => {
    client.getBool.mockReturnValue(true)
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('dark-mode', false), { wrapper })

    expect(result.current).toBe(true)
  })

  it('updates when a boolean flag changes', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('dark-mode', false), { wrapper })
    expect(result.current).toBe(false)

    // Simulate a flag change via the 'change' event
    client.getBool.mockReturnValue(true)
    act(() => {
      client.emit('change', {
        flagKey: 'dark-mode',
        value: true,
        variant: 'on',
      } satisfies FlagChangeEvent)
    })

    expect(result.current).toBe(true)
  })

  it('does not update when a different flag changes', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('dark-mode', false), { wrapper })
    expect(result.current).toBe(false)

    act(() => {
      client.emit('change', {
        flagKey: 'other-flag',
        value: true,
        variant: 'on',
      } satisfies FlagChangeEvent)
    })

    // Should still be the default; getBool should not have been called again
    // for the other flag key
    expect(result.current).toBe(false)
  })

  // -------------------------------------------------------------------------
  // String flags
  // -------------------------------------------------------------------------

  it('returns the default string value', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('theme', 'light'), { wrapper })

    expect(result.current).toBe('light')
    expect(client.getString).toHaveBeenCalledWith('theme', 'light')
  })

  it('updates when a string flag changes', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('theme', 'light'), { wrapper })
    expect(result.current).toBe('light')

    client.getString.mockReturnValue('dark')
    act(() => {
      client.emit('change', {
        flagKey: 'theme',
        value: 'dark',
        variant: 'dark',
      } satisfies FlagChangeEvent)
    })

    expect(result.current).toBe('dark')
  })

  // -------------------------------------------------------------------------
  // Number flags
  // -------------------------------------------------------------------------

  it('returns the default number value', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('max-items', 10), { wrapper })

    expect(result.current).toBe(10)
    expect(client.getNumber).toHaveBeenCalledWith('max-items', 10)
  })

  it('updates when a number flag changes', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('max-items', 10), { wrapper })

    client.getNumber.mockReturnValue(50)
    act(() => {
      client.emit('change', {
        flagKey: 'max-items',
        value: 50,
        variant: 'high',
      } satisfies FlagChangeEvent)
    })

    expect(result.current).toBe(50)
  })

  // -------------------------------------------------------------------------
  // JSON flags
  // -------------------------------------------------------------------------

  it('returns the default JSON value', () => {
    const defaultVal = { color: 'red', size: 12 }
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useFlag('ui-config', defaultVal), {
      wrapper,
    })

    expect(result.current).toEqual(defaultVal)
    expect(client.getJson).toHaveBeenCalledWith('ui-config', defaultVal)
  })

  // -------------------------------------------------------------------------
  // Unsubscribe on unmount
  // -------------------------------------------------------------------------

  it('unsubscribes from change events on unmount', () => {
    const wrapper = createWrapper(client)
    const { unmount } = renderHook(() => useFlag('dark-mode', false), { wrapper })

    // There should be one listener registered
    expect(client._listeners.get('change')?.size).toBe(1)

    unmount()

    // After unmount the listener should be removed
    expect(client._listeners.get('change')?.size).toBe(0)
  })

  // -------------------------------------------------------------------------
  // Re-subscribes when key changes
  // -------------------------------------------------------------------------

  it('re-subscribes when the flag key changes', () => {
    const wrapper = createWrapper(client)
    const { rerender } = renderHook(
      ({ key }: { key: string }) => useFlag(key, false),
      {
        wrapper,
        initialProps: { key: 'flag-a' },
      }
    )

    expect(client.on).toHaveBeenCalledTimes(1)

    rerender({ key: 'flag-b' })

    // Should have subscribed again for the new key
    expect(client.on).toHaveBeenCalledTimes(2)
  })
})

describe('useTogglerinoContext', () => {
  let client: MockClient

  beforeEach(() => {
    client = createMockClient()
  })

  it('returns the current context from the client', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useTogglerinoContext(), { wrapper })

    expect(result.current.context).toEqual({ userId: 'user-1', attributes: { plan: 'pro' } })
  })

  it('provides an updateContext function', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useTogglerinoContext(), { wrapper })

    expect(typeof result.current.updateContext).toBe('function')
  })

  it('calls client.updateContext when updateContext is called', async () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useTogglerinoContext(), { wrapper })

    await act(async () => {
      await result.current.updateContext({ userId: 'user-2' })
    })

    expect(client.updateContext).toHaveBeenCalledWith({ userId: 'user-2' })
  })

  it('updates context state when context_change event is emitted', () => {
    const wrapper = createWrapper(client)
    const { result } = renderHook(() => useTogglerinoContext(), { wrapper })

    expect(result.current.context).toEqual({ userId: 'user-1', attributes: { plan: 'pro' } })

    const newContext: EvaluationContext = { userId: 'user-2', attributes: { plan: 'enterprise' } }
    client.getContext.mockReturnValue(newContext)

    act(() => {
      client.emit('context_change', newContext)
    })

    expect(result.current.context).toEqual(newContext)
  })

  it('unsubscribes from context_change on unmount', () => {
    const wrapper = createWrapper(client)
    const { unmount } = renderHook(() => useTogglerinoContext(), { wrapper })

    expect(client._listeners.get('context_change')?.size).toBe(1)

    unmount()

    expect(client._listeners.get('context_change')?.size).toBe(0)
  })
})
