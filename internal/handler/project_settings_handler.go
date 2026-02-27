package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ProjectSettingsHandler struct {
	settings *store.ProjectSettingsStore
	projects *store.ProjectStore
}

func NewProjectSettingsHandler(settings *store.ProjectSettingsStore, projects *store.ProjectStore) *ProjectSettingsHandler {
	return &ProjectSettingsHandler{settings: settings, projects: projects}
}

// Get handles GET /api/v1/projects/{key}/settings/flags
func (h *ProjectSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	settings, err := h.settings.Get(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project settings")
		return
	}

	// Merge with defaults for any missing keys
	merged := model.DefaultFlagLifetimes()
	if settings != nil && settings.FlagLifetimes != nil {
		for k, v := range settings.FlagLifetimes {
			merged[k] = v
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"flag_lifetimes": merged,
	})
}

// Update handles PUT /api/v1/projects/{key}/settings/flags
func (h *ProjectSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
		FlagLifetimes map[model.FlagType]*int `json:"flag_lifetimes"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for k, v := range req.FlagLifetimes {
		if !model.ValidFlagTypes[k] {
			writeError(w, http.StatusBadRequest, "invalid flag type key: "+string(k))
			return
		}
		if v != nil && *v <= 0 {
			writeError(w, http.StatusBadRequest, "flag lifetime for "+string(k)+" must be a positive integer")
			return
		}
	}

	settings, err := h.settings.Upsert(r.Context(), project.ID, req.FlagLifetimes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update project settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"flag_lifetimes": settings.FlagLifetimes,
	})
}
