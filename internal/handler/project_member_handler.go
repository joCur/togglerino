package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// ProjectMemberHandler manages project membership endpoints.
type ProjectMemberHandler struct {
	members  *store.ProjectMemberStore
	projects *store.ProjectStore
	users    *store.UserStore
	audit    *store.AuditStore
}

// NewProjectMemberHandler creates a new ProjectMemberHandler.
func NewProjectMemberHandler(members *store.ProjectMemberStore, projects *store.ProjectStore, users *store.UserStore, audit *store.AuditStore) *ProjectMemberHandler {
	return &ProjectMemberHandler{members: members, projects: projects, users: users, audit: audit}
}

// List returns all members of a project.
// GET /api/v1/projects/{key}/members
func (h *ProjectMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		var err error
		project, err = h.projects.FindByKey(r.Context(), r.PathValue("key"))
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
	}

	members, err := h.members.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	if members == nil {
		members = []model.ProjectMemberWithUser{}
	}
	writeJSON(w, http.StatusOK, members)
}

// Add adds a user as a member of a project.
// POST /api/v1/projects/{key}/members
func (h *ProjectMemberHandler) Add(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Role   string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Resolve user_id from email if needed
	if req.UserID == "" && req.Email != "" {
		user, err := h.users.FindByEmail(r.Context(), req.Email)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		req.UserID = user.ID
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id or email is required")
		return
	}
	if req.Role == "" {
		writeError(w, http.StatusBadRequest, "role is required")
		return
	}
	if !model.ValidProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "role must be admin, editor, or viewer")
		return
	}

	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		var err error
		project, err = h.projects.FindByKey(r.Context(), r.PathValue("key"))
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
	}

	// Verify the user exists.
	if _, err := h.users.FindByID(r.Context(), req.UserID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	member, err := h.members.Add(r.Context(), project.ID, req.UserID, model.ProjectRole(req.Role))
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "user is already a member of this project")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		newVal, _ := json.Marshal(member)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "create",
			EntityType: "project_member",
			EntityID:   req.UserID,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, member)
}

// Update changes the role of an existing project member.
// PUT /api/v1/projects/{key}/members/{userId}
func (h *ProjectMemberHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !model.ValidProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "role must be admin, editor, or viewer")
		return
	}

	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		var err error
		project, err = h.projects.FindByKey(r.Context(), r.PathValue("key"))
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
	}

	userID := r.PathValue("userId")

	// Read old role before updating (for audit logging).
	oldRole, _ := h.members.GetRole(r.Context(), project.ID, userID)

	// Prevent demoting the last project admin.
	if oldRole == model.ProjectRoleAdmin && model.ProjectRole(req.Role) != model.ProjectRoleAdmin {
		members, _ := h.members.ListByProject(r.Context(), project.ID)
		adminCount := 0
		for _, m := range members {
			if model.ProjectRole(m.Role) == model.ProjectRoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			writeError(w, http.StatusBadRequest, "cannot demote the only project admin")
			return
		}
	}

	member, err := h.members.Update(r.Context(), project.ID, userID, model.ProjectRole(req.Role))
	if err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(map[string]string{"user_id": userID, "role": string(oldRole)})
		newVal, _ := json.Marshal(member)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "project_member",
			EntityID:   userID,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, member)
}

// Remove deletes a project membership.
// DELETE /api/v1/projects/{key}/members/{userId}
func (h *ProjectMemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		var err error
		project, err = h.projects.FindByKey(r.Context(), r.PathValue("key"))
		if err != nil {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
	}

	userID := r.PathValue("userId")

	// Read old role before removing (for audit logging).
	oldRole, _ := h.members.GetRole(r.Context(), project.ID, userID)

	// Prevent removing the last project admin.
	if oldRole == model.ProjectRoleAdmin {
		members, _ := h.members.ListByProject(r.Context(), project.ID)
		adminCount := 0
		for _, m := range members {
			if model.ProjectRole(m.Role) == model.ProjectRoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			writeError(w, http.StatusBadRequest, "cannot remove the only project admin")
			return
		}
	}

	if err := h.members.Remove(r.Context(), project.ID, userID); err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(map[string]string{"user_id": userID, "role": string(oldRole)})
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID:  &project.ID,
			UserID:     &user.ID,
			Action:     "delete",
			EntityType: "project_member",
			EntityID:   userID,
			OldValue:   oldVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
