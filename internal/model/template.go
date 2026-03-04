package model

import (
	"encoding/json"
	"time"
)

// FlagTemplate represents a pre-configured template for creating flags.
type FlagTemplate struct {
	ID                  string          `json:"id"`
	ProjectID           *string         `json:"project_id,omitempty"`
	Key                 string          `json:"key"`
	Name                string          `json:"name"`
	Description         string          `json:"description"`
	FlagType            FlagType        `json:"flag_type"`
	ValueType           ValueType       `json:"value_type"`
	DefaultValue        json.RawMessage `json:"default_value"`
	Tags                []string        `json:"tags"`
	EnvironmentDefaults json.RawMessage `json:"environment_defaults"`
	VariantConfig       json.RawMessage `json:"variant_config"`
	IsSystem            bool            `json:"is_system"`
	SortOrder           int             `json:"sort_order"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// VariantConfig holds the pre-configured variant setup for a template.
type VariantConfig struct {
	Variants       []Variant       `json:"variants"`
	DefaultVariant string          `json:"default_variant"`
	TargetingRules []TargetingRule `json:"targeting_rules,omitempty"`
}
