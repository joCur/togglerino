package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/metrics"
	"github.com/togglerino/togglerino/internal/model"

	dto "github.com/prometheus/client_model/go"
)

func collectCounterValue(c interface{ Write(*dto.Metric) error }) float64 {
	m := &dto.Metric{}
	c.Write(m)
	return m.GetCounter().GetValue()
}

func TestEvaluateAll_RecordsMetrics(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	reg := metrics.NewRegistry()
	h := NewEvaluateHandler(cache, engine, nil, nil, reg, nil)

	cache.SetFlag("proj", "dev", "flag-1", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-1",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled: true,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate", bytes.NewBufferString(`{}`))
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Boolean flags return empty variant
	counter, err := reg.EvaluationsTotal.GetMetricWithLabelValues("proj", "dev", "flag-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectCounterValue(counter); got != 1 {
		t.Errorf("expected 1 evaluation, got %f", got)
	}
}

func TestEvaluateSingle_RecordsMetrics(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	reg := metrics.NewRegistry()
	h := NewEvaluateHandler(cache, engine, nil, nil, reg, nil)

	cache.SetFlag("proj", "dev", "my-flag", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "my-flag",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled: true,
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate/my-flag", bytes.NewBufferString(`{}`))
	req.SetPathValue("flag", "my-flag")
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateSingle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Boolean flags return empty variant
	counter, err := reg.EvaluationsTotal.GetMetricWithLabelValues("proj", "dev", "my-flag", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectCounterValue(counter); got != 1 {
		t.Errorf("expected 1 evaluation, got %f", got)
	}
}

func TestEvaluateAll_OverrideRecordsMetrics(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	reg := metrics.NewRegistry()
	h := NewEvaluateHandler(cache, engine, nil, nil, reg, nil)

	cache.SetFlag("proj", "dev", "flag-a", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-a",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			DefaultVariant: "off",
			Variants:       []model.Variant{{Key: "off", Value: rawJSON(false)}},
		},
	})

	cache.SetOverride("proj", "dev", "user-1", "flag-a", json.RawMessage(`true`), nil)

	body := `{"context":{"user_id":"user-1"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate", bytes.NewBufferString(body))
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Override should record with variant "override"
	counter, err := reg.EvaluationsTotal.GetMetricWithLabelValues("proj", "dev", "flag-a", "override")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectCounterValue(counter); got != 1 {
		t.Errorf("expected 1 evaluation for override, got %f", got)
	}
}

func TestEvaluateHandler_NilMetricsDoesNotPanic(t *testing.T) {
	cache := evaluation.NewCache()
	engine := evaluation.NewEngine()
	h := NewEvaluateHandler(cache, engine, nil, nil, nil, nil)

	cache.SetFlag("proj", "dev", "flag-1", evaluation.FlagData{
		Flag: model.Flag{
			Key:             "flag-1",
			ValueType:       "boolean",
			DefaultValue:    rawJSON(false),
			LifecycleStatus: model.LifecycleActive,
		},
		Config: model.FlagEnvironmentConfig{
			Enabled:        true,
			DefaultVariant: "off",
			Variants:       []model.Variant{{Key: "off", Value: rawJSON(false)}},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluate", bytes.NewBufferString(`{}`))
	ctx := auth.ContextWithSDKKey(req.Context(), &model.SDKKey{ProjectKey: "proj", EnvironmentKey: "dev"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.EvaluateAll(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
