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
	h := NewEvaluateHandler(cache, engine, nil, nil)

	// Set up a flag
	cache.SetFlag("proj", "dev", "my-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "my-flag",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			DefaultVariant: "off",
			Variants: []model.Variant{
				{Key: "off", Value: rawJSON(false)},
			},
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
}

func TestEvaluateHandler_NoOverrideFallsThrough(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewEvaluateHandler(cache, engine, nil, nil)

	cache.SetFlag("proj", "dev", "my-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "my-flag",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			DefaultVariant: "off",
			Variants: []model.Variant{
				{Key: "off", Value: rawJSON(false)},
			},
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
