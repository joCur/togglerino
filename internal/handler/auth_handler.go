package handler

import (
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type AuthHandler struct {
	users          *store.UserStore
	sessions       *store.SessionStore
	invites        *store.InviteStore
	baseURL        string
	oidcConfigured func() bool
}

// SetOIDCChecker sets the function used to check if OIDC is configured.
func (h *AuthHandler) SetOIDCChecker(fn func() bool) {
	h.oidcConfigured = fn
}

func NewAuthHandler(users *store.UserStore, sessions *store.SessionStore, invites *store.InviteStore, baseURL string) *AuthHandler {
	return &AuthHandler{users: users, sessions: sessions, invites: invites, baseURL: baseURL}
}

func (h *AuthHandler) secureCookies() bool {
	return strings.HasPrefix(h.baseURL, "https://")
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
		slog.Error("failed to count users", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.users.Create(r.Context(), req.Email, hash, model.RoleAdmin)
	if err != nil {
		slog.Error("failed to create user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	session, err := h.sessions.Create(r.Context(), user.ID, 7*24*time.Hour)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies(),
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
		slog.Error("failed to create session", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies(),
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
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
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

	// Build org-level permissions list
	permissions := []string{}
	for _, perm := range []model.Permission{
		model.PermOrgUsersManage,
		model.PermOrgOIDCManage,
		model.PermOrgProjectsCreate,
		model.PermOrgProjectsDelete,
	} {
		if user.Role.HasOrgPermission(perm) {
			permissions = append(permissions, string(perm))
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"created_at":   user.CreatedAt,
		"updated_at":   user.UpdatedAt,
		"permissions":  permissions,
	})
}

// GET /api/v1/auth/status — returns whether setup is needed (no auth required)
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.Count(r.Context())
	if err != nil {
		slog.Error("failed to count users", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	oidcEnabled := h.oidcConfigured != nil && h.oidcConfigured()
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": count == 0,
		"oidc_enabled":   oidcEnabled,
	})
}

// PUT /api/v1/auth/me — update own profile (session-authed)
func (h *AuthHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Email       *string `json:"email"`
		DisplayName *string `json:"display_name"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == nil && req.DisplayName == nil {
		writeError(w, http.StatusBadRequest, "at least one field is required")
		return
	}

	if req.Email != nil {
		if *req.Email == "" {
			writeError(w, http.StatusBadRequest, "email cannot be empty")
			return
		}
		if _, err := mail.ParseAddress(*req.Email); err != nil {
			writeError(w, http.StatusBadRequest, "invalid email format")
			return
		}
	}

	updated, err := h.users.UpdateProfile(r.Context(), user.ID, req.Email, req.DisplayName)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "email already in use")
			return
		}
		slog.Error("failed to update profile", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// POST /api/v1/auth/change-password — change own password (session-authed)
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.CurrentPassword == "" {
		writeError(w, http.StatusBadRequest, "current password is required")
		return
	}
	if req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "new password is required")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	dbUser, err := h.users.FindByID(r.Context(), user.ID)
	if err != nil {
		slog.Error("failed to find user by ID", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !auth.VerifyPassword(dbUser.PasswordHash, req.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "current password is incorrect")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.users.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		slog.Error("failed to update password", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
		slog.Error("failed to process reset token", "error", err)
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
		slog.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.users.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		slog.Error("failed to update password", "error", err)
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
		slog.Error("failed to mark invite accepted", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mark invite accepted")
		return
	}
	if !claimed {
		writeError(w, http.StatusConflict, "invite already accepted")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_, err = h.users.Create(r.Context(), invite.Email, hash, invite.Role)
	if err != nil {
		slog.Error("failed to create user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"email": invite.Email,
	})
}
