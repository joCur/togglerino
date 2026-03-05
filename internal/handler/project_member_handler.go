package handler

import (
	"net/http"
	"strings"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// ProjectMemberHandler manages project membership endpoints.
type ProjectMemberHandler struct {
	members  *store.ProjectMemberStore
	projects *store.ProjectStore
	users    *store.UserStore
}

// NewProjectMemberHandler creates a new ProjectMemberHandler.
func NewProjectMemberHandler(members *store.ProjectMemberStore, projects *store.ProjectStore, users *store.UserStore) *ProjectMemberHandler {
	return &ProjectMemberHandler{members: members, projects: projects, users: users}
}

// List returns all members of a project.
// GET /api/v1/projects/{key}/members
func (h *ProjectMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	project, err := h.projects.FindByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
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
		Role   string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
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

	project, err := h.projects.FindByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
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

	project, err := h.projects.FindByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	userID := r.PathValue("userId")
	member, err := h.members.Update(r.Context(), project.ID, userID, model.ProjectRole(req.Role))
	if err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	writeJSON(w, http.StatusOK, member)
}

// Remove deletes a project membership.
// DELETE /api/v1/projects/{key}/members/{userId}
func (h *ProjectMemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	project, err := h.projects.FindByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	userID := r.PathValue("userId")
	if err := h.members.Remove(r.Context(), project.ID, userID); err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
