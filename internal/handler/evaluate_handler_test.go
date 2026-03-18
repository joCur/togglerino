package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

func TestEvaluateHandler_PersonalOverride(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewEvaluateHandler(cache, engine, nil, nil, nil, nil)

	// Set up a flag
	cache.SetFlag("proj", "dev", "my-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "my-flag",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
			Variants:        []model.Variant{{Name: "off", Value: rawJSON(false)}},
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:            true,
			FallthroughVariant: "off",
		},
	})

	// Set a personal override for app-user-1
	cache.SetOverride("proj", "dev", "app-user-1", "my-flag", json.RawMessage(`true`), nil)

	// Evaluate with the overridden user
	body := `{"context":{"user_id":"app-user-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/my-flag", bytes.NewBufferString(body))
	req.SetPathValue("flag", "my-flag")

	sdkKey := &model.SDKKey{
		ProjectKey:     "proj",
		EnvironmentKey: "dev",
	}
	ctx := auth.ContextWithSDKKey(req.Context(), sdkKey)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateSingle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result model.EvaluationResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Value != true {
		t.Fatalf("expected true, got %v", result.Value)
	}
	if result.Reason != "override" {
		t.Fatalf("expected reason 'override', got %q", result.Reason)
	}
	if result.Variant != "override" {
		t.Fatalf("expected variant 'override', got %q", result.Variant)
	}
}

func TestEvaluateHandler_NoOverrideFallsThrough(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewEvaluateHandler(cache, engine, nil, nil, nil, nil)

	cache.SetFlag("proj", "dev", "my-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "my-flag",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
			Variants:        []model.Variant{{Name: "off", Value: rawJSON(false)}},
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:            true,
			FallthroughVariant: "off",
		},
	})

	// No override set - evaluate should fall through to normal evaluation
	body := `{"context":{"user_id":"no-override-user"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/my-flag", bytes.NewBufferString(body))
	req.SetPathValue("flag", "my-flag")

	sdkKey := &model.SDKKey{
		ProjectKey:     "proj",
		EnvironmentKey: "dev",
	}
	ctx := auth.ContextWithSDKKey(req.Context(), sdkKey)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateSingle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result model.EvaluationResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Reason == "override" {
		t.Fatal("expected normal evaluation, not override")
	}
}

func TestEvaluateHandler_PersonalOverride_EvaluateAll(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewEvaluateHandler(cache, engine, nil, nil, nil, nil)

	cache.SetFlag("proj", "dev", "flag-a", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-a",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
			Variants:        []model.Variant{{Name: "off", Value: rawJSON(false)}},
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:            true,
			FallthroughVariant: "off",
		},
	})
	cache.SetFlag("proj", "dev", "flag-b", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-b",
			ValueType:       "string",
			DefaultValue:    rawJSON("default"),
			LifecycleStatus: model.LifecycleActive,
			Variants:        []model.Variant{{Name: "default", Value: rawJSON("default")}},
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:            true,
			FallthroughVariant: "default",
		},
	})

	// Override only flag-a
	cache.SetOverride("proj", "dev", "bulk-user", "flag-a", json.RawMessage(`true`), nil)

	body := `{"context":{"user_id":"bulk-user"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate", bytes.NewBufferString(body))
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp evaluateAllResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// flag-a should be overridden
	a := resp.Flags["flag-a"]
	if a == nil {
		t.Fatal("flag-a missing from results")
	}
	if a.Value != true {
		t.Fatalf("flag-a: expected true, got %v", a.Value)
	}
	if a.Reason != "override" {
		t.Fatalf("flag-a: expected reason 'override', got %q", a.Reason)
	}
	if a.Variant != "override" {
		t.Fatalf("flag-a: expected variant 'override', got %q", a.Variant)
	}

	// flag-b should be normal evaluation
	b := resp.Flags["flag-b"]
	if b == nil {
		t.Fatal("flag-b missing from results")
	}
	if b.Reason == "override" {
		t.Fatal("flag-b should not be overridden")
	}
}

func TestEvaluateHandler_OverrideSkipsArchivedFlag(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewEvaluateHandler(cache, engine, nil, nil, nil, nil)

	cache.SetFlag("proj", "dev", "archived-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "archived-flag",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleArchived,
			Variants:        []model.Variant{{Name: "off", Value: rawJSON(false)}},
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:            true,
			FallthroughVariant: "off",
		},
	})

	// Set override for the archived flag
	cache.SetOverride("proj", "dev", "user-1", "archived-flag", json.RawMessage(`true`), nil)

	// EvaluateSingle: override should be skipped for archived flag
	body := `{"context":{"user_id":"user-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/archived-flag", bytes.NewBufferString(body))
	req.SetPathValue("flag", "archived-flag")
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateSingle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result model.EvaluationResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Reason != "archived" {
		t.Fatalf("expected reason 'archived', got %q", result.Reason)
	}
	if result.Value != false {
		t.Fatalf("expected default value false for archived flag, got %v", result.Value)
	}

	// EvaluateAll: override should also be skipped for archived flag
	req = httptest.NewRequest(http.MethodPost, "/api/v1/evaluate", bytes.NewBufferString(body))
	ctx = auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec = httptest.NewRecorder()
	h.EvaluateAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp evaluateAllResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	af := resp.Flags["archived-flag"]
	if af == nil {
		t.Fatal("archived-flag missing from results")
	}
	if af.Reason != "archived" {
		t.Fatalf("EvaluateAll: expected reason 'archived', got %q", af.Reason)
	}
}
