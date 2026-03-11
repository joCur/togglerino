package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/metrics"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// EvaluateHandler handles flag evaluation requests from SDKs.
type EvaluateHandler struct {
	cache        *evaluation.Cache
	engine       *evaluation.Engine
	unknownFlags *store.UnknownFlagStore
	contextAttrs *store.ContextAttributeStore
	metrics      *metrics.Registry
	tracker      *evaluation.Tracker
}

// NewEvaluateHandler creates a new EvaluateHandler.
func NewEvaluateHandler(cache *evaluation.Cache, engine *evaluation.Engine, unknownFlags *store.UnknownFlagStore, contextAttrs *store.ContextAttributeStore, metricsReg *metrics.Registry, tracker *evaluation.Tracker) *EvaluateHandler {
	return &EvaluateHandler{cache: cache, engine: engine, unknownFlags: unknownFlags, contextAttrs: contextAttrs, metrics: metricsReg, tracker: tracker}
}

type evaluateRequest struct {
	Context *model.EvaluationContext `json:"context"`
}

type evaluateAllResponse struct {
	Flags map[string]*model.EvaluationResult `json:"flags"`
}

// trackAttributes asynchronously records the context attribute names sent
// by SDK clients so the management UI can offer autocomplete suggestions.
func (h *EvaluateHandler) trackAttributes(projectKey string, evalCtx *model.EvaluationContext) {
	if len(evalCtx.Attributes) == 0 {
		return
	}

	names := make([]string, 0, len(evalCtx.Attributes))
	for k := range evalCtx.Attributes {
		names = append(names, k)
	}

	go func() {
		if err := h.contextAttrs.UpsertByProjectKey(context.Background(), projectKey, names); err != nil {
			slog.Error("tracking context attributes", "error", err, "project", projectKey)
		}
	}()
}

// EvaluateAll evaluates all flags for the SDK key's project/environment.
// POST /api/v1/evaluate
func (h *EvaluateHandler) EvaluateAll(w http.ResponseWriter, r *http.Request) {
	sdkKey := auth.SDKKeyFromContext(r.Context())

	evalCtx := h.parseContext(r)
	h.trackAttributes(sdkKey.ProjectKey, evalCtx)

	start := time.Now() // after JSON parsing, measures evaluation only
	flags := h.cache.GetFlags(sdkKey.ProjectKey, sdkKey.EnvironmentKey)
	segments := h.cache.GetSegments(sdkKey.ProjectKey)
	results := make(map[string]*model.EvaluationResult, len(flags))
	for flagKey, fd := range flags {
		// Personal overrides bypass disabled flags (by design) but respect archived flags.
		if evalCtx.UserID != "" && fd.Flag.LifecycleStatus != model.LifecycleArchived {
			if overrideVal, ok := h.cache.GetOverride(sdkKey.ProjectKey, sdkKey.EnvironmentKey, evalCtx.UserID, flagKey); ok {
				results[flagKey] = &model.EvaluationResult{
					Value:   rawToAny(overrideVal),
					Variant: "override",
					Reason:  "override",
				}
				if h.metrics != nil {
					h.metrics.RecordEvaluation(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey, "override")
				}
				continue
			}
		}
		result := h.engine.EvaluateWithSegments(&fd.Flag, &fd.Config, evalCtx, segments)
		results[flagKey] = result
		if h.metrics != nil {
			h.metrics.RecordEvaluation(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey, result.Variant)
		}
	}

	if h.tracker != nil {
		for _, fd := range flags {
			h.tracker.Track(fd.Flag.ID)
		}
	}

	if h.metrics != nil {
		h.metrics.ObserveEvaluationDuration(time.Since(start).Seconds())
	}

	writeJSON(w, http.StatusOK, evaluateAllResponse{Flags: results})
}

// EvaluateSingle evaluates a single flag for the SDK key's project/environment.
// POST /api/v1/evaluate/{flag}
func (h *EvaluateHandler) EvaluateSingle(w http.ResponseWriter, r *http.Request) {
	flagKey := r.PathValue("flag")

	sdkKey := auth.SDKKeyFromContext(r.Context())
	evalCtx := h.parseContext(r)
	h.trackAttributes(sdkKey.ProjectKey, evalCtx)

	start := time.Now() // after JSON parsing, measures evaluation only
	fd, ok := h.cache.GetFlag(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey)
	if !ok {
		// Best-effort unknown flag tracking
		go func() {
			if err := h.unknownFlags.Upsert(context.Background(), sdkKey.ProjectID, sdkKey.EnvironmentID, flagKey); err != nil {
				slog.Warn("failed to track unknown flag", "flag_key", flagKey, "error", err)
			}
		}()
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	if h.tracker != nil {
		h.tracker.Track(fd.Flag.ID)
	}

	// Personal overrides bypass disabled flags (by design) but respect archived flags.
	if evalCtx.UserID != "" && fd.Flag.LifecycleStatus != model.LifecycleArchived {
		if overrideVal, ok := h.cache.GetOverride(sdkKey.ProjectKey, sdkKey.EnvironmentKey, evalCtx.UserID, flagKey); ok {
			if h.metrics != nil {
				h.metrics.RecordEvaluation(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey, "override")
				h.metrics.ObserveEvaluationDuration(time.Since(start).Seconds())
			}
			writeJSON(w, http.StatusOK, &model.EvaluationResult{
				Value:   rawToAny(overrideVal),
				Variant: "override",
				Reason:  "override",
			})
			return
		}
	}

	segments := h.cache.GetSegments(sdkKey.ProjectKey)
	result := h.engine.EvaluateWithSegments(&fd.Flag, &fd.Config, evalCtx, segments)
	if h.metrics != nil {
		h.metrics.RecordEvaluation(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey, result.Variant)
		h.metrics.ObserveEvaluationDuration(time.Since(start).Seconds())
	}
	writeJSON(w, http.StatusOK, result)
}

// parseContext reads the evaluation context from the request body.
// If the body is empty or context is nil, returns an empty context.
func (h *EvaluateHandler) parseContext(r *http.Request) *model.EvaluationContext {
	var req evaluateRequest
	_ = readJSON(r, &req)

	if req.Context == nil {
		return &model.EvaluationContext{
			UserID:     "",
			Attributes: map[string]any{},
		}
	}

	if req.Context.Attributes == nil {
		req.Context.Attributes = map[string]any{}
	}

	return req.Context
}

// rawToAny converts a json.RawMessage to a native Go value.
func rawToAny(raw json.RawMessage) any {
	if raw == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return v
}
