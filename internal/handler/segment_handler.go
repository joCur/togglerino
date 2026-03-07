package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

var segmentKeyRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)

type SegmentHandler struct {
	segments     *store.SegmentStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
}

func NewSegmentHandler(segments *store.SegmentStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool) *SegmentHandler {
	return &SegmentHandler{segments: segments, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool}
}

// validateSegmentConditions checks that at least one condition exists and that
// none reference other segments (no nesting).
func validateSegmentConditions(conditions []model.Condition) string {
	if len(conditions) == 0 {
		return "at least one condition is required"
	}
	for _, c := range conditions {
		if c.Operator == string(model.OpSegmentMatch) {
			return "segment conditions cannot reference other segments"
		}
	}
	return ""
}

func (h *SegmentHandler) refreshSegmentCache(ctx context.Context, projectKey string) {
	if err := h.cache.RefreshSegments(ctx, h.pool, projectKey); err != nil {
		slog.Warn("failed to refresh segment cache", "project", projectKey, "error", err)
	}
}

func (h *SegmentHandler) refreshSegmentCacheAndBroadcast(ctx context.Context, projectKey, projectID string) {
	h.refreshSegmentCache(ctx, projectKey)
	envs, err := h.environments.ListByProject(ctx, projectID)
	if err != nil {
		slog.Warn("failed to list environments for segment broadcast", "error", err)
		return
	}
	for _, env := range envs {
		h.hub.Broadcast(projectKey, env.Key, stream.Event{Type: "flag_update"})
	}
}

// List handles GET /api/v1/projects/{key}/segments
func (h *SegmentHandler) List(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	segments, totalCount, err := h.segments.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list segments")
		return
	}
	if segments == nil {
		segments = []model.Segment{}
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:       segments,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	})
}

// Create handles POST /api/v1/projects/{key}/segments
func (h *SegmentHandler) Create(w http.ResponseWriter, r *http.Request) {
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
		Key         string           `json:"key"`
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Conditions  []model.Condition `json:"conditions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}
	if !segmentKeyRegex.MatchString(req.Key) {
		writeError(w, http.StatusBadRequest, "invalid segment key: must be 3-64 lowercase alphanumeric characters or hyphens, starting and ending with alphanumeric")
		return
	}

	if msg := validateSegmentConditions(req.Conditions); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conditions")
		return
	}

	segment, err := h.segments.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, conditionsJSON)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "segment key already exists for this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create segment")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(segment)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "create",
			EntityType: "segment",
			EntityID:   segment.Key,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refreshSegmentCache(r.Context(), projectKey)

	writeJSON(w, http.StatusCreated, segment)
}

// Get handles GET /api/v1/projects/{key}/segments/{segmentKey}
func (h *SegmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	segmentKey := r.PathValue("segmentKey")
	if segmentKey == "" {
		writeError(w, http.StatusBadRequest, "segment key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	segment, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	writeJSON(w, http.StatusOK, segment)
}

// Update handles PUT /api/v1/projects/{key}/segments/{segmentKey}
func (h *SegmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	segmentKey := r.PathValue("segmentKey")
	if segmentKey == "" {
		writeError(w, http.StatusBadRequest, "segment key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	segment, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	var req struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Conditions  []model.Condition `json:"conditions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if msg := validateSegmentConditions(req.Conditions); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conditions")
		return
	}

	updated, err := h.segments.Update(r.Context(), segment.ID, req.Name, req.Description, conditionsJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update segment")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(segment)
		newVal, _ := json.Marshal(updated)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "segment",
			EntityID:   segment.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refreshSegmentCacheAndBroadcast(r.Context(), projectKey, project.ID)

	writeJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/projects/{key}/segments/{segmentKey}
func (h *SegmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	segmentKey := r.PathValue("segmentKey")
	if segmentKey == "" {
		writeError(w, http.StatusBadRequest, "segment key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	segment, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	// Check for referencing flags before deleting
	refs, err := h.segments.FindReferencingFlags(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check segment references")
		return
	}
	if len(refs) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":             "segment is referenced by active flags",
			"referencing_flags": refs,
		})
		return
	}

	if err := h.segments.Delete(r.Context(), segment.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete segment")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(segment)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "delete",
			EntityType: "segment",
			EntityID:   segment.Key,
			OldValue:   oldVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	h.refreshSegmentCache(r.Context(), projectKey)

	w.WriteHeader(http.StatusNoContent)
}

// Usage handles GET /api/v1/projects/{key}/segments/{segmentKey}/usage
func (h *SegmentHandler) Usage(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	segmentKey := r.PathValue("segmentKey")
	if segmentKey == "" {
		writeError(w, http.StatusBadRequest, "segment key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Verify segment exists
	if _, err := h.segments.GetByKey(r.Context(), project.ID, segmentKey); err != nil {
		writeError(w, http.StatusNotFound, "segment not found")
		return
	}

	refs, err := h.segments.FindReferencingFlags(r.Context(), project.ID, segmentKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to find referencing flags")
		return
	}
	if refs == nil {
		refs = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"referencing_flags": refs,
	})
}
