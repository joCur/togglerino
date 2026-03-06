package evaluation

import (
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestEvaluateWithTrace_ArchivedFlag(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("archived-flag", "default-val", model.LifecycleArchived)
	config := makeConfig(true, "on", []model.Variant{
		{Key: "on", Value: rawJSON("on-val")},
	}, nil)
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.FlagKey != "archived-flag" {
		t.Errorf("expected flag_key 'archived-flag', got %q", trace.FlagKey)
	}
	if trace.Reason != "archived" {
		t.Errorf("expected reason 'archived', got %q", trace.Reason)
	}
	if trace.Value != "default-val" {
		t.Errorf("expected value 'default-val', got %v", trace.Value)
	}
	if trace.SelectedStep != 0 {
		t.Errorf("expected selected_step 0, got %d", trace.SelectedStep)
	}
	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(trace.Steps))
	}
	step := trace.Steps[0]
	if step.Type != "lifecycle_check" {
		t.Errorf("expected step type 'lifecycle_check', got %q", step.Type)
	}
	if step.Status != string(model.LifecycleArchived) {
		t.Errorf("expected status 'archived', got %q", step.Status)
	}
	if step.Passed {
		t.Error("expected passed=false for archived flag")
	}
}

func TestEvaluateWithTrace_DisabledFlag(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("disabled-flag", false, model.LifecycleActive)
	config := makeConfig(false, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
	}, nil)
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "disabled" {
		t.Errorf("expected reason 'disabled', got %q", trace.Reason)
	}
	if trace.Value != false {
		t.Errorf("expected value false, got %v", trace.Value)
	}
	if trace.SelectedStep != 1 {
		t.Errorf("expected selected_step 1, got %d", trace.SelectedStep)
	}
	if len(trace.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(trace.Steps))
	}

	// Step 0: lifecycle_check should pass (not archived).
	if trace.Steps[0].Type != "lifecycle_check" {
		t.Errorf("expected step 0 type 'lifecycle_check', got %q", trace.Steps[0].Type)
	}
	if !trace.Steps[0].Passed {
		t.Error("expected lifecycle_check passed=true for active flag")
	}

	// Step 1: enabled_check should not pass.
	if trace.Steps[1].Type != "enabled_check" {
		t.Errorf("expected step 1 type 'enabled_check', got %q", trace.Steps[1].Type)
	}
	if trace.Steps[1].Enabled == nil || *trace.Steps[1].Enabled != false {
		t.Error("expected enabled=false")
	}
	if trace.Steps[1].Passed {
		t.Error("expected passed=false for disabled flag")
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
		UserID: "user-1",
		Attributes: map[string]any{
			"country": "US",
		},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
	}
	if trace.Value != true {
		t.Errorf("expected value true, got %v", trace.Value)
	}
	if trace.Variant != "on" {
		t.Errorf("expected variant 'on', got %q", trace.Variant)
	}
	if trace.SelectedStep != 2 {
		t.Errorf("expected selected_step 2, got %d", trace.SelectedStep)
	}
	// 3 steps: lifecycle_check, enabled_check, rule
	if len(trace.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(trace.Steps))
	}

	ruleStep := trace.Steps[2]
	if ruleStep.Type != "rule" {
		t.Errorf("expected step type 'rule', got %q", ruleStep.Type)
	}
	if ruleStep.RuleIndex == nil || *ruleStep.RuleIndex != 0 {
		t.Errorf("expected rule_index 0, got %v", ruleStep.RuleIndex)
	}
	if ruleStep.Matched == nil || !*ruleStep.Matched {
		t.Error("expected matched=true")
	}
	if ruleStep.Variant != "on" {
		t.Errorf("expected variant 'on', got %q", ruleStep.Variant)
	}

	// Verify condition traces.
	if len(ruleStep.Conditions) != 1 {
		t.Fatalf("expected 1 condition trace, got %d", len(ruleStep.Conditions))
	}
	cond := ruleStep.Conditions[0]
	if cond.Attribute != "country" {
		t.Errorf("expected attribute 'country', got %q", cond.Attribute)
	}
	if cond.Operator != "equals" {
		t.Errorf("expected operator 'equals', got %q", cond.Operator)
	}
	if cond.ConditionValue != "US" {
		t.Errorf("expected condition_value 'US', got %v", cond.ConditionValue)
	}
	if cond.ActualValue != "US" {
		t.Errorf("expected actual_value 'US', got %v", cond.ActualValue)
	}
	if !cond.Passed {
		t.Error("expected condition passed=true")
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
		UserID: "user-1",
		Attributes: map[string]any{
			"country": "UK",
		},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", trace.Reason)
	}
	if trace.Value != false {
		t.Errorf("expected value false, got %v", trace.Value)
	}
	if trace.Variant != "off" {
		t.Errorf("expected variant 'off', got %q", trace.Variant)
	}
	if trace.SelectedStep != -1 {
		t.Errorf("expected selected_step -1, got %d", trace.SelectedStep)
	}

	// Rule step should show condition failed.
	if len(trace.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(trace.Steps))
	}
	ruleStep := trace.Steps[2]
	if ruleStep.Matched == nil || *ruleStep.Matched {
		t.Error("expected matched=false")
	}

	// Condition should show failure with actual value.
	if len(ruleStep.Conditions) != 1 {
		t.Fatalf("expected 1 condition trace, got %d", len(ruleStep.Conditions))
	}
	cond := ruleStep.Conditions[0]
	if cond.ActualValue != "UK" {
		t.Errorf("expected actual_value 'UK', got %v", cond.ActualValue)
	}
	if cond.Passed {
		t.Error("expected condition passed=false")
	}
}

func TestEvaluateWithTrace_PercentageRollout(t *testing.T) {
	t.Run("in-rollout", func(t *testing.T) {
		// rollout-flag + user-xyz = bucket 28, 50% rollout -> in rollout
		engine := NewEngine()
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
		ctx := &model.EvaluationContext{
			UserID: "user-xyz",
			Attributes: map[string]any{
				"country": "US",
			},
		}

		trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

		if trace.Reason != "rule_match" {
			t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
		}
		if trace.SelectedStep != 2 {
			t.Errorf("expected selected_step 2, got %d", trace.SelectedStep)
		}

		ruleStep := trace.Steps[2]
		if ruleStep.PercentageRollout == nil || *ruleStep.PercentageRollout != 50 {
			t.Errorf("expected percentage_rollout 50, got %v", ruleStep.PercentageRollout)
		}
		if ruleStep.HashBucket == nil || *ruleStep.HashBucket != 28 {
			t.Errorf("expected hash_bucket 28, got %v", ruleStep.HashBucket)
		}
		if ruleStep.InRollout == nil || !*ruleStep.InRollout {
			t.Error("expected in_rollout=true")
		}
		if ruleStep.Matched == nil || !*ruleStep.Matched {
			t.Error("expected matched=true")
		}
	})

	t.Run("out-of-rollout", func(t *testing.T) {
		// rollout-flag + user-abc = bucket 89, 50% rollout -> out of rollout
		engine := NewEngine()
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
		ctx := &model.EvaluationContext{
			UserID: "user-abc",
			Attributes: map[string]any{
				"country": "US",
			},
		}

		trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

		if trace.Reason != "default" {
			t.Errorf("expected reason 'default', got %q", trace.Reason)
		}
		if trace.SelectedStep != -1 {
			t.Errorf("expected selected_step -1, got %d", trace.SelectedStep)
		}

		ruleStep := trace.Steps[2]
		if ruleStep.PercentageRollout == nil || *ruleStep.PercentageRollout != 50 {
			t.Errorf("expected percentage_rollout 50, got %v", ruleStep.PercentageRollout)
		}
		if ruleStep.HashBucket == nil || *ruleStep.HashBucket != 89 {
			t.Errorf("expected hash_bucket 89, got %v", ruleStep.HashBucket)
		}
		if ruleStep.InRollout == nil || *ruleStep.InRollout {
			t.Error("expected in_rollout=false")
		}
		if ruleStep.Matched == nil || *ruleStep.Matched {
			t.Error("expected matched=false")
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
				{Attribute: "beta_opted_in", Operator: "equals", Value: "true"},
			},
		},
	}

	ctx := &model.EvaluationContext{
		UserID: "user-1",
		Attributes: map[string]any{
			"plan":          "enterprise",
			"beta_opted_in": "true",
		},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, segments)

	if trace.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
	}

	ruleStep := trace.Steps[2]
	if len(ruleStep.Conditions) != 1 {
		t.Fatalf("expected 1 condition trace, got %d", len(ruleStep.Conditions))
	}

	cond := ruleStep.Conditions[0]
	if cond.Operator != "segment_match" {
		t.Errorf("expected operator 'segment_match', got %q", cond.Operator)
	}
	if cond.ConditionValue != "beta-users" {
		t.Errorf("expected condition_value 'beta-users', got %v", cond.ConditionValue)
	}
	if !cond.Passed {
		t.Error("expected condition passed=true")
	}

	// Verify segment_trace has individual condition results.
	if len(cond.SegmentTrace) != 2 {
		t.Fatalf("expected 2 segment trace conditions, got %d", len(cond.SegmentTrace))
	}

	segCond0 := cond.SegmentTrace[0]
	if segCond0.Attribute != "plan" {
		t.Errorf("expected attribute 'plan', got %q", segCond0.Attribute)
	}
	if segCond0.ActualValue != "enterprise" {
		t.Errorf("expected actual_value 'enterprise', got %v", segCond0.ActualValue)
	}
	if !segCond0.Passed {
		t.Error("expected segment condition 0 passed=true")
	}

	segCond1 := cond.SegmentTrace[1]
	if segCond1.Attribute != "beta_opted_in" {
		t.Errorf("expected attribute 'beta_opted_in', got %q", segCond1.Attribute)
	}
	if !segCond1.Passed {
		t.Error("expected segment condition 1 passed=true")
	}
}

func TestEvaluateWithTrace_SkippedRules(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "none", model.LifecycleActive)
	config := makeConfig(true, "default", []model.Variant{
		{Key: "default", Value: rawJSON("none")},
		{Key: "beta", Value: rawJSON("beta-experience")},
		{Key: "vip", Value: rawJSON("vip-experience")},
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

	// User matches both rules; first should win, second should be skipped.
	ctx := &model.EvaluationContext{
		UserID: "user-1",
		Attributes: map[string]any{
			"plan": "enterprise",
			"beta": "true",
		},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
	}
	if trace.Variant != "vip" {
		t.Errorf("expected variant 'vip', got %q", trace.Variant)
	}
	if trace.SelectedStep != 2 {
		t.Errorf("expected selected_step 2, got %d", trace.SelectedStep)
	}

	// 4 steps: lifecycle_check, enabled_check, rule 0, rule 1
	if len(trace.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(trace.Steps))
	}

	// First rule matched.
	rule0 := trace.Steps[2]
	if rule0.Matched == nil || !*rule0.Matched {
		t.Error("expected rule 0 matched=true")
	}
	if rule0.Skipped {
		t.Error("expected rule 0 skipped=false")
	}
	if len(rule0.Conditions) != 1 {
		t.Errorf("expected 1 condition for rule 0, got %d", len(rule0.Conditions))
	}

	// Second rule should be skipped with empty conditions.
	rule1 := trace.Steps[3]
	if !rule1.Skipped {
		t.Error("expected rule 1 skipped=true")
	}
	if len(rule1.Conditions) != 0 {
		t.Errorf("expected 0 conditions for skipped rule, got %d", len(rule1.Conditions))
	}
}
