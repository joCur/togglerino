import { describe, it, expect } from 'vitest'
import { canonicalize } from '../flag-diff'

describe('canonicalize', () => {
  it('serializes objects with sorted keys', () => {
    const a = canonicalize({ z: 1, a: 2 })
    const b = canonicalize({ a: 2, z: 1 })
    expect(a).toBe(b)
  })

  it('handles nested objects with different key order', () => {
    const a = canonicalize({ outer: { z: 1, a: 2 } })
    const b = canonicalize({ outer: { a: 2, z: 1 } })
    expect(a).toBe(b)
  })

  it('handles arrays (preserves order)', () => {
    const a = canonicalize([{ b: 1, a: 2 }, { d: 3, c: 4 }])
    const b = canonicalize([{ a: 2, b: 1 }, { c: 4, d: 3 }])
    expect(a).toBe(b)
  })

  it('handles primitives', () => {
    expect(canonicalize('hello')).toBe('"hello"')
    expect(canonicalize(42)).toBe('42')
    expect(canonicalize(true)).toBe('true')
    expect(canonicalize(null)).toBe('null')
  })

  it('handles undefined values by omitting them', () => {
    const a = canonicalize({ a: 1, b: undefined })
    const b = canonicalize({ a: 1 })
    expect(a).toBe(b)
  })
})
