package handler

import (
	"encoding/json"
	"fmt"
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
}

func NewFlagHandler(flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool) *FlagHandler {
	return &FlagHandler{flags: flags, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool}
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
		Key          string          `json:"key"`
		Name         string          `json:"name"`
		Description  string          `json:"description"`
		FlagType     model.FlagType  `json:"flag_type"`
		DefaultValue json.RawMessage `json:"default_value"`
		Tags         []string        `json:"tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}
	if req.FlagType == "" {
		req.FlagType = model.FlagTypeBoolean
	}
	if req.DefaultValue == nil {
		req.DefaultValue = json.RawMessage(`false`)
	}
	if req.Tags == nil {
		req.Tags = []string{}
	}

	flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.FlagType, req.DefaultValue, req.Tags)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "flag key already exists for this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create flag")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(flag)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "create",
			EntityType: "flag",
			EntityID:   flag.Key,
			NewValue:   newVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
		}
	}

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

	flags, err := h.flags.ListByProject(r.Context(), project.ID, tag, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list flags")
		return
	}
	if flags == nil {
		flags = []model.Flag{}
	}
	writeJSON(w, http.StatusOK, flags)
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
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.flags.Update(r.Context(), flag.ID, req.Name, req.Description, req.Tags)
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
			Action:     "update",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
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
			Action:     "delete",
			EntityType: "flag",
			EntityID:   flag.Key,
			OldValue:   oldVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
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

	cfg, err := h.flags.UpdateEnvironmentConfig(r.Context(), flag.ID, env.ID, req.Enabled, req.DefaultVariant, req.Variants, req.TargetingRules)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update environment config")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(cfg)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "flag_config",
			EntityID:   flag.Key,
			NewValue:   newVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
		}
	}

	// Refresh cache and broadcast SSE event
	if err := h.cache.Refresh(r.Context(), h.pool, projectKey, envKey); err != nil {
		fmt.Printf("warning: failed to refresh cache: %v\n", err)
	}
	h.hub.Broadcast(projectKey, envKey, stream.Event{
		FlagKey: flagKey,
		Value:   cfg.Enabled,
		Variant: cfg.DefaultVariant,
	})

	writeJSON(w, http.StatusOK, cfg)
}
