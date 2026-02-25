package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ProjectHandler struct {
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
}

func NewProjectHandler(projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore) *ProjectHandler {
	return &ProjectHandler{projects: projects, environments: environments, audit: audit}
}

// Create handles POST /api/v1/projects
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key         string `json:"key"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "key and name are required")
		return
	}

	project, err := h.projects.Create(r.Context(), req.Key, req.Name, req.Description)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "project key already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	if err := h.environments.CreateDefaultEnvironments(r.Context(), project.ID); err != nil {
		// Log but don't fail — the project was created successfully
		fmt.Printf("warning: failed to create default environments: %v\n", err)
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(project)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "create",
			EntityType: "project",
			EntityID:   project.Key,
			NewValue:   newVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
		}
	}

	writeJSON(w, http.StatusCreated, project)
}

// List handles GET /api/v1/projects
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projects.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	if projects == nil {
		projects = []model.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
}

// Get handles GET /api/v1/projects/{key}
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// Update handles PUT /api/v1/projects/{key}
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	// Fetch old project for audit log
	oldProject, err := h.projects.FindByKey(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	project, err := h.projects.Update(r.Context(), key, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(oldProject)
		newVal, _ := json.Marshal(project)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "project",
			EntityID:   project.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
		}
	}

	writeJSON(w, http.StatusOK, project)
}

// Delete handles DELETE /api/v1/projects/{key}
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	// Fetch project before deletion for audit log
	project, err := h.projects.FindByKey(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := h.projects.Delete(r.Context(), key); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}

	// Best-effort audit logging (project_id may be invalid after delete due to FK, use nil)
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(project)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  nil,
			UserID:     &user.ID,
			Action:     "delete",
			EntityType: "project",
			EntityID:   project.Key,
			OldValue:   oldVal,
		}); err != nil {
			fmt.Printf("warning: failed to record audit log: %v\n", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
