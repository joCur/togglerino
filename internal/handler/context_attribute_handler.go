package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ContextAttributeHandler struct {
	contextAttrs *store.ContextAttributeStore
	projects     *store.ProjectStore
}

func NewContextAttributeHandler(contextAttrs *store.ContextAttributeStore, projects *store.ProjectStore) *ContextAttributeHandler {
	return &ContextAttributeHandler{contextAttrs: contextAttrs, projects: projects}
}

// List handles GET /api/v1/projects/{key}/context-attributes
func (h *ContextAttributeHandler) List(w http.ResponseWriter, r *http.Request) {
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

	attrs, err := h.contextAttrs.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list context attributes")
		return
	}
	if attrs == nil {
		attrs = []model.ContextAttribute{}
	}

	writeJSON(w, http.StatusOK, attrs)
}
