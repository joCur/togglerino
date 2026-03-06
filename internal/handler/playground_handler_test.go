package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func setupPlaygroundTest() (*PlaygroundHandler, *evaluation.Cache) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewPlaygroundHandler(cache, engine)
	return h, cache
}

func TestPlaygroundHandler_SingleFlag(t *testing.T) {
	h, cache := setupPlaygroundTest()

	cache.SetFlag("myproject", "production", "my-flag", evaluation.FlagData{
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
				{Key: "on", Value: rawJSON(true)},
			},
		},
	})

	body := `{"environment_key":"production","flag_key":"my-flag"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	rec := httptest.NewRecorder()

	h.Evaluate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp playgroundResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}

	trace := resp.Results[0]
	if trace.FlagKey != "my-flag" {
		t.Errorf("expected flag_key 'my-flag', got %q", trace.FlagKey)
	}
	if trace.Reason != "default" {
		t.Errorf("expected reason 'default', got %q", trace.Reason)
	}
}

func TestPlaygroundHandler_MissingEnvironmentKey(t *testing.T) {
	h, _ := setupPlaygroundTest()

	body := `{"flag_key":"some-flag"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	rec := httptest.NewRecorder()

	h.Evaluate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlaygroundHandler_FlagNotFound(t *testing.T) {
	h, _ := setupPlaygroundTest()

	body := `{"environment_key":"production","flag_key":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	rec := httptest.NewRecorder()

	h.Evaluate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPlaygroundHandler_AllFlags(t *testing.T) {
	h, cache := setupPlaygroundTest()

	flags := map[string]evaluation.FlagData{
		"flag-a": {
			Flag: model.Flag{
				Key:             "flag-a",
				ValueType:       "boolean",
				DefaultValue:    rawJSON(false),
				LifecycleStatus: model.LifecycleActive,
			},
			Config: model.FlagEnvironmentConfig{
				Enabled:        true,
				DefaultVariant: "off",
				Variants: []model.Variant{
					{Key: "off", Value: rawJSON(false)},
					{Key: "on", Value: rawJSON(true)},
				},
			},
		},
		"flag-b": {
			Flag: model.Flag{
				Key:             "flag-b",
				ValueType:       "string",
				DefaultValue:    rawJSON("default"),
				LifecycleStatus: model.LifecycleActive,
			},
			Config: model.FlagEnvironmentConfig{
				Enabled:        false,
				DefaultVariant: "off",
				Variants: []model.Variant{
					{Key: "off", Value: rawJSON("off-val")},
					{Key: "on", Value: rawJSON("on-val")},
				},
			},
		},
	}
	cache.Set("myproject", "staging", flags)

	body := `{"environment_key":"staging"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/myproject/playground", bytes.NewBufferString(body))
	req.SetPathValue("key", "myproject")
	rec := httptest.NewRecorder()

	h.Evaluate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp playgroundResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}

	// Verify both flags are present.
	keys := map[string]bool{}
	for _, r := range resp.Results {
		keys[r.FlagKey] = true
	}
	if !keys["flag-a"] || !keys["flag-b"] {
		t.Errorf("expected both flag-a and flag-b in results, got %v", keys)
	}
}
