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
func (e *Engine) Evaluate(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext) *model.EvaluationResult {
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
		if matchesAllConditions(rule.Conditions, ctx) {
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
func matchesAllConditions(conditions []model.Condition, ctx *model.EvaluationContext) bool {
	for _, cond := range conditions {
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
