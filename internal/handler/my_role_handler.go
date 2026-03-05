package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
)

// MyRoleHandler handles the current user's effective project role endpoint.
type MyRoleHandler struct {
	resolve auth.RoleResolver
}

// NewMyRoleHandler creates a new MyRoleHandler.
func NewMyRoleHandler(resolve auth.RoleResolver) *MyRoleHandler {
	return &MyRoleHandler{resolve: resolve}
}

// GetProjectRole returns the current user's effective role for a project.
// GET /api/v1/auth/me/project-role/{key}
func (h *MyRoleHandler) GetProjectRole(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	// Org admins have full access
	if user.Role == "admin" {
		writeJSON(w, http.StatusOK, map[string]string{"role": "admin"})
		return
	}

	role, err := h.resolve(r.Context(), projectKey, user.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"role": "none"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"role": string(role)})
}
