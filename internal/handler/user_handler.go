package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type UserHandler struct {
	users   *store.UserStore
	invites *store.InviteStore
}

func NewUserHandler(users *store.UserStore, invites *store.InviteStore) *UserHandler {
	return &UserHandler{users: users, invites: invites}
}

// GET /api/v1/management/users — returns all users (password_hash stripped via json:"-")
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
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
