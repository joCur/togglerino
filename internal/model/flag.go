package model

import (
	"encoding/json"
	"time"
)

// ValueType describes the data type of a flag's value.
type ValueType string

const (
	ValueTypeBoolean ValueType = "boolean"
	ValueTypeString  ValueType = "string"
	ValueTypeNumber  ValueType = "number"
	ValueTypeJSON    ValueType = "json"
)

// FlagType describes the purpose/category of a flag.
type FlagType string

const (
	FlagTypeRelease     FlagType = "release"
	FlagTypeExperiment  FlagType = "experiment"
	FlagTypeOperational FlagType = "operational"
	FlagTypeKillSwitch  FlagType = "kill-switch"
	FlagTypePermission  FlagType = "permission"
)

// LifecycleStatus describes the lifecycle state of a flag.
type LifecycleStatus string

const (
	LifecycleActive           LifecycleStatus = "active"
	LifecyclePotentiallyStale LifecycleStatus = "potentially_stale"
	LifecycleStale            LifecycleStatus = "stale"
	LifecycleArchived         LifecycleStatus = "archived"
)

// LifecycleCountRow holds a per-project, per-status flag count.
type LifecycleCountRow struct {
	ProjectID string
	Status    string
	Count     int
}

// LifecycleSummary holds per-status flag counts and a derived health score.
type LifecycleSummary struct {
	Active           int     `json:"active"`
	PotentiallyStale int     `json:"potentially_stale"`
	Stale            int     `json:"stale"`
	Archived         int     `json:"archived"`
	HealthScore      float64 `json:"health_score"`
}

// LifecycleSnapshot holds a single day's lifecycle counts for a project.
type LifecycleSnapshot struct {
	Date                  string `json:"date"`
	ActiveCount           int    `json:"active"`
	PotentiallyStaleCount int    `json:"potentially_stale"`
	StaleCount            int    `json:"stale"`
	ArchivedCount         int    `json:"archived"`
}

type Flag struct {
	ID                       string          `json:"id"`
	ProjectID                string          `json:"project_id"`
	Key                      string          `json:"key"`
	Name                     string          `json:"name"`
	Description              string          `json:"description"`
	ValueType                ValueType       `json:"value_type"`
	FlagType                 FlagType        `json:"flag_type"`
	DefaultValue             json.RawMessage `json:"default_value"`
	Tags                     []string        `json:"tags"`
	LifecycleStatus          LifecycleStatus `json:"lifecycle_status"`
	LifecycleStatusChangedAt *time.Time      `json:"lifecycle_status_changed_at"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	OwnerID                  *string         `json:"owner_id,omitempty"`
	Owner                    *FlagOwner      `json:"owner,omitempty"`
	EnvironmentConfigs       []FlagEnvironmentConfig `json:"environment_configs,omitempty"`
}

type FlagOwner struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name,omitempty"`
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
	UpdatedBy      *string         `json:"updated_by,omitempty"`
	UpdatedByUser  *FlagOwner      `json:"updated_by_user,omitempty"`
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
	OpMatches      Operator = "matches"
	OpSegmentMatch Operator = "segment_match"
)

// ValidValueTypes is the set of all valid value types.
var ValidValueTypes = map[ValueType]bool{
	ValueTypeBoolean: true,
	ValueTypeString:  true,
	ValueTypeNumber:  true,
	ValueTypeJSON:    true,
}

// ValidFlagTypes is the set of all valid flag types.
var ValidFlagTypes = map[FlagType]bool{
	FlagTypeRelease:     true,
	FlagTypeExperiment:  true,
	FlagTypeOperational: true,
	FlagTypeKillSwitch:  true,
	FlagTypePermission:  true,
}

type EvaluationContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes"`
}

type EvaluationResult struct {
	Value   any    `json:"value"`
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

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
	Type              string           `json:"type"`
	Passed            bool             `json:"passed"`
	Status            string           `json:"status,omitempty"`
	Enabled           *bool            `json:"enabled,omitempty"`
	RuleIndex         *int             `json:"rule_index,omitempty"`
	Variant           string           `json:"variant,omitempty"`
	PercentageRollout *int             `json:"percentage_rollout,omitempty"`
	HashBucket        *int             `json:"hash_bucket,omitempty"`
	InRollout         *bool            `json:"in_rollout,omitempty"`
	Matched           *bool            `json:"matched,omitempty"`
	Skipped           bool             `json:"skipped,omitempty"`
	Conditions        []ConditionTrace `json:"conditions,omitempty"`
}

// EvaluationTrace contains a detailed trace of the evaluation process.
type EvaluationTrace struct {
	FlagKey        string      `json:"flag_key"`
	Value          any         `json:"value"`
	Variant        string      `json:"variant"`
	Reason         string      `json:"reason"`
	Steps          []TraceStep `json:"steps"`
	DefaultVariant string      `json:"default_variant"`
	SelectedStep   int         `json:"selected_step"`
}

type ContextAttribute struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Name       string    `json:"name"`
	LastSeenAt time.Time `json:"last_seen_at"`
}
