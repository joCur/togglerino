package togglerino

import "encoding/json"

// evaluateFlag evaluates a flag for a given context with segment support.
// This is the SDK-side local evaluation engine, producing results identical
// to the Go backend engine (internal/evaluation/engine.go).
func evaluateFlag(flag FlagDefinition, ctx EvaluationContext, segments []SegmentDefinition) EvaluationResult {
	segMap := buildSegmentMap(segments)

	config := flag.Config

	// 1. If flag is archived, return the fallthrough variant value with reason "archived".
	if flag.Status == "archived" {
		return EvaluationResult{
			Value:   findVariantValue(config.Variants, config.FallthroughVariant, flag.DefaultValue),
			Variant: "",
			Reason:  "archived",
		}
	}

	// 2. If config is disabled, return off variant value with reason "disabled".
	if !config.Enabled {
		return EvaluationResult{
			Value:   findVariantValue(config.Variants, config.OffVariant, flag.DefaultValue),
			Variant: config.OffVariant,
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
			value := findVariantValue(config.Variants, rule.Variant, flag.DefaultValue)
			return EvaluationResult{
				Value:   value,
				Variant: rule.Variant,
				Reason:  "rule_match",
			}
		}
	}

	// 4. Return fallthrough variant.
	value := findVariantValue(config.Variants, config.FallthroughVariant, flag.DefaultValue)
	return EvaluationResult{
		Value:   value,
		Variant: config.FallthroughVariant,
		Reason:  "default",
	}
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

// findVariantValue finds the value for a variant name in the variants list.
// Returns the deserialized value, or defaultValue if not found.
func findVariantValue(variants []VariantDefinition, name string, defaultValue interface{}) any {
	for _, v := range variants {
		if v.Name == name {
			return rawToAny(v.Value)
		}
	}
	return defaultValue
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
