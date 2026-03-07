package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type HistoryHandler struct {
	audit        *store.AuditStore
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
}

func NewHistoryHandler(audit *store.AuditStore, flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore) *HistoryHandler {
	return &HistoryHandler{audit: audit, flags: flags, projects: projects, environments: environments}
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

	limit, offset := parsePagination(r)

	var envID *string
	if envKey := r.URL.Query().Get("env"); envKey != "" {
		env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
		if err != nil {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		envID = &env.ID
	}

	entries, totalCount, err := h.audit.ListByFlag(r.Context(), project.ID, flagKey, envID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list flag history")
		return
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:       entries,
		Total: totalCount,
		Limit:      limit,
		Offset:     offset,
	})
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
