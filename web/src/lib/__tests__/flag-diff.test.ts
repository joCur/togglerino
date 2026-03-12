import { describe, it, expect } from 'vitest'
import {
  canonicalize,
  compareEnabled,
  compareDefaultVariant,
  compareVariants,
  compareRules,
  compareFlag,
} from '../flag-diff'
import type { FlagEnvironmentConfig } from '@/api/types'

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

function makeConfig(overrides: Partial<FlagEnvironmentConfig> & { environment_id: string }): FlagEnvironmentConfig {
  return {
    id: 'cfg-' + overrides.environment_id,
    flag_id: 'flag-1',
    enabled: false,
    default_variant: 'off',
    variants: [],
    targeting_rules: [],
    updated_at: '2026-01-01T00:00:00Z',
    locked: false,
    ...overrides,
  }
}

describe('compareEnabled', () => {
  it('returns match when all environments have same enabled state', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true }),
      makeConfig({ environment_id: 'env-2', enabled: true }),
    ]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-1')).toBe(true)
    expect(result.values.get('env-2')).toBe(true)
  })

  it('returns differs when enabled states differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true }),
      makeConfig({ environment_id: 'env-2', enabled: false }),
    ]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('treats missing config as disabled', () => {
    const configs = [makeConfig({ environment_id: 'env-1', enabled: false })]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-2')).toBe(false)
  })

  it('returns match for single environment', () => {
    const configs = [makeConfig({ environment_id: 'env-1', enabled: true })]
    const result = compareEnabled(configs, ['env-1'])
    expect(result.status).toBe('match')
  })
})

describe('compareDefaultVariant', () => {
  it('returns match when all variants are the same', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', default_variant: 'on' }),
      makeConfig({ environment_id: 'env-2', default_variant: 'on' }),
    ]
    const result = compareDefaultVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-1')).toBe('on')
  })

  it('returns differs when variants differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', default_variant: 'on' }),
      makeConfig({ environment_id: 'env-2', default_variant: 'off' }),
    ]
    const result = compareDefaultVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('treats missing config as empty string', () => {
    const configs = [makeConfig({ environment_id: 'env-1', default_variant: '' })]
    const result = compareDefaultVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-2')).toBe('')
  })
})

describe('compareVariants', () => {
  it('returns match when all environments have identical variants', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', variants: [{ key: 'on', value: true }, { key: 'off', value: false }] }),
      makeConfig({ environment_id: 'env-2', variants: [{ key: 'on', value: true }, { key: 'off', value: false }] }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.perVariant.get('on')?.status).toBe('match')
    expect(result.perVariant.get('off')?.status).toBe('match')
  })

  it('returns differs when variant values differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', variants: [{ key: 'on', value: true }] }),
      makeConfig({ environment_id: 'env-2', variants: [{ key: 'on', value: false }] }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.perVariant.get('on')?.status).toBe('differs')
  })

  it('returns differs when variant sets differ (check missing variant)', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', variants: [{ key: 'on', value: true }, { key: 'extra', value: 'x' }] }),
      makeConfig({ environment_id: 'env-2', variants: [{ key: 'on', value: true }] }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.perVariant.get('extra')?.status).toBe('differs')
  })

  it('handles missing config as empty variants', () => {
    const configs = [makeConfig({ environment_id: 'env-1', variants: [] })]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.perVariant.size).toBe(0)
  })

  it('compares variant values independent of property order', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', variants: [{ key: 'v', value: { z: 1, a: 2 } }] }),
      makeConfig({ environment_id: 'env-2', variants: [{ key: 'v', value: { a: 2, z: 1 } }] }),
    ]
    const result = compareVariants(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.perVariant.get('v')?.status).toBe('match')
  })
})

describe('compareRules', () => {
  it('returns match when all environments have identical rules', () => {
    const rule = { conditions: [{ attribute: 'userId', operator: 'equals' as const, value: '123' }], variant: 'on' }
    const configs = [
      makeConfig({ environment_id: 'env-1', targeting_rules: [rule] }),
      makeConfig({ environment_id: 'env-2', targeting_rules: [rule] }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('returns differs when rule counts differ', () => {
    const rule = { conditions: [], variant: 'on' }
    const configs = [
      makeConfig({ environment_id: 'env-1', targeting_rules: [rule, rule] }),
      makeConfig({ environment_id: 'env-2', targeting_rules: [rule] }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('returns differs when rule content differs', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', targeting_rules: [{ conditions: [], variant: 'on' }] }),
      makeConfig({ environment_id: 'env-2', targeting_rules: [{ conditions: [], variant: 'off' }] }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('compares rules independent of condition property order', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', targeting_rules: [{ conditions: [{ attribute: 'x', operator: 'equals' as const, value: '1' }], variant: 'on' }] }),
      makeConfig({ environment_id: 'env-2', targeting_rules: [{ conditions: [{ value: '1', operator: 'equals' as const, attribute: 'x' }], variant: 'on' }] }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('treats missing config as no rules', () => {
    const configs = [makeConfig({ environment_id: 'env-1', targeting_rules: [] })]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-2')).toEqual([])
  })
})

describe('compareFlag', () => {
  it('returns a full ComparisonResult', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', enabled: true, default_variant: 'on', variants: [], targeting_rules: [] }),
      makeConfig({ environment_id: 'env-2', enabled: false, default_variant: 'on', variants: [], targeting_rules: [] }),
    ]
    const result = compareFlag(configs, ['env-1', 'env-2'])
    expect(result.enabled.status).toBe('differs')
    expect(result.defaultVariant.status).toBe('match')
    expect(result.variants.status).toBe('match')
    expect(result.rules.status).toBe('match')
  })

  it('handles empty configs array', () => {
    const result = compareFlag([], ['env-1', 'env-2'])
    expect(result.enabled.status).toBe('match')
    expect(result.defaultVariant.status).toBe('match')
    expect(result.variants.status).toBe('match')
    expect(result.rules.status).toBe('match')
  })
})
