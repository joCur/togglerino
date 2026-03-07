package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

type FlagHandler struct {
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
	unknownFlags *store.UnknownFlagStore
	schedules    *store.ScheduleStore
	settings     *store.ProjectSettingsStore
}

func NewFlagHandler(flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool, unknownFlags *store.UnknownFlagStore, schedules *store.ScheduleStore, settings *store.ProjectSettingsStore) *FlagHandler {
	return &FlagHandler{flags: flags, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool, unknownFlags: unknownFlags, schedules: schedules, settings: settings}
}

// refreshAllEnvironments refreshes the evaluation cache and broadcasts SSE events
// for all environments in a project after a flag change (archive/delete).
func (h *FlagHandler) refreshAllEnvironments(ctx context.Context, projectKey, projectID, flagKey string, event stream.Event) {
	envs, err := h.environments.ListByProject(ctx, projectID)
	if err != nil {
		slog.Warn("failed to list environments for cache refresh", "error", err)
		return
	}
	event.FlagKey = flagKey
	for _, env := range envs {
		if err := h.cache.Refresh(ctx, h.pool, projectKey, env.Key); err != nil {
			slog.Warn("failed to refresh cache", "project", projectKey, "env", env.Key, "error", err)
		}
		h.hub.Broadcast(projectKey, env.Key, event)
	}
}

// Create handles POST /api/v1/projects/{key}/flags
func (h *FlagHandler) Create(w http.ResponseWriter, r *http.Request) {
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
		Key                  string                              `json:"key"`
		Name                 string                              `json:"name"`
		Description          string                              `json:"description"`
		ValueType            model.ValueType                     `json:"value_type"`
		FlagType             model.FlagType                      `json:"flag_type"`
		DefaultValue         json.RawMessage                     `json:"default_value"`
		Tags                 []string                            `json:"tags"`
		OwnerID              *string                             `json:"owner_id"`
		EnvironmentOverrides map[string]model.EnvironmentDefault `json:"environment_overrides"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}
	if req.ValueType == "" {
		req.ValueType = model.ValueTypeBoolean
	}
	if req.FlagType == "" {
		req.FlagType = model.FlagTypeRelease
	}
	if !model.ValidValueTypes[req.ValueType] {
		writeError(w, http.StatusBadRequest, "invalid value_type: must be one of boolean, string, number, json")
		return
	}
	if !model.ValidFlagTypes[req.FlagType] {
		writeError(w, http.StatusBadRequest, "invalid flag_type: must be one of release, experiment, operational, kill-switch, permission")
		return
	}
	if req.DefaultValue == nil {
		req.DefaultValue = json.RawMessage(`false`)
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	// Resolve environment defaults
	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	envKeys := make([]string, len(envs))
	for i, e := range envs {
		envKeys[i] = e.Key
	}

	projectSettings, err := h.settings.Get(r.Context(), project.ID)
	if err != nil {
		slog.Warn("failed to load project settings for env defaults, using fallbacks", "error", err)
	}
	envEnabled := projectSettings.ResolveEnvironmentDefaults(envKeys, req.EnvironmentOverrides)

	flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.ValueType, req.FlagType, req.DefaultValue, req.Tags, envEnabled, req.OwnerID, req.EnvironmentOverrides)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "flag key already exists for this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create flag")
		return
	}

	// Best-effort cleanup of unknown flags with this key
	if err := h.unknownFlags.DeleteByProjectAndKey(r.Context(), project.ID, req.Key); err != nil {
		slog.Warn("failed to cleanup unknown flags", "flag_key", req.Key, "error", err)
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(flag)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "create",
			EntityType: "flag",
			EntityID:   flag.Key,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache for all environments so the new flag is immediately available
	h.refreshAllEnvironments(r.Context(), projectKey, project.ID, req.Key, stream.Event{
		Type: "flag_create",
	})

	writeJSON(w, http.StatusCreated, flag)
}

// List handles GET /api/v1/projects/{key}/flags?tag=ui&search=dark
func (h *FlagHandler) List(w http.ResponseWriter, r *http.Request) {
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

	tag := r.URL.Query().Get("tag")
	search := r.URL.Query().Get("search")
	lifecycleStatus := r.URL.Query().Get("lifecycle_status")
	flagType := r.URL.Query().Get("flag_type")
	owner := r.URL.Query().Get("owner")
	limit, offset := parsePagination(r)

	flags, totalCount, err := h.flags.ListByProject(r.Context(), project.ID, tag, search, lifecycleStatus, flagType, owner, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list flags")
		return
	}
	if flags == nil {
		flags = []model.Flag{}
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:       flags,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	})
}

// Get handles GET /api/v1/projects/{key}/flags/{flag}
func (h *FlagHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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

	configs, err := h.flags.GetAllEnvironmentConfigs(r.Context(), flag.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get environment configs")
		return
	}
	if configs == nil {
		configs = []model.FlagEnvironmentConfig{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"flag":                flag,
		"environment_configs": configs,
	})
}

// Update handles PUT /api/v1/projects/{key}/flags/{flag}
func (h *FlagHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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

	var req struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Tags        []string       `json:"tags"`
		FlagType    model.FlagType `json:"flag_type"`
		OwnerID     *string        `json:"owner_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	flagTypeToUse := req.FlagType
	if flagTypeToUse == "" {
		flagTypeToUse = flag.FlagType
	} else if !model.ValidFlagTypes[flagTypeToUse] {
		writeError(w, http.StatusBadRequest, "invalid flag_type: must be one of release, experiment, operational, kill-switch, permission")
		return
	}
	updated, err := h.flags.Update(r.Context(), flag.ID, req.Name, req.Description, req.Tags, flagTypeToUse, req.OwnerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update flag")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "update",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/projects/{key}/flags/{flag}
func (h *FlagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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

	// Guard: only archived flags can be deleted
	if flag.LifecycleStatus != model.LifecycleArchived {
		writeError(w, http.StatusConflict, "flag must be archived before it can be deleted")
		return
	}

	// Cancel pending schedules before delete (cascade would delete them without audit trail)
	if err := h.schedules.CancelByFlag(r.Context(), flag.ID, "flag_deleted"); err != nil {
		slog.Warn("failed to cancel schedules for deleted flag", "flag", flag.Key, "error", err)
	}

	if err := h.flags.Delete(r.Context(), flag.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete flag")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "delete",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache and broadcast deletion for all environments
	h.refreshAllEnvironments(r.Context(), projectKey, project.ID, flagKey, stream.Event{
		Type: "flag_deleted",
	})

	w.WriteHeader(http.StatusNoContent)
}

// Archive handles PUT /api/v1/projects/{key}/flags/{flag}/archive
func (h *FlagHandler) Archive(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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

	var req struct {
		Archived bool `json:"archived"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var status model.LifecycleStatus
	if req.Archived {
		status = model.LifecycleArchived
	} else {
		status = model.LifecycleActive
	}

	updated, err := h.flags.SetLifecycleStatus(r.Context(), flag.ID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update flag archive status")
		return
	}

	// Best-effort cancel pending schedules on archive
	if req.Archived {
		if err := h.schedules.CancelByFlag(r.Context(), flag.ID, "flag_archived"); err != nil {
			slog.Warn("failed to cancel schedules for archived flag", "flag", flag.Key, "error", err)
		}
	}

	// Best-effort audit logging
	action := "archive"
	if !req.Archived {
		action = "unarchive"
	}
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     action,
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache and broadcast for all environments
	h.refreshAllEnvironments(r.Context(), projectKey, project.ID, flagKey, stream.Event{
		Type:    "flag_update",
		Value:   updated.LifecycleStatus == model.LifecycleArchived,
		Variant: "",
	})

	writeJSON(w, http.StatusOK, updated)
}

// UpdateEnvironmentConfig handles PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}
func (h *FlagHandler) UpdateEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
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
		Enabled        bool            `json:"enabled"`
		DefaultVariant string          `json:"default_variant"`
		Variants       json.RawMessage `json:"variants"`
		TargetingRules json.RawMessage `json:"targeting_rules"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Variants == nil {
		req.Variants = json.RawMessage(`[]`)
	}
	if req.TargetingRules == nil {
		req.TargetingRules = json.RawMessage(`[]`)
	}

	// Fetch old config for audit logging
	oldConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, env.ID)
	if err != nil {
		slog.Warn("failed to fetch old config for audit", "error", err)
		// Continue — audit old_value will be nil, but the update should still proceed
	}

	var updatedBy *string
	if user := auth.UserFromContext(r.Context()); user != nil {
		updatedBy = &user.ID
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(r.Context(), flag.ID, env.ID, req.Enabled, req.DefaultVariant, req.Variants, req.TargetingRules, updatedBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update environment config")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if oldConfig != nil {
			oldVal, _ = json.Marshal(oldConfig)
		}
		newVal, _ := json.Marshal(cfg)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			Action:        "update",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache and broadcast SSE event
	if err := h.cache.Refresh(r.Context(), h.pool, projectKey, envKey); err != nil {
		slog.Warn("failed to refresh cache", "error", err)
	}
	h.hub.Broadcast(projectKey, envKey, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
		Value:   cfg.Enabled,
		Variant: cfg.DefaultVariant,
	})

	writeJSON(w, http.StatusOK, cfg)
}

// SetStaleness handles PUT /api/v1/projects/{key}/flags/{flag}/staleness
func (h *FlagHandler) SetStaleness(w http.ResponseWriter, r *http.Request) {
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

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status != "stale" {
		writeError(w, http.StatusBadRequest, "only 'stale' status is accepted")
		return
	}

	updated, err := h.flags.SetLifecycleStatus(r.Context(), flag.ID, model.LifecycleStale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update staleness")
		return
	}

	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "staleness_change",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, updated)
}

// BulkAction handles POST /api/v1/projects/{key}/flags/bulk
func (h *FlagHandler) BulkAction(w http.ResponseWriter, r *http.Request) {
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
		Action         string   `json:"action"`
		FlagKeys       []string `json:"flag_keys"`
		EnvironmentKey string   `json:"environment_key"`
		Tags           []string `json:"tags"`
		OwnerID        *string  `json:"owner_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.FlagKeys) == 0 {
		writeError(w, http.StatusBadRequest, "flag_keys is required and must not be empty")
		return
	}
	if len(req.FlagKeys) > 200 {
		writeError(w, http.StatusBadRequest, "flag_keys must not exceed 200 entries")
		return
	}

	validActions := map[string]bool{
		"enable": true, "disable": true, "archive": true,
		"add_tags": true, "remove_tags": true, "set_owner": true,
	}
	if !validActions[req.Action] {
		writeError(w, http.StatusBadRequest, "invalid action: must be one of enable, disable, archive, add_tags, remove_tags, set_owner")
		return
	}

	if (req.Action == "enable" || req.Action == "disable") && req.EnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "environment_key is required for enable/disable actions")
		return
	}
	if (req.Action == "add_tags" || req.Action == "remove_tags") && len(req.Tags) == 0 {
		writeError(w, http.StatusBadRequest, "tags is required and must not be empty for tag actions")
		return
	}

	var env *model.Environment
	if req.Action == "enable" || req.Action == "disable" {
		env, err = h.environments.FindByKey(r.Context(), project.ID, req.EnvironmentKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "environment not found: "+req.EnvironmentKey)
			return
		}
	}

	user := auth.UserFromContext(r.Context())
	batchID := generateUUID()

	type bulkResult struct {
		FlagKey string `json:"flag_key"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	results := make([]bulkResult, 0, len(req.FlagKeys))

	for _, flagKey := range req.FlagKeys {
		flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
		if err != nil {
			results = append(results, bulkResult{FlagKey: flagKey, Error: "flag not found"})
			continue
		}

		var opErr error
		switch req.Action {
		case "enable", "disable":
			opErr = h.bulkEnableDisable(r.Context(), project, flag, env, req.Action == "enable", user, &batchID)
		case "archive":
			opErr = h.bulkArchive(r.Context(), project, flag, user, &batchID)
		case "add_tags":
			opErr = h.bulkAddTags(r.Context(), project, flag, req.Tags, user, &batchID)
		case "remove_tags":
			opErr = h.bulkRemoveTags(r.Context(), project, flag, req.Tags, user, &batchID)
		case "set_owner":
			opErr = h.bulkSetOwner(r.Context(), project, flag, req.OwnerID, user, &batchID)
		}

		if opErr != nil {
			results = append(results, bulkResult{FlagKey: flagKey, Error: opErr.Error()})
		} else {
			results = append(results, bulkResult{FlagKey: flagKey, Success: true})
		}
	}

	// Deduplicated cache refresh + SSE broadcast for enable/disable
	if env != nil {
		if err := h.cache.Refresh(r.Context(), h.pool, projectKey, req.EnvironmentKey); err != nil {
			slog.Warn("failed to refresh cache after bulk action", "error", err)
		}
		h.hub.Broadcast(projectKey, req.EnvironmentKey, stream.Event{
			Type: "flag_update",
		})
	}

	// For archive actions, refresh all environments
	if req.Action == "archive" {
		envs, err := h.environments.ListByProject(r.Context(), project.ID)
		if err != nil {
			slog.Warn("failed to list environments for bulk archive cache refresh", "error", err)
		} else {
			for _, e := range envs {
				if err := h.cache.Refresh(r.Context(), h.pool, projectKey, e.Key); err != nil {
					slog.Warn("failed to refresh cache", "project", projectKey, "env", e.Key, "error", err)
				}
				h.hub.Broadcast(projectKey, e.Key, stream.Event{Type: "flag_update"})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"batch_id": batchID,
		"results":  results,
	})
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (h *FlagHandler) bulkEnableDisable(ctx context.Context, project *model.Project, flag *model.Flag, env *model.Environment, enable bool, user *model.User, batchID *string) error {
	if flag.LifecycleStatus == model.LifecycleArchived {
		return fmt.Errorf("flag is archived")
	}

	oldConfig, err := h.flags.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		return fmt.Errorf("failed to get environment config")
	}

	var updatedBy *string
	if user != nil {
		updatedBy = &user.ID
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, enable, oldConfig.DefaultVariant,
		marshalJSON(oldConfig.Variants), marshalJSON(oldConfig.TargetingRules), updatedBy)
	if err != nil {
		return fmt.Errorf("failed to update environment config")
	}

	if user != nil {
		oldVal, _ := json.Marshal(oldConfig)
		newVal, _ := json.Marshal(cfg)
		action := "enable"
		if !enable {
			action = "disable"
		}
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			BatchID:       batchID,
			Action:        action,
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkArchive(ctx context.Context, project *model.Project, flag *model.Flag, user *model.User, batchID *string) error {
	if flag.LifecycleStatus == model.LifecycleArchived {
		return fmt.Errorf("flag is already archived")
	}

	updated, err := h.flags.SetLifecycleStatus(ctx, flag.ID, model.LifecycleArchived)
	if err != nil {
		return fmt.Errorf("failed to archive flag")
	}

	if err := h.schedules.CancelByFlag(ctx, flag.ID, "bulk_archived"); err != nil {
		slog.Warn("failed to cancel schedules for bulk archived flag", "flag", flag.Key, "error", err)
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "archive",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkAddTags(ctx context.Context, project *model.Project, flag *model.Flag, tags []string, user *model.User, batchID *string) error {
	existing := make(map[string]bool, len(flag.Tags))
	for _, t := range flag.Tags {
		existing[t] = true
	}
	newTags := append([]string{}, flag.Tags...)
	for _, t := range tags {
		if !existing[t] {
			newTags = append(newTags, t)
		}
	}

	updated, err := h.flags.Update(ctx, flag.ID, flag.Name, flag.Description, newTags, flag.FlagType, flag.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to update tags")
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "add_tags",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkRemoveTags(ctx context.Context, project *model.Project, flag *model.Flag, tags []string, user *model.User, batchID *string) error {
	toRemove := make(map[string]bool, len(tags))
	for _, t := range tags {
		toRemove[t] = true
	}
	newTags := make([]string, 0, len(flag.Tags))
	for _, t := range flag.Tags {
		if !toRemove[t] {
			newTags = append(newTags, t)
		}
	}

	updated, err := h.flags.Update(ctx, flag.ID, flag.Name, flag.Description, newTags, flag.FlagType, flag.OwnerID)
	if err != nil {
		return fmt.Errorf("failed to update tags")
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "remove_tags",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func (h *FlagHandler) bulkSetOwner(ctx context.Context, project *model.Project, flag *model.Flag, ownerID *string, user *model.User, batchID *string) error {
	updated, err := h.flags.Update(ctx, flag.ID, flag.Name, flag.Description, flag.Tags, flag.FlagType, ownerID)
	if err != nil {
		return fmt.Errorf("failed to set owner")
	}

	if user != nil {
		oldVal, _ := json.Marshal(flag)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(ctx, model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			BatchID:    batchID,
			Action:     "set_owner",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record bulk audit log", "error", err)
		}
	}
	return nil
}

func marshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}
