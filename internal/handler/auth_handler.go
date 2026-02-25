package handler

import (
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type AuthHandler struct {
	users    *store.UserStore
	sessions *store.SessionStore
	invites  *store.InviteStore
}

func NewAuthHandler(users *store.UserStore, sessions *store.SessionStore, invites *store.InviteStore) *AuthHandler {
	return &AuthHandler{users: users, sessions: sessions, invites: invites}
}

// POST /api/v1/auth/setup — create the initial admin user (only works when no users exist)
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	count, err := h.users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.users.Create(r.Context(), req.Email, hash, model.RoleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	session, err := h.sessions.Create(r.Context(), user.ID, 7*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	writeJSON(w, http.StatusCreated, user)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.users.FindByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	session, err := h.sessions.Create(r.Context(), user.ID, 7*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	writeJSON(w, http.StatusOK, user)
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.sessions.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// GET /api/v1/auth/me — returns the current user (requires session)
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// GET /api/v1/auth/status — returns whether setup is needed (no auth required)
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": count == 0,
	})
}

// POST /api/v1/auth/reset-password — reset password using a token (public, rate-limited)
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	invite, err := h.invites.FindByToken(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusGone, "token has expired")
		return
	}

	// Atomically claim the token to prevent reuse
	claimed, err := h.invites.MarkAccepted(r.Context(), invite.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to process reset token")
		return
	}
	if !claimed {
		writeError(w, http.StatusConflict, "token already used")
		return
	}

	// Find the user by email from the invite record
	user, err := h.users.FindByEmail(r.Context(), invite.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.users.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/auth/accept-invite — accept an invite and create a new user account
func (h *AuthHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	invite, err := h.invites.FindByToken(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusNotFound, "invite not found")
		return
	}

	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusGone, "invite has expired")
		return
	}

	// Atomically claim the invite. The conditional UPDATE ensures only one
	// concurrent request can succeed, preventing the TOCTOU race where two
	// requests both see accepted_at == nil before either marks it accepted.
	claimed, err := h.invites.MarkAccepted(r.Context(), invite.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark invite accepted")
		return
	}
	if !claimed {
		writeError(w, http.StatusConflict, "invite already accepted")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_, err = h.users.Create(r.Context(), invite.Email, hash, invite.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"email": invite.Email,
	})
}
