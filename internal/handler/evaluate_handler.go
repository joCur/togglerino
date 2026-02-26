package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// EvaluateHandler handles flag evaluation requests from SDKs.
type EvaluateHandler struct {
	cache        *evaluation.Cache
	engine       *evaluation.Engine
	unknownFlags *store.UnknownFlagStore
}

// NewEvaluateHandler creates a new EvaluateHandler.
func NewEvaluateHandler(cache *evaluation.Cache, engine *evaluation.Engine, unknownFlags *store.UnknownFlagStore) *EvaluateHandler {
	return &EvaluateHandler{cache: cache, engine: engine, unknownFlags: unknownFlags}
}

type evaluateRequest struct {
	Context *model.EvaluationContext `json:"context"`
}

type evaluateAllResponse struct {
	Flags map[string]*model.EvaluationResult `json:"flags"`
}

// EvaluateAll evaluates all flags for the SDK key's project/environment.
// POST /api/v1/evaluate
func (h *EvaluateHandler) EvaluateAll(w http.ResponseWriter, r *http.Request) {
	sdkKey := auth.SDKKeyFromContext(r.Context())

	evalCtx := h.parseContext(r)

	flags := h.cache.GetFlags(sdkKey.ProjectKey, sdkKey.EnvironmentKey)
	results := make(map[string]*model.EvaluationResult, len(flags))
	for flagKey, fd := range flags {
		results[flagKey] = h.engine.Evaluate(&fd.Flag, &fd.Config, evalCtx)
	}

	writeJSON(w, http.StatusOK, evaluateAllResponse{Flags: results})
}

// EvaluateSingle evaluates a single flag for the SDK key's project/environment.
// POST /api/v1/evaluate/{flag}
func (h *EvaluateHandler) EvaluateSingle(w http.ResponseWriter, r *http.Request) {
	flagKey := r.PathValue("flag")

	sdkKey := auth.SDKKeyFromContext(r.Context())
	evalCtx := h.parseContext(r)

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

	result := h.engine.Evaluate(&fd.Flag, &fd.Config, evalCtx)
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
