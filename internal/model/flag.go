package model

import (
	"encoding/json"
	"time"
)

type FlagType string

const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeString  FlagType = "string"
	FlagTypeNumber  FlagType = "number"
	FlagTypeJSON    FlagType = "json"
)

type Flag struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"project_id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	FlagType     FlagType        `json:"flag_type"`
	DefaultValue json.RawMessage `json:"default_value"`
	Tags         []string        `json:"tags"`
	Archived     bool            `json:"archived"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type FlagEnvironmentConfig struct {
	ID             string          `json:"id"`
	FlagID         string          `json:"flag_id"`
	EnvironmentID  string          `json:"environment_id"`
	Enabled        bool            `json:"enabled"`
	DefaultVariant string          `json:"default_variant"`
	Variants       []Variant       `json:"variants"`
	TargetingRules []TargetingRule `json:"targeting_rules"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Variant struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type TargetingRule struct {
	Conditions        []Condition `json:"conditions"`
	Variant           string      `json:"variant"`
	PercentageRollout *int        `json:"percentage_rollout,omitempty"`
}

type Condition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     any    `json:"value"`
}

type Operator string

const (
	OpEquals      Operator = "equals"
	OpNotEquals   Operator = "not_equals"
	OpContains    Operator = "contains"
	OpNotContains Operator = "not_contains"
	OpStartsWith  Operator = "starts_with"
	OpEndsWith    Operator = "ends_with"
	OpGreaterThan Operator = "greater_than"
	OpLessThan    Operator = "less_than"
	OpGTE         Operator = "gte"
	OpLTE         Operator = "lte"
	OpIn          Operator = "in"
	OpNotIn       Operator = "not_in"
	OpExists      Operator = "exists"
	OpNotExists   Operator = "not_exists"
	OpMatches     Operator = "matches"
)

type EvaluationContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes"`
}

type EvaluationResult struct {
	Value   any    `json:"value"`
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}
