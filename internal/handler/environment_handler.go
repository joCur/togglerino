package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/webhook"
)

type EnvironmentHandler struct {
	environments *store.EnvironmentStore
	projects     *store.ProjectStore
	webhooks     *webhook.Dispatcher
}

func NewEnvironmentHandler(environments *store.EnvironmentStore, projects *store.ProjectStore, webhooks *webhook.Dispatcher) *EnvironmentHandler {
	return &EnvironmentHandler{environments: environments, projects: projects, webhooks: webhooks}
}

// Create handles POST /api/v1/projects/{key}/environments
func (h *EnvironmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}

	env, err := h.environments.Create(r.Context(), project.ID, req.Key, req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "environment key already exists for this project")
			return
		}
		slog.Error("failed to create environment", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create environment")
		return
	}

	if h.webhooks != nil {
		envJSON, _ := json.Marshal(env)
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventEnvironmentCreated,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    envJSON,
		})
	}

	writeJSON(w, http.StatusCreated, env)
}

// List handles GET /api/v1/projects/{key}/environments
func (h *EnvironmentHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		slog.Error("failed to list environments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	if envs == nil {
		envs = []model.Environment{}
	}
	writeJSON(w, http.StatusOK, envs)
}

// UpdateOrder handles PUT /api/v1/projects/{key}/environments/order
func (h *EnvironmentHandler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		EnvironmentIDs []string `json:"environment_ids"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.EnvironmentIDs) == 0 {
		writeError(w, http.StatusBadRequest, "environment_ids is required")
		return
	}

	if err := h.environments.UpdateOrder(r.Context(), project.ID, req.EnvironmentIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		slog.Error("failed to list environments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	writeJSON(w, http.StatusOK, envs)
}
