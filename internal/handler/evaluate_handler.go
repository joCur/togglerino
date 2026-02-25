package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

// EvaluateHandler handles flag evaluation requests from SDKs.
type EvaluateHandler struct {
	cache  *evaluation.Cache
	engine *evaluation.Engine
}

// NewEvaluateHandler creates a new EvaluateHandler.
func NewEvaluateHandler(cache *evaluation.Cache, engine *evaluation.Engine) *EvaluateHandler {
	return &EvaluateHandler{cache: cache, engine: engine}
}

type evaluateRequest struct {
	Context *model.EvaluationContext `json:"context"`
}

type evaluateAllResponse struct {
	Flags map[string]*model.EvaluationResult `json:"flags"`
}

// EvaluateAll evaluates all flags for a project/environment and returns the results.
// POST /api/v1/evaluate/{project}/{env}
func (h *EvaluateHandler) EvaluateAll(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("project")
	envKey := r.PathValue("env")

	sdkKey := auth.SDKKeyFromContext(r.Context())
	if sdkKey.ProjectKey != projectKey || sdkKey.EnvironmentKey != envKey {
		writeError(w, http.StatusForbidden, "SDK key is not authorized for this project/environment")
		return
	}

	evalCtx := h.parseContext(r)

	flags := h.cache.GetFlags(projectKey, envKey)
	results := make(map[string]*model.EvaluationResult, len(flags))
	for flagKey, fd := range flags {
		results[flagKey] = h.engine.Evaluate(&fd.Flag, &fd.Config, evalCtx)
	}

	writeJSON(w, http.StatusOK, evaluateAllResponse{Flags: results})
}

// EvaluateSingle evaluates a single flag for a project/environment and returns the result.
// POST /api/v1/evaluate/{project}/{env}/{flag}
func (h *EvaluateHandler) EvaluateSingle(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("project")
	envKey := r.PathValue("env")
	flagKey := r.PathValue("flag")

	sdkKey := auth.SDKKeyFromContext(r.Context())
	if sdkKey.ProjectKey != projectKey || sdkKey.EnvironmentKey != envKey {
		writeError(w, http.StatusForbidden, "SDK key is not authorized for this project/environment")
		return
	}

	evalCtx := h.parseContext(r)

	fd, ok := h.cache.GetFlag(projectKey, envKey, flagKey)
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
