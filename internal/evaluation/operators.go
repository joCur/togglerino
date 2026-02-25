package evaluation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EvaluateCondition checks if an attribute value satisfies a condition.
func EvaluateCondition(attributeValue any, operator string, conditionValue any) bool {
	switch operator {
	case "equals":
		return toString(attributeValue) == toString(conditionValue)
	case "not_equals":
		return toString(attributeValue) != toString(conditionValue)
	case "contains":
		return evalContains(attributeValue, conditionValue)
	case "not_contains":
		return !evalContains(attributeValue, conditionValue)
	case "starts_with":
		return strings.HasPrefix(toString(attributeValue), toString(conditionValue))
	case "ends_with":
		return strings.HasSuffix(toString(attributeValue), toString(conditionValue))
	case "greater_than":
		a, b, ok := toFloat64Pair(attributeValue, conditionValue)
		return ok && a > b
	case "less_than":
		a, b, ok := toFloat64Pair(attributeValue, conditionValue)
		return ok && a < b
	case "gte":
		a, b, ok := toFloat64Pair(attributeValue, conditionValue)
		return ok && a >= b
	case "lte":
		a, b, ok := toFloat64Pair(attributeValue, conditionValue)
		return ok && a <= b
	case "in":
		return evalIn(attributeValue, conditionValue)
	case "not_in":
		return !evalIn(attributeValue, conditionValue)
	case "exists":
		return attributeValue != nil
	case "not_exists":
		return attributeValue == nil
	case "matches":
		pattern := toString(conditionValue)
		matched, err := regexp.MatchString(pattern, toString(attributeValue))
		return err == nil && matched
	default:
		return false
	}
}

// toString converts any value to its string representation.
func toString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// toFloat64Pair converts both values to float64.
func toFloat64Pair(a, b any) (float64, float64, bool) {
	fa, okA := toFloat64(a)
	fb, okB := toFloat64(b)
	return fa, fb, okA && okB
}

// evalContains checks if the attribute contains the condition value.
// For strings, it checks substring containment.
// For slices, it checks if the slice contains the value.
func evalContains(attributeValue, conditionValue any) bool {
	// Check if attributeValue is a slice.
	if slice, ok := toSlice(attributeValue); ok {
		target := toString(conditionValue)
		for _, item := range slice {
			if toString(item) == target {
				return true
			}
		}
		return false
	}
	// Default: string contains check.
	return strings.Contains(toString(attributeValue), toString(conditionValue))
}

// evalIn checks if the attribute value is in the condition list.
func evalIn(attributeValue, conditionValue any) bool {
	list, ok := toSlice(conditionValue)
	if !ok {
		return false
	}
	target := toString(attributeValue)
	for _, item := range list {
		if toString(item) == target {
			return true
		}
	}
	return false
}

// toSlice attempts to convert a value to []any.
func toSlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []string:
		result := make([]any, len(s))
		for i, item := range s {
			result[i] = item
		}
		return result, true
	default:
		return nil, false
	}
}
