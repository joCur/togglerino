package handler

import (
	"net/http"
	"strconv"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type AuditHandler struct {
	audit    *store.AuditStore
	projects *store.ProjectStore
}

func NewAuditHandler(audit *store.AuditStore, projects *store.ProjectStore) *AuditHandler {
	return &AuditHandler{audit: audit, projects: projects}
}

// List handles GET /api/v1/projects/{key}/audit-log?limit=50&offset=0
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
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

	entries, err := h.audit.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit log")
		return
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}
