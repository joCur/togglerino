package handler

import (
	"net/http"
	"sort"

	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
)

// PlaygroundHandler handles playground evaluation requests with detailed traces.
type PlaygroundHandler struct {
	cache  *evaluation.Cache
	engine *evaluation.Engine
}

// NewPlaygroundHandler creates a new PlaygroundHandler.
func NewPlaygroundHandler(cache *evaluation.Cache, engine *evaluation.Engine) *PlaygroundHandler {
	return &PlaygroundHandler{cache: cache, engine: engine}
}

type playgroundRequest struct {
	EnvironmentKey string                   `json:"environment_key"`
	FlagKey        string                   `json:"flag_key,omitempty"`
	Context        *model.EvaluationContext `json:"context,omitempty"`
}

type playgroundResponse struct {
	Results []*model.EvaluationTrace `json:"results"`
}

// Evaluate handles POST requests to evaluate flags with detailed traces.
func (h *PlaygroundHandler) Evaluate(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")

	var req playgroundRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "environment_key is required")
		return
	}

	if req.Context == nil {
		req.Context = &model.EvaluationContext{
			UserID:     "",
			Attributes: map[string]any{},
		}
	}
	if req.Context.Attributes == nil {
		req.Context.Attributes = map[string]any{}
	}

	segments := h.cache.GetSegments(projectKey)

	if req.FlagKey != "" {
		fd, ok := h.cache.GetFlag(projectKey, req.EnvironmentKey, req.FlagKey)
		if !ok {
			writeError(w, http.StatusNotFound, "flag not found")
			return
		}

		trace := h.engine.EvaluateWithTrace(&fd.Flag, &fd.Config, req.Context, segments)
		writeJSON(w, http.StatusOK, playgroundResponse{Results: []*model.EvaluationTrace{trace}})
		return
	}

	// Evaluate all flags for the project/environment.
	flags := h.cache.GetFlags(projectKey, req.EnvironmentKey)
	results := make([]*model.EvaluationTrace, 0, len(flags))
	for _, fd := range flags {
		trace := h.engine.EvaluateWithTrace(&fd.Flag, &fd.Config, req.Context, segments)
		results = append(results, trace)
	}

	// Sort by flag key for deterministic output.
	sort.Slice(results, func(i, j int) bool {
		return results[i].FlagKey < results[j].FlagKey
	})

	writeJSON(w, http.StatusOK, playgroundResponse{Results: results})
}
