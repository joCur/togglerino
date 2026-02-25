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

func makeFlag(key string, defaultValue any, archived bool) *model.Flag {
	return &model.Flag{
		Key:          key,
		DefaultValue: rawJSON(defaultValue),
		Archived:     archived,
	}
}

func makeConfig(enabled bool, defaultVariant string, variants []model.Variant, rules []model.TargetingRule) *model.FlagEnvironmentConfig {
	return &model.FlagEnvironmentConfig{
		Enabled:        enabled,
		DefaultVariant: defaultVariant,
		Variants:       variants,
		TargetingRules: rules,
	}
}

func TestEngine_FlagDisabled(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", false, false)
	config := makeConfig(false, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
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
}

func TestEngine_FlagArchived(t *testing.T) {
	engine := NewEngine()
	flag := makeFlag("test-flag", "default-val", true)
	config := makeConfig(true, "on", []model.Variant{
		{Key: "on", Value: rawJSON("on-val")},
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
	flag := makeFlag("test-flag", false, false)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
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
	flag := makeFlag("test-flag", false, false)
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
	flag := makeFlag("test-flag", false, false)
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
	flag := makeFlag("test-flag", "none", false)
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
	flag := makeFlag("rollout-flag", false, false)
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
	flag := makeFlag("rollout-flag", false, false)
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
	flag := makeFlag("test-flag", "default", false)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON("default")},
		{Key: "premium", Value: rawJSON("premium-feature")},
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
	flag := makeFlag("test-flag", false, false)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
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
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
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
	flag := makeFlag("test-flag", "fallback-value", false)
	config := makeConfig(true, "nonexistent-variant", []model.Variant{
		{Key: "on", Value: rawJSON(true)},
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
	flag := makeFlag("full-rollout", false, false)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
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
	flag := makeFlag("zero-rollout", false, false)
	config := makeConfig(true, "off", []model.Variant{
		{Key: "off", Value: rawJSON(false)},
		{Key: "on", Value: rawJSON(true)},
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
