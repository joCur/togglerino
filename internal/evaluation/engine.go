package evaluation

import (
	"encoding/json"

	"github.com/togglerino/togglerino/internal/model"
)

// Engine evaluates feature flags for a given context.
type Engine struct{}

// NewEngine creates a new evaluation engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Evaluate evaluates a flag for a given context.
// Returns the evaluation result with value, variant key, and reason.
// This is a backward-compatible wrapper around EvaluateWithSegments with no segments.
func (e *Engine) Evaluate(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext) *model.EvaluationResult {
	return e.EvaluateWithSegments(flag, config, ctx, nil)
}

// EvaluateWithSegments evaluates a flag for a given context, resolving segment_match conditions
// against the provided segments map. Segments are keyed by segment key.
func (e *Engine) EvaluateWithSegments(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext, segments map[string]model.Segment) *model.EvaluationResult {
	// Boolean flags use a simplified evaluation path.
	if flag.ValueType == model.ValueTypeBoolean {
		return e.evaluateBoolean(flag, config, ctx, segments)
	}

	// 1. If flag is archived, return default value with reason "archived".
	if flag.LifecycleStatus == model.LifecycleArchived {
		return &model.EvaluationResult{
			Value:   rawToAny(flag.DefaultValue),
			Variant: "",
			Reason:  "archived",
		}
	}

	// 2. If config is disabled, return default value with reason "disabled".
	if !config.Enabled {
		return &model.EvaluationResult{
			Value:   rawToAny(flag.DefaultValue),
			Variant: "",
			Reason:  "disabled",
		}
	}

	// 3. Evaluate targeting rules in order.
	for _, rule := range config.TargetingRules {
		if matchesAllConditions(rule.Conditions, ctx, segments) {
			// Check percentage rollout.
			if rule.PercentageRollout != nil {
				bucket := ConsistentHash(flag.Key, ctx.UserID)
				if bucket >= *rule.PercentageRollout {
					// User is outside the rollout percentage; continue to next rule.
					continue
				}
			}
			// Rule matched.
			value := lookupVariantValue(config.Variants, rule.Variant, flag.DefaultValue)
			return &model.EvaluationResult{
				Value:   value,
				Variant: rule.Variant,
				Reason:  "rule_match",
			}
		}
	}

	// 4. Return default variant.
	value := lookupVariantValue(config.Variants, config.DefaultVariant, flag.DefaultValue)
	return &model.EvaluationResult{
		Value:   value,
		Variant: config.DefaultVariant,
		Reason:  "default",
	}
}

// matchesAllConditions checks if all conditions in a rule match the evaluation context.
// When a condition has operator "segment_match", the engine looks up the segment by key
// and evaluates its conditions. Passing nil for segments in the recursive call prevents
// nesting (segments cannot reference other segments).
func matchesAllConditions(conditions []model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) bool {
	for _, cond := range conditions {
		if cond.Operator == string(model.OpSegmentMatch) {
			segKey, ok := cond.Value.(string)
			if !ok {
				return false
			}
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
		attrValue := ctx.Attributes[cond.Attribute]
		if !EvaluateCondition(attrValue, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}

// lookupVariantValue finds the value for a variant key in the variants list.
// If the variant is not found, returns the flag's default value.
func lookupVariantValue(variants []model.Variant, variantKey string, defaultValue json.RawMessage) any {
	for _, v := range variants {
		if v.Key == variantKey {
			return rawToAny(v.Value)
		}
	}
	return rawToAny(defaultValue)
}

// evaluateBoolean handles the simplified evaluation path for boolean flags.
// For boolean flags: enabled = true, disabled = false, archived = false.
// Targeting rules use "true"/"false" strings as variant values.
func (e *Engine) evaluateBoolean(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext, segments map[string]model.Segment) *model.EvaluationResult {
	if flag.LifecycleStatus == model.LifecycleArchived {
		return &model.EvaluationResult{Value: false, Variant: "", Reason: "archived"}
	}

	if !config.Enabled {
		return &model.EvaluationResult{Value: false, Variant: "", Reason: "disabled"}
	}

	// Evaluate targeting rules.
	for _, rule := range config.TargetingRules {
		if matchesAllConditions(rule.Conditions, ctx, segments) {
			if rule.PercentageRollout != nil {
				bucket := ConsistentHash(flag.Key, ctx.UserID)
				if bucket >= *rule.PercentageRollout {
					continue
				}
			}
			return &model.EvaluationResult{
				Value:   rule.Variant == "true",
				Variant: "",
				Reason:  "rule_match",
			}
		}
	}

	// Default: enabled = true
	return &model.EvaluationResult{Value: true, Variant: "", Reason: "default"}
}

// rawToAny converts json.RawMessage to a Go value.
func rawToAny(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		// If unmarshaling fails, return the raw string.
		return string(raw)
	}
	return v
}
