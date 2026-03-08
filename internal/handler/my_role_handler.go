package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
)

// MyRoleHandler handles the current user's effective project role endpoint.
type MyRoleHandler struct {
	resolve   auth.RoleResolver
	roleCache *auth.RoleCache
}

// NewMyRoleHandler creates a new MyRoleHandler.
func NewMyRoleHandler(resolve auth.RoleResolver, roleCache *auth.RoleCache) *MyRoleHandler {
	return &MyRoleHandler{resolve: resolve, roleCache: roleCache}
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

	// Org admins have full access — return all project permissions
	if user.Role == "admin" {
		allPerms := make([]string, len(model.AllProjectPermissions))
		for i, p := range model.AllProjectPermissions {
			allPerms[i] = string(p)
		}
		writeJSON(w, http.StatusOK, map[string]any{"role": "admin", "permissions": allPerms})
		return
	}

	role, err := h.resolve(r.Context(), projectKey, user.ID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"role": "none", "permissions": []string{}})
		return
	}

	perms := h.roleCache.Permissions(string(role))
	if perms == nil {
		perms = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"role": string(role), "permissions": perms})
}
