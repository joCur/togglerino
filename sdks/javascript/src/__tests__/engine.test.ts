import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { evaluate } from '../engine'

interface TestCase {
  name: string
  flag: {
    key: string
    valueType: string
    status: string
    config: {
      enabled: boolean
      defaultVariant: string
      variants: { key: string; value: unknown }[]
      targetingRules: {
        variant: string
        percentage: number | null
        conditions: { attribute: string; operator: string; value: string }[]
      }[]
    }
  }
  segments: { key: string; conditions: { attribute: string; operator: string; value: string }[] }[]
  context: { userId: string; attributes: Record<string, unknown> }
  expected: { value: unknown; variant: string; reason: string }
}

const casesPath = resolve(__dirname, '../../../../testdata/evaluation_cases.json')
const cases: TestCase[] = JSON.parse(readFileSync(casesPath, 'utf-8'))

describe('evaluation engine (shared fixtures)', () => {
  for (const tc of cases) {
    it(tc.name, () => {
      const result = evaluate(tc.flag, tc.context, tc.segments)
      expect(result.value).toEqual(tc.expected.value)
      expect(result.variant).toBe(tc.expected.variant)
      expect(result.reason).toBe(tc.expected.reason)
    })
  }
})
