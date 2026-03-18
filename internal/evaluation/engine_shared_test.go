package evaluation

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

// fixtureCase represents a single test case from evaluation_cases.json.
type fixtureCase struct {
	Name     string          `json:"name"`
	Flag     fixtureFlag     `json:"flag"`
	Segments []fixtureSegment `json:"segments"`
	Context  fixtureContext  `json:"context"`
	Expected fixtureExpected `json:"expected"`
}

type fixtureFlag struct {
	Key          string            `json:"key"`
	ValueType    string            `json:"valueType"`
	Status       string            `json:"status"`
	DefaultValue json.RawMessage   `json:"defaultValue"`
	Variants     []fixtureVariant  `json:"variants"`
	Config       fixtureFlagConfig `json:"config"`
}

type fixtureFlagConfig struct {
	Enabled            bool                   `json:"enabled"`
	OffVariant         string                 `json:"offVariant"`
	FallthroughVariant string                 `json:"fallthroughVariant"`
	TargetingRules     []fixtureTargetingRule `json:"targetingRules"`
}

type fixtureVariant struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type fixtureTargetingRule struct {
	Variant    string             `json:"variant"`
	Percentage *int               `json:"percentage"`
	Conditions []fixtureCondition `json:"conditions"`
}

type fixtureCondition struct {
	Attribute string          `json:"attribute"`
	Operator  string          `json:"operator"`
	Value     json.RawMessage `json:"value"`
}

type fixtureSegment struct {
	Key        string             `json:"key"`
	Conditions []fixtureCondition `json:"conditions"`
}

type fixtureContext struct {
	UserID     string         `json:"userId"`
	Attributes map[string]any `json:"attributes"`
}

type fixtureExpected struct {
	Value   any    `json:"value"`
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

// convertConditionValue converts a fixture condition value to the backend's model.Condition.Value format.
// For in/not_in operators, the fixture stores JSON-encoded arrays as strings (e.g., "[\"US\",\"CA\"]").
// The backend expects []any for these operators.
// For other operators, the fixture stores plain strings which are kept as-is.
func convertConditionValue(operator string, raw json.RawMessage) any {
	if raw == nil || string(raw) == "null" {
		return nil
	}

	// Try to unmarshal as a string first (the most common case in fixtures).
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		// For in/not_in, the string is a JSON-encoded array — parse it.
		if operator == "in" || operator == "not_in" {
			var arr []any
			if err := json.Unmarshal([]byte(s), &arr); err == nil {
				return arr
			}
		}
		return s
	}

	// Not a string — could be number, boolean, object, array, etc.
	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		return v
	}
	return string(raw)
}

func convertFixtureConditions(conditions []fixtureCondition) []model.Condition {
	result := make([]model.Condition, len(conditions))
	for i, c := range conditions {
		result[i] = model.Condition{
			Attribute: c.Attribute,
			Operator:  c.Operator,
			Value:     convertConditionValue(c.Operator, c.Value),
		}
	}
	return result
}

// toFlag converts a fixture flag to the backend *model.Flag.
func toFlag(f fixtureFlag) *model.Flag {
	// Determine the lifecycle status.
	var lifecycle model.LifecycleStatus
	switch f.Status {
	case "archived":
		lifecycle = model.LifecycleArchived
	case "potentially_stale":
		lifecycle = model.LifecyclePotentiallyStale
	case "stale":
		lifecycle = model.LifecycleStale
	default:
		lifecycle = model.LifecycleActive
	}

	variants := make([]model.Variant, len(f.Variants))
	for i, v := range f.Variants {
		variants[i] = model.Variant{
			Name:  v.Name,
			Value: v.Value,
		}
	}

	return &model.Flag{
		Key:             f.Key,
		ValueType:       model.ValueType(f.ValueType),
		LifecycleStatus: lifecycle,
		DefaultValue:    f.DefaultValue,
		Variants:        variants,
	}
}

// toConfig converts a fixture flag config to the backend *model.FlagEnvironmentConfig.
func toConfig(f fixtureFlag) *model.FlagEnvironmentConfig {
	rules := make([]model.TargetingRule, len(f.Config.TargetingRules))
	for i, r := range f.Config.TargetingRules {
		rules[i] = model.TargetingRule{
			Conditions: convertFixtureConditions(r.Conditions),
			Variant:    r.Variant,
		}
		if r.Percentage != nil {
			p := *r.Percentage
			rules[i].PercentageRollout = &p
		}
	}

	return &model.FlagEnvironmentConfig{
		Enabled:            f.Config.Enabled,
		OffVariant:         f.Config.OffVariant,
		FallthroughVariant: f.Config.FallthroughVariant,
		TargetingRules:     rules,
	}
}

// toSegments converts fixture segments to the backend segments map.
func toSegments(segs []fixtureSegment) map[string]model.Segment {
	if len(segs) == 0 {
		return nil
	}
	result := make(map[string]model.Segment, len(segs))
	for _, s := range segs {
		result[s.Key] = model.Segment{
			Key:        s.Key,
			Conditions: convertFixtureConditions(s.Conditions),
		}
	}
	return result
}

// toContext converts a fixture context to the backend *model.EvaluationContext.
// The backend reads ctx.UserID for consistent hashing and ctx.Attributes["user_id"]
// for targeting rules. The fixture stores userId in a dedicated field, so we set both.
func toContext(fc fixtureContext) *model.EvaluationContext {
	attrs := make(map[string]any, len(fc.Attributes)+1)
	for k, v := range fc.Attributes {
		attrs[k] = v
	}
	// Always map userId to user_id attribute so targeting rules on "user_id" work.
	if fc.UserID != "" {
		attrs["user_id"] = fc.UserID
	}
	return &model.EvaluationContext{
		UserID:     fc.UserID,
		Attributes: attrs,
	}
}

func TestSharedFixtures(t *testing.T) {
	data, err := os.ReadFile("../../testdata/evaluation_cases.json")
	if err != nil {
		t.Fatalf("failed to read evaluation_cases.json: %v", err)
	}

	var cases []fixtureCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal evaluation_cases.json: %v", err)
	}

	engine := NewEngine()

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			flag := toFlag(tc.Flag)
			config := toConfig(tc.Flag)
			segments := toSegments(tc.Segments)
			ctx := toContext(tc.Context)

			result := engine.EvaluateWithSegments(flag, config, ctx, segments)

			// Compare reason.
			if result.Reason != tc.Expected.Reason {
				t.Errorf("reason: got %q, want %q", result.Reason, tc.Expected.Reason)
			}

			// Compare variant.
			if result.Variant != tc.Expected.Variant {
				t.Errorf("variant: got %q, want %q", result.Variant, tc.Expected.Variant)
			}

			// Compare value.
			// The engine returns Go types (bool, float64, string, map, etc.)
			// The expected value from JSON is also a Go type after unmarshaling.
			if !valuesEqual(result.Value, tc.Expected.Value) {
				t.Errorf("value: got %v (%T), want %v (%T)", result.Value, result.Value, tc.Expected.Value, tc.Expected.Value)
			}
		})
	}
}

// valuesEqual compares two evaluation result values.
// JSON numbers unmarshal as float64, so we need to handle comparisons carefully.
func valuesEqual(got, want any) bool {
	// Handle nil.
	if got == nil && want == nil {
		return true
	}
	if got == nil || want == nil {
		return false
	}

	// Both booleans.
	if gb, ok := got.(bool); ok {
		if wb, ok := want.(bool); ok {
			return gb == wb
		}
		return false
	}

	// Both strings.
	if gs, ok := got.(string); ok {
		if ws, ok := want.(string); ok {
			return gs == ws
		}
		return false
	}

	// Both numbers — JSON always unmarshals numbers as float64.
	if gf, ok := got.(float64); ok {
		if wf, ok := want.(float64); ok {
			return gf == wf
		}
		return false
	}

	// Fall back to deep equal for maps (JSON objects), slices, etc.
	return reflect.DeepEqual(got, want)
}
