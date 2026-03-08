package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type UserHandler struct {
	users   *store.UserStore
	invites *store.InviteStore
	members *store.ProjectMemberStore
	roles   *store.RoleStore
	pool    *pgxpool.Pool
	audit   *store.AuditStore
}

func NewUserHandler(users *store.UserStore, invites *store.InviteStore, members *store.ProjectMemberStore, roles *store.RoleStore, pool *pgxpool.Pool, audit *store.AuditStore) *UserHandler {
	return &UserHandler{users: users, invites: invites, members: members, roles: roles, pool: pool, audit: audit}
}

// GET /api/v1/management/users — returns users with pagination (password_hash stripped via json:"-")
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	users, totalCount, err := h.users.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:       users,
		Total: totalCount,
		Limit:      limit,
		Offset:     offset,
	})
}

// POST /api/v1/management/users/invite — create an invite
func (h *UserHandler) Invite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string     `json:"email"`
		Role  model.Role `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if req.Role == "" {
		req.Role = model.RoleMember
	}
	if req.Role != model.RoleAdmin && req.Role != model.RoleMember {
		writeError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}

	// Generate 32 random bytes, hex-encoded
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	token := hex.EncodeToString(b)

	currentUser := auth.UserFromContext(r.Context())
	var invitedBy *string
	if currentUser != nil {
		invitedBy = &currentUser.ID
	}

	invite := &model.Invite{
		Email:     req.Email,
		Role:      req.Role,
		Token:     token,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		InvitedBy: invitedBy,
	}

	if err := h.invites.Create(r.Context(), invite); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	// Return the invite with the token explicitly included
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         invite.ID,
		"token":      token,
		"expires_at": invite.ExpiresAt,
	})
}

// POST /api/v1/management/users/{id}/reset-password — generate a password reset token (admin-only)
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	// Verify the target user exists
	user, err := h.users.FindByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Generate 32 random bytes, hex-encoded (same approach as Invite)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	token := hex.EncodeToString(b)

	currentUser := auth.UserFromContext(r.Context())
	var createdBy *string
	if currentUser != nil {
		createdBy = &currentUser.ID
	}

	// Reuse the invites table to store the reset token
	invite := &model.Invite{
		Email:     user.Email,
		Role:      user.Role,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		InvitedBy: createdBy,
	}

	if err := h.invites.Create(r.Context(), invite); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_at": invite.ExpiresAt,
	})
}

// DELETE /api/v1/management/users/{id} — delete a user (cannot delete self)
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	currentUser := auth.UserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == id {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	if err := h.users.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/v1/management/users/invites — returns pending invites
func (h *UserHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.invites.ListPending(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

// GET /api/v1/management/users/{id}/projects — list project assignments for a user
func (h *UserHandler) ListProjectAssignments(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	assignments, err := h.members.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list project assignments")
		return
	}
	if assignments == nil {
		assignments = []model.UserProjectAssignment{}
	}
	writeJSON(w, http.StatusOK, assignments)
}

// PUT /api/v1/management/users/{id}/projects — update project assignments for a user
func (h *UserHandler) UpdateProjectAssignments(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id is required")
		return
	}

	var req struct {
		Assignments []struct {
			ProjectID string `json:"project_id"`
			Role      string `json:"role"`
		} `json:"assignments"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate all roles
	for _, a := range req.Assignments {
		if a.ProjectID == "" {
			writeError(w, http.StatusBadRequest, "project_id is required for each assignment")
			return
		}
		exists, err := h.roles.Exists(r.Context(), a.Role)
		if err != nil || !exists {
			writeError(w, http.StatusBadRequest, "invalid role: "+a.Role)
			return
		}
	}

	// Verify user exists
	if _, err := h.users.FindByID(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Get current assignments
	current, err := h.members.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list current assignments")
		return
	}

	// Build maps for diffing
	currentMap := make(map[string]model.ProjectRole, len(current))
	for _, c := range current {
		currentMap[c.ProjectID] = c.Role
	}

	desiredMap := make(map[string]model.ProjectRole, len(req.Assignments))
	for _, a := range req.Assignments {
		desiredMap[a.ProjectID] = model.ProjectRole(a.Role)
	}

	// Use a transaction so all changes are atomic
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())

	// Remove assignments not in the new list
	for _, c := range current {
		if _, exists := desiredMap[c.ProjectID]; !exists {
			_, err := tx.Exec(r.Context(),
				`DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`,
				c.ProjectID, userID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to remove assignment")
				return
			}
		}
	}

	// Add or update assignments
	for projectID, role := range desiredMap {
		if currentRole, exists := currentMap[projectID]; exists {
			if currentRole != role {
				_, err := tx.Exec(r.Context(),
					`UPDATE project_members SET role = $3, updated_at = NOW() WHERE project_id = $1 AND user_id = $2`,
					projectID, userID, role)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "failed to update assignment")
					return
				}
			}
		} else {
			_, err := tx.Exec(r.Context(),
				`INSERT INTO project_members (project_id, user_id, role) VALUES ($1, $2, $3)`,
				projectID, userID, role)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to add assignment")
				return
			}
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit transaction")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(current)
		newVal, _ := json.Marshal(req.Assignments)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			UserID:     &user.ID,
			Action:     "update",
			EntityType: "user_project_assignments",
			EntityID:   userID,
			OldValue:   oldVal,
			NewValue:   newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	// Return updated list
	assignments, err := h.members.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list updated assignments")
		return
	}
	if assignments == nil {
		assignments = []model.UserProjectAssignment{}
	}
	writeJSON(w, http.StatusOK, assignments)
}
