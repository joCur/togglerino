package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// EnvironmentAccessHandler manages per-project, per-role environment access restrictions.
type EnvironmentAccessHandler struct {
	envAccess    *store.EnvironmentAccessStore
	environments *store.EnvironmentStore
	projects     *store.ProjectStore
	roles        *store.RoleStore
	audit        *store.AuditStore
}

// NewEnvironmentAccessHandler creates a new EnvironmentAccessHandler.
func NewEnvironmentAccessHandler(envAccess *store.EnvironmentAccessStore, environments *store.EnvironmentStore, projects *store.ProjectStore, roles *store.RoleStore, audit *store.AuditStore) *EnvironmentAccessHandler {
	return &EnvironmentAccessHandler{
		envAccess:    envAccess,
		environments: environments,
		projects:     projects,
		roles:        roles,
		audit:        audit,
	}
}

type environmentAccessResponse struct {
	Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
	Environments []environmentSummary                 `json:"environments"`
}

type environmentSummary struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Get returns the current environment access restrictions and environments for a project.
// GET /api/v1/projects/{key}/environment-access
func (h *EnvironmentAccessHandler) Get(w http.ResponseWriter, r *http.Request) {
	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	restrictions, err := h.envAccess.ListByProject(r.Context(), project.ID)
	if err != nil {
		slog.Error("failed to list environment access restrictions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list environment access restrictions")
		return
	}
	if restrictions == nil {
		restrictions = []store.EnvironmentAccessRestriction{}
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		slog.Error("failed to list environments", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}

	summaries := make([]environmentSummary, len(envs))
	for i, e := range envs {
		summaries[i] = environmentSummary{
			ID:   e.ID,
			Key:  e.Key,
			Name: e.Name,
		}
	}

	writeJSON(w, http.StatusOK, environmentAccessResponse{
		Restrictions: restrictions,
		Environments: summaries,
	})
}

// Update replaces all environment access restrictions for a project.
// PUT /api/v1/projects/{key}/environment-access
func (h *EnvironmentAccessHandler) Update(w http.ResponseWriter, r *http.Request) {
	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// nil/empty restrictions = remove all restrictions (unrestricted access for all roles)

	// Validate role names exist
	for _, restriction := range req.Restrictions {
		exists, err := h.roles.Exists(r.Context(), restriction.RoleName)
		if err != nil {
			slog.Error("failed to validate role", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to validate role")
			return
		}
		if !exists {
			writeError(w, http.StatusBadRequest, "role not found: "+restriction.RoleName)
			return
		}
	}

	// Validate environment IDs belong to the project
	if len(req.Restrictions) > 0 {
		envs, err := h.environments.ListByProject(r.Context(), project.ID)
		if err != nil {
			slog.Error("failed to list environments", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to list environments")
			return
		}
		validEnvIDs := make(map[string]bool, len(envs))
		for _, e := range envs {
			validEnvIDs[e.ID] = true
		}
		for _, restriction := range req.Restrictions {
			for _, envID := range restriction.EnvironmentIDs {
				if !validEnvIDs[envID] {
					writeError(w, http.StatusBadRequest, "environment not found in project: "+envID)
					return
				}
			}
		}
	}

	// Get old restrictions for audit log
	oldRestrictions, _ := h.envAccess.ListByProject(r.Context(), project.ID)

	if err := h.envAccess.ReplaceForProject(r.Context(), project.ID, req.Restrictions); err != nil {
		slog.Error("failed to update environment access restrictions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update environment access restrictions")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(oldRestrictions)
		newVal, _ := json.Marshal(req.Restrictions)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			UserEmail:  &user.Email,
			Action:     "update",
			EntityType: "environment_access",
			EntityID:   project.Key,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
