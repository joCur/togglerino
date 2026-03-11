package handler

import (
	"log/slog"
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ProjectSettingsHandler struct {
	settings     *store.ProjectSettingsStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
}

func NewProjectSettingsHandler(settings *store.ProjectSettingsStore, projects *store.ProjectStore, environments *store.EnvironmentStore) *ProjectSettingsHandler {
	return &ProjectSettingsHandler{settings: settings, projects: projects, environments: environments}
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
		slog.Error("failed to get project settings", "error", err)
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

	resp := map[string]any{
		"flag_lifetimes": merged,
	}
	if settings != nil && settings.UnevaluatedStaleAfterDays != nil {
		resp["unevaluated_stale_after_days"] = *settings.UnevaluatedStaleAfterDays
	}
	writeJSON(w, http.StatusOK, resp)
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
		FlagLifetimes             map[model.FlagType]*int `json:"flag_lifetimes"`
		UnevaluatedStaleAfterDays *int                    `json:"unevaluated_stale_after_days"`
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

	// Normalize: 0 or negative means disabled (nil)
	unevalDays := req.UnevaluatedStaleAfterDays
	if unevalDays != nil && *unevalDays <= 0 {
		unevalDays = nil
	}

	settings, err := h.settings.UpsertFlagSettings(r.Context(), project.ID, req.FlagLifetimes, unevalDays)
	if err != nil {
		slog.Error("failed to update project settings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update project settings")
		return
	}

	resp := map[string]any{
		"flag_lifetimes": settings.FlagLifetimes,
	}
	if settings.UnevaluatedStaleAfterDays != nil {
		resp["unevaluated_stale_after_days"] = *settings.UnevaluatedStaleAfterDays
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetEnvironmentDefaults handles GET /api/v1/projects/{key}/settings/environments
func (h *ProjectSettingsHandler) GetEnvironmentDefaults(w http.ResponseWriter, r *http.Request) {
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

	settings, err := h.settings.Get(r.Context(), project.ID)
	if err != nil {
		slog.Error("failed to get project settings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get project settings")
		return
	}

	envKeys := make([]string, len(envs))
	for i, e := range envs {
		envKeys[i] = e.Key
	}

	resolved := settings.ResolveEnvironmentDefaults(envKeys, nil)

	type envDefault struct {
		Key     string `json:"key"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	result := make([]envDefault, len(envs))
	for i, e := range envs {
		result[i] = envDefault{Key: e.Key, Name: e.Name, Enabled: resolved[e.Key]}
	}

	writeJSON(w, http.StatusOK, map[string]any{"environment_defaults": result})
}

// UpdateEnvironmentDefaults handles PUT /api/v1/projects/{key}/settings/environments
func (h *ProjectSettingsHandler) UpdateEnvironmentDefaults(w http.ResponseWriter, r *http.Request) {
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
		EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EnvironmentDefaults == nil {
		writeError(w, http.StatusBadRequest, "environment_defaults is required")
		return
	}

	_, err = h.settings.UpsertEnvironmentDefaults(r.Context(), project.ID, req.EnvironmentDefaults)
	if err != nil {
		slog.Error("failed to update environment defaults", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update environment defaults")
		return
	}

	// Return the resolved view (same as GET)
	h.GetEnvironmentDefaults(w, r)
}
