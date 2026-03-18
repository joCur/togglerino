package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

func setupDefinitionsTest() (*DefinitionsHandler, *evaluation.Cache) {
	cache := evaluation.NewCache()
	h := NewDefinitionsHandler(cache)
	return h, cache
}

func makeDefinitionsRequest(h *DefinitionsHandler, projectKey, envKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/definitions", nil)
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{
		ProjectKey:     projectKey,
		EnvironmentKey: envKey,
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.Handle(rec, req)
	return rec
}

func TestDefinitions_ReturnsFlagsAndSegments(t *testing.T) {
	h, cache := setupDefinitionsTest()

	fifty := 50
	cache.SetFlag("proj", "dev", "flag-bool", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-bool",
			ValueType:       model.ValueTypeBoolean,
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			FallthroughVariant: "on",
			Variants: []model.Variant{
				{Name: "on", Value: rawJSON(true)},
				{Name: "off", Value: rawJSON(false)},
			},
			TargetingRules: []model.TargetingRule{
				{
					Variant:           "on",
					PercentageRollout: &fifty,
					Conditions: []model.Condition{
						{Attribute: "country", Operator: "in", Value: []any{"US", "CA"}},
					},
				},
			},
		},
	})

	cache.SetSegments("proj", map[string]model.Segment{
		"beta-users": {
			Key: "beta-users",
			Conditions: []model.Condition{
				{Attribute: "email", Operator: "ends_with", Value: "@beta.com"},
			},
		},
	})

	rec := makeDefinitionsRequest(h, "proj", "dev")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp definitionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify flags
	if len(resp.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(resp.Flags))
	}
	f := resp.Flags[0]
	if f.Key != "flag-bool" {
		t.Errorf("expected flag key 'flag-bool', got %q", f.Key)
	}
	if f.ValueType != model.ValueTypeBoolean {
		t.Errorf("expected valueType 'boolean', got %q", f.ValueType)
	}
	if f.Status != model.LifecycleActive {
		t.Errorf("expected status 'active', got %q", f.Status)
	}
	if !f.Config.Enabled {
		t.Error("expected config.enabled to be true")
	}
	if f.Config.FallthroughVariant != "on" {
		t.Errorf("expected fallthroughVariant 'on', got %q", f.Config.FallthroughVariant)
	}
	if len(f.Config.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(f.Config.Variants))
	}

	// Verify targeting rules
	if len(f.Config.TargetingRules) != 1 {
		t.Fatalf("expected 1 targeting rule, got %d", len(f.Config.TargetingRules))
	}
	rule := f.Config.TargetingRules[0]
	if rule.Variant != "on" {
		t.Errorf("expected rule variant 'on', got %q", rule.Variant)
	}
	if rule.Percentage != 50 {
		t.Errorf("expected percentage 50, got %d", rule.Percentage)
	}
	if len(rule.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(rule.Conditions))
	}
	cond := rule.Conditions[0]
	if cond.Attribute != "country" {
		t.Errorf("expected attribute 'country', got %q", cond.Attribute)
	}
	if cond.Operator != "in" {
		t.Errorf("expected operator 'in', got %q", cond.Operator)
	}
	if cond.Value != `["US","CA"]` {
		t.Errorf("expected value '[\"US\",\"CA\"]', got %q", cond.Value)
	}

	// Verify segments
	if len(resp.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(resp.Segments))
	}
	seg := resp.Segments[0]
	if seg.Key != "beta-users" {
		t.Errorf("expected segment key 'beta-users', got %q", seg.Key)
	}
	if len(seg.Conditions) != 1 {
		t.Fatalf("expected 1 segment condition, got %d", len(seg.Conditions))
	}
	if seg.Conditions[0].Attribute != "email" {
		t.Errorf("expected segment condition attribute 'email', got %q", seg.Conditions[0].Attribute)
	}
	if seg.Conditions[0].Operator != "ends_with" {
		t.Errorf("expected segment condition operator 'ends_with', got %q", seg.Conditions[0].Operator)
	}
	if seg.Conditions[0].Value != "@beta.com" {
		t.Errorf("expected segment condition value '@beta.com', got %q", seg.Conditions[0].Value)
	}
}

func TestDefinitions_EmptyWhenNoFlags(t *testing.T) {
	h, _ := setupDefinitionsTest()

	rec := makeDefinitionsRequest(h, "empty-proj", "dev")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp definitionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Flags) != 0 {
		t.Errorf("expected 0 flags, got %d", len(resp.Flags))
	}
	if len(resp.Segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(resp.Segments))
	}
}

func TestDefinitions_IncludesArchivedFlags(t *testing.T) {
	h, cache := setupDefinitionsTest()

	cache.SetFlag("proj", "dev", "active-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "active-flag",
			ValueType:       model.ValueTypeBoolean,
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			FallthroughVariant: "on",
			Variants:       []model.Variant{{Name: "on", Value: rawJSON(true)}},
		},
	})
	cache.SetFlag("proj", "dev", "archived-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "archived-flag",
			ValueType:       model.ValueTypeString,
			LifecycleStatus: model.LifecycleArchived,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        false,
			FallthroughVariant: "default",
			Variants:       []model.Variant{{Name: "default", Value: rawJSON("hello")}},
		},
	})

	rec := makeDefinitionsRequest(h, "proj", "dev")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp definitionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Flags) != 2 {
		t.Fatalf("expected 2 flags (including archived), got %d", len(resp.Flags))
	}

	found := map[string]bool{}
	for _, f := range resp.Flags {
		found[f.Key] = true
		if f.Key == "archived-flag" && f.Status != model.LifecycleArchived {
			t.Errorf("expected archived flag status 'archived', got %q", f.Status)
		}
	}
	if !found["active-flag"] {
		t.Error("expected active-flag in response")
	}
	if !found["archived-flag"] {
		t.Error("expected archived-flag in response")
	}
}

func TestDefinitions_SegmentConditions(t *testing.T) {
	h, cache := setupDefinitionsTest()

	cache.SetSegments("proj", map[string]model.Segment{
		"enterprise": {
			Key: "enterprise",
			Conditions: []model.Condition{
				{Attribute: "plan", Operator: "equals", Value: "enterprise"},
				{Attribute: "seats", Operator: "greater_than", Value: "10"},
			},
		},
	})

	// Need at least an empty flags map so the project/env is "valid"
	cache.Set("proj", "dev", map[string]evaluation.FlagData{})

	rec := makeDefinitionsRequest(h, "proj", "dev")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp definitionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(resp.Segments))
	}

	seg := resp.Segments[0]
	if seg.Key != "enterprise" {
		t.Errorf("expected segment key 'enterprise', got %q", seg.Key)
	}
	if len(seg.Conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(seg.Conditions))
	}

	// Verify first condition
	if seg.Conditions[0].Attribute != "plan" {
		t.Errorf("expected attribute 'plan', got %q", seg.Conditions[0].Attribute)
	}
	if seg.Conditions[0].Operator != "equals" {
		t.Errorf("expected operator 'equals', got %q", seg.Conditions[0].Operator)
	}
	if seg.Conditions[0].Value != "enterprise" {
		t.Errorf("expected value 'enterprise', got %q", seg.Conditions[0].Value)
	}

	// Verify second condition
	if seg.Conditions[1].Attribute != "seats" {
		t.Errorf("expected attribute 'seats', got %q", seg.Conditions[1].Attribute)
	}
	if seg.Conditions[1].Operator != "greater_than" {
		t.Errorf("expected operator 'greater_than', got %q", seg.Conditions[1].Operator)
	}
	if seg.Conditions[1].Value != "10" {
		t.Errorf("expected value '10', got %q", seg.Conditions[1].Value)
	}
}

func TestDefinitions_NilPercentageDefaultsTo100(t *testing.T) {
	h, cache := setupDefinitionsTest()

	cache.SetFlag("proj", "dev", "flag-no-pct", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-no-pct",
			ValueType:       model.ValueTypeBoolean,
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			FallthroughVariant: "on",
			Variants:       []model.Variant{{Name: "on", Value: rawJSON(true)}},
			TargetingRules: []model.TargetingRule{
				{
					Variant:           "on",
					PercentageRollout: nil, // nil should default to 100
					Conditions: []model.Condition{
						{Attribute: "country", Operator: "equals", Value: "US"},
					},
				},
			},
		},
	})

	rec := makeDefinitionsRequest(h, "proj", "dev")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp definitionsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(resp.Flags))
	}
	if len(resp.Flags[0].Config.TargetingRules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(resp.Flags[0].Config.TargetingRules))
	}
	if resp.Flags[0].Config.TargetingRules[0].Percentage != 100 {
		t.Errorf("expected percentage 100 for nil rollout, got %d", resp.Flags[0].Config.TargetingRules[0].Percentage)
	}
}
