package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	pool         *pgxpool.Pool
	audit        *store.AuditStore
}

func NewOverrideHandler(
	overrides *store.OverrideStore,
	identities *store.AppIdentityStore,
	projects *store.ProjectStore,
	flags *store.FlagStore,
	environments *store.EnvironmentStore,
	cache *evaluation.Cache,
	pool *pgxpool.Pool,
	audit *store.AuditStore,
) *OverrideHandler {
	return &OverrideHandler{
		overrides:    overrides,
		identities:   identities,
		projects:     projects,
		flags:        flags,
		environments: environments,
		cache:        cache,
		pool:         pool,
		audit:        audit,
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
		if errors.Is(err, store.ErrDuplicateAppUserID) {
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

	// Look up identity before deleting so we can clean up the cache
	identity, _ := h.identities.Get(r.Context(), user.ID, project.ID)

	// Delete overrides and identity in a transaction to avoid inconsistent state
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		slog.Error("starting transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback(r.Context())

	// Delete overrides scoped to this project only
	_, err = tx.Exec(r.Context(),
		`DELETE FROM flag_overrides WHERE user_id = $1 AND flag_id IN (SELECT id FROM flags WHERE project_id = $2)`,
		user.ID, project.ID)
	if err != nil {
		slog.Error("deleting overrides on identity removal", "error", err)
	}

	// Delete identity
	_, err = tx.Exec(r.Context(),
		`DELETE FROM user_app_identities WHERE user_id = $1 AND project_id = $2`,
		user.ID, project.ID)
	if err != nil {
		slog.Error("deleting app identity", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete app identity")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		slog.Error("committing app identity deletion", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete app identity")
		return
	}

	// Clear cached overrides for this user
	if identity != nil {
		envs, err := h.environments.ListByProject(r.Context(), project.ID)
		if err != nil {
			slog.Error("listing environments for cache cleanup", "error", err)
		}
		for _, env := range envs {
			h.cache.DeleteOverridesForUser(projectKey, env.Key, identity.AppUserID)
		}
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

	expiresAt, err := parseExpiry(req.Duration)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	override, err := h.overrides.Set(r.Context(), user.ID, flag.ID, env.ID, req.Value, expiresAt)
	if err != nil {
		slog.Error("setting override", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to set override")
		return
	}

	h.cache.SetOverride(projectKey, envKey, identity.AppUserID, flagKey, req.Value, expiresAt)

	go func() {
		newVal, _ := json.Marshal(map[string]any{"value": req.Value, "environment": envKey, "expires_at": expiresAt})
		if err := h.audit.Record(context.Background(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			Action:        "override.set",
			EntityType:    "flag",
			EntityID:      flag.ID,
			NewValue:      newVal,
		}); err != nil {
			slog.Error("recording override audit", "error", err)
		}
	}()

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

	go func() {
		if err := h.audit.Record(context.Background(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			Action:        "override.delete",
			EntityType:    "flag",
			EntityID:      flag.ID,
		}); err != nil {
			slog.Error("recording override audit", "error", err)
		}
	}()

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

	expiresAt, err := parseExpiry(req.Duration)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var failed []string
	for _, env := range envs {
		if _, err := h.overrides.Set(r.Context(), user.ID, flag.ID, env.ID, req.Value, expiresAt); err != nil {
			slog.Error("setting override for env", "env", env.Key, "error", err)
			failed = append(failed, env.Key)
			continue
		}
		h.cache.SetOverride(projectKey, env.Key, identity.AppUserID, flagKey, req.Value, expiresAt)
	}

	if len(failed) > 0 {
		writeError(w, http.StatusInternalServerError, "failed to set override for some environments")
		return
	}

	go func() {
		newVal, _ := json.Marshal(map[string]any{"value": req.Value, "all_environments": true, "expires_at": expiresAt})
		if err := h.audit.Record(context.Background(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "override.set",
			EntityType: "flag",
			EntityID:   flag.ID,
			NewValue:   newVal,
		}); err != nil {
			slog.Error("recording override audit", "error", err)
		}
	}()

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

	go func() {
		if err := h.audit.Record(context.Background(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "override.delete",
			EntityType: "flag",
			EntityID:   flag.ID,
		}); err != nil {
			slog.Error("recording override audit", "error", err)
		}
	}()

	if identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey); err == nil {
		envs, err := h.environments.ListByProject(r.Context(), project.ID)
		if err != nil {
			slog.Error("listing environments for cache cleanup", "error", err)
		}
		for _, env := range envs {
			h.cache.DeleteOverride(projectKey, env.Key, identity.AppUserID, flagKey)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}


// ListAppIdentities handles GET /api/v1/app-identities/me
func (h *OverrideHandler) ListAppIdentities(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	identities, err := h.identities.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("listing app identities", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list identities")
		return
	}

	writeJSON(w, http.StatusOK, identities)
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

	flagOverrides, err := h.overrides.ListByUserAndFlag(r.Context(), user.ID, flag.ID)
	if err != nil {
		slog.Error("listing flag overrides", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list overrides")
		return
	}

	writeJSON(w, http.StatusOK, flagOverrides)
}

// parseExpiry converts a duration string to an expiry time.
// - nil (field omitted): defaults to 24h expiry
// - "" (empty string): no expiry (override persists until manually deleted)
// - "1h", "8h", "24h", "7d": specific duration
// - invalid value: returns error via 400 in the caller
func parseExpiry(duration *string) (*time.Time, error) {
	if duration == nil {
		t := time.Now().Add(24 * time.Hour)
		return &t, nil
	}
	if *duration == "" {
		return nil, nil
	}
	dur, ok := durationMap[*duration]
	if !ok {
		return nil, errors.New("invalid duration, use: 1h, 8h, 24h, 7d, or empty string for no expiry")
	}
	t := time.Now().Add(dur)
	return &t, nil
}
