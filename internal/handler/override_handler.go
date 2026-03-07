package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type OverrideHandler struct {
	overrides    *store.OverrideStore
	identities   *store.AppIdentityStore
	projects     *store.ProjectStore
	flags        *store.FlagStore
	environments *store.EnvironmentStore
	cache        *evaluation.Cache
}

func NewOverrideHandler(
	overrides *store.OverrideStore,
	identities *store.AppIdentityStore,
	projects *store.ProjectStore,
	flags *store.FlagStore,
	environments *store.EnvironmentStore,
	cache *evaluation.Cache,
) *OverrideHandler {
	return &OverrideHandler{
		overrides:    overrides,
		identities:   identities,
		projects:     projects,
		flags:        flags,
		environments: environments,
		cache:        cache,
	}
}

// SetAppIdentity handles PUT /api/v1/projects/{key}/app-identity
func (h *OverrideHandler) SetAppIdentity(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		AppUserID string `json:"app_user_id"`
	}
	if err := readJSON(r, &req); err != nil || req.AppUserID == "" {
		writeError(w, http.StatusBadRequest, "app_user_id is required")
		return
	}

	identity, err := h.identities.Set(r.Context(), user.ID, project.ID, req.AppUserID)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "app user ID already claimed by another user")
			return
		}
		slog.Error("setting app identity", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to set app identity")
		return
	}

	writeJSON(w, http.StatusOK, identity)
}

// GetAppIdentity handles GET /api/v1/projects/{key}/app-identity
func (h *OverrideHandler) GetAppIdentity(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "app identity not configured")
		return
	}

	writeJSON(w, http.StatusOK, identity)
}

// DeleteAppIdentity handles DELETE /api/v1/projects/{key}/app-identity
func (h *OverrideHandler) DeleteAppIdentity(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := h.identities.Delete(r.Context(), user.ID, project.ID); err != nil {
		slog.Error("deleting app identity", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete app identity")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var durationMap = map[string]time.Duration{
	"1h":  1 * time.Hour,
	"8h":  8 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

// SetOverride handles PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/override
func (h *OverrideHandler) SetOverride(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	envKey := r.PathValue("env")

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "app identity not configured for this project")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	var req struct {
		Value    json.RawMessage `json:"value"`
		Duration *string         `json:"duration"`
	}
	if err := readJSON(r, &req); err != nil || req.Value == nil {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	expiresAt := parseExpiry(req.Duration)

	override, err := h.overrides.Set(r.Context(), user.ID, flag.ID, env.ID, req.Value, expiresAt)
	if err != nil {
		slog.Error("setting override", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to set override")
		return
	}

	h.cache.SetOverride(projectKey, envKey, identity.AppUserID, flagKey, req.Value, expiresAt)

	writeJSON(w, http.StatusOK, override)
}

// DeleteOverride handles DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/override
func (h *OverrideHandler) DeleteOverride(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	envKey := r.PathValue("env")

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	if err := h.overrides.Delete(r.Context(), user.ID, flag.ID, env.ID); err != nil {
		slog.Error("deleting override", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete override")
		return
	}

	if identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey); err == nil {
		h.cache.DeleteOverride(projectKey, envKey, identity.AppUserID, flagKey)
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetOverrideAllEnvs handles PUT /api/v1/projects/{key}/flags/{flag}/override
func (h *OverrideHandler) SetOverrideAllEnvs(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "app identity not configured for this project")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}

	var req struct {
		Value    json.RawMessage `json:"value"`
		Duration *string         `json:"duration"`
	}
	if err := readJSON(r, &req); err != nil || req.Value == nil {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	expiresAt := parseExpiry(req.Duration)

	for _, env := range envs {
		if _, err := h.overrides.Set(r.Context(), user.ID, flag.ID, env.ID, req.Value, expiresAt); err != nil {
			slog.Error("setting override for env", "env", env.Key, "error", err)
			continue
		}
		h.cache.SetOverride(projectKey, env.Key, identity.AppUserID, flagKey, req.Value, expiresAt)
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteOverrideAllEnvs handles DELETE /api/v1/projects/{key}/flags/{flag}/override
func (h *OverrideHandler) DeleteOverrideAllEnvs(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	if err := h.overrides.DeleteByUserAndFlag(r.Context(), user.ID, flag.ID); err != nil {
		slog.Error("deleting all overrides for flag", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete overrides")
		return
	}

	if identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey); err == nil {
		envs, _ := h.environments.ListByProject(r.Context(), project.ID)
		for _, env := range envs {
			h.cache.DeleteOverride(projectKey, env.Key, identity.AppUserID, flagKey)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMyOverrides handles GET /api/v1/overrides/me
func (h *OverrideHandler) ListMyOverrides(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	overrides, err := h.overrides.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing overrides", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list overrides")
		return
	}

	writeJSON(w, http.StatusOK, overrides)
}

// GetFlagOverrides handles GET /api/v1/projects/{key}/flags/{flag}/overrides/me
func (h *OverrideHandler) GetFlagOverrides(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	allOverrides, err := h.overrides.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing overrides", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list overrides")
		return
	}

	var flagOverrides []model.FlagOverride
	for _, o := range allOverrides {
		if o.FlagID == flag.ID {
			flagOverrides = append(flagOverrides, o)
		}
	}
	if flagOverrides == nil {
		flagOverrides = []model.FlagOverride{}
	}

	writeJSON(w, http.StatusOK, flagOverrides)
}

func isUniqueViolation(err error) bool {
	s := err.Error()
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate key")
}

func parseExpiry(duration *string) *time.Time {
	if duration == nil {
		// Default: 24h
		t := time.Now().Add(24 * time.Hour)
		return &t
	}
	if *duration == "" {
		// Explicitly no expiry
		return nil
	}
	dur, ok := durationMap[*duration]
	if !ok {
		// Invalid duration, fall back to 24h
		t := time.Now().Add(24 * time.Hour)
		return &t
	}
	t := time.Now().Add(dur)
	return &t
}
