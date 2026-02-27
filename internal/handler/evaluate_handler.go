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
	contextAttrs *store.ContextAttributeStore
}

// NewEvaluateHandler creates a new EvaluateHandler.
func NewEvaluateHandler(cache *evaluation.Cache, engine *evaluation.Engine, contextAttrs *store.ContextAttributeStore) *EvaluateHandler {
	return &EvaluateHandler{cache: cache, engine: engine, contextAttrs: contextAttrs}
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
	h.trackAttributes(sdkKey.ProjectKey, evalCtx)

	fd, ok := h.cache.GetFlag(sdkKey.ProjectKey, sdkKey.EnvironmentKey, flagKey)
	if !ok {
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
