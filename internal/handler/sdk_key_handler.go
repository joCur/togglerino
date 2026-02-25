package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type SDKKeyHandler struct {
	sdkKeys      *store.SDKKeyStore
	environments *store.EnvironmentStore
	projects     *store.ProjectStore
}

func NewSDKKeyHandler(sdkKeys *store.SDKKeyStore, environments *store.EnvironmentStore, projects *store.ProjectStore) *SDKKeyHandler {
	return &SDKKeyHandler{sdkKeys: sdkKeys, environments: environments, projects: projects}
}

// Create handles POST /api/v1/projects/{key}/environments/{env}/sdk-keys
func (h *SDKKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	envKey := r.PathValue("env")
	if envKey == "" {
		writeError(w, http.StatusBadRequest, "environment key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	sdkKey, err := h.sdkKeys.Create(r.Context(), env.ID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create SDK key")
		return
	}

	writeJSON(w, http.StatusCreated, sdkKey)
}

// List handles GET /api/v1/projects/{key}/environments/{env}/sdk-keys
func (h *SDKKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	envKey := r.PathValue("env")
	if envKey == "" {
		writeError(w, http.StatusBadRequest, "environment key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	keys, err := h.sdkKeys.ListByEnvironment(r.Context(), env.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list SDK keys")
		return
	}
	if keys == nil {
		keys = []model.SDKKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

// Revoke handles DELETE /api/v1/projects/{key}/environments/{env}/sdk-keys/{id}
func (h *SDKKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "SDK key id is required")
		return
	}

	if err := h.sdkKeys.Revoke(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke SDK key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
