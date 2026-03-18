package evaluation

import (
	"github.com/togglerino/togglerino/internal/model"
)

// EvaluateWithTrace mirrors EvaluateWithSegments but returns a detailed trace
// of every evaluation step for the playground feature.
func (e *Engine) EvaluateWithTrace(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext, segments map[string]model.Segment) *model.EvaluationTrace {
	trace := &model.EvaluationTrace{
		FlagKey:            flag.Key,
		FallthroughVariant: config.FallthroughVariant,
		SelectedStep:       -1,
	}

	// Step 1: Lifecycle check.
	lifecyclePassed := flag.LifecycleStatus != model.LifecycleArchived
	trace.Steps = append(trace.Steps, model.TraceStep{
		Type:   "lifecycle_check",
		Status: string(flag.LifecycleStatus),
		Passed: lifecyclePassed,
	})
	if !lifecyclePassed {
		trace.SelectedStep = 0
		trace.Value = rawToAny(flag.DefaultValue)
		trace.Reason = "archived"
		return trace
	}

	// Step 2: Enabled check.
	enabled := config.Enabled
	enabledPassed := enabled
	trace.Steps = append(trace.Steps, model.TraceStep{
		Type:    "enabled_check",
		Enabled: &enabled,
		Passed:  enabledPassed,
	})
	if !enabledPassed {
		trace.SelectedStep = 1
		trace.Value = lookupVariantValue(config.Variants, config.OffVariant, flag.DefaultValue)
		trace.Variant = config.OffVariant
		trace.Reason = "disabled"
		return trace
	}

	// Step 3: Evaluate targeting rules.
	matchedStepIndex := -1
	for i, rule := range config.TargetingRules {
		stepIndex := len(trace.Steps)

		// If a previous rule already matched, mark this one as skipped.
		if matchedStepIndex >= 0 {
			ruleIdx := i
			trace.Steps = append(trace.Steps, model.TraceStep{
				Type:      "rule",
				RuleIndex: &ruleIdx,
				Variant:   rule.Variant,
				Skipped:   true,
				Passed:    false,
			})
			continue
		}

		// Evaluate conditions with traces.
		condTraces := traceConditions(rule.Conditions, ctx, segments)

		// Check if all conditions passed.
		allPassed := true
		for _, ct := range condTraces {
			if !ct.Passed {
				allPassed = false
				break
			}
		}

		ruleIdx := i
		matched := false
		step := model.TraceStep{
			Type:       "rule",
			RuleIndex:  &ruleIdx,
			Variant:    rule.Variant,
			Conditions: condTraces,
		}

		if allPassed {
			// Conditions matched; check percentage rollout.
			if rule.PercentageRollout != nil {
				bucket := ConsistentHash(flag.Key, ctx.UserID)
				inRollout := bucket < *rule.PercentageRollout
				step.PercentageRollout = rule.PercentageRollout
				step.HashBucket = &bucket
				step.InRollout = &inRollout

				if inRollout {
					matched = true
				}
			} else {
				matched = true
			}
		}

		step.Matched = &matched
		step.Passed = matched
		trace.Steps = append(trace.Steps, step)

		if matched {
			matchedStepIndex = stepIndex
		}
	}

	// Determine final result.
	if matchedStepIndex >= 0 {
		trace.SelectedStep = matchedStepIndex
		trace.Reason = "rule_match"
		matchedRule := config.TargetingRules[*trace.Steps[matchedStepIndex].RuleIndex]
		trace.Variant = matchedRule.Variant
		trace.Value = lookupVariantValue(config.Variants, matchedRule.Variant, flag.DefaultValue)
	} else {
		trace.SelectedStep = -1
		trace.Reason = "default"
		trace.Variant = config.FallthroughVariant
		trace.Value = lookupVariantValue(config.Variants, config.FallthroughVariant, flag.DefaultValue)
	}

	return trace
}

// traceConditions evaluates each condition and returns detailed traces.
func traceConditions(conditions []model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) []model.ConditionTrace {
	traces := make([]model.ConditionTrace, 0, len(conditions))
	for _, cond := range conditions {
		if cond.Operator == string(model.OpSegmentMatch) {
			traces = append(traces, traceSegmentCondition(cond, ctx, segments))
			continue
		}

		attrValue := ctx.Attributes[cond.Attribute]
		passed := EvaluateCondition(attrValue, cond.Operator, cond.Value)
		traces = append(traces, model.ConditionTrace{
			Attribute:      cond.Attribute,
			Operator:       cond.Operator,
			ConditionValue: cond.Value,
			ActualValue:    attrValue,
			Passed:         passed,
		})
	}
	return traces
}

// traceSegmentCondition evaluates a segment_match condition and includes
// the individual segment condition traces.
func traceSegmentCondition(cond model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) model.ConditionTrace {
	ct := model.ConditionTrace{
		Attribute:      cond.Attribute,
		Operator:       cond.Operator,
		ConditionValue: cond.Value,
	}

	segKey, ok := cond.Value.(string)
	if !ok || segments == nil {
		ct.Passed = false
		return ct
	}

	seg, exists := segments[segKey]
	if !exists {
		ct.Passed = false
		return ct
	}

	// Evaluate segment conditions individually (no nested segments).
	segTraces := traceConditions(seg.Conditions, ctx, nil)
	ct.SegmentTrace = segTraces

	allPassed := true
	for _, st := range segTraces {
		if !st.Passed {
			allPassed = false
			break
		}
	}
	ct.Passed = allPassed
	return ct
}
