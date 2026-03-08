package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
)

// ListPermissions returns the canonical list of project-level permissions.
// GET /api/v1/permissions
func ListPermissions(w http.ResponseWriter, r *http.Request) {
	type permissionInfo struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}

	labels := map[model.Permission]string{
		model.PermFlagsRead:         "View flags",
		model.PermFlagsWrite:        "Create & edit flags",
		model.PermEnvironmentsRead:  "View environments",
		model.PermEnvironmentsWrite: "Create environments",
		model.PermSDKKeysManage:     "Manage SDK keys",
		model.PermSegmentsWrite:     "Create & edit segments",
		model.PermTemplatesManage:   "Manage templates",
		model.PermProjectSettings:   "Project settings",
	}

	perms := make([]permissionInfo, 0, len(model.AllProjectPermissions))
	for _, p := range model.AllProjectPermissions {
		perms = append(perms, permissionInfo{
			Key:   string(p),
			Label: labels[p],
		})
	}
	writeJSON(w, http.StatusOK, perms)
}
