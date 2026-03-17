/**
 * Condition operators for flag evaluation.
 * Ports all 16 operators from the Go backend (internal/evaluation/operators.go).
 */

/**
 * Evaluates whether an attribute value satisfies a condition.
 */
export function evaluateCondition(attributeValue: unknown, operator: string, conditionValue: unknown): boolean {
  switch (operator) {
    case 'equals':
      return toString(attributeValue) === toString(conditionValue)
    case 'not_equals':
      return toString(attributeValue) !== toString(conditionValue)
    case 'contains':
      return evalContains(attributeValue, conditionValue)
    case 'not_contains':
      return !evalContains(attributeValue, conditionValue)
    case 'starts_with':
      return toString(attributeValue).startsWith(toString(conditionValue))
    case 'ends_with':
      return toString(attributeValue).endsWith(toString(conditionValue))
    case 'greater_than': {
      const pair = toFloat64Pair(attributeValue, conditionValue)
      return pair !== null && pair[0] > pair[1]
    }
    case 'less_than': {
      const pair = toFloat64Pair(attributeValue, conditionValue)
      return pair !== null && pair[0] < pair[1]
    }
    case 'gte': {
      const pair = toFloat64Pair(attributeValue, conditionValue)
      return pair !== null && pair[0] >= pair[1]
    }
    case 'lte': {
      const pair = toFloat64Pair(attributeValue, conditionValue)
      return pair !== null && pair[0] <= pair[1]
    }
    case 'in':
      return evalIn(attributeValue, conditionValue)
    case 'not_in':
      return !evalIn(attributeValue, conditionValue)
    case 'exists':
      return attributeValue != null
    case 'not_exists':
      return attributeValue == null
    case 'matches': {
      const pattern = toString(conditionValue)
      try {
        return new RegExp(pattern).test(toString(attributeValue))
      } catch {
        return false
      }
    }
    default:
      return false
  }
}

/**
 * Converts any value to its string representation.
 * Matches Go's fmt.Sprintf("%v", v) behavior for common types.
 */
function toString(v: unknown): string {
  if (v == null) return ''
  return String(v)
}

/**
 * Attempts to convert a value to a number (float64 equivalent).
 * Returns null if conversion fails.
 */
function toFloat64(v: unknown): number | null {
  if (typeof v === 'number') {
    return isNaN(v) ? null : v
  }
  if (typeof v === 'string') {
    const n = Number(v)
    return isNaN(n) ? null : n
  }
  return null
}

/**
 * Converts both values to numbers. Returns null if either conversion fails.
 */
function toFloat64Pair(a: unknown, b: unknown): [number, number] | null {
  const fa = toFloat64(a)
  const fb = toFloat64(b)
  if (fa === null || fb === null) return null
  return [fa, fb]
}

/**
 * Checks if the attribute contains the condition value.
 * For arrays: checks if the array contains the value (string comparison).
 * For strings: checks substring containment.
 */
function evalContains(attributeValue: unknown, conditionValue: unknown): boolean {
  if (Array.isArray(attributeValue)) {
    const target = toString(conditionValue)
    for (const item of attributeValue) {
      if (toString(item) === target) return true
    }
    return false
  }
  return toString(attributeValue).includes(toString(conditionValue))
}

/**
 * Checks if the attribute value is in the condition list.
 * The condition value should be an array (for SDK-side evaluation, the API
 * stores in/not_in values as JSON-encoded string arrays that are parsed before
 * reaching this function).
 */
function evalIn(attributeValue: unknown, conditionValue: unknown): boolean {
  if (!Array.isArray(conditionValue)) return false
  const target = toString(attributeValue)
  for (const item of conditionValue) {
    if (toString(item) === target) return true
  }
  return false
}
