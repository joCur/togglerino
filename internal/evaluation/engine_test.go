package evaluation

import (
	"encoding/json"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func intPtr(n int) *int {
	return &n
}

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func makeFlag(key string, defaultValue any, lifecycleStatus model.LifecycleStatus) *model.Flag {
	return &model.Flag{
		Key:             key,
		DefaultValue:    rawJSON(defaultValue),
		LifecycleStatus: lifecycleStatus,
	}
}

func makeBoolFlag(key string, lifecycleStatus model.LifecycleStatus) *model.Flag {
	return &model.Flag{
		Key:             key,
		ValueType:       model.ValueTypeBoolean,
		DefaultValue:    rawJSON(false),
		LifecycleStatus: lifecycleStatus,
	}
}

func makeConfig(enabled bool, fallthroughVariant string, variants []model.Variant, rules []model.TargetingRule) *model.FlagEnvironmentConfig {
	return &model.FlagEnvironmentConfig{
		Enabled:            enabled,
		FallthroughVariant: fallthroughVariant,
		Variants:           variants,
		TargetingRules:     rules,
	}
}

func makeConfigWithOff(enabled bool, fallthroughVariant, offVariant string, variants []model.Variant, rules []model.TargetingRule) *model.FlagEnvironmentConfig {
	return &model.FlagEnvironmentConfig{
		Enabled:            enabled,
		OffVariant:         offVariant,
		FallthroughVariant: fallthroughVariant,
		Variants:           variants,
		TargetingRules:     rules,
	}
}

func TestEngine_FlagDisabled(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfigWithOff(false, "on", "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
	}, nil)
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "disabled" {
		t.Errorf("expected reason 'disabled', got %q", result.Reason)
	}
	if result.Value != false {
		t.Errorf("expected value false, got %v", result.Value)
	}
	if result.Variant != "off" {
		t.Errorf("expected variant 'off', got %q", result.Variant)
	}
}

func TestEngine_FlagArchived(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "default-val", model.LifecycleArchived)
	config := makeConfig(true, "on", []model.Variant{
		{Name: "on", Value: rawJSON("on-val")},
	}, nil)
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "archived" {
		t.Errorf("expected reason 'archived', got %q", result.Reason)
	}
	if result.Value != "default-val" {
		t.Errorf("expected value 'default-val', got %v", result.Value)
	}
}

func TestEngine_NoRulesEnabled(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
	}, nil)
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result.Reason)
	}
	if result.Variant != "off" {
		t.Errorf("expected variant 'off', got %q", result.Variant)
	}
	if result.Value != false {
		t.Errorf("expected value false, got %v", result.Value)
	}
}

func TestEngine_SingleRuleMatches(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
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

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
	if result.Variant != "on" {
		t.Errorf("expected variant 'on', got %q", result.Variant)
	}
	if result.Value != true {
		t.Errorf("expected value true, got %v", result.Value)
	}
}

func TestEngine_SingleRuleDoesNotMatch(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
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

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result.Reason)
	}
	if result.Variant != "off" {
		t.Errorf("expected variant 'off', got %q", result.Variant)
	}
}

func TestEngine_MultipleRulesFirstMatchWins(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "none", model.LifecycleActive)
	config := makeConfig(true, "default", []model.Variant{
		{Name: "default", Value: rawJSON("none")},
		{Name: "beta", Value: rawJSON("beta-experience")},
		{Name: "vip", Value: rawJSON("vip-experience")},
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

	// User matches both rules; first should win.
	ctx := &model.EvaluationContext{
		UserID: "user-1",
		Attributes: map[string]any{
			"plan": "enterprise",
			"beta": "true",
		},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Variant != "vip" {
		t.Errorf("expected variant 'vip' (first match), got %q", result.Variant)
	}
	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
	if result.Value != "vip-experience" {
		t.Errorf("expected value 'vip-experience', got %v", result.Value)
	}
}

func TestEngine_PercentageRollout_InBucket(t *testing.T) {
	// rollout-flag + user-xyz = bucket 28
	// With 50% rollout, bucket 28 < 50, so user IS in rollout.
	engine := NewEngine()
	flag := makeFlag("rollout-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
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

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
	if result.Variant != "on" {
		t.Errorf("expected variant 'on', got %q", result.Variant)
	}
	if result.Value != true {
		t.Errorf("expected value true, got %v", result.Value)
	}
}

func TestEngine_PercentageRollout_OutOfBucket(t *testing.T) {
	// rollout-flag + user-abc = bucket 89
	// With 50% rollout, bucket 89 >= 50, so user is NOT in rollout.
	engine := NewEngine()
	flag := makeFlag("rollout-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
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

	result := engine.Evaluate(flag, config, ctx)

	// User-abc hashes to bucket 89, which is >= 50, so rollout does not apply.
	// Falls through to default.
	if result.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result.Reason)
	}
	if result.Variant != "off" {
		t.Errorf("expected variant 'off', got %q", result.Variant)
	}
}

func TestEngine_ComplexConditionsANDLogic(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "default", model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON("default")},
		{Name: "premium", Value: rawJSON("premium-feature")},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "country", Operator: "equals", Value: "US"},
				{Attribute: "age", Operator: "gte", Value: float64(18)},
				{Attribute: "plan", Operator: "in", Value: []any{"pro", "enterprise"}},
			},
			Variant: "premium",
		},
	})

	tests := []struct {
		name           string
		attrs          map[string]any
		expectedReason string
		expectedVariant string
	}{
		{
			name: "all conditions match",
			attrs: map[string]any{
				"country": "US",
				"age":     float64(25),
				"plan":    "pro",
			},
			expectedReason:  "rule_match",
			expectedVariant: "premium",
		},
		{
			name: "country mismatch",
			attrs: map[string]any{
				"country": "UK",
				"age":     float64(25),
				"plan":    "pro",
			},
			expectedReason:  "default",
			expectedVariant: "off",
		},
		{
			name: "age too low",
			attrs: map[string]any{
				"country": "US",
				"age":     float64(16),
				"plan":    "pro",
			},
			expectedReason:  "default",
			expectedVariant: "off",
		},
		{
			name: "plan not in list",
			attrs: map[string]any{
				"country": "US",
				"age":     float64(25),
				"plan":    "free",
			},
			expectedReason:  "default",
			expectedVariant: "off",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &model.EvaluationContext{
				UserID:     "user-1",
				Attributes: tt.attrs,
			}
			result := engine.Evaluate(flag, config, ctx)
			if result.Reason != tt.expectedReason {
				t.Errorf("expected reason %q, got %q", tt.expectedReason, result.Reason)
			}
			if result.Variant != tt.expectedVariant {
				t.Errorf("expected variant %q, got %q", tt.expectedVariant, result.Variant)
			}
		})
	}
}

func TestEngine_ExistsNotExistsOperators(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "email", Operator: "exists", Value: nil},
			},
			Variant: "on",
		},
	})

	t.Run("attribute exists", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-1",
			Attributes: map[string]any{
				"email": "user@example.com",
			},
		}
		result := engine.Evaluate(flag, config, ctx)
		if result.Reason != "rule_match" {
			t.Errorf("expected reason 'rule_match', got %q", result.Reason)
		}
		if result.Variant != "on" {
			t.Errorf("expected variant 'on', got %q", result.Variant)
		}
	})

	t.Run("attribute does not exist", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID:     "user-2",
			Attributes: map[string]any{},
		}
		result := engine.Evaluate(flag, config, ctx)
		if result.Reason != "default" {
			t.Errorf("expected reason 'default', got %q", result.Reason)
		}
		if result.Variant != "off" {
			t.Errorf("expected variant 'off', got %q", result.Variant)
		}
	})

	// Test not_exists operator.
	configNotExists := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "email", Operator: "not_exists", Value: nil},
			},
			Variant: "on",
		},
	})

	t.Run("not_exists - attribute missing", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID:     "user-3",
			Attributes: map[string]any{},
		}
		result := engine.Evaluate(flag, configNotExists, ctx)
		if result.Reason != "rule_match" {
			t.Errorf("expected reason 'rule_match', got %q", result.Reason)
		}
	})

	t.Run("not_exists - attribute present", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-4",
			Attributes: map[string]any{
				"email": "user@example.com",
			},
		}
		result := engine.Evaluate(flag, configNotExists, ctx)
		if result.Reason != "default" {
			t.Errorf("expected reason 'default', got %q", result.Reason)
		}
	})
}

func TestEngine_VariantNotFound_FallbackToDefault(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "fallback-value", model.LifecycleActive)
	config := makeConfig(true, "nonexistent-variant", []model.Variant{
		{Name: "on", Value: rawJSON(true)},
	}, nil)
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result.Reason)
	}
	// Variant key is still set to what the config says, even if not found.
	if result.Variant != "nonexistent-variant" {
		t.Errorf("expected variant 'nonexistent-variant', got %q", result.Variant)
	}
	// Value should fall back to the flag's default value.
	if result.Value != "fallback-value" {
		t.Errorf("expected value 'fallback-value', got %v", result.Value)
	}
}

func TestEngine_PercentageRollout_100Percent(t *testing.T) {
	// 100% rollout means all users should be included.
	engine := NewEngine()
	flag := makeFlag("full-rollout", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "active", Operator: "equals", Value: "true"},
			},
			Variant:           "on",
			PercentageRollout: intPtr(100),
		},
	})
	ctx := &model.EvaluationContext{
		UserID: "any-user",
		Attributes: map[string]any{
			"active": "true",
		},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
	if result.Value != true {
		t.Errorf("expected value true, got %v", result.Value)
	}
}

func TestEngine_PercentageRollout_0Percent(t *testing.T) {
	// 0% rollout means no users should be included.
	engine := NewEngine()
	flag := makeFlag("zero-rollout", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "active", Operator: "equals", Value: "true"},
			},
			Variant:           "on",
			PercentageRollout: intPtr(0),
		},
	})
	ctx := &model.EvaluationContext{
		UserID: "any-user",
		Attributes: map[string]any{
			"active": "true",
		},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result.Reason)
	}
}

func TestEngine_SegmentMatchCondition(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, model.LifecycleActive)
	config := makeConfig(true, "off", []model.Variant{
		{Name: "off", Value: rawJSON(false)},
		{Name: "on", Value: rawJSON(true)},
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

	t.Run("matches segment conditions", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-1",
			Attributes: map[string]any{
				"plan":          "enterprise",
				"beta_opted_in": "true",
			},
		}
		result := engine.EvaluateWithSegments(flag, config, ctx, segments)
		if result.Reason != "rule_match" {
			t.Errorf("expected reason 'rule_match', got %q", result.Reason)
		}
		if result.Variant != "on" {
			t.Errorf("expected variant 'on', got %q", result.Variant)
		}
	})

	t.Run("does not match segment conditions", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-2",
			Attributes: map[string]any{
				"plan":          "free",
				"beta_opted_in": "true",
			},
		}
		result := engine.EvaluateWithSegments(flag, config, ctx, segments)
		if result.Reason != "default" {
			t.Errorf("expected reason 'default', got %q", result.Reason)
		}
	})

	t.Run("segment not found fails silently", func(t *testing.T) {
		ctx := &model.EvaluationContext{
			UserID: "user-3",
			Attributes: map[string]any{
				"plan": "enterprise",
			},
		}
		result := engine.EvaluateWithSegments(flag, config, ctx, map[string]model.Segment{})
		if result.Reason != "default" {
			t.Errorf("expected reason 'default' for missing segment, got %q", result.Reason)
		}
	})

	t.Run("segment_match mixed with inline conditions", func(t *testing.T) {
		mixedConfig := makeConfig(true, "off", []model.Variant{
			{Name: "off", Value: rawJSON(false)},
			{Name: "on", Value: rawJSON(true)},
		}, []model.TargetingRule{
			{
				Conditions: []model.Condition{
					{Attribute: "", Operator: "segment_match", Value: "beta-users"},
					{Attribute: "country", Operator: "equals", Value: "US"},
				},
				Variant: "on",
			},
		})

		ctx := &model.EvaluationContext{
			UserID: "user-4",
			Attributes: map[string]any{
				"plan":          "enterprise",
				"beta_opted_in": "true",
				"country":       "US",
			},
		}
		result := engine.EvaluateWithSegments(flag, mixedConfig, ctx, segments)
		if result.Reason != "rule_match" {
			t.Errorf("expected 'rule_match', got %q", result.Reason)
		}

		ctx.Attributes["country"] = "UK"
		result = engine.EvaluateWithSegments(flag, mixedConfig, ctx, segments)
		if result.Reason != "default" {
			t.Errorf("expected 'default' when inline condition fails, got %q", result.Reason)
		}
	})
}

func TestEngine_BooleanFlag_EnabledNoRules(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("maintenance-mode", model.LifecycleActive)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, nil)
	ctx := &model.EvaluationContext{UserID: "user-1", Attributes: map[string]any{}}

	result := engine.Evaluate(flag, config, ctx)

	if result.Value != true {
		t.Errorf("expected enabled boolean flag to return true, got %v", result.Value)
	}
	if result.Variant != "true" {
		t.Errorf("expected variant 'true' for boolean flag, got %q", result.Variant)
	}
	if result.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_Disabled(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("maintenance-mode", model.LifecycleActive)
	config := makeConfigWithOff(false, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, nil)
	ctx := &model.EvaluationContext{UserID: "user-1", Attributes: map[string]any{}}

	result := engine.Evaluate(flag, config, ctx)

	if result.Value != false {
		t.Errorf("expected disabled boolean flag to return false, got %v", result.Value)
	}
	if result.Variant != "false" {
		t.Errorf("expected variant 'false', got %q", result.Variant)
	}
	if result.Reason != "disabled" {
		t.Errorf("expected reason 'disabled', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_Archived(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("maintenance-mode", model.LifecycleArchived)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, nil)
	ctx := &model.EvaluationContext{UserID: "user-1", Attributes: map[string]any{}}

	result := engine.Evaluate(flag, config, ctx)

	if result.Value != false {
		t.Errorf("expected archived boolean flag to return false, got %v", result.Value)
	}
	if result.Reason != "archived" {
		t.Errorf("expected reason 'archived', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_TargetingRuleServesTrue(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("beta-feature", model.LifecycleActive)
	config := makeConfigWithOff(true, "false", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
			},
			Variant: "true",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"plan": "enterprise"},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Value != true {
		t.Errorf("expected rule match to return true, got %v", result.Value)
	}
	if result.Variant != "true" {
		t.Errorf("expected variant 'true' for boolean flag, got %q", result.Variant)
	}
	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_TargetingRuleServesFalse(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("beta-feature", model.LifecycleActive)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "blocked", Operator: "equals", Value: "true"},
			},
			Variant: "false",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"blocked": "true"},
	}

	result := engine.Evaluate(flag, config, ctx)

	if result.Value != false {
		t.Errorf("expected rule match serving 'false' to return false, got %v", result.Value)
	}
	if result.Variant != "false" {
		t.Errorf("expected variant 'false', got %q", result.Variant)
	}
	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_InvalidVariant(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("test-flag", model.LifecycleActive)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
			},
			Variant: "on",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"plan": "enterprise"},
	}

	result := engine.Evaluate(flag, config, ctx)

	// "on" variant not found, falls back to flag.DefaultValue (false).
	if result.Value != false {
		t.Errorf("expected invalid variant 'on' to evaluate as false (default), got %v", result.Value)
	}
	if result.Variant != "on" {
		t.Errorf("expected variant 'on', got %q", result.Variant)
	}
	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_SegmentMatch(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("beta-feature", model.LifecycleActive)
	config := makeConfigWithOff(true, "false", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "", Operator: "segment_match", Value: "beta-users"},
			},
			Variant: "true",
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

	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"plan": "enterprise"},
	}
	result := engine.EvaluateWithSegments(flag, config, ctx, segments)
	if result.Value != true {
		t.Errorf("expected segment match to return true, got %v", result.Value)
	}
	if result.Variant != "true" {
		t.Errorf("expected variant 'true', got %q", result.Variant)
	}
	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}
}

func TestEngine_BooleanFlag_MultipleRulesFirstMatchWins(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("test-flag", model.LifecycleActive)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "blocked", Operator: "equals", Value: "true"},
			},
			Variant: "false",
		},
		{
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
			},
			Variant: "true",
		},
	})

	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"blocked": "true", "plan": "enterprise"},
	}
	result := engine.Evaluate(flag, config, ctx)
	if result.Value != false {
		t.Errorf("expected first-match-wins to return false, got %v", result.Value)
	}
	if result.Variant != "false" {
		t.Errorf("expected variant 'false', got %q", result.Variant)
	}
}

func TestEngine_BooleanFlag_PercentageRollout(t *testing.T) {
	engine := NewEngine()
	// Flag enabled (fallthrough=true), rule serves "false" to 50% of US users.
	// gradual-rollout + user-xyz = bucket 28 (in rollout, gets false)
	// gradual-rollout + user-abc = bucket 89 (outside rollout, gets fallthrough true)
	flag := makeBoolFlag("gradual-rollout", model.LifecycleActive)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "country", Operator: "equals", Value: "US"},
			},
			Variant:           "false",
			PercentageRollout: intPtr(50),
		},
	})

	ctx := &model.EvaluationContext{
		UserID:     "user-xyz",
		Attributes: map[string]any{"country": "US"},
	}
	result := engine.Evaluate(flag, config, ctx)
	if result.Value != false {
		t.Errorf("expected user in rollout to get false (rule match), got %v", result.Value)
	}
	if result.Variant != "false" {
		t.Errorf("expected variant 'false', got %q", result.Variant)
	}
	if result.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", result.Reason)
	}

	ctx2 := &model.EvaluationContext{
		UserID:     "user-abc",
		Attributes: map[string]any{"country": "US"},
	}
	result2 := engine.Evaluate(flag, config, ctx2)
	if result2.Value != true {
		t.Errorf("expected user outside rollout to get true (fallthrough), got %v", result2.Value)
	}
	if result2.Variant != "true" {
		t.Errorf("expected variant 'true', got %q", result2.Variant)
	}
	if result2.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", result2.Reason)
	}
}

func TestEngine_BooleanFlag_Trace(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("maintenance-mode", model.LifecycleActive)
	config := makeConfigWithOff(true, "true", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, nil)
	ctx := &model.EvaluationContext{UserID: "user-1", Attributes: map[string]any{}}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Value != true {
		t.Errorf("expected trace value true for enabled boolean, got %v", trace.Value)
	}
	if trace.Variant != "true" {
		t.Errorf("expected variant 'true' in trace, got %q", trace.Variant)
	}
	if trace.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", trace.Reason)
	}
}

func TestEngine_BooleanFlag_Trace_RuleMatch(t *testing.T) {
	engine := NewEngine()
	flag := makeBoolFlag("beta-feature", model.LifecycleActive)
	config := makeConfigWithOff(true, "false", "false", []model.Variant{
		{Name: "true", Value: rawJSON(true)},
		{Name: "false", Value: rawJSON(false)},
	}, []model.TargetingRule{
		{
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
			},
			Variant: "true",
		},
	})
	ctx := &model.EvaluationContext{
		UserID:     "user-1",
		Attributes: map[string]any{"plan": "enterprise"},
	}

	trace := engine.EvaluateWithTrace(flag, config, ctx, nil)

	if trace.Value != true {
		t.Errorf("expected trace value true for rule match, got %v", trace.Value)
	}
	if trace.Variant != "true" {
		t.Errorf("expected variant 'true', got %q", trace.Variant)
	}
	if trace.Reason != "rule_match" {
		t.Errorf("expected reason 'rule_match', got %q", trace.Reason)
	}
}

func TestCache_SegmentStorage(t *testing.T) {
	cache := NewCache()
	segments := map[string]model.Segment{
		"beta": {Name: "beta", Conditions: []model.Condition{{Attribute: "plan", Operator: "equals", Value: "pro"}}},
	}
	cache.SetSegments("myproject", segments)
	got := cache.GetSegments("myproject")
	if got == nil {
		t.Fatal("expected segments, got nil")
	}
	if _, ok := got["beta"]; !ok {
		t.Error("expected 'beta' segment")
	}
	got = cache.GetSegments("other")
	if got != nil {
		t.Error("expected nil for non-existent project")
	}
}
