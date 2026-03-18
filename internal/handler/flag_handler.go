package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
	"github.com/togglerino/togglerino/internal/webhook"
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
	webhooks     *webhook.Dispatcher
}

func NewFlagHandler(flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool, unknownFlags *store.UnknownFlagStore, schedules *store.ScheduleStore, settings *store.ProjectSettingsStore, webhooks *webhook.Dispatcher) *FlagHandler {
	return &FlagHandler{flags: flags, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool, unknownFlags: unknownFlags, schedules: schedules, settings: settings, webhooks: webhooks}
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
		slog.Error("failed to list environments", "error", err)
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

	// Boolean flags: enforce canonical variants, validate targeting rules.
	if req.ValueType == model.ValueTypeBoolean {
		for key, override := range req.EnvironmentOverrides {
			if override.TargetingRules != nil && string(override.TargetingRules) != "[]" {
				var rules []model.TargetingRule
				if err := json.Unmarshal(override.TargetingRules, &rules); err == nil {
					for _, rule := range rules {
						if rule.Variant != "true" && rule.Variant != "false" {
							writeError(w, http.StatusBadRequest, "boolean flag targeting rules must use variant 'true' or 'false'")
							return
						}
					}
				}
			}
			req.EnvironmentOverrides[key] = override
		}
	}

	flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.ValueType, req.FlagType, req.DefaultValue, req.Tags, envEnabled, req.OwnerID, req.EnvironmentOverrides)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "flag key already exists for this project")
			return
		}
		slog.Error("failed to create flag", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create flag")
		return
	}

	// Best-effort cleanup of unknown flags with this key
	if err := h.unknownFlags.DeleteByProjectAndKey(r.Context(), project.ID, req.Key); err != nil {
		slog.Warn("failed to cleanup unknown flags", "flag_key", req.Key, "error", err)
	}

	// Best-effort audit logging
	newVal, _ := json.Marshal(flag)
	if user := auth.UserFromContext(r.Context()); user != nil {
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

	if h.webhooks != nil {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagCreated,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    newVal,
		})
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
	unevaluatedDays := r.URL.Query().Get("unevaluated_days")
	if unevaluatedDays != "" && unevaluatedDays != "never" {
		if n, err := strconv.Atoi(unevaluatedDays); err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "unevaluated_days must be 'never' or a positive integer")
			return
		}
	}
	limit, offset := parsePagination(r)

	flags, totalCount, err := h.flags.ListByProject(r.Context(), project.ID, tag, search, lifecycleStatus, flagType, owner, unevaluatedDays, limit, offset)
	if err != nil {
		slog.Error("failed to list flags", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list flags")
		return
	}
	if flags == nil {
		flags = []model.Flag{}
	}

	// Check for include=environment_configs
	if include := r.URL.Query().Get("include"); include == "environment_configs" {
		flagIDs := make([]string, len(flags))
		for i, f := range flags {
			flagIDs[i] = f.ID
		}
		configsMap, err := h.flags.GetEnvironmentConfigsByFlagIDs(r.Context(), flagIDs)
		if err != nil {
			slog.Error("failed to get environment configs", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to get environment configs")
			return
		}
		for i := range flags {
			configs := configsMap[flags[i].ID]
			if configs == nil {
				configs = []model.FlagEnvironmentConfig{}
			}
			flags[i].EnvironmentConfigs = configs
		}
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:       flags,
		Total: totalCount,
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
		slog.Error("failed to get environment configs", "error", err)
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
		slog.Error("failed to update flag", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update flag")
		return
	}

	// Best-effort audit logging
	newVal, _ := json.Marshal(updated)
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
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

	if h.webhooks != nil {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagUpdated,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    newVal,
		})
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
		slog.Error("failed to delete flag", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete flag")
		return
	}

	// Best-effort audit logging
	oldVal, _ := json.Marshal(flag)
	if user := auth.UserFromContext(r.Context()); user != nil {
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

	if h.webhooks != nil {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagDeleted,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    oldVal,
		})
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

	// Check if flag is locked in any environment before archiving
	if req.Archived {
		locked, lockErr := h.flags.IsLockedInAnyEnvironment(r.Context(), flag.ID)
		if lockErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to check lock status")
			return
		}
		if locked {
			writeError(w, http.StatusConflict, "cannot archive flag that is locked in one or more environments")
			return
		}
	}

	var status model.LifecycleStatus
	if req.Archived {
		status = model.LifecycleArchived
	} else {
		status = model.LifecycleActive
	}

	updated, err := h.flags.SetLifecycleStatus(r.Context(), flag.ID, status)
	if err != nil {
		slog.Error("failed to update flag archive status", "error", err)
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
	newVal, _ := json.Marshal(updated)
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(flag)
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

	if h.webhooks != nil && req.Archived {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagArchived,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    newVal,
		})
	}

	// Refresh cache and broadcast for all environments
	h.refreshAllEnvironments(r.Context(), projectKey, project.ID, flagKey, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
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
		Enabled            bool            `json:"enabled"`
		FallthroughVariant string          `json:"fallthrough_variant"`
		OffVariant         string          `json:"off_variant"`
		Variants           json.RawMessage `json:"variants"`
		TargetingRules     json.RawMessage `json:"targeting_rules"`
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

	// Parse variants for validation
	var variants []model.Variant
	if err := json.Unmarshal(req.Variants, &variants); err != nil {
		writeError(w, http.StatusBadRequest, "invalid variants")
		return
	}

	// Boolean flags: enforce exactly two variants with values true/false, validate targeting rules.
	if flag.ValueType == model.ValueTypeBoolean {
		if len(variants) != 2 {
			writeError(w, http.StatusBadRequest, "boolean flags must have exactly two variants")
			return
		}
		hasTrue, hasFalse := false, false
		for _, v := range variants {
			var val any
			if err := json.Unmarshal(v.Value, &val); err == nil {
				if val == true {
					hasTrue = true
				}
				if val == false {
					hasFalse = true
				}
			}
		}
		if !hasTrue || !hasFalse {
			writeError(w, http.StatusBadRequest, "boolean flag variants must contain one true and one false value")
			return
		}

		// Validate targeting rule variant names for boolean flags.
		if req.TargetingRules != nil && string(req.TargetingRules) != "[]" {
			var rules []model.TargetingRule
			if err := json.Unmarshal(req.TargetingRules, &rules); err == nil {
				for _, rule := range rules {
					if rule.Variant != "true" && rule.Variant != "false" {
						writeError(w, http.StatusBadRequest, "boolean flag targeting rules must use variant 'true' or 'false'")
						return
					}
				}
			}
		}
	}

	// Validate that fallthrough_variant and off_variant reference existing variant names.
	variantNames := make(map[string]bool, len(variants))
	for _, v := range variants {
		variantNames[v.Name] = true
	}
	if req.FallthroughVariant != "" && !variantNames[req.FallthroughVariant] {
		writeError(w, http.StatusBadRequest, "fallthrough_variant must reference an existing variant name")
		return
	}
	if req.OffVariant != "" && !variantNames[req.OffVariant] {
		writeError(w, http.StatusBadRequest, "off_variant must reference an existing variant name")
		return
	}

	// Fetch old config for audit logging and lock check
	oldConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, env.ID)
	if err != nil {
		slog.Warn("failed to fetch old config for audit", "error", err)
		// Continue — audit old_value will be nil, but the update should still proceed
	}
	if oldConfig != nil && oldConfig.Locked {
		msg := "flag is locked in this environment"
		if oldConfig.LockedByUser != nil {
			msg += " by " + oldConfig.LockedByUser.Email
		}
		if oldConfig.LockReason != nil {
			msg += ": " + *oldConfig.LockReason
		}
		writeError(w, http.StatusConflict, msg)
		return
	}

	var updatedBy *string
	if user := auth.UserFromContext(r.Context()); user != nil {
		updatedBy = &user.ID
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(r.Context(), flag.ID, env.ID, req.Enabled, req.FallthroughVariant, req.OffVariant, req.Variants, req.TargetingRules, updatedBy)
	if err != nil {
		slog.Error("failed to update environment config", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update environment config")
		return
	}

	// Best-effort audit logging
	newVal, _ := json.Marshal(cfg)
	if user := auth.UserFromContext(r.Context()); user != nil {
		var oldVal json.RawMessage
		if oldConfig != nil {
			oldVal, _ = json.Marshal(oldConfig)
		}
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

	if h.webhooks != nil {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagConfigUpdated,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    newVal,
		})
	}

	// Refresh cache and broadcast SSE event
	if err := h.cache.Refresh(r.Context(), h.pool, projectKey, envKey); err != nil {
		slog.Warn("failed to refresh cache", "error", err)
	}
	h.hub.Broadcast(projectKey, envKey, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
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
		slog.Error("failed to update staleness", "error", err)
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

	// Cache refresh + SSE broadcast for enable/disable
	if env != nil {
		if err := h.cache.Refresh(r.Context(), h.pool, projectKey, req.EnvironmentKey); err != nil {
			slog.Warn("failed to refresh cache after bulk action", "error", err)
		}
		for _, res := range results {
			if res.Success {
				h.hub.Broadcast(projectKey, req.EnvironmentKey, stream.Event{
					Type:    "flag_update",
					FlagKey: res.FlagKey,
				})
			}
		}
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
				for _, res := range results {
					if res.Success {
						h.hub.Broadcast(projectKey, e.Key, stream.Event{
							Type:    "flag_update",
							FlagKey: res.FlagKey,
						})
					}
				}
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
	if oldConfig.Locked {
		return fmt.Errorf("flag is locked in this environment")
	}

	var updatedBy *string
	if user != nil {
		updatedBy = &user.ID
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, enable, oldConfig.FallthroughVariant, oldConfig.OffVariant,
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

	locked, err := h.flags.IsLockedInAnyEnvironment(ctx, flag.ID)
	if err != nil {
		return fmt.Errorf("failed to check lock status")
	}
	if locked {
		return fmt.Errorf("flag is locked in one or more environments")
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

// PromoteEnvironmentConfig handles POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote
func (h *FlagHandler) PromoteEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	targetEnvKey := r.PathValue("env")
	if projectKey == "" || flagKey == "" || targetEnvKey == "" {
		writeError(w, http.StatusBadRequest, "project key, flag key, and environment key are required")
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
		SourceEnvironmentKey string `json:"source_environment_key"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SourceEnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "source_environment_key is required")
		return
	}

	sourceEnv, err := h.environments.FindByKey(r.Context(), project.ID, req.SourceEnvironmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "source environment not found")
		return
	}

	targetEnv, err := h.environments.FindByKey(r.Context(), project.ID, targetEnvKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "target environment not found")
		return
	}

	// Enforce forward-only promotion
	if targetEnv.SortOrder <= sourceEnv.SortOrder {
		writeError(w, http.StatusBadRequest, "can only promote forward: target environment must come after source in sort order")
		return
	}

	// Check if target environment is locked
	targetConfig, lockErr := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, targetEnv.ID)
	if lockErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to check target environment lock status")
		return
	}
	if targetConfig.Locked {
		msg := "flag is locked in target environment"
		if targetConfig.LockedByUser != nil {
			msg += " by " + targetConfig.LockedByUser.Email
		}
		if targetConfig.LockReason != nil {
			msg += ": " + *targetConfig.LockReason
		}
		writeError(w, http.StatusConflict, msg)
		return
	}

	// Get source config
	sourceConfig, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, sourceEnv.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "source environment config not found")
		return
	}

	// Reuse target config from lock check for audit
	oldTargetConfig := targetConfig

	// Copy source config to target, preserving target's enabled state
	targetEnabled := false
	if oldTargetConfig != nil {
		targetEnabled = oldTargetConfig.Enabled
	}

	user := auth.UserFromContext(r.Context())
	var updatedBy *string
	if user != nil {
		updatedBy = &user.ID
	}

	cfg, err := h.flags.UpdateEnvironmentConfig(
		r.Context(),
		flag.ID,
		targetEnv.ID,
		targetEnabled,
		sourceConfig.FallthroughVariant,
		sourceConfig.OffVariant,
		marshalJSON(sourceConfig.Variants),
		marshalJSON(sourceConfig.TargetingRules),
		updatedBy,
	)
	if err != nil {
		slog.Error("failed to promote environment config", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to promote environment config")
		return
	}

	// Best-effort audit logging
	if user != nil {
		var oldVal json.RawMessage
		if oldTargetConfig != nil {
			oldVal, _ = json.Marshal(oldTargetConfig)
		}
		newValMap := map[string]any{
			"config":             cfg,
			"promoted_from_env":  req.SourceEnvironmentKey,
		}
		newVal, _ := json.Marshal(newValMap)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &targetEnv.ID,
			Action:        "promoted",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Refresh cache and broadcast SSE for target env
	if err := h.cache.Refresh(r.Context(), h.pool, projectKey, targetEnvKey); err != nil {
		slog.Warn("failed to refresh cache", "error", err)
	}
	h.hub.Broadcast(projectKey, targetEnvKey, stream.Event{
		Type:    "flag_update",
		FlagKey: flagKey,
	})

	writeJSON(w, http.StatusOK, cfg)
}

func marshalJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return b
}

// LockEnvironmentConfig handles POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/lock
func (h *FlagHandler) LockEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	envKey := r.PathValue("env")
	if projectKey == "" || flagKey == "" || envKey == "" {
		writeError(w, http.StatusBadRequest, "missing required path parameters")
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
		Reason *string `json:"reason"`
	}
	if r.ContentLength > 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	if req.Reason != nil && len(*req.Reason) > 255 {
		writeError(w, http.StatusBadRequest, "lock reason must be 255 characters or fewer")
		return
	}

	// Check if already locked
	existing, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, env.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get environment config")
		return
	}
	if existing.Locked {
		writeError(w, http.StatusConflict, "flag is already locked in this environment")
		return
	}

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	cfg, err := h.flags.LockEnvironmentConfig(r.Context(), flag.ID, env.ID, user.ID, req.Reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to lock environment config")
		return
	}

	// Best-effort audit log
	oldVal, _ := json.Marshal(existing)
	newVal, _ := json.Marshal(cfg)
	if err := h.audit.Record(r.Context(), model.AuditEntry{
		ProjectID:     &project.ID,
		UserID:        &user.ID,
		UserEmail:     &user.Email,
		EnvironmentID: &env.ID,
		Action:        "lock",
		EntityType:    "flag_config",
		EntityID:      flag.Key,
		OldValue:      oldVal,
		NewValue:      newVal,
	}); err != nil {
		slog.Warn("failed to record audit log", "error", err)
	}

	// Webhook dispatch
	if h.webhooks != nil {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagConfigLocked,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    newVal,
		})
	}

	writeJSON(w, http.StatusOK, cfg)
}

// UnlockEnvironmentConfig handles DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/lock
func (h *FlagHandler) UnlockEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	flagKey := r.PathValue("flag")
	envKey := r.PathValue("env")
	if projectKey == "" || flagKey == "" || envKey == "" {
		writeError(w, http.StatusBadRequest, "missing required path parameters")
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

	// Check if actually locked
	existing, err := h.flags.GetEnvironmentConfig(r.Context(), flag.ID, env.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get environment config")
		return
	}
	if !existing.Locked {
		writeError(w, http.StatusConflict, "flag is not locked in this environment")
		return
	}

	cfg, err := h.flags.UnlockEnvironmentConfig(r.Context(), flag.ID, env.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlock environment config")
		return
	}

	// Best-effort audit log
	user := auth.UserFromContext(r.Context())
	oldVal, _ := json.Marshal(existing)
	newVal, _ := json.Marshal(cfg)
	if user != nil {
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			Action:        "unlock",
			EntityType:    "flag_config",
			EntityID:      flag.Key,
			OldValue:      oldVal,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Webhook dispatch
	if h.webhooks != nil {
		h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
			Type:      webhook.EventFlagConfigUnlocked,
			Timestamp: time.Now().UTC(),
			ProjectID: project.ID,
			Actor:     webhookActorFromContext(r.Context()),
			Entity:    newVal,
		})
	}

	writeJSON(w, http.StatusOK, cfg)
}

// BulkLockFlags handles POST /api/v1/projects/{key}/flags/bulk-lock
func (h *FlagHandler) BulkLockFlags(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "missing project key")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		FlagKeys       []string `json:"flag_keys"`
		EnvironmentKey string   `json:"environment_key"`
		Reason         *string  `json:"reason"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.FlagKeys) == 0 {
		writeError(w, http.StatusBadRequest, "flag_keys is required")
		return
	}
	if len(req.FlagKeys) > 200 {
		writeError(w, http.StatusBadRequest, "flag_keys must not exceed 200 entries")
		return
	}
	if req.EnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "environment_key is required")
		return
	}
	if req.Reason != nil && len(*req.Reason) > 255 {
		writeError(w, http.StatusBadRequest, "lock reason must be 255 characters or fewer")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, req.EnvironmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var locked, alreadyLocked int
	errors := make([]string, 0)
	for _, flagKey := range req.FlagKeys {
		f, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
		if err != nil {
			errors = append(errors, flagKey+": not found")
			continue
		}
		cfg, err := h.flags.GetEnvironmentConfig(r.Context(), f.ID, env.ID)
		if err != nil {
			errors = append(errors, flagKey+": config not found")
			continue
		}
		if cfg.Locked {
			alreadyLocked++
			continue
		}
		lockedCfg, err := h.flags.LockEnvironmentConfig(r.Context(), f.ID, env.ID, user.ID, req.Reason)
		if err != nil {
			errors = append(errors, flagKey+": "+err.Error())
			continue
		}
		locked++

		// Best-effort audit + webhook per flag
		newVal, _ := json.Marshal(lockedCfg)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:     &project.ID,
			UserID:        &user.ID,
			UserEmail:     &user.Email,
			EnvironmentID: &env.ID,
			Action:        "lock",
			EntityType:    "flag_config",
			EntityID:      flagKey,
			NewValue:      newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
		if h.webhooks != nil {
			h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
				Type:      webhook.EventFlagConfigLocked,
				Timestamp: time.Now().UTC(),
				ProjectID: project.ID,
				Actor:     webhookActorFromContext(r.Context()),
				Entity:    newVal,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"locked":         locked,
		"already_locked": alreadyLocked,
		"errors":         errors,
	})
}

// BulkUnlockFlags handles POST /api/v1/projects/{key}/flags/bulk-unlock
func (h *FlagHandler) BulkUnlockFlags(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "missing project key")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		FlagKeys       []string `json:"flag_keys"`
		EnvironmentKey string   `json:"environment_key"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.FlagKeys) == 0 {
		writeError(w, http.StatusBadRequest, "flag_keys is required")
		return
	}
	if len(req.FlagKeys) > 200 {
		writeError(w, http.StatusBadRequest, "flag_keys must not exceed 200 entries")
		return
	}
	if req.EnvironmentKey == "" {
		writeError(w, http.StatusBadRequest, "environment_key is required")
		return
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, req.EnvironmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return
	}

	user := auth.UserFromContext(r.Context())

	var unlocked, alreadyUnlocked int
	errors := make([]string, 0)
	for _, flagKey := range req.FlagKeys {
		f, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
		if err != nil {
			errors = append(errors, flagKey+": not found")
			continue
		}
		cfg, err := h.flags.GetEnvironmentConfig(r.Context(), f.ID, env.ID)
		if err != nil {
			errors = append(errors, flagKey+": config not found")
			continue
		}
		if !cfg.Locked {
			alreadyUnlocked++
			continue
		}
		unlockedCfg, err := h.flags.UnlockEnvironmentConfig(r.Context(), f.ID, env.ID)
		if err != nil {
			errors = append(errors, flagKey+": "+err.Error())
			continue
		}
		unlocked++

		// Best-effort audit + webhook per flag
		if user != nil {
			if err := h.audit.Record(r.Context(), model.AuditEntry{
				ProjectID:     &project.ID,
				UserID:        &user.ID,
				UserEmail:     &user.Email,
				EnvironmentID: &env.ID,
				Action:        "unlock",
				EntityType:    "flag_config",
				EntityID:      flagKey,
			}); err != nil {
				slog.Warn("failed to record audit log", "error", err)
			}
		}
		if h.webhooks != nil {
			newVal, _ := json.Marshal(unlockedCfg)
			h.webhooks.Dispatch(r.Context(), project.ID, webhook.Event{
				Type:      webhook.EventFlagConfigUnlocked,
				Timestamp: time.Now().UTC(),
				ProjectID: project.ID,
				Actor:     webhookActorFromContext(r.Context()),
				Entity:    newVal,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"unlocked":         unlocked,
		"already_unlocked": alreadyUnlocked,
		"errors":           errors,
	})
}
