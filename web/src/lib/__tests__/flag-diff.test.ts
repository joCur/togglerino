import { describe, it, expect } from 'vitest'
import {
  canonicalize,
  compareEnabled,
  compareFallthroughVariant,
  compareOffVariant,
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
    fallthrough_variant: 'off',
    off_variant: 'off',
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

  it('returns differs when enabled config is true and other env is missing', () => {
    const configs = [makeConfig({ environment_id: 'env-1', enabled: true })]
    const result = compareEnabled(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
    expect(result.values.get('env-1')).toBe(true)
    expect(result.values.get('env-2')).toBe(false)
  })

  it('returns match for single environment', () => {
    const configs = [makeConfig({ environment_id: 'env-1', enabled: true })]
    const result = compareEnabled(configs, ['env-1'])
    expect(result.status).toBe('match')
  })
})

describe('compareFallthroughVariant', () => {
  it('returns match when all variants are the same', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', fallthrough_variant: 'on' }),
      makeConfig({ environment_id: 'env-2', fallthrough_variant: 'on' }),
    ]
    const result = compareFallthroughVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-1')).toBe('on')
  })

  it('returns differs when variants differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', fallthrough_variant: 'on' }),
      makeConfig({ environment_id: 'env-2', fallthrough_variant: 'off' }),
    ]
    const result = compareFallthroughVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
  })

  it('treats missing config as empty string', () => {
    const configs = [makeConfig({ environment_id: 'env-1', fallthrough_variant: '' })]
    const result = compareFallthroughVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
    expect(result.values.get('env-2')).toBe('')
  })
})

describe('compareOffVariant', () => {
  it('returns match when all off variants are the same', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', off_variant: 'off' }),
      makeConfig({ environment_id: 'env-2', off_variant: 'off' }),
    ]
    const result = compareOffVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('returns differs when off variants differ', () => {
    const configs = [
      makeConfig({ environment_id: 'env-1', off_variant: 'off' }),
      makeConfig({ environment_id: 'env-2', off_variant: 'on' }),
    ]
    const result = compareOffVariant(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('differs')
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

  it('compares rules independent of condition array order', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        targeting_rules: [{
          conditions: [
            { attribute: 'country', operator: 'equals', value: 'US' },
            { attribute: 'plan', operator: 'equals', value: 'pro' },
          ],
          variant: 'on',
        }],
      }),
      makeConfig({
        environment_id: 'env-2',
        targeting_rules: [{
          conditions: [
            { attribute: 'plan', operator: 'equals', value: 'pro' },
            { attribute: 'country', operator: 'equals', value: 'US' },
          ],
          variant: 'on',
        }],
      }),
    ]
    const result = compareRules(configs, ['env-1', 'env-2'])
    expect(result.status).toBe('match')
  })

  it('compares rules independent of condition order with same attribute but different operators', () => {
    const configs = [
      makeConfig({
        environment_id: 'env-1',
        targeting_rules: [{
          conditions: [
            { attribute: 'age', operator: 'gte', value: '18' },
            { attribute: 'age', operator: 'lte', value: '65' },
          ],
          variant: 'on',
        }],
      }),
      makeConfig({
        environment_id: 'env-2',
        targeting_rules: [{
          conditions: [
            { attribute: 'age', operator: 'lte', value: '65' },
            { attribute: 'age', operator: 'gte', value: '18' },
          ],
          variant: 'on',
        }],
      }),
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
      makeConfig({ environment_id: 'env-1', enabled: true, fallthrough_variant: 'on', targeting_rules: [] }),
      makeConfig({ environment_id: 'env-2', enabled: false, fallthrough_variant: 'on', targeting_rules: [] }),
    ]
    const result = compareFlag(configs, ['env-1', 'env-2'])
    expect(result.enabled.status).toBe('differs')
    expect(result.fallthroughVariant.status).toBe('match')
    expect(result.offVariant.status).toBe('match')
    expect(result.rules.status).toBe('match')
  })

  it('handles empty configs array', () => {
    const result = compareFlag([], ['env-1', 'env-2'])
    expect(result.enabled.status).toBe('match')
    expect(result.fallthroughVariant.status).toBe('match')
    expect(result.offVariant.status).toBe('match')
    expect(result.rules.status).toBe('match')
  })
})
