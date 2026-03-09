package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ScheduleHandler struct {
	schedules    *store.ScheduleStore
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
}

func NewScheduleHandler(schedules *store.ScheduleStore, flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore) *ScheduleHandler {
	return &ScheduleHandler{schedules: schedules, flags: flags, projects: projects, environments: environments, audit: audit}
}

// List handles GET /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules
func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	_, flag, env, ok := h.resolveContext(w, r)
	if !ok {
		return
	}

	schedules, err := h.schedules.ListByFlagEnvironment(r.Context(), flag.ID, env.ID)
	if err != nil {
		slog.Error("failed to list schedules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}
	if schedules == nil {
		schedules = []model.ScheduledFlagChange{}
	}
	writeJSON(w, http.StatusOK, schedules)
}

// Create handles POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules
func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	project, flag, env, ok := h.resolveContext(w, r)
	if !ok {
		return
	}

	if flag.LifecycleStatus == model.LifecycleArchived {
		writeError(w, http.StatusConflict, "cannot schedule changes for an archived flag")
		return
	}

	var req struct {
		ScheduledAt    string          `json:"scheduled_at"`
		ConfigSnapshot json.RawMessage `json:"config_snapshot"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scheduled_at must be a valid RFC3339 timestamp")
		return
	}
	if !scheduledAt.After(time.Now()) {
		writeError(w, http.StatusBadRequest, "scheduled_at must be in the future")
		return
	}

	// Validate config snapshot has required fields
	var snapshot model.ConfigSnapshotPayload
	if err := json.Unmarshal(req.ConfigSnapshot, &snapshot); err != nil {
		writeError(w, http.StatusBadRequest, "invalid config_snapshot")
		return
	}

	var createdBy *string
	if user := auth.UserFromContext(r.Context()); user != nil {
		createdBy = &user.ID
	}

	sc, err := h.schedules.Create(r.Context(), flag.ID, env.ID, scheduledAt, req.ConfigSnapshot, createdBy)
	if err != nil {
		slog.Error("failed to create schedule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create schedule")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(sc)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "schedule_created",
			EntityType: "flag_config",
			EntityID:   flag.Key,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, sc)
}

// Update handles PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}
func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	project, flag, env, ok := h.resolveContext(w, r)
	if !ok {
		return
	}

	scheduleID := r.PathValue("id")
	if scheduleID == "" {
		writeError(w, http.StatusBadRequest, "schedule id is required")
		return
	}

	var req struct {
		ScheduledAt    string          `json:"scheduled_at"`
		ConfigSnapshot json.RawMessage `json:"config_snapshot"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	scheduledAt, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "scheduled_at must be a valid RFC3339 timestamp")
		return
	}
	if !scheduledAt.After(time.Now()) {
		writeError(w, http.StatusBadRequest, "scheduled_at must be in the future")
		return
	}

	var snapshot model.ConfigSnapshotPayload
	if err := json.Unmarshal(req.ConfigSnapshot, &snapshot); err != nil {
		writeError(w, http.StatusBadRequest, "invalid config_snapshot")
		return
	}

	sc, err := h.schedules.Update(r.Context(), scheduleID, flag.ID, env.ID, scheduledAt, req.ConfigSnapshot)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "schedule not found or not pending")
		return
	}
	if err != nil {
		slog.Error("failed to update schedule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update schedule")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(sc)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "schedule_updated",
			EntityType: "flag_config",
			EntityID:   flag.Key,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, sc)
}

// Cancel handles DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}
func (h *ScheduleHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	project, flag, env, ok := h.resolveContext(w, r)
	if !ok {
		return
	}

	scheduleID := r.PathValue("id")
	if scheduleID == "" {
		writeError(w, http.StatusBadRequest, "schedule id is required")
		return
	}

	err := h.schedules.Cancel(r.Context(), scheduleID, flag.ID, env.ID, "user_cancelled")
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "schedule not found or not pending")
		return
	}
	if err != nil {
		slog.Error("failed to cancel schedule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to cancel schedule")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "schedule_cancelled",
			EntityType: "flag_config",
			EntityID:   flag.Key,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// resolveContext extracts and validates the project, flag, and environment from the request path.
func (h *ScheduleHandler) resolveContext(w http.ResponseWriter, r *http.Request) (*model.Project, *model.Flag, *model.Environment, bool) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return nil, nil, nil, false
	}

	flagKey := r.PathValue("flag")
	if flagKey == "" {
		writeError(w, http.StatusBadRequest, "flag key is required")
		return nil, nil, nil, false
	}

	envKey := r.PathValue("env")
	if envKey == "" {
		writeError(w, http.StatusBadRequest, "environment key is required")
		return nil, nil, nil, false
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return nil, nil, nil, false
	}

	flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "flag not found")
		return nil, nil, nil, false
	}

	env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "environment not found")
		return nil, nil, nil, false
	}

	return project, flag, env, true
}
