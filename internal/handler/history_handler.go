package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

type HistoryHandler struct {
	audit        *store.AuditStore
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
}

func NewHistoryHandler(audit *store.AuditStore, flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool) *HistoryHandler {
	return &HistoryHandler{audit: audit, flags: flags, projects: projects, environments: environments, hub: hub, cache: cache, pool: pool}
}

// List handles GET /api/v1/projects/{key}/flags/{flag}/history?env=&limit=50&offset=0
func (h *HistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	if projectKey == "" || flagKey == "" {
		writeError(w, http.StatusBadRequest, "project key and flag key are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Verify flag exists
	if _, err := h.flags.FindByKey(r.Context(), project.ID, flagKey); err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var envID *string
	if envKey := r.URL.Query().Get("env"); envKey != "" {
		env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		envID = &env.ID
	}

	entries, err := h.audit.ListByFlag(r.Context(), project.ID, flagKey, envID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list flag history")
		return
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

// Get handles GET /api/v1/projects/{key}/flags/{flag}/history/{id}
func (h *HistoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	entryID := r.PathValue("id")
	if projectKey == "" || flagKey == "" || entryID == "" {
		writeError(w, http.StatusBadRequest, "project key, flag key, and history id are required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	entry, err := h.audit.GetByID(r.Context(), entryID)
	if err != nil {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	// Verify entry belongs to this project and flag
	if entry.ProjectID == nil || *entry.ProjectID != project.ID || entry.EntityID != flagKey {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	writeJSON(w, http.StatusOK, entry)
}

// Restore handles POST /api/v1/projects/{key}/flags/{flag}/history/{id}/restore
func (h *HistoryHandler) Restore(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	entryID := r.PathValue("id")
	if projectKey == "" || flagKey == "" || entryID == "" {
		writeError(w, http.StatusBadRequest, "project key, flag key, and history id are required")
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

	entry, err := h.audit.GetByID(r.Context(), entryID)
	if err != nil {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	// Verify entry belongs to this project and flag
	if entry.ProjectID == nil || *entry.ProjectID != project.ID || entry.EntityID != flagKey {
		writeError(w, http.StatusNotFound, "history entry not found")
		return
	}

	// Only flag_config entries can be restored
	if entry.EntityType != "flag_config" {
		writeError(w, http.StatusBadRequest, "only flag_config entries can be restored")
		return
	}

	// Must have environment_id to know which env to restore to
	if entry.EnvironmentID == nil {
		writeError(w, http.StatusBadRequest, "this entry has no associated environment and cannot be restored")
		return
	}

	// Determine snapshot to restore: new_value is the config that resulted from this change,
	// which is what the user expects when clicking "Restore this version".
	snapshot := entry.NewValue
	if snapshot == nil {
		snapshot = entry.OldValue
	}
	if snapshot == nil {
		writeError(w, http.StatusBadRequest, "no snapshot available to restore from this entry")
		return
	}

	// Parse the snapshot
	var cfg model.FlagEnvironmentConfig
	if err := json.Unmarshal(snapshot, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse config snapshot")
		return
	}

	// Marshal variants and targeting rules back to JSON for the store method
	variantsJSON, err := json.Marshal(cfg.Variants)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal variants from snapshot")
		return
	}
	rulesJSON, err := json.Marshal(cfg.TargetingRules)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal targeting rules from snapshot")
		return
	}

	// Verify the environment still exists before attempting restore
	env, err := h.environments.FindByID(r.Context(), *entry.EnvironmentID)
	if err != nil {
		writeError(w, http.StatusConflict, "the environment for this configuration no longer exists")
		return
	}

	// Fetch current config before restore for audit old_value
	oldConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, *entry.EnvironmentID)
	if err != nil {
		slog.Warn("failed to fetch old config for restore audit", "error", err)
	}

	// Apply the restored config using the same store method
	updated, err := h.flags.UpdateEnvironmentConfig(r.Context(), flag.ID, *entry.EnvironmentID, cfg.Enabled, cfg.DefaultVariant, variantsJSON, rulesJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to restore config")
		return
	}

	// Audit the restore action
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if oldConfig != nil {
			oldVal, _ = json.Marshal(oldConfig)
		}
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: entry.EnvironmentID,
			Action:        "restore",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record restore audit", "error", err)
		}
	}

	// Refresh cache and broadcast SSE
	if err := h.cache.Refresh(r.Context(), h.pool, projectKey, env.Key); err != nil {
		slog.Warn("failed to refresh cache after restore", "error", err)
	}
	h.hub.Broadcast(projectKey, env.Key, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
		Value:   updated.Enabled,
		Variant: updated.DefaultVariant,
	})

	writeJSON(w, http.StatusOK, updated)
}
