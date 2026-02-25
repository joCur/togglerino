package handler

import (
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type EnvironmentHandler struct {
	environments *store.EnvironmentStore
	projects     *store.ProjectStore
}

func NewEnvironmentHandler(environments *store.EnvironmentStore, projects *store.ProjectStore) *EnvironmentHandler {
	return &EnvironmentHandler{environments: environments, projects: projects}
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
		writeError(w, http.StatusInternalServerError, "failed to create environment")
		return
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
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	if envs == nil {
		envs = []model.Environment{}
	}
	writeJSON(w, http.StatusOK, envs)
}
