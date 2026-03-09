package handler

import (
	"log/slog"
	"errors"
	"net/http"

	"github.com/togglerino/togglerino/internal/store"
)

// OrgSettingsHandler manages organization-wide settings endpoints.
type OrgSettingsHandler struct {
	orgSettings *store.OrgSettingsStore
}

// NewOrgSettingsHandler creates a new OrgSettingsHandler.
func NewOrgSettingsHandler(orgSettings *store.OrgSettingsStore) *OrgSettingsHandler {
	return &OrgSettingsHandler{orgSettings: orgSettings}
}

// GetBaseProjectRole returns the current base project role setting.
// GET /api/v1/settings/base-project-role
func (h *OrgSettingsHandler) GetBaseProjectRole(w http.ResponseWriter, r *http.Request) {
	role, err := h.orgSettings.GetBaseProjectRole(r.Context())
	if err != nil {
		slog.Error("failed to get base project role", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get base project role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_project_role": role})
}

// SetBaseProjectRole updates the base project role setting.
// PUT /api/v1/settings/base-project-role
func (h *OrgSettingsHandler) SetBaseProjectRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseProjectRole string `json:"base_project_role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.BaseProjectRole == "" {
		writeError(w, http.StatusBadRequest, "base_project_role is required")
		return
	}

	if err := h.orgSettings.SetBaseProjectRole(r.Context(), req.BaseProjectRole); err != nil {
		if errors.Is(err, store.ErrInvalidRole) {
			writeError(w, http.StatusBadRequest, "invalid role")
			return
		}
		slog.Error("failed to set base project role", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to set base project role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_project_role": req.BaseProjectRole})
}
