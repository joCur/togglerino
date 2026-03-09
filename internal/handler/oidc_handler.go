package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
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

func (h *OIDCHandler) secureCookies() bool {
	return strings.HasPrefix(h.baseURL, "https://")
}

// InitProvider loads the OIDC provider from DB config on startup.
// When env vars are set, they take priority and are synced to the DB so the
// callback flow can look up the provider record.
func (h *OIDCHandler) InitProvider(ctx context.Context, callbackURL string, envIssuer, envClientID, envClientSecret, envScopes, envDefaultRole string) {
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

	if callbackURL == "" {
		slog.Error("OIDC configured but BASE_URL is not set — cannot determine callback URL at startup; set BASE_URL to enable OIDC")
		return
	}

	p, err := oidc.NewProvider(ctx, issuer, clientID, clientSecret, scopes, callbackURL)
	if err != nil {
		slog.Error("failed to initialize oidc provider", "error", err)
		return
	}

	// Sync env-var config to DB so the callback can find the provider record
	if envIssuer != "" {
		role := model.Role(envDefaultRole)
		if role != model.RoleAdmin {
			role = model.RoleMember
		}
		effectiveScopes := scopes
		if effectiveScopes == "" {
			effectiveScopes = "openid email profile"
		}
		dbP := &model.OIDCProvider{
			Name:         "Environment",
			IssuerURL:    issuer,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Scopes:       effectiveScopes,
			DefaultRole:  role,
			Enabled:      true,
		}
		if err := h.oidcStore.UpsertProvider(ctx, dbP); err != nil {
			slog.Error("failed to sync env oidc config to db", "error", err)
		}
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
		slog.Error("failed to generate OIDC state", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	nonce, err := oidc.GenerateRandom(32)
	if err != nil {
		slog.Error("failed to generate OIDC nonce", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := oidc.SetStateCookie(w, h.secret, oidc.StateData{State: state, Nonce: nonce}, h.secureCookies()); err != nil {
		slog.Error("failed to set OIDC state cookie", "error", err)
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
		_, err := h.userStore.FindByEmail(r.Context(), claims.Email)
		switch {
		case err == nil:
			// Existing user found — redirect to account linking
			if err := oidc.SetPendingLinkCookie(w, h.secret, oidc.PendingLink{
				ProviderID: dbProvider.ID,
				Subject:    claims.Subject,
				Email:      claims.Email,
			}, h.secureCookies()); err != nil {
				slog.Error("failed to set pending link cookie", "error", err)
				http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
				return
			}
			http.Redirect(w, r, "/link-account", http.StatusFound)
			return
		case !store.IsNotFound(err):
			// Transient DB error — do not fall through to provisioning
			slog.Error("oidc email lookup failed", "email", claims.Email, "error", err)
			http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
			return
		}
		// err is pgx.ErrNoRows — user doesn't exist, proceed to provisioning
	} else {
		slog.Error("oidc provider did not return an email claim, cannot provision user")
		http.Redirect(w, r, "/?error=oidc_no_email", http.StatusFound)
		return
	}

	userID, err := h.oidcStore.ProvisionUser(r.Context(), claims.Email, claims.Name, dbProvider.DefaultRole, dbProvider.ID, claims.Subject)
	if err != nil {
		slog.Error("oidc user provisioning failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	h.createSessionAndRedirect(w, r, userID)
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

	if user.PasswordHash == "" {
		writeError(w, http.StatusConflict, "this account was created via SSO and has no password - set a password first via account settings")
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	// Check if identity is already linked (idempotent on duplicate submit)
	existing, err := h.oidcStore.FindIdentity(r.Context(), pending.ProviderID, pending.Subject)
	if err != nil {
		slog.Error("failed to check existing identity", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to check existing identity")
		return
	}
	if existing == nil {
		if err := h.oidcStore.CreateIdentity(r.Context(), &model.OIDCIdentity{
			UserID:     user.ID,
			ProviderID: pending.ProviderID,
			Subject:    pending.Subject,
			Email:      pending.Email,
		}); err != nil {
			slog.Error("failed to link account", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to link account")
			return
		}
	}

	oidc.ClearPendingLinkCookie(w)

	session, err := h.sessionStore.Create(r.Context(), user.ID, 7*24*time.Hour)
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

// GetConfig returns the current OIDC config (admin-only, secret redacted).
func (h *OIDCHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	provider, err := h.oidcStore.GetProvider(r.Context())
	if err != nil {
		slog.Error("failed to load oidc config", "error", err)
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

	// Validate OIDC discovery before persisting — reject broken configs early
	var newProvider *oidc.Provider
	if req.Enabled {
		callbackURL := h.callbackURL(r)
		p, err := oidc.NewProvider(r.Context(), req.IssuerURL, req.ClientID, req.ClientSecret, req.Scopes, callbackURL)
		if err != nil {
			slog.Error("oidc discovery validation failed", "error", err)
			writeError(w, http.StatusBadRequest, "failed to connect to oidc issuer - check issuer_url and try again")
			return
		}
		newProvider = p
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
		slog.Error("failed to save oidc config", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save oidc config")
		return
	}

	h.mu.Lock()
	h.provider = newProvider // nil when disabled
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
		slog.Error("failed to delete oidc config", "error", err)
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
		slog.Error("failed to load identities", "error", err)
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
		Secure:   h.secureCookies(),
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
