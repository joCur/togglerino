package togglerino

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// evaluateCondition checks if a context value satisfies a condition.
// The condition value is always a string (the definitions API serializes all values).
// For in/not_in operators, the value is a JSON-encoded array string like ["US","CA"].
func evaluateCondition(cond ConditionDefinition, contextValue any) bool {
	switch cond.Operator {
	case "equals":
		return toString(contextValue) == cond.Value
	case "not_equals":
		return toString(contextValue) != cond.Value
	case "contains":
		return evalContains(contextValue, cond.Value)
	case "not_contains":
		return !evalContains(contextValue, cond.Value)
	case "starts_with":
		return strings.HasPrefix(toString(contextValue), cond.Value)
	case "ends_with":
		return strings.HasSuffix(toString(contextValue), cond.Value)
	case "greater_than":
		a, b, ok := toFloat64Pair(contextValue, cond.Value)
		return ok && a > b
	case "less_than":
		a, b, ok := toFloat64Pair(contextValue, cond.Value)
		return ok && a < b
	case "gte":
		a, b, ok := toFloat64Pair(contextValue, cond.Value)
		return ok && a >= b
	case "lte":
		a, b, ok := toFloat64Pair(contextValue, cond.Value)
		return ok && a <= b
	case "in":
		return evalIn(contextValue, cond.Value)
	case "not_in":
		return !evalIn(contextValue, cond.Value)
	case "exists":
		return contextValue != nil
	case "not_exists":
		return contextValue == nil
	case "matches":
		matched, err := regexp.MatchString(cond.Value, toString(contextValue))
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
func evalContains(attributeValue any, conditionValue string) bool {
	if slice, ok := toSlice(attributeValue); ok {
		for _, item := range slice {
			if toString(item) == conditionValue {
				return true
			}
		}
		return false
	}
	return strings.Contains(toString(attributeValue), conditionValue)
}

// evalIn checks if the attribute value is in the condition list.
// The condition value is a JSON-encoded array string like ["US","CA"].
func evalIn(attributeValue any, conditionValue string) bool {
	var list []any
	if err := json.Unmarshal([]byte(conditionValue), &list); err != nil {
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
