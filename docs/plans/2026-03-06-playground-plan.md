# Flag Evaluation Playground Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a playground API + UI that evaluates flags with a test context and shows a step-by-step rule trace.

**Architecture:** New `EvaluateWithTrace` engine method returns detailed trace alongside result. New `PlaygroundHandler` (session-authed, `flags:read`) calls engine against cache. React page with form + trace visualization.

**Tech Stack:** Go (stdlib `net/http`, `encoding/json`), React 19, TypeScript, TanStack Query, shadcn/ui, Tailwind CSS v4.

---

### Task 1: Add trace model types

**Files:**
- Modify: `internal/model/flag.go` (append after `EvaluationResult` struct, ~line 138)

**Step 1: Add the trace types to the model package**

Append these types after `EvaluationResult` in `internal/model/flag.go`:

```go
// ConditionTrace records the evaluation of a single condition.
type ConditionTrace struct {
	Attribute      string           `json:"attribute"`
	Operator       string           `json:"operator"`
	ConditionValue any              `json:"condition_value"`
	ActualValue    any              `json:"actual_value,omitempty"`
	Passed         bool             `json:"passed"`
	SegmentTrace   []ConditionTrace `json:"segment_trace,omitempty"`
}

// TraceStep records one step in the evaluation process.
type TraceStep struct {
	Type              string           `json:"type"`                         // "lifecycle_check", "enabled_check", "rule"
	Passed            bool             `json:"passed"`                       // Did this step pass?
	Status            string           `json:"status,omitempty"`             // For lifecycle_check
	Enabled           *bool            `json:"enabled,omitempty"`            // For enabled_check
	RuleIndex         *int             `json:"rule_index,omitempty"`         // For rule steps
	Variant           string           `json:"variant,omitempty"`            // For rule steps
	PercentageRollout *int             `json:"percentage_rollout,omitempty"` // For rule steps with rollout
	HashBucket        *int             `json:"hash_bucket,omitempty"`        // For rule steps with rollout
	InRollout         *bool            `json:"in_rollout,omitempty"`         // For rule steps with rollout
	Matched           *bool            `json:"matched,omitempty"`            // For rule steps
	Skipped           bool             `json:"skipped,omitempty"`            // For rules after the match
	Conditions        []ConditionTrace `json:"conditions,omitempty"`         // For rule steps
}

// EvaluationTrace contains a detailed trace of the evaluation process.
type EvaluationTrace struct {
	FlagKey        string           `json:"flag_key"`
	Value          any              `json:"value"`
	Variant        string           `json:"variant"`
	Reason         string           `json:"reason"`
	Steps          []TraceStep      `json:"steps"`
	DefaultVariant string           `json:"default_variant"`
	SelectedStep   int              `json:"selected_step"`
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/model/...`
Expected: SUCCESS (no errors)

**Step 3: Commit**

```
feat: add evaluation trace model types (#34)
```

---

### Task 2: Implement EvaluateWithTrace engine method — tests first

**Files:**
- Create: `internal/evaluation/trace_test.go`
- Create: `internal/evaluation/trace.go`

**Step 1: Write failing tests**

Create `internal/evaluation/trace_test.go`:

```go
package evaluation

import (
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func boolPtr(b bool) *bool { return &b }

func TestEvaluateWithTrace_ArchivedFlag(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "default-val", model.LifecycleArchived)
	config := makeConfig(true, "on", []model.Variant{
		{Key: "on", Value: rawJSON("on-val")},
	}, nil)
	ctx := &model.EvaluationContext{UserID: "user-1", Attributes: map[string]any{}}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "archived" {
		t.Errorf("expected reason 'archived', got %q", trace.Reason)
	}
	if trace.Value != "default-val" {
		t.Errorf("expected value 'default-val', got %v", trace.Value)
	}
	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Type != "lifecycle_check" {
		t.Errorf("expected step type 'lifecycle_check', got %q", trace.Steps[0].Type)
	}
	if trace.Steps[0].Passed {
		t.Error("expected lifecycle_check to not pass for archived flag")
	}
	if trace.SelectedStep != 0 {
		t.Errorf("expected selected_step 0, got %d", trace.SelectedStep)
	}
}

func TestEvaluateWithTrace_DisabledFlag(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(false, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
	}, nil)
	ctx := &model.EvaluationContext{UserID: "user-1", Attributes: map[string]any{}}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "disabled" {
		t.Errorf("expected reason 'disabled', got %q", trace.Reason)
	}
	if len(trace.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Type != "lifecycle_check" || !trace.Steps[0].Passed {
		t.Error("expected lifecycle_check to pass")
	}
	if trace.Steps[1].Type != "enabled_check" || trace.Steps[1].Passed {
		t.Error("expected enabled_check to not pass")
	}
	if trace.SelectedStep != 1 {
		t.Errorf("expected selected_step 1, got %d", trace.SelectedStep)
	}
}

func TestEvaluateWithTrace_RuleMatch(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "country", Operator: "equals", Value: "US"},
			},
			Variant: "on",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"country": "US"},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
	}
	if trace.Variant != "on" {
		t.Errorf("expected variant 'on', got %q", trace.Variant)
	}
	// lifecycle_check + enabled_check + 1 rule = 3 steps
	if len(trace.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(trace.Steps))
	}
	ruleStep := trace.Steps[2]
	if ruleStep.Type != "rule" {
		t.Errorf("expected step type 'rule', got %q", ruleStep.Type)
	}
	if ruleStep.Matched == nil || !*ruleStep.Matched {
		t.Error("expected rule to match")
	}
	if len(ruleStep.Conditions) != 1 {
		t.Fatalf("expected 1 condition trace, got %d", len(ruleStep.Conditions))
	}
	cond := ruleStep.Conditions[0]
	if cond.Attribute != "country" {
		t.Errorf("expected attribute 'country', got %q", cond.Attribute)
	}
	if cond.ActualValue != "US" {
		t.Errorf("expected actual_value 'US', got %v", cond.ActualValue)
	}
	if !cond.Passed {
		t.Error("expected condition to pass")
	}
	if trace.SelectedStep != 2 {
		t.Errorf("expected selected_step 2, got %d", trace.SelectedStep)
	}
}

func TestEvaluateWithTrace_Default(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "country", Operator: "equals", Value: "US"},
			},
			Variant: "on",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"country": "UK"},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", trace.Reason)
	}
	if trace.Variant != "off" {
		t.Errorf("expected variant 'off', got %q", trace.Variant)
	}
	// The unmatched rule should still appear with matched=false
	ruleStep := trace.Steps[2]
	if ruleStep.Matched == nil || *ruleStep.Matched {
		t.Error("expected rule to not match")
	}
	if ruleStep.Conditions[0].Passed {
		t.Error("expected condition to fail")
	}
	// selected_step should be -1 for default (no step selected)
	if trace.SelectedStep != -1 {
		t.Errorf("expected selected_step -1, got %d", trace.SelectedStep)
	}
}

func TestEvaluateWithTrace_PercentageRollout(t *testing.T) {
	engine := NewEngine()
	// rollout-flag + user-xyz = bucket 28
	flag := makeFlag("rollout-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "country", Operator: "equals", Value: "US"},
			},
			Variant:           "on",
			PercentageRollout: intPtr(50),
		},
	})

	t.Run("in rollout", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID:     "user-xyz", // bucket 28
			Attributes: map[string]any{"country": "US"},
		}
		trace := engine.EvaluateWithTrace(flag, config, ctx, nil)
		ruleStep := trace.Steps[2]
		if ruleStep.PercentageRollout == nil || *ruleStep.PercentageRollout != 50 {
			t.Error("expected percentage_rollout 50")
		}
		if ruleStep.HashBucket == nil || *ruleStep.HashBucket != 28 {
			t.Errorf("expected hash_bucket 28, got %v", ruleStep.HashBucket)
		}
		if ruleStep.InRollout == nil || !*ruleStep.InRollout {
			t.Error("expected in_rollout true")
		}
		if trace.Reason != "rule_match" {
			t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
		}
	})

	t.Run("out of rollout", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID:     "user-abc", // bucket 89
			Attributes: map[string]any{"country": "US"},
		}
		trace := engine.EvaluateWithTrace(flag, config, ctx, nil)
		ruleStep := trace.Steps[2]
		if ruleStep.InRollout == nil || *ruleStep.InRollout {
			t.Error("expected in_rollout false")
		}
		if ruleStep.Matched == nil || *ruleStep.Matched {
			t.Error("expected matched false")
		}
		if trace.Reason != "default" {
			t.Errorf("expected reason 'default', got %q", trace.Reason)
		}
	})
}

func TestEvaluateWithTrace_SegmentMatch(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "", Operator: "segment_match", Value: "beta-users"},
			},
			Variant: "on",
		},
	})
	segments := map[string]model.Segment{
		"beta-users": {
			Key: "beta-users",
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
			},
		},
	}

	t.Run("segment matches", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID:     "user-1",
			Attributes: map[string]any{"plan": "enterprise"},
		}
		trace := engine.EvaluateWithTrace(flag, config, ctx, segments)
		ruleStep := trace.Steps[2]
		if len(ruleStep.Conditions) != 1 {
			t.Fatalf("expected 1 condition, got %d", len(ruleStep.Conditions))
		}
		cond := ruleStep.Conditions[0]
		if cond.Operator != "segment_match" {
			t.Errorf("expected operator 'segment_match', got %q", cond.Operator)
		}
		if !cond.Passed {
			t.Error("expected segment_match condition to pass")
		}
		if len(cond.SegmentTrace) != 1 {
			t.Fatalf("expected 1 segment trace condition, got %d", len(cond.SegmentTrace))
		}
		segCond := cond.SegmentTrace[0]
		if segCond.Attribute != "plan" {
			t.Errorf("expected segment condition attribute 'plan', got %q", segCond.Attribute)
		}
		if !segCond.Passed {
			t.Error("expected segment condition to pass")
		}
	})

	t.Run("segment does not match", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID:     "user-2",
			Attributes: map[string]any{"plan": "free"},
		}
		trace := engine.EvaluateWithTrace(flag, config, ctx, segments)
		ruleStep := trace.Steps[2]
		cond := ruleStep.Conditions[0]
		if cond.Passed {
			t.Error("expected segment_match condition to fail")
		}
		segCond := cond.SegmentTrace[0]
		if segCond.Passed {
			t.Error("expected segment condition to fail")
		}
		if segCond.ActualValue != "free" {
			t.Errorf("expected actual_value 'free', got %v", segCond.ActualValue)
		}
	})
}

func TestEvaluateWithTrace_SkippedRules(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "none", model.LifecycleActive)
	config := makeConfig(true, "default", []model.Variant{
		{Key: "default", Value: rawJSON("none")},
		{Key: "beta", Value: rawJSON("beta")},
		{Key: "vip", Value: rawJSON("vip")},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
			},
			Variant: "vip",
		},
		{
			Conditions: []model.Condition{
				{Attribute: "beta", Operator: "equals", Value: "true"},
			},
			Variant: "beta",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"plan": "enterprise", "beta": "true"},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	// First rule matches -> second rule should be skipped
	if len(trace.Steps) != 4 { // lifecycle + enabled + 2 rules
		t.Fatalf("expected 4 steps, got %d", len(trace.Steps))
	}
	if !*trace.Steps[2].Matched {
		t.Error("expected first rule to match")
	}
	if !trace.Steps[3].Skipped {
		t.Error("expected second rule to be skipped")
	}
	if len(trace.Steps[3].Conditions) != 0 {
		t.Error("expected skipped rule to have no condition traces")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/evaluation/... -run TestEvaluateWithTrace -v`
Expected: FAIL — `engine.EvaluateWithTrace` undefined

**Step 3: Implement EvaluateWithTrace**

Create `internal/evaluation/trace.go`:

```go
package evaluation

import (
	"encoding/json"

	"github.com/togglerino/togglerino/internal/model"
)

// EvaluateWithTrace evaluates a flag and returns a detailed trace of the evaluation process.
func (e *Engine) EvaluateWithTrace(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext, segments map[string]model.Segment) *model.EvaluationTrace {
	trace := &model.EvaluationTrace{
		FlagKey:        flag.Key,
		DefaultVariant: config.DefaultVariant,
		SelectedStep:   -1,
	}

	// 1. Lifecycle check
	lifecyclePassed := flag.LifecycleStatus != model.LifecycleArchived
	trace.Steps = append(trace.Steps, model.TraceStep{
		Type:   "lifecycle_check",
		Status: string(flag.LifecycleStatus),
		Passed: lifecyclePassed,
	})
	if !lifecyclePassed {
		trace.Value = rawToAny(flag.DefaultValue)
		trace.Reason = "archived"
		trace.SelectedStep = 0
		return trace
	}

	// 2. Enabled check
	enabledVal := config.Enabled
	trace.Steps = append(trace.Steps, model.TraceStep{
		Type:    "enabled_check",
		Enabled: &enabledVal,
		Passed:  config.Enabled,
	})
	if !config.Enabled {
		trace.Value = rawToAny(flag.DefaultValue)
		trace.Reason = "disabled"
		trace.SelectedStep = 1
		return trace
	}

	// 3. Targeting rules
	matched := false
	for i, rule := range config.TargetingRules {
		if matched {
			// Mark remaining rules as skipped
			ruleIdx := i
			trace.Steps = append(trace.Steps, model.TraceStep{
				Type:      "rule",
				RuleIndex: &ruleIdx,
				Variant:   rule.Variant,
				Skipped:   true,
			})
			continue
		}

		ruleIdx := i
		step := model.TraceStep{
			Type:      "rule",
			RuleIndex: &ruleIdx,
			Variant:   rule.Variant,
		}

		// Evaluate conditions with trace
		conditionTraces := traceConditions(rule.Conditions, ctx, segments)
		step.Conditions = conditionTraces

		allConditionsPass := true
		for _, ct := range conditionTraces {
			if !ct.Passed {
				allConditionsPass = false
				break
			}
		}

		if allConditionsPass {
			// Check percentage rollout
			if rule.PercentageRollout != nil {
				step.PercentageRollout = rule.PercentageRollout
				bucket := ConsistentHash(flag.Key, ctx.UserID)
				step.HashBucket = &bucket
				inRollout := bucket < *rule.PercentageRollout
				step.InRollout = &inRollout

				if !inRollout {
					matchedFalse := false
					step.Matched = &matchedFalse
					step.Passed = false
					trace.Steps = append(trace.Steps, step)
					continue
				}
			}

			matchedTrue := true
			step.Matched = &matchedTrue
			step.Passed = true
			trace.Steps = append(trace.Steps, step)

			value := lookupVariantValue(config.Variants, rule.Variant, flag.DefaultValue)
			trace.Value = value
			trace.Variant = rule.Variant
			trace.Reason = "rule_match"
			trace.SelectedStep = len(trace.Steps) - 1
			matched = true
		} else {
			matchedFalse := false
			step.Matched = &matchedFalse
			step.Passed = false
			trace.Steps = append(trace.Steps, step)
		}
	}

	if !matched {
		// 4. Default variant
		value := lookupVariantValue(config.Variants, config.DefaultVariant, flag.DefaultValue)
		trace.Value = value
		trace.Variant = config.DefaultVariant
		trace.Reason = "default"
	}

	return trace
}

// traceConditions evaluates each condition and returns detailed traces.
func traceConditions(conditions []model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) []model.ConditionTrace {
	traces := make([]model.ConditionTrace, 0, len(conditions))

	for _, cond := range conditions {
		if cond.Operator == string(model.OpSegmentMatch) {
			ct := traceSegmentCondition(cond, ctx, segments)
			traces = append(traces, ct)
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

// traceSegmentCondition evaluates a segment_match condition with detailed segment traces.
func traceSegmentCondition(cond model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) model.ConditionTrace {
	ct := model.ConditionTrace{
		Attribute:      cond.Attribute,
		Operator:       cond.Operator,
		ConditionValue: cond.Value,
	}

	segKey, ok := cond.Value.(string)
	if !ok {
		ct.Passed = false
		return ct
	}

	if segments == nil {
		ct.Passed = false
		return ct
	}

	seg, exists := segments[segKey]
	if !exists {
		ct.Passed = false
		return ct
	}

	// Evaluate segment conditions (nil segments to prevent nesting)
	segTraces := traceConditions(seg.Conditions, ctx, nil)
	ct.SegmentTrace = segTraces

	allPass := true
	for _, st := range segTraces {
		if !st.Passed {
			allPass = false
			break
		}
	}
	ct.Passed = allPass

	return ct
}

// rawToAnyTrace is identical to rawToAny but avoids duplication.
// We reuse rawToAny from engine.go (same package).
func init() {
	// rawToAny is available from engine.go
	_ = json.Marshal // ensure json import is used
}
```

Wait — `rawToAny` is already in `engine.go` in the same package, so we can call it directly. Remove the `init()` function and the `encoding/json` import:

```go
package evaluation

import (
	"github.com/togglerino/togglerino/internal/model"
)

// EvaluateWithTrace evaluates a flag and returns a detailed trace of the evaluation process.
func (e *Engine) EvaluateWithTrace(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext, segments map[string]model.Segment) *model.EvaluationTrace {
	trace := &model.EvaluationTrace{
		FlagKey:        flag.Key,
		DefaultVariant: config.DefaultVariant,
		SelectedStep:   -1,
	}

	// 1. Lifecycle check
	lifecyclePassed := flag.LifecycleStatus != model.LifecycleArchived
	trace.Steps = append(trace.Steps, model.TraceStep{
		Type:   "lifecycle_check",
		Status: string(flag.LifecycleStatus),
		Passed: lifecyclePassed,
	})
	if !lifecyclePassed {
		trace.Value = rawToAny(flag.DefaultValue)
		trace.Reason = "archived"
		trace.SelectedStep = 0
		return trace
	}

	// 2. Enabled check
	enabledVal := config.Enabled
	trace.Steps = append(trace.Steps, model.TraceStep{
		Type:    "enabled_check",
		Enabled: &enabledVal,
		Passed:  config.Enabled,
	})
	if !config.Enabled {
		trace.Value = rawToAny(flag.DefaultValue)
		trace.Reason = "disabled"
		trace.SelectedStep = 1
		return trace
	}

	// 3. Targeting rules
	matched := false
	for i, rule := range config.TargetingRules {
		if matched {
			ruleIdx := i
			trace.Steps = append(trace.Steps, model.TraceStep{
				Type:      "rule",
				RuleIndex: &ruleIdx,
				Variant:   rule.Variant,
				Skipped:   true,
			})
			continue
		}

		ruleIdx := i
		step := model.TraceStep{
			Type:      "rule",
			RuleIndex: &ruleIdx,
			Variant:   rule.Variant,
		}

		conditionTraces := traceConditions(rule.Conditions, ctx, segments)
		step.Conditions = conditionTraces

		allConditionsPass := true
		for _, ct := range conditionTraces {
			if !ct.Passed {
				allConditionsPass = false
				break
			}
		}

		if allConditionsPass {
			if rule.PercentageRollout != nil {
				step.PercentageRollout = rule.PercentageRollout
				bucket := ConsistentHash(flag.Key, ctx.UserID)
				step.HashBucket = &bucket
				inRollout := bucket < *rule.PercentageRollout
				step.InRollout = &inRollout

				if !inRollout {
					matchedFalse := false
					step.Matched = &matchedFalse
					step.Passed = false
					trace.Steps = append(trace.Steps, step)
					continue
				}
			}

			matchedTrue := true
			step.Matched = &matchedTrue
			step.Passed = true
			trace.Steps = append(trace.Steps, step)

			value := lookupVariantValue(config.Variants, rule.Variant, flag.DefaultValue)
			trace.Value = value
			trace.Variant = rule.Variant
			trace.Reason = "rule_match"
			trace.SelectedStep = len(trace.Steps) - 1
			matched = true
		} else {
			matchedFalse := false
			step.Matched = &matchedFalse
			step.Passed = false
			trace.Steps = append(trace.Steps, step)
		}
	}

	if !matched {
		value := lookupVariantValue(config.Variants, config.DefaultVariant, flag.DefaultValue)
		trace.Value = value
		trace.Variant = config.DefaultVariant
		trace.Reason = "default"
	}

	return trace
}

func traceConditions(conditions []model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) []model.ConditionTrace {
	traces := make([]model.ConditionTrace, 0, len(conditions))

	for _, cond := range conditions {
		if cond.Operator == string(model.OpSegmentMatch) {
			ct := traceSegmentCondition(cond, ctx, segments)
			traces = append(traces, ct)
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

func traceSegmentCondition(cond model.Condition, ctx *model.EvaluationContext, segments map[string]model.Segment) model.ConditionTrace {
	ct := model.ConditionTrace{
		Attribute:      cond.Attribute,
		Operator:       cond.Operator,
		ConditionValue: cond.Value,
	}

	segKey, ok := cond.Value.(string)
	if !ok {
		ct.Passed = false
		return ct
	}

	if segments == nil {
		ct.Passed = false
		return ct
	}

	seg, exists := segments[segKey]
	if !exists {
		ct.Passed = false
		return ct
	}

	segTraces := traceConditions(seg.Conditions, ctx, nil)
	ct.SegmentTrace = segTraces

	allPass := true
	for _, st := range segTraces {
		if !st.Passed {
			allPass = false
			break
		}
	}
	ct.Passed = allPass

	return ct
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/evaluation/... -run TestEvaluateWithTrace -v`
Expected: ALL PASS

**Step 5: Run all evaluation tests to check for regressions**

Run: `go test ./internal/evaluation/... -v`
Expected: ALL PASS

**Step 6: Commit**

```
feat: add EvaluateWithTrace engine method (#34)
```

---

### Task 3: Implement PlaygroundHandler — tests first

**Files:**
- Create: `internal/handler/playground_handler.go`
- Create: `internal/handler/playground_handler_test.go`

**Step 1: Write the handler**

Create `internal/handler/playground_handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

type PlaygroundHandler struct {
	cache  *evaluation.Cache
	engine *evaluation.Engine
}

func NewPlaygroundHandler(cache *evaluation.Cache, engine *evaluation.Engine) *PlaygroundHandler {
	return &PlaygroundHandler{cache: cache, engine: engine}
}

type playgroundRequest struct {
	EnvironmentKey string                   `json:"environment_key"`
	FlagKey        string                   `json:"flag_key,omitempty"`
	Context        *model.EvaluationContext `json:"context,omitempty"`
}

type playgroundResponse struct {
	Results []*model.EvaluationTrace `json:"results"`
}

func (h *PlaygroundHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")

	var req playgroundRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "environment_key is required")
		return
	}

	evalCtx := req.Context
	if evalCtx == nil {
		evalCtx = &model.EvaluationContext{
			UserID:     "",
			Attributes: map[string]any{},
		}
	}
	if evalCtx.Attributes == nil {
		evalCtx.Attributes = map[string]any{}
	}

	segments := h.cache.GetSegments(projectKey)

	if req.FlagKey != "" {
		fd, ok := h.cache.GetFlag(projectKey, req.EnvironmentKey, req.FlagKey)
		if !ok {
			writeError(w, http.StatusNotFound, "flag not found in this environment")
			return
		}
		trace := h.engine.EvaluateWithTrace(&fd.Flag, &fd.Config, evalCtx, segments)
		writeJSON(w, http.StatusOK, playgroundResponse{Results: []*model.EvaluationTrace{trace}})
		return
	}

	flags := h.cache.GetFlags(projectKey, req.EnvironmentKey)
	if flags == nil {
		writeJSON(w, http.StatusOK, playgroundResponse{Results: []*model.EvaluationTrace{}})
		return
	}

	results := make([]*model.EvaluationTrace, 0, len(flags))
	for _, fd := range flags {
		trace := h.engine.EvaluateWithTrace(&fd.Flag, &fd.Config, evalCtx, segments)
		results = append(results, trace)
	}

	writeJSON(w, http.StatusOK, playgroundResponse{Results: results})
}
```

**Step 2: Write handler tests**

Create `internal/handler/playground_handler_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

func setupPlaygroundTest() (*PlaygroundHandler, *evaluation.Cache) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	handler := NewPlaygroundHandler(cache, engine)
	return handler, cache
}

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestPlaygroundHandler_SingleFlag(t *testing.T) {
	h, cache := setupPlaygroundTest()

	cache.SetFlag("myproject", "production", "test-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "test-flag",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			DefaultVariant: "off",
			Variants: []model.Variant{
				{Key: "off", Value: rawJSON(false)},
				{Key: "on", Value: rawJSON(true)},
			},
			TargetingRules: []model.TargetingRule{
				{
					Conditions: []model.Condition{
						{Attribute: "country", Operator: "equals", Value: "US"},
					},
					Variant: "on",
				},
			},
		},
	})

	body := `{"environment_key":"production","flag_key":"test-flag","context":{"user_id":"u1","attributes":{"country":"US"}}}`
	req := httptest.NewRequest("POST", "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	w := httptest.NewRecorder()

	h.Evaluate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp playgroundResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", resp.Results[0].Reason)
	}
	if resp.Results[0].FlagKey != "test-flag" {
		t.Errorf("expected flag_key 'test-flag', got %q", resp.Results[0].FlagKey)
	}
}

func TestPlaygroundHandler_MissingEnvironmentKey(t *testing.T) {
	h, _ := setupPlaygroundTest()

	body := `{"flag_key":"test-flag"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	w := httptest.NewRecorder()

	h.Evaluate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPlaygroundHandler_FlagNotFound(t *testing.T) {
	h, _ := setupPlaygroundTest()

	body := `{"environment_key":"production","flag_key":"nonexistent"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	w := httptest.NewRecorder()

	h.Evaluate(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestPlaygroundHandler_AllFlags(t *testing.T) {
	h, cache := setupPlaygroundTest()

	cache.SetFlag("myproject", "production", "flag-a", evaluation.FlagData{
		Flag:   model.Flag{Key: "flag-a", DefaultValue: rawJSON(true), LifecycleStatus: model.LifecycleActive},
		Config: model.FlagEnvironmentConfig{Enabled: true, DefaultVariant: "on", Variants: []model.Variant{{Key: "on", Value: rawJSON(true)}}},
	})
	cache.SetFlag("myproject", "production", "flag-b", evaluation.FlagData{
		Flag:   model.Flag{Key: "flag-b", DefaultValue: rawJSON("hello"), LifecycleStatus: model.LifecycleActive},
		Config: model.FlagEnvironmentConfig{Enabled: true, DefaultVariant: "default", Variants: []model.Variant{{Key: "default", Value: rawJSON("hello")}}},
	})

	body := `{"environment_key":"production"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	w := httptest.NewRecorder()

	h.Evaluate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp playgroundResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
}
```

**Important:** The handler tests use `cache.SetFlag()`. This method may not exist yet. Check the cache API — if only `LoadAll`/`Refresh` exist (which load from DB), we need to add a `SetFlag` test helper. Looking at the existing test `TestCache_SegmentStorage` which uses `cache.SetSegments()`, a similar `SetFlag` for testing should exist or we add it.

**Step 3: Add SetFlag to cache if needed**

Check if `cache.SetFlag()` exists. If not, add it to `internal/evaluation/cache.go`:

```go
// SetFlag sets a single flag in the cache. Used for testing.
func (c *Cache) SetFlag(projectKey, envKey, flagKey string, data FlagData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cacheKey := projectKey + ":" + envKey
	if c.data[cacheKey] == nil {
		c.data[cacheKey] = make(map[string]FlagData)
	}
	c.data[cacheKey][flagKey] = data
}
```

**Step 4: Run handler tests**

Run: `go test ./internal/handler/... -run TestPlayground -v`
Expected: ALL PASS

**Step 5: Commit**

```
feat: add playground handler (#34)
```

---

### Task 4: Wire up playground route in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add handler initialization**

In `cmd/togglerino/main.go`, after `evaluateHandler` initialization (~line 124), add:

```go
playgroundHandler := handler.NewPlaygroundHandler(cache, engine)
```

**Step 2: Add route**

After the context-attributes route (~line 260), add:

```go
// Playground
mux.Handle("POST /api/v1/projects/{key}/playground", wrap(playgroundHandler.Evaluate, sessionAuth, requireFlagsRead))
```

**Step 3: Verify it compiles**

Run: `go build ./cmd/togglerino/...`
Expected: SUCCESS

**Step 4: Commit**

```
feat: wire playground route in main.go (#34)
```

---

### Task 5: Add frontend types and API client

**Files:**
- Modify: `web/src/api/types.ts` (append playground types)
- Modify: `web/src/api/client.ts` (add playground API method)

**Step 1: Add TypeScript types**

Append to `web/src/api/types.ts`:

```ts
// Playground types
export interface ConditionTrace {
  attribute: string
  operator: string
  condition_value: unknown
  actual_value?: unknown
  passed: boolean
  segment_trace?: ConditionTrace[]
}

export interface TraceStep {
  type: 'lifecycle_check' | 'enabled_check' | 'rule'
  passed: boolean
  status?: string
  enabled?: boolean
  rule_index?: number
  variant?: string
  percentage_rollout?: number
  hash_bucket?: number
  in_rollout?: boolean
  matched?: boolean
  skipped?: boolean
  conditions?: ConditionTrace[]
}

export interface EvaluationTrace {
  flag_key: string
  value: unknown
  variant: string
  reason: 'archived' | 'disabled' | 'rule_match' | 'default'
  steps: TraceStep[]
  default_variant: string
  selected_step: number
}

export interface PlaygroundRequest {
  environment_key: string
  flag_key?: string
  context?: {
    user_id: string
    attributes: Record<string, unknown>
  }
}

export interface PlaygroundResponse {
  results: EvaluationTrace[]
}
```

**Step 2: Add API client method**

In `web/src/api/client.ts`, add a `playground` section to the `api` object:

```ts
playground: {
  evaluate: (projectKey: string, body: PlaygroundRequest) =>
    request<PlaygroundResponse>(`/projects/${projectKey}/playground`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
},
```

And add `PlaygroundRequest, PlaygroundResponse` to the import from `./types`.

**Step 3: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`
Expected: SUCCESS

**Step 4: Commit**

```
feat: add playground API types and client (#34)
```

---

### Task 6: Create PlaygroundPage with form

**Files:**
- Create: `web/src/pages/PlaygroundPage.tsx`
- Modify: `web/src/App.tsx` (add route)
- Modify: `web/src/components/ProjectLayout.tsx` (add nav link)

**Step 1: Create the playground page**

Create `web/src/pages/PlaygroundPage.tsx` with:
- Environment selector dropdown (fetches from `/projects/{key}/environments`)
- Optional flag key input (combobox searching `/projects/{key}/flags`)
- User ID text input
- Dynamic key-value attribute editor (add/remove rows)
- "Evaluate" button that calls `api.playground.evaluate()`
- URL query param sync for shareability (`?env=`, `&flag=`, `&uid=`, `&attr.key=value`)
- Results section showing cards per flag with value, variant, reason badge
- Click to expand → renders `PlaygroundTrace` component

This is the largest frontend task. Key implementation details:
- Use `useSearchParams` from react-router to read/write query params
- Use `useMutation` from TanStack Query for the evaluate call
- Auto-evaluate on page load if query params are present
- Environment list via `useQuery(['environments', key])`
- Flag list for combobox via `useQuery(['flags', key])` (reuse existing)

**Step 2: Add route in App.tsx**

Inside the `<Route path="/projects/:key" element={<ProjectLayout />}>` block, add:

```tsx
<Route path="playground" element={<PlaygroundPage />} />
```

And import `PlaygroundPage` at the top.

**Step 3: Add nav link in ProjectLayout.tsx**

In the `navLinks` function in `ProjectLayout.tsx`, add after the "Segments" link:

```tsx
<NavLink to={`/projects/${key}/playground`} className={navLinkClass} onClick={onNavigate}>Playground</NavLink>
```

**Step 4: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`
Expected: SUCCESS

**Step 5: Commit**

```
feat: add playground page with evaluation form (#34)
```

---

### Task 7: Create PlaygroundTrace visualization component

**Files:**
- Create: `web/src/components/PlaygroundTrace.tsx`

**Step 1: Create the trace component**

Create `web/src/components/PlaygroundTrace.tsx`:
- Vertical stepper timeline with each `TraceStep`
- Lifecycle check: shows status with pass/fail icon
- Enabled check: shows enabled state with pass/fail icon
- Rule steps: shows rule index, variant, condition details
- Winning step highlighted with amber accent
- Skipped steps grayed out
- Each condition as a row: attribute | operator | expected | actual | pass/fail
- `segment_match` conditions expand to show nested segment condition traces (indented)
- Percentage rollout: show a small progress bar with bucket position and rollout threshold
- Use shadcn `Collapsible` for expandable rule details
- Use shadcn `Badge` for reason, variant, pass/fail indicators
- Use shadcn `Table` for condition rows

**Step 2: Wire it into PlaygroundPage**

In `PlaygroundPage.tsx`, render `<PlaygroundTrace trace={result} />` in the expanded state of each result card.

**Step 3: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`
Expected: SUCCESS

**Step 4: Commit**

```
feat: add playground trace visualization (#34)
```

---

### Task 8: Add "Test this flag" link on flag detail page

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Add the link**

In `FlagDetailPage.tsx`, add a "Test in Playground" link/button near the flag header that navigates to `/projects/${key}/playground?flag=${flag.key}`.

Use a `Link` from react-router with a small beaker/play icon. Position it near other flag actions.

**Step 2: Verify frontend builds**

Run: `cd web && npx tsc --noEmit`
Expected: SUCCESS

**Step 3: Commit**

```
feat: add "Test in Playground" link to flag detail page (#34)
```

---

### Task 9: Run full test suite and lint

**Files:** None (verification only)

**Step 1: Run Go tests**

Run: `go test ./...`
Expected: ALL PASS

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 3: Run frontend type check**

Run: `cd web && npx tsc --noEmit`
Expected: No errors

**Step 4: Build the full binary**

Run: `cd web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: SUCCESS

---

### Task 10: Final commit and cleanup

**Step 1: Review all changes**

Run: `git diff main --stat`
Verify only expected files changed.

**Step 2: Squash or clean up commits if needed**

Ensure all commits reference `(#34)`.
