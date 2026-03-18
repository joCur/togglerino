package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

// DefinitionsHandler serves flag definitions and segments for server-side SDK evaluation.
type DefinitionsHandler struct {
	cache *evaluation.Cache
}

// NewDefinitionsHandler creates a new DefinitionsHandler.
func NewDefinitionsHandler(cache *evaluation.Cache) *DefinitionsHandler {
	return &DefinitionsHandler{cache: cache}
}

// --- response types (camelCase JSON) ---

type definitionsResponse struct {
	Flags    []flagDefinition    `json:"flags"`
	Segments []segmentDefinition `json:"segments"`
}

type flagDefinition struct {
	Key          string               `json:"key"`
	ValueType    model.ValueType      `json:"valueType"`
	Status       model.LifecycleStatus `json:"status"`
	DefaultValue json.RawMessage      `json:"defaultValue"`
	Config       flagConfigDefinition `json:"config"`
}

type flagConfigDefinition struct {
	Enabled            bool                      `json:"enabled"`
	FallthroughVariant string                    `json:"fallthroughVariant"`
	OffVariant         string                    `json:"offVariant"`
	Variants           []variantDefinition       `json:"variants"`
	TargetingRules     []targetingRuleDefinition `json:"targetingRules"`
}

type variantDefinition struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

type targetingRuleDefinition struct {
	Variant    string                `json:"variant"`
	Percentage int                   `json:"percentage"`
	Conditions []conditionDefinition `json:"conditions"`
}

type conditionDefinition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     string `json:"value"`
}

type segmentDefinition struct {
	Key        string                `json:"key"`
	Conditions []conditionDefinition `json:"conditions"`
}

// Handle serves GET /api/v1/definitions.
func (h *DefinitionsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	sdkKey := auth.SDKKeyFromContext(r.Context())

	flags := h.cache.GetFlags(sdkKey.ProjectKey, sdkKey.EnvironmentKey)
	segments := h.cache.GetSegments(sdkKey.ProjectKey)

	flagDefs := make([]flagDefinition, 0, len(flags))
	for _, fd := range flags {
		flagDefs = append(flagDefs, convertFlag(fd))
	}

	segDefs := make([]segmentDefinition, 0, len(segments))
	for _, seg := range segments {
		segDefs = append(segDefs, convertSegment(seg))
	}

	writeJSON(w, http.StatusOK, definitionsResponse{
		Flags:    flagDefs,
		Segments: segDefs,
	})
}

func convertFlag(fd evaluation.FlagData) flagDefinition {
	variants := make([]variantDefinition, 0, len(fd.Config.Variants))
	for _, v := range fd.Config.Variants {
		variants = append(variants, variantDefinition{
			Name:  v.Name,
			Value: v.Value,
		})
	}

	rules := make([]targetingRuleDefinition, 0, len(fd.Config.TargetingRules))
	for _, tr := range fd.Config.TargetingRules {
		conditions := convertConditions(tr.Conditions)
		rules = append(rules, targetingRuleDefinition{
			Variant:    tr.Variant,
			Percentage: derefInt(tr.PercentageRollout),
			Conditions: conditions,
		})
	}

	return flagDefinition{
		Key:          fd.Flag.Key,
		ValueType:    fd.Flag.ValueType,
		Status:       fd.Flag.LifecycleStatus,
		DefaultValue: fd.Flag.DefaultValue,
		Config: flagConfigDefinition{
			Enabled:            fd.Config.Enabled,
			FallthroughVariant: fd.Config.FallthroughVariant,
			OffVariant:         fd.Config.OffVariant,
			Variants:           variants,
			TargetingRules:     rules,
		},
	}
}

func convertSegment(seg model.Segment) segmentDefinition {
	return segmentDefinition{
		Key:        seg.Key,
		Conditions: convertConditions(seg.Conditions),
	}
}

func convertConditions(conditions []model.Condition) []conditionDefinition {
	out := make([]conditionDefinition, 0, len(conditions))
	for _, c := range conditions {
		out = append(out, conditionDefinition{
			Attribute: c.Attribute,
			Operator:  c.Operator,
			Value:     conditionValueToString(c.Value),
		})
	}
	return out
}

func conditionValueToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func derefInt(p *int) int {
	if p == nil {
		return 100
	}
	return *p
}
