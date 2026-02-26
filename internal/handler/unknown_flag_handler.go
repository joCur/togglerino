package handler

import (
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/store"
)

type UnknownFlagHandler struct {
	unknownFlags *store.UnknownFlagStore
	projects     *store.ProjectStore
}

func NewUnknownFlagHandler(unknownFlags *store.UnknownFlagStore, projects *store.ProjectStore) *UnknownFlagHandler {
	return &UnknownFlagHandler{unknownFlags: unknownFlags, projects: projects}
}

// List handles GET /api/v1/projects/{key}/unknown-flags
func (h *UnknownFlagHandler) List(w http.ResponseWriter, r *http.Request) {
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

	flags, err := h.unknownFlags.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unknown flags")
		return
	}

	writeJSON(w, http.StatusOK, flags)
}

// Dismiss handles POST /api/v1/projects/{key}/unknown-flags/{id}/dismiss
func (h *UnknownFlagHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "unknown flag id is required")
		return
	}

	if err := h.unknownFlags.Dismiss(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "unknown flag not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to dismiss unknown flag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
