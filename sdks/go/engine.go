package togglerino

import "encoding/json"

// evaluateFlag evaluates a flag for a given context with segment support.
// This is the SDK-side local evaluation engine, producing results identical
// to the Go backend engine (internal/evaluation/engine.go).
func evaluateFlag(flag FlagDefinition, ctx EvaluationContext, segments []SegmentDefinition) EvaluationResult {
	segMap := buildSegmentMap(segments)

	// Boolean flags use a simplified evaluation path.
	if flag.ValueType == "boolean" {
		return evaluateBooleanFlag(flag, ctx, segMap)
	}

	config := flag.Config

	// 1. If flag is archived, return default value with reason "archived".
	if flag.Status == "archived" {
		return EvaluationResult{
			Value:   findVariantValue(config.Variants, config.DefaultVariant),
			Variant: "",
			Reason:  "archived",
		}
	}

	// 2. If config is disabled, return default value with reason "disabled".
	if !config.Enabled {
		return EvaluationResult{
			Value:   findVariantValue(config.Variants, config.DefaultVariant),
			Variant: "",
			Reason:  "disabled",
		}
	}

	// 3. Evaluate targeting rules in order (first match wins).
	for _, rule := range config.TargetingRules {
		if matchesAllConditions(rule.Conditions, ctx, segMap) {
			// Check percentage rollout.
			if rule.Percentage != nil {
				bucket := consistentHash(flag.Key, ctx.UserID)
				if bucket >= *rule.Percentage {
					// User is outside the rollout percentage; continue to next rule.
					continue
				}
			}
			// Rule matched — find the variant value.
			value := findVariantValue(config.Variants, rule.Variant)
			return EvaluationResult{
				Value:   value,
				Variant: rule.Variant,
				Reason:  "rule_match",
			}
		}
	}

	// 4. Return default variant.
	value := findVariantValue(config.Variants, config.DefaultVariant)
	return EvaluationResult{
		Value:   value,
		Variant: config.DefaultVariant,
		Reason:  "default",
	}
}

// evaluateBooleanFlag handles the simplified evaluation path for boolean flags.
// For boolean flags: enabled = true, disabled = false, archived = false.
// Targeting rules use "true"/"false" strings as variant keys.
func evaluateBooleanFlag(flag FlagDefinition, ctx EvaluationContext, segments map[string]SegmentDefinition) EvaluationResult {
	if flag.Status == "archived" {
		return EvaluationResult{Value: false, Variant: "", Reason: "archived"}
	}

	if !flag.Config.Enabled {
		return EvaluationResult{Value: false, Variant: "", Reason: "disabled"}
	}

	// Evaluate targeting rules.
	for _, rule := range flag.Config.TargetingRules {
		if matchesAllConditions(rule.Conditions, ctx, segments) {
			if rule.Percentage != nil {
				bucket := consistentHash(flag.Key, ctx.UserID)
				if bucket >= *rule.Percentage {
					continue
				}
			}
			return EvaluationResult{
				Value:   rule.Variant == "true",
				Variant: "",
				Reason:  "rule_match",
			}
		}
	}

	// Default: enabled = true.
	return EvaluationResult{Value: true, Variant: "", Reason: "default"}
}

// matchesAllConditions checks if all conditions in a rule match the evaluation context.
// When a condition has operator "segment_match", the engine looks up the segment by key
// and evaluates its conditions. Passing nil for segments in the recursive call prevents
// nesting (segments cannot reference other segments).
func matchesAllConditions(conditions []ConditionDefinition, ctx EvaluationContext, segments map[string]SegmentDefinition) bool {
	for _, cond := range conditions {
		if cond.Operator == "segment_match" {
			segKey := cond.Value
			if segments == nil {
				return false
			}
			seg, exists := segments[segKey]
			if !exists {
				return false
			}
			// Evaluate segment conditions (pass nil for segments to prevent nesting).
			if !matchesAllConditions(seg.Conditions, ctx, nil) {
				return false
			}
			continue
		}

		attrValue := getContextValue(cond.Attribute, ctx)
		if !evaluateCondition(cond, attrValue) {
			return false
		}
	}
	return true
}

// getContextValue retrieves a value from the evaluation context.
// Maps "user_id" to ctx.UserID; other attributes come from ctx.Attributes.
func getContextValue(attribute string, ctx EvaluationContext) any {
	if attribute == "user_id" {
		return ctx.UserID
	}
	if ctx.Attributes == nil {
		return nil
	}
	v, ok := ctx.Attributes[attribute]
	if !ok {
		return nil
	}
	return v
}

// findVariantValue finds the value for a variant key in the variants list.
// Returns the deserialized value, or nil if not found.
func findVariantValue(variants []VariantDefinition, key string) any {
	for _, v := range variants {
		if v.Key == key {
			return rawToAny(v.Value)
		}
	}
	return nil
}

// rawToAny converts json.RawMessage to a Go value.
func rawToAny(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}

// buildSegmentMap builds a map from segment key to segment definition for efficient lookup.
func buildSegmentMap(segments []SegmentDefinition) map[string]SegmentDefinition {
	m := make(map[string]SegmentDefinition, len(segments))
	for _, s := range segments {
		m[s.Key] = s
	}
	return m
}
