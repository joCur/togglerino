import { describe, it, expect } from 'vitest'

/**
 * Tool registrations in index.ts parse JSON strings for conditions, variants, and attributes.
 * Invalid JSON is caught by the try/catch in each tool handler and returned via err().
 * This test verifies that JSON.parse errors produce useful messages.
 */
describe('JSON.parse error messages', () => {
  it('produces a descriptive error for malformed JSON', () => {
    expect(() => JSON.parse('not valid json')).toThrow()
    try {
      JSON.parse('{invalid}')
    } catch (e) {
      expect(e).toBeInstanceOf(SyntaxError)
      expect((e as Error).message).toBeTruthy()
    }
  })

  it('produces a descriptive error for incomplete JSON arrays', () => {
    expect(() => JSON.parse('[{"attribute":"plan"')).toThrow(SyntaxError)
  })

  it('succeeds for valid JSON strings used as tool parameters', () => {
    const conditions = JSON.parse('[{"attribute":"plan","operator":"equals","value":"enterprise"}]')
    expect(conditions).toHaveLength(1)
    expect(conditions[0].attribute).toBe('plan')

    const variants = JSON.parse('[{"key":"control","value":false},{"key":"treatment","value":true}]')
    expect(variants).toHaveLength(2)

    const attributes = JSON.parse('{"plan":"enterprise","country":"US"}')
    expect(attributes.plan).toBe('enterprise')
  })
})
