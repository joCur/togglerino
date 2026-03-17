package togglerino

import "encoding/json"

// FlagDefinition represents a single flag as returned by the definitions endpoint.
type FlagDefinition struct {
	Key       string               `json:"key"`
	ValueType string               `json:"valueType"`
	Status    string               `json:"status"`
	Config    FlagDefinitionConfig `json:"config"`
}

// FlagDefinitionConfig holds per-environment configuration for a flag definition.
type FlagDefinitionConfig struct {
	Enabled        bool                      `json:"enabled"`
	DefaultVariant string                    `json:"defaultVariant"`
	Variants       []VariantDefinition       `json:"variants"`
	TargetingRules []TargetingRuleDefinition `json:"targetingRules"`
}

// VariantDefinition is a variant with a key and arbitrary JSON value.
type VariantDefinition struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// TargetingRuleDefinition is a targeting rule with conditions, a variant to serve,
// and an optional percentage rollout.
type TargetingRuleDefinition struct {
	Variant    string                `json:"variant"`
	Percentage *int                  `json:"percentage"`
	Conditions []ConditionDefinition `json:"conditions"`
}

// ConditionDefinition is a single condition within a targeting rule.
type ConditionDefinition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
}

// SegmentDefinition is a reusable segment definition.
type SegmentDefinition struct {
	Key        string                `json:"key"`
	Conditions []ConditionDefinition `json:"conditions"`
}

// definitionsResponse is the response from the definitions API endpoint.
type definitionsResponse struct {
	Flags    []FlagDefinition  `json:"flags"`
	Segments []SegmentDefinition `json:"segments"`
}
