package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/oidc"
	"github.com/togglerino/togglerino/internal/store"
)

type OIDCHandler struct {
	oidcStore    *store.OIDCStore
	userStore    *store.UserStore
	sessionStore *store.SessionStore
	secret       []byte
	baseURL      string

	mu       sync.RWMutex
	provider *oidc.Provider
}

func NewOIDCHandler(oidcStore *store.OIDCStore, userStore *store.UserStore, sessionStore *store.SessionStore, secret []byte, baseURL string) *OIDCHandler {
	return &OIDCHandler{
		oidcStore:    oidcStore,
		userStore:    userStore,
		sessionStore: sessionStore,
		secret:       secret,
		baseURL:      baseURL,
	}
}

// InitProvider loads the OIDC provider from DB config on startup.
func (h *OIDCHandler) InitProvider(ctx context.Context, callbackURL string, envIssuer, envClientID, envClientSecret, envScopes string) {
	dbProvider, err := h.oidcStore.GetProvider(ctx)
	if err != nil {
		slog.Error("failed to load oidc provider from db", "error", err)
		return
	}

	issuer := envIssuer
	clientID := envClientID
	clientSecret := envClientSecret
	scopes := envScopes

	if dbProvider != nil && issuer == "" {
		if !dbProvider.Enabled {
			slog.Info("oidc provider is disabled, skipping init")
			return
		}
		issuer = dbProvider.IssuerURL
		clientID = dbProvider.ClientID
		clientSecret = dbProvider.ClientSecret
		scopes = dbProvider.Scopes
	}

	if issuer == "" || clientID == "" || clientSecret == "" {
		slog.Info("oidc not configured, skipping provider init")
		return
	}

	p, err := oidc.NewProvider(ctx, issuer, clientID, clientSecret, scopes, callbackURL)
	if err != nil {
		slog.Error("failed to initialize oidc provider", "error", err)
		return
	}

	h.mu.Lock()
	h.provider = p
	h.mu.Unlock()
	slog.Info("oidc provider initialized", "issuer", issuer)
}

func (h *OIDCHandler) getProvider() *oidc.Provider {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.provider
}

// IsConfigured returns true if an OIDC provider is active.
func (h *OIDCHandler) IsConfigured() bool {
	return h.getProvider() != nil
}

// Authorize starts the OIDC flow by redirecting to the identity provider.
func (h *OIDCHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	p := h.getProvider()
	if p == nil {
		writeError(w, http.StatusNotFound, "oidc not configured")
		return
	}

	state, err := oidc.GenerateRandom(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	nonce, err := oidc.GenerateRandom(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := oidc.SetStateCookie(w, h.secret, oidc.StateData{State: state, Nonce: nonce}); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.Redirect(w, r, p.AuthURL(state, nonce), http.StatusFound)
}

// Callback handles the OIDC callback from the identity provider.
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	p := h.getProvider()
	if p == nil {
		writeError(w, http.StatusNotFound, "oidc not configured")
		return
	}

	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		slog.Warn("oidc callback error", "error", errParam, "description", desc)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	stateData, err := oidc.GetStateCookie(r, h.secret)
	if err != nil {
		slog.Warn("oidc state cookie invalid", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}
	oidc.ClearStateCookie(w)

	queryState := r.URL.Query().Get("state")
	if queryState != stateData.State {
		slog.Warn("oidc state mismatch")
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	claims, err := p.Exchange(r.Context(), code, stateData.Nonce)
	if err != nil {
		slog.Error("oidc token exchange failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	dbProvider, err := h.oidcStore.GetProvider(r.Context())
	if err != nil || dbProvider == nil {
		slog.Error("oidc provider not found in db after successful auth")
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	if !dbProvider.Enabled {
		slog.Warn("oidc provider is disabled")
		http.Redirect(w, r, "/?error=oidc_disabled", http.StatusFound)
		return
	}

	identity, err := h.oidcStore.FindIdentity(r.Context(), dbProvider.ID, claims.Subject)
	if err != nil {
		slog.Error("oidc identity lookup failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	if identity != nil {
		h.createSessionAndRedirect(w, r, identity.UserID)
		return
	}

	if claims.Email != "" {
		existingUser, err := h.userStore.FindByEmail(r.Context(), claims.Email)
		if err == nil && existingUser != nil {
			if err := oidc.SetPendingLinkCookie(w, h.secret, oidc.PendingLink{
				ProviderID: dbProvider.ID,
				Subject:    claims.Subject,
				Email:      claims.Email,
			}); err != nil {
				slog.Error("failed to set pending link cookie", "error", err)
				http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
				return
			}
			http.Redirect(w, r, "/link-account", http.StatusFound)
			return
		}
	} else {
		slog.Error("oidc provider did not return an email claim, cannot provision user")
		http.Redirect(w, r, "/?error=oidc_no_email", http.StatusFound)
		return
	}

	user, err := h.userStore.Create(r.Context(), claims.Email, "", dbProvider.DefaultRole)
	if err != nil {
		slog.Error("oidc user provisioning failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	if claims.Name != "" {
		if _, err := h.userStore.UpdateProfile(r.Context(), user.ID, nil, &claims.Name); err != nil {
			slog.Error("oidc profile update failed", "user_id", user.ID, "error", err)
		}
	}

	if err := h.oidcStore.CreateIdentity(r.Context(), &model.OIDCIdentity{
		UserID:     user.ID,
		ProviderID: dbProvider.ID,
		Subject:    claims.Subject,
		Email:      claims.Email,
	}); err != nil {
		slog.Error("oidc identity creation failed", "error", err)
		if delErr := h.userStore.Delete(r.Context(), user.ID); delErr != nil {
			slog.Error("failed to clean up orphaned user after identity creation failure", "user_id", user.ID, "error", delErr)
		}
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	h.createSessionAndRedirect(w, r, user.ID)
}

// Link confirms account linking with password verification.
func (h *OIDCHandler) Link(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	pending, err := oidc.GetPendingLinkCookie(r, h.secret)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no pending link - please start the SSO flow again")
		return
	}

	user, err := h.userStore.FindByEmail(r.Context(), pending.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	if err := h.oidcStore.CreateIdentity(r.Context(), &model.OIDCIdentity{
		UserID:     user.ID,
		ProviderID: pending.ProviderID,
		Subject:    pending.Subject,
		Email:      pending.Email,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to link account")
		return
	}

	oidc.ClearPendingLinkCookie(w)

	session, err := h.sessionStore.Create(r.Context(), user.ID, 7*24*time.Hour)
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

// GetConfig returns the current OIDC config (admin-only, secret redacted).
func (h *OIDCHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	provider, err := h.oidcStore.GetProvider(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load oidc config")
		return
	}
	if provider == nil {
		writeJSON(w, http.StatusOK, map[string]any{"configured": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"provider":   provider,
	})
}

// UpdateConfig creates or updates the OIDC provider config (admin-only).
func (h *OIDCHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string     `json:"name"`
		IssuerURL    string     `json:"issuer_url"`
		ClientID     string     `json:"client_id"`
		ClientSecret string     `json:"client_secret"`
		Scopes       string     `json:"scopes"`
		DefaultRole  model.Role `json:"default_role"`
		Enabled      bool       `json:"enabled"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.IssuerURL == "" || req.ClientID == "" {
		writeError(w, http.StatusBadRequest, "name, issuer_url, and client_id are required")
		return
	}

	// On updates, allow omitting client_secret to keep the existing one
	if req.ClientSecret == "" {
		existing, err := h.oidcStore.GetProvider(r.Context())
		if err != nil || existing == nil {
			writeError(w, http.StatusBadRequest, "client_secret is required for initial setup")
			return
		}
		req.ClientSecret = existing.ClientSecret
	}

	if req.DefaultRole != model.RoleAdmin && req.DefaultRole != model.RoleMember {
		req.DefaultRole = model.RoleMember
	}
	if req.Scopes == "" {
		req.Scopes = "openid email profile"
	}

	provider := &model.OIDCProvider{
		Name:         req.Name,
		IssuerURL:    req.IssuerURL,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		Scopes:       req.Scopes,
		DefaultRole:  req.DefaultRole,
		Enabled:      req.Enabled,
	}

	if err := h.oidcStore.UpsertProvider(r.Context(), provider); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save oidc config")
		return
	}

	h.mu.Lock()
	if req.Enabled {
		callbackURL := h.callbackURL(r)
		p, err := oidc.NewProvider(r.Context(), req.IssuerURL, req.ClientID, req.ClientSecret, req.Scopes, callbackURL)
		if err != nil {
			h.mu.Unlock()
			slog.Error("oidc provider rebuild failed after config update", "error", err)
			writeError(w, http.StatusBadRequest, "failed to connect to oidc issuer - config saved but provider not active")
			return
		}
		h.provider = p
	} else {
		h.provider = nil
	}
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, provider)
}

// DeleteConfig removes the OIDC provider config (admin-only).
func (h *OIDCHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	provider, err := h.oidcStore.GetProvider(r.Context())
	if err != nil || provider == nil {
		writeError(w, http.StatusNotFound, "no oidc config found")
		return
	}

	if err := h.oidcStore.DeleteProvider(r.Context(), provider.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete oidc config")
		return
	}

	h.mu.Lock()
	h.provider = nil
	h.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

// OIDCIdentities returns OIDC identities for the current user.
func (h *OIDCHandler) OIDCIdentities(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	identities, err := h.oidcStore.FindIdentitiesByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load identities")
		return
	}

	if identities == nil {
		identities = []model.OIDCIdentity{}
	}

	writeJSON(w, http.StatusOK, identities)
}

func (h *OIDCHandler) createSessionAndRedirect(w http.ResponseWriter, r *http.Request, userID string) {
	session, err := h.sessionStore.Create(r.Context(), userID, 7*24*time.Hour)
	if err != nil {
		slog.Error("oidc session creation failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
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

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *OIDCHandler) callbackURL(r *http.Request) string {
	if h.baseURL != "" {
		return h.baseURL + "/api/v1/auth/oidc/callback"
	}
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd == "http" || fwd == "https" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host + "/api/v1/auth/oidc/callback"
}
