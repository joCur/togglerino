package handler

import (
	"errors"
	"net/http"

	"github.com/togglerino/togglerino/internal/store"
)

// UnknownFlagHandler handles unknown flag management endpoints.
type UnknownFlagHandler struct {
	unknownFlags *store.UnknownFlagStore
	projects     *store.ProjectStore
}

// NewUnknownFlagHandler creates a new UnknownFlagHandler.
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

// Dismiss handles DELETE /api/v1/projects/{key}/unknown-flags/{id}
func (h *UnknownFlagHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "unknown flag id is required")
		return
	}

	if err := h.unknownFlags.Dismiss(r.Context(), id, project.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "unknown flag not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to dismiss unknown flag")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
