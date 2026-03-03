# OIDC SSO Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add OIDC Single Sign-On support so users can authenticate via identity providers (Okta, Azure AD, Google Workspace, etc.)

**Architecture:** New `internal/oidc/` package wraps `coreos/go-oidc` + `golang.org/x/oauth2`. New `OIDCStore` + `OIDCHandler` follow existing patterns. OIDC config stored in DB with env var overrides. Frontend gets SSO button on login, admin OIDC settings, and account linking page.

**Tech Stack:** Go stdlib `net/http`, `coreos/go-oidc/v3`, `golang.org/x/oauth2`, pgx/v5, React 19, TanStack Query, shadcn/ui

**Design doc:** `docs/plans/2026-03-03-oidc-sso-design.md`

---

### Task 1: Add Go dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Add oidc and oauth2 modules**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc
go get github.com/coreos/go-oidc/v3@latest
go get golang.org/x/oauth2@latest
```

**Step 2: Verify they resolve**

```bash
go mod tidy
```

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "feat(oidc): add go-oidc and oauth2 dependencies"
```

---

### Task 2: Database migration

**Files:**
- Create: `migrations/012_oidc.up.sql`
- Create: `migrations/012_oidc.down.sql`

**Step 1: Create up migration**

```sql
-- migrations/012_oidc.up.sql
CREATE TABLE oidc_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    issuer_url TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    scopes TEXT NOT NULL DEFAULT 'openid email profile',
    default_role TEXT NOT NULL DEFAULT 'member' CHECK (default_role IN ('admin', 'member')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE oidc_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES oidc_providers(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    email TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider_id, subject)
);
CREATE INDEX idx_oidc_identities_user_id ON oidc_identities(user_id);
```

**Step 2: Create down migration**

```sql
-- migrations/012_oidc.down.sql
DROP TABLE IF EXISTS oidc_identities;
DROP TABLE IF EXISTS oidc_providers;
```

**Step 3: Verify migration compiles into embedded FS**

```bash
go build ./migrations/...
```
Expected: no errors (the `embed.FS` picks up new .sql files automatically).

**Step 4: Commit**

```bash
git add migrations/012_oidc.up.sql migrations/012_oidc.down.sql
git commit -m "feat(oidc): add database migration for oidc_providers and oidc_identities"
```

---

### Task 3: Models

**Files:**
- Create: `internal/model/oidc.go`

**Step 1: Create model file**

```go
package model

import "time"

type OIDCProvider struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	IssuerURL    string    `json:"issuer_url"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"-"`
	Scopes       string    `json:"scopes"`
	DefaultRole  Role      `json:"default_role"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OIDCIdentity struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	ProviderID string    `json:"provider_id"`
	Subject    string    `json:"subject"`
	Email      string    `json:"email,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/model/...
```

**Step 3: Commit**

```bash
git add internal/model/oidc.go
git commit -m "feat(oidc): add OIDCProvider and OIDCIdentity models"
```

---

### Task 4: Config additions

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add OIDC fields to Config struct and Load()**

Add to the `Config` struct:

```go
// OIDC (optional, overrides DB config when set)
OIDCIssuerURL    string
OIDCClientID     string
OIDCClientSecret string
OIDCDefaultRole  string
SessionSecret    string
```

Add to `Load()`:

```go
cfg.OIDCIssuerURL = os.Getenv("OIDC_ISSUER_URL")
cfg.OIDCClientID = os.Getenv("OIDC_CLIENT_ID")
cfg.OIDCClientSecret = os.Getenv("OIDC_CLIENT_SECRET")
cfg.OIDCDefaultRole = envOr("OIDC_DEFAULT_ROLE", "member")
cfg.SessionSecret = os.Getenv("SESSION_SECRET")
```

Add a helper method:

```go
// OIDCConfigured returns true if OIDC provider is configured via env vars.
func (c *Config) OIDCConfigured() bool {
	return c.OIDCIssuerURL != "" && c.OIDCClientID != "" && c.OIDCClientSecret != ""
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/config/...
```

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(oidc): add OIDC and SESSION_SECRET config env vars"
```

---

### Task 5: OIDC Store

**Files:**
- Create: `internal/store/oidc_store.go`

**Step 1: Create the store**

```go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type OIDCStore struct {
	pool *pgxpool.Pool
}

func NewOIDCStore(pool *pgxpool.Pool) *OIDCStore {
	return &OIDCStore{pool: pool}
}

// GetProvider returns the first (only) OIDC provider, or nil if none configured.
func (s *OIDCStore) GetProvider(ctx context.Context) (*model.OIDCProvider, error) {
	var p model.OIDCProvider
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, issuer_url, client_id, client_secret, scopes, default_role, enabled, created_at, updated_at
		 FROM oidc_providers ORDER BY created_at LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.IssuerURL, &p.ClientID, &p.ClientSecret, &p.Scopes, &p.DefaultRole, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("getting oidc provider: %w", err)
	}
	return &p, nil
}

// UpsertProvider creates or updates the OIDC provider config.
// For single-provider mode, this deletes any existing provider and inserts the new one.
func (s *OIDCStore) UpsertProvider(ctx context.Context, p *model.OIDCProvider) error {
	// Delete existing providers (single-provider mode)
	_, err := s.pool.Exec(ctx, `DELETE FROM oidc_providers`)
	if err != nil {
		return fmt.Errorf("clearing oidc providers: %w", err)
	}

	err = s.pool.QueryRow(ctx,
		`INSERT INTO oidc_providers (name, issuer_url, client_id, client_secret, scopes, default_role, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, created_at, updated_at`,
		p.Name, p.IssuerURL, p.ClientID, p.ClientSecret, p.Scopes, p.DefaultRole, p.Enabled,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting oidc provider: %w", err)
	}
	return nil
}

// DeleteProvider removes an OIDC provider by ID.
func (s *OIDCStore) DeleteProvider(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM oidc_providers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting oidc provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("oidc provider not found")
	}
	return nil
}

// FindIdentity looks up an OIDC identity by provider and subject.
func (s *OIDCStore) FindIdentity(ctx context.Context, providerID, subject string) (*model.OIDCIdentity, error) {
	var ident model.OIDCIdentity
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, provider_id, subject, email, created_at
		 FROM oidc_identities WHERE provider_id = $1 AND subject = $2`,
		providerID, subject,
	).Scan(&ident.ID, &ident.UserID, &ident.ProviderID, &ident.Subject, &ident.Email, &ident.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("finding oidc identity: %w", err)
	}
	return &ident, nil
}

// CreateIdentity links an OIDC subject to a user.
func (s *OIDCStore) CreateIdentity(ctx context.Context, ident *model.OIDCIdentity) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO oidc_identities (user_id, provider_id, subject, email)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		ident.UserID, ident.ProviderID, ident.Subject, ident.Email,
	).Scan(&ident.ID, &ident.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating oidc identity: %w", err)
	}
	return nil
}

// FindIdentitiesByUser returns all OIDC identities linked to a user.
func (s *OIDCStore) FindIdentitiesByUser(ctx context.Context, userID string) ([]model.OIDCIdentity, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, provider_id, subject, email, created_at
		 FROM oidc_identities WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing oidc identities: %w", err)
	}
	defer rows.Close()

	var identities []model.OIDCIdentity
	for rows.Next() {
		var i model.OIDCIdentity
		if err := rows.Scan(&i.ID, &i.UserID, &i.ProviderID, &i.Subject, &i.Email, &i.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning oidc identity: %w", err)
		}
		identities = append(identities, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating oidc identities: %w", err)
	}
	return identities, nil
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/store/...
```

**Step 3: Commit**

```bash
git add internal/store/oidc_store.go
git commit -m "feat(oidc): add OIDCStore for provider config and identity management"
```

---

### Task 6: OIDC provider package

**Files:**
- Create: `internal/oidc/provider.go`
- Create: `internal/oidc/state.go`

**Step 1: Create provider.go**

This wraps `go-oidc` and `oauth2` into a simple interface used by the handler.

```go
package oidc

import (
	"context"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Provider wraps go-oidc and oauth2 for OIDC authentication.
type Provider struct {
	verifier    *gooidc.IDTokenVerifier
	oauth2Cfg   oauth2.Config
	oidcProvider *gooidc.Provider
}

// Claims holds the claims extracted from an OIDC ID token.
type Claims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

// NewProvider creates a new OIDC provider from configuration.
// callbackURL is the full URL to the OIDC callback endpoint (e.g. https://example.com/api/v1/auth/oidc/callback).
func NewProvider(ctx context.Context, issuerURL, clientID, clientSecret, scopes, callbackURL string) (*Provider, error) {
	oidcProvider, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery failed for %s: %w", issuerURL, err)
	}

	oauth2Cfg := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     oidcProvider.Endpoint(),
		RedirectURL:  callbackURL,
		Scopes:       parseScopes(scopes),
	}

	verifier := oidcProvider.Verifier(&gooidc.Config{ClientID: clientID})

	return &Provider{
		verifier:    verifier,
		oauth2Cfg:   oauth2Cfg,
		oidcProvider: oidcProvider,
	}, nil
}

// AuthURL returns the URL to redirect the user to for OIDC authentication.
func (p *Provider) AuthURL(state, nonce string) string {
	return p.oauth2Cfg.AuthCodeURL(state, gooidc.Nonce(nonce))
}

// Exchange trades an authorization code for an ID token and returns its claims.
func (p *Provider) Exchange(ctx context.Context, code, nonce string) (*Claims, error) {
	token, err := p.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}

	if idToken.Nonce != nonce {
		return nil, fmt.Errorf("nonce mismatch")
	}

	var claims Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("id_token missing sub claim")
	}

	return &claims, nil
}

func parseScopes(scopes string) []string {
	var result []string
	for _, s := range splitAndTrim(scopes) {
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return []string{gooidc.ScopeOpenID, "email", "profile"}
	}
	return result
}

func splitAndTrim(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == ',' {
			part := s[start:i]
			if part != "" {
				parts = append(parts, part)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
```

**Step 2: Create state.go**

Cookie-based state management for OIDC flow. Uses HMAC-SHA256 for signing.

```go
package oidc

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// StateData holds OIDC state stored in a cookie during the auth flow.
type StateData struct {
	State string `json:"s"`
	Nonce string `json:"n"`
}

// PendingLink holds OIDC claims for account linking (stored in cookie after callback).
type PendingLink struct {
	ProviderID string `json:"p"`
	Subject    string `json:"s"`
	Email      string `json:"e"`
}

// GenerateRandom returns a random hex string of the given byte length.
func GenerateRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SetStateCookie stores OIDC state+nonce in a signed, HttpOnly cookie.
func SetStateCookie(w http.ResponseWriter, secret []byte, data StateData) error {
	return setSignedCookie(w, "oidc_state", secret, data, 10*time.Minute)
}

// GetStateCookie reads and verifies the OIDC state cookie.
func GetStateCookie(r *http.Request, secret []byte) (*StateData, error) {
	var data StateData
	if err := getSignedCookie(r, "oidc_state", secret, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ClearStateCookie removes the OIDC state cookie.
func ClearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_state",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// SetPendingLinkCookie stores pending link data in a signed, HttpOnly cookie.
func SetPendingLinkCookie(w http.ResponseWriter, secret []byte, data PendingLink) error {
	return setSignedCookie(w, "oidc_pending", secret, data, 5*time.Minute)
}

// GetPendingLinkCookie reads and verifies the pending link cookie.
func GetPendingLinkCookie(r *http.Request, secret []byte) (*PendingLink, error) {
	var data PendingLink
	if err := getSignedCookie(r, "oidc_pending", secret, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// ClearPendingLinkCookie removes the pending link cookie.
func ClearPendingLinkCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "oidc_pending",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// setSignedCookie JSON-encodes data, signs it with HMAC-SHA256, and sets a cookie.
func setSignedCookie(w http.ResponseWriter, name string, secret []byte, data any, maxAge time.Duration) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling cookie data: %w", err)
	}

	sig := sign(secret, payload)
	value := base64.URLEncoding.EncodeToString(payload) + "." + base64.URLEncoding.EncodeToString(sig)

	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(maxAge.Seconds()),
	})
	return nil
}

// getSignedCookie reads a signed cookie, verifies the HMAC, and decodes the JSON payload.
func getSignedCookie(r *http.Request, name string, secret []byte, dest any) error {
	cookie, err := r.Cookie(name)
	if err != nil {
		return fmt.Errorf("cookie %s not found: %w", name, err)
	}

	// Split value into payload.signature
	dot := -1
	for i := len(cookie.Value) - 1; i >= 0; i-- {
		if cookie.Value[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return fmt.Errorf("invalid cookie format")
	}

	payloadB64 := cookie.Value[:dot]
	sigB64 := cookie.Value[dot+1:]

	payload, err := base64.URLEncoding.DecodeString(payloadB64)
	if err != nil {
		return fmt.Errorf("decoding payload: %w", err)
	}

	sig, err := base64.URLEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	if !hmac.Equal(sig, sign(secret, payload)) {
		return fmt.Errorf("invalid cookie signature")
	}

	if err := json.Unmarshal(payload, dest); err != nil {
		return fmt.Errorf("unmarshaling cookie data: %w", err)
	}

	return nil
}

func sign(secret, data []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return mac.Sum(nil)
}
```

**Step 3: Write a test for state cookie round-trip**

Create `internal/oidc/state_test.go`:

```go
package oidc

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStateCookieRoundTrip(t *testing.T) {
	secret := []byte("test-secret-key-for-hmac-signing")

	original := StateData{State: "abc123", Nonce: "nonce456"}

	// Write cookie
	w := httptest.NewRecorder()
	if err := SetStateCookie(w, secret, original); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}

	// Read cookie back
	r := &http.Request{Header: http.Header{"Cookie": w.Header()["Set-Cookie"]}}
	got, err := GetStateCookie(r, secret)
	if err != nil {
		t.Fatalf("GetStateCookie: %v", err)
	}

	if got.State != original.State || got.Nonce != original.Nonce {
		t.Errorf("got %+v, want %+v", got, original)
	}
}

func TestStateCookieTamperedSignature(t *testing.T) {
	secret := []byte("test-secret-key-for-hmac-signing")
	wrongSecret := []byte("wrong-secret-key-for-hmac-check")

	original := StateData{State: "abc123", Nonce: "nonce456"}

	w := httptest.NewRecorder()
	if err := SetStateCookie(w, secret, original); err != nil {
		t.Fatalf("SetStateCookie: %v", err)
	}

	// Try to read with wrong secret
	r := &http.Request{Header: http.Header{"Cookie": w.Header()["Set-Cookie"]}}
	_, err := GetStateCookie(r, wrongSecret)
	if err == nil {
		t.Fatal("expected error for tampered cookie, got nil")
	}
}

func TestPendingLinkCookieRoundTrip(t *testing.T) {
	secret := []byte("test-secret-key-for-hmac-signing")

	original := PendingLink{ProviderID: "prov-1", Subject: "sub-123", Email: "user@example.com"}

	w := httptest.NewRecorder()
	if err := SetPendingLinkCookie(w, secret, original); err != nil {
		t.Fatalf("SetPendingLinkCookie: %v", err)
	}

	r := &http.Request{Header: http.Header{"Cookie": w.Header()["Set-Cookie"]}}
	got, err := GetPendingLinkCookie(r, secret)
	if err != nil {
		t.Fatalf("GetPendingLinkCookie: %v", err)
	}

	if got.ProviderID != original.ProviderID || got.Subject != original.Subject || got.Email != original.Email {
		t.Errorf("got %+v, want %+v", got, original)
	}
}

func TestGenerateRandom(t *testing.T) {
	a, err := GenerateRandom(32)
	if err != nil {
		t.Fatalf("GenerateRandom: %v", err)
	}
	b, err := GenerateRandom(32)
	if err != nil {
		t.Fatalf("GenerateRandom: %v", err)
	}
	if len(a) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(a))
	}
	if a == b {
		t.Error("two random values should not be equal")
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/oidc/...
```
Expected: all pass.

**Step 5: Commit**

```bash
git add internal/oidc/
git commit -m "feat(oidc): add OIDC provider wrapper and signed cookie state management"
```

---

### Task 7: OIDC Handler

**Files:**
- Create: `internal/handler/oidc_handler.go`

This is the largest task. The handler manages:
- `GET /api/v1/auth/oidc/authorize` — start OIDC flow
- `GET /api/v1/auth/oidc/callback` — handle IdP callback
- `POST /api/v1/auth/oidc/link` — confirm account linking with password
- `GET /api/v1/auth/oidc/config` — get OIDC config (admin)
- `PUT /api/v1/auth/oidc/config` — update OIDC config (admin)
- `DELETE /api/v1/auth/oidc/config` — delete OIDC config (admin)

**Step 1: Create the handler**

```go
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

	mu       sync.RWMutex
	provider *oidc.Provider // nil when not configured
}

func NewOIDCHandler(oidcStore *store.OIDCStore, userStore *store.UserStore, sessionStore *store.SessionStore, secret []byte) *OIDCHandler {
	return &OIDCHandler{
		oidcStore:    oidcStore,
		userStore:    userStore,
		sessionStore: sessionStore,
		secret:       secret,
	}
}

// InitProvider loads the OIDC provider from DB config on startup.
// Should be called after migrations and before serving requests.
// cfg parameters are env var overrides (empty strings mean "use DB").
func (h *OIDCHandler) InitProvider(ctx context.Context, callbackURL string, envIssuer, envClientID, envClientSecret, envScopes string) {
	dbProvider, err := h.oidcStore.GetProvider(ctx)
	if err != nil {
		slog.Error("failed to load oidc provider from db", "error", err)
		return
	}

	// Env vars override DB config
	issuer := envIssuer
	clientID := envClientID
	clientSecret := envClientSecret
	scopes := envScopes

	if dbProvider != nil && issuer == "" {
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
// GET /api/v1/auth/oidc/authorize
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
// GET /api/v1/auth/oidc/callback
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	p := h.getProvider()
	if p == nil {
		writeError(w, http.StatusNotFound, "oidc not configured")
		return
	}

	// Check for error from IdP
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		slog.Warn("oidc callback error", "error", errParam, "description", desc)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	// Validate state
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

	// Exchange code for ID token
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

	// Get current provider config for provider ID and default role
	dbProvider, err := h.oidcStore.GetProvider(r.Context())
	if err != nil || dbProvider == nil {
		slog.Error("oidc provider not found in db after successful auth")
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	// Check if OIDC identity already exists → login
	identity, err := h.oidcStore.FindIdentity(r.Context(), dbProvider.ID, claims.Subject)
	if err != nil {
		slog.Error("oidc identity lookup failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	if identity != nil {
		// Existing identity → create session and redirect
		h.createSessionAndRedirect(w, r, identity.UserID)
		return
	}

	// Check if email matches existing user → require account linking
	if claims.Email != "" {
		existingUser, err := h.userStore.FindByEmail(r.Context(), claims.Email)
		if err == nil && existingUser != nil {
			// Email match → redirect to link page with pending cookie
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
	}

	// No existing identity, no email match → auto-provision new user
	user, err := h.userStore.Create(r.Context(), claims.Email, "", dbProvider.DefaultRole)
	if err != nil {
		slog.Error("oidc user provisioning failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	// Set display name if available
	if claims.Name != "" {
		h.userStore.UpdateProfile(r.Context(), user.ID, nil, &claims.Name)
	}

	// Create OIDC identity link
	if err := h.oidcStore.CreateIdentity(r.Context(), &model.OIDCIdentity{
		UserID:     user.ID,
		ProviderID: dbProvider.ID,
		Subject:    claims.Subject,
		Email:      claims.Email,
	}); err != nil {
		slog.Error("oidc identity creation failed", "error", err)
		http.Redirect(w, r, "/?error=oidc_failed", http.StatusFound)
		return
	}

	h.createSessionAndRedirect(w, r, user.ID)
}

// Link confirms account linking with password verification.
// POST /api/v1/auth/oidc/link
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

	// Create OIDC identity link
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

	// Create session
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
// GET /api/v1/auth/oidc/config
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
	// ClientSecret is already json:"-", so it won't be serialized
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": true,
		"provider":   provider,
	})
}

// UpdateConfig creates or updates the OIDC provider config (admin-only).
// PUT /api/v1/auth/oidc/config
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

	if req.Name == "" || req.IssuerURL == "" || req.ClientID == "" || req.ClientSecret == "" {
		writeError(w, http.StatusBadRequest, "name, issuer_url, client_id, and client_secret are required")
		return
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

	// Rebuild the provider
	callbackURL := callbackURLFromRequest(r)
	p, err := oidc.NewProvider(r.Context(), req.IssuerURL, req.ClientID, req.ClientSecret, req.Scopes, callbackURL)
	if err != nil {
		slog.Error("oidc provider rebuild failed after config update", "error", err)
		writeError(w, http.StatusBadRequest, "failed to connect to oidc issuer - config saved but provider not active")
		return
	}

	h.mu.Lock()
	h.provider = p
	h.mu.Unlock()

	writeJSON(w, http.StatusOK, provider)
}

// DeleteConfig removes the OIDC provider config (admin-only).
// DELETE /api/v1/auth/oidc/config
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

// OIDCIdentities returns OIDC identities for the current user (session-authed).
// GET /api/v1/auth/oidc/identities
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

// callbackURLFromRequest constructs the OIDC callback URL from the current request.
func callbackURLFromRequest(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host + "/api/v1/auth/oidc/callback"
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/handler/...
```

**Step 3: Commit**

```bash
git add internal/handler/oidc_handler.go
git commit -m "feat(oidc): add OIDC handler with authorize, callback, link, and config endpoints"
```

---

### Task 8: Update auth status endpoint

**Files:**
- Modify: `internal/handler/auth_handler.go`

**Step 1: Add OIDCHandler reference to AuthHandler**

Add a field and setter so AuthHandler can check if OIDC is configured:

```go
// Add to AuthHandler struct:
oidcConfigured func() bool

// Add method:
func (h *AuthHandler) SetOIDCChecker(fn func() bool) {
	h.oidcConfigured = fn
}
```

**Step 2: Update Status method to include oidc_enabled**

Change the `Status` method response from:

```go
writeJSON(w, http.StatusOK, map[string]any{
    "setup_required": count == 0,
})
```

to:

```go
oidcEnabled := h.oidcConfigured != nil && h.oidcConfigured()
writeJSON(w, http.StatusOK, map[string]any{
    "setup_required": count == 0,
    "oidc_enabled":   oidcEnabled,
})
```

**Step 3: Verify it compiles**

```bash
go build ./internal/handler/...
```

**Step 4: Commit**

```bash
git add internal/handler/auth_handler.go
git commit -m "feat(oidc): extend auth status endpoint with oidc_enabled field"
```

---

### Task 9: Wire up in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Generate session secret if not provided**

After config loading, add:

```go
// Ensure session secret exists (for OIDC cookie signing)
sessionSecret := cfg.SessionSecret
if sessionSecret == "" {
    // Generate a random secret for this instance
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        log.Fatal(err)
    }
    sessionSecret = hex.EncodeToString(b)
    slog.Warn("SESSION_SECRET not set, generated random secret (OIDC state cookies will not survive restarts)")
}
```

Add `"crypto/rand"` and `"encoding/hex"` to imports.

**Step 2: Initialize OIDC store and handler**

After the existing store initializations (after `scheduleStore`), add:

```go
oidcStore := store.NewOIDCStore(pool)
```

After handler initializations, add:

```go
oidcHandler := handler.NewOIDCHandler(oidcStore, userStore, sessionStore, []byte(sessionSecret))
authHandler.SetOIDCChecker(oidcHandler.IsConfigured)
```

**Step 3: Initialize OIDC provider after migrations**

After `go scheduleChecker.Run(ctx)`, add:

```go
// Initialize OIDC provider (non-blocking, logs errors)
oidcHandler.InitProvider(ctx, "", cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, "")
```

Note: callback URL will be empty here and constructed from the first request in UpdateConfig. For env-var-only configs, the callback URL is determined at request time anyway.

Actually, let's pass the callback URL properly. The handler's `InitProvider` needs the callback URL, but we don't know the external hostname at startup. Change the approach: skip the callback URL at init, and have the provider reconstruct it per-request. This is already handled — the `Authorize` method uses the provider which was built with a callback URL. For startup init, we need a way to provide it.

Simpler approach: don't pass callback URL at init. Instead, have `InitProvider` accept callbackURL as empty string and skip provider creation if it's empty. The provider will be lazily initialized on the first UpdateConfig call or the first Authorize request.

Let's simplify: at startup, if env vars are set, log that they need `BASE_URL` env var too, or just skip provider init and let it be configured via the admin UI which knows the correct URL.

Better: add a `BASE_URL` config option, and construct the callback URL from it.

Actually, for simplicity, let's add the callback URL construction from the request to `Authorize` — if provider needs rebuilding. But this is overcomplicating it.

Simplest approach: Add `BASE_URL` env var. When set + OIDC env vars are set, init provider at startup. Otherwise, provider gets initialized via admin UI config update.

Add to config:

```go
BaseURL string // e.g. "https://flags.example.com"
```

And in main.go:

```go
callbackURL := ""
if cfg.BaseURL != "" {
    callbackURL = cfg.BaseURL + "/api/v1/auth/oidc/callback"
}
oidcHandler.InitProvider(ctx, callbackURL, cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, "")
```

**Step 4: Register OIDC routes**

After the existing public routes section, add:

```go
// --- OIDC routes ---
mux.HandleFunc("GET /api/v1/auth/oidc/authorize", oidcHandler.Authorize)
mux.Handle("GET /api/v1/auth/oidc/callback", authLimiter.Middleware(http.HandlerFunc(oidcHandler.Callback)))
mux.Handle("POST /api/v1/auth/oidc/link", authLimiter.Middleware(http.HandlerFunc(oidcHandler.Link)))
mux.Handle("GET /api/v1/auth/oidc/config", wrap(oidcHandler.GetConfig, sessionAuth, requireAdmin))
mux.Handle("PUT /api/v1/auth/oidc/config", wrap(oidcHandler.UpdateConfig, sessionAuth, requireAdmin))
mux.Handle("DELETE /api/v1/auth/oidc/config", wrap(oidcHandler.DeleteConfig, sessionAuth, requireAdmin))
mux.Handle("GET /api/v1/auth/oidc/identities", wrap(oidcHandler.OIDCIdentities, sessionAuth))
```

**Step 5: Build the full binary to verify wiring**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc/web && npm run build
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc && go build -o /dev/null ./cmd/togglerino
```
Expected: builds successfully.

**Step 6: Commit**

```bash
git add cmd/togglerino/main.go internal/config/config.go
git commit -m "feat(oidc): wire up OIDC handler, store, and routes in main.go"
```

---

### Task 10: Frontend — Update useAuth hook and types

**Files:**
- Modify: `web/src/hooks/useAuth.ts`
- Modify: `web/src/api/types.ts`

**Step 1: Update AuthStatus interface in useAuth.ts**

```typescript
interface AuthStatus {
  setup_required: boolean
  oidc_enabled: boolean
}
```

**Step 2: Expose oidcEnabled in the return value**

Add to the return object:

```typescript
oidcEnabled: statusQuery.data?.oidc_enabled ?? false,
```

**Step 3: Add OIDCProvider and OIDCIdentity types to api/types.ts**

```typescript
export interface OIDCProvider {
  id: string
  name: string
  issuer_url: string
  client_id: string
  scopes: string
  default_role: 'admin' | 'member'
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface OIDCIdentity {
  id: string
  user_id: string
  provider_id: string
  subject: string
  email?: string
  created_at: string
}
```

**Step 4: Commit**

```bash
git add web/src/hooks/useAuth.ts web/src/api/types.ts
git commit -m "feat(oidc): add oidc_enabled to auth status and OIDC types"
```

---

### Task 11: Frontend — SSO button on login page

**Files:**
- Modify: `web/src/pages/LoginPage.tsx`

**Step 1: Add oidcEnabled to the hook destructure**

```typescript
const { login, loginError, oidcEnabled } = useAuth()
```

**Step 2: Add SSO button after the sign-in button**

After the `<Button>` for "Sign In", add:

```tsx
{oidcEnabled && (
  <>
    <div className="relative my-4">
      <div className="absolute inset-0 flex items-center">
        <div className="w-full border-t border-border" />
      </div>
      <div className="relative flex justify-center text-xs">
        <span className="bg-card px-2 text-muted-foreground">or</span>
      </div>
    </div>
    <a
      href="/api/v1/auth/oidc/authorize"
      className="inline-flex items-center justify-center gap-2 rounded-md border border-border bg-background px-4 py-2 text-sm font-medium text-foreground shadow-xs transition-colors hover:bg-accent hover:text-accent-foreground w-full"
    >
      Sign in with SSO
    </a>
  </>
)}
```

**Step 3: Commit**

```bash
git add web/src/pages/LoginPage.tsx
git commit -m "feat(oidc): add SSO button to login page"
```

---

### Task 12: Frontend — Account linking page

**Files:**
- Create: `web/src/pages/LinkAccountPage.tsx`
- Modify: `web/src/App.tsx`

**Step 1: Create LinkAccountPage component**

```tsx
import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function LinkAccountPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      await api.post('/auth/oidc/link', { password })
      queryClient.invalidateQueries({ queryKey: ['auth'] })
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to link account')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen px-6 md:px-0 bg-background bg-[radial-gradient(ellipse_60%_50%_at_50%_40%,rgba(212,149,106,0.04)_0%,transparent_70%)]">
      <div className="w-full max-w-[400px] p-6 md:p-10 rounded-2xl md:bg-card md:border md:shadow-lg animate-[fadeInUp_400ms_ease]">
        <div className="flex items-center justify-center gap-2.5 mb-2">
          <svg width="24" height="14" viewBox="0 0 24 14" fill="none">
            <rect width="24" height="14" rx="7" fill="#d4956a" opacity="0.25" />
            <circle cx="17" cy="7" r="5" fill="#d4956a" />
          </svg>
          <span className="font-mono text-lg font-semibold text-[#d4956a] tracking-wide">togglerino</span>
        </div>
        <div className="text-[13px] text-muted-foreground text-center mb-2">
          Link your SSO identity
        </div>
        <div className="text-[12px] text-muted-foreground/60 text-center mb-9">
          An account with your email already exists. Enter your password to link your SSO identity.
        </div>

        <form onSubmit={handleSubmit}>
          {error && (
            <Alert variant="destructive" className="mb-5">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-1.5">
            <Label>Password</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Your existing password"
              required
              autoFocus
            />
          </div>

          <Button className="w-full mt-6" disabled={submitting}>
            {submitting ? 'Linking...' : 'Link Account & Sign In'}
          </Button>
        </form>
      </div>
    </div>
  )
}
```

**Step 2: Add route to App.tsx**

Import `LinkAccountPage` and add the route in the public routes section (alongside `/invite/:token` and `/reset-password/:token`):

```tsx
import LinkAccountPage from './pages/LinkAccountPage.tsx'
```

In the `App` function's `Routes`, add before `<Route path="*" element={<AuthRouter />} />`:

```tsx
<Route path="/link-account" element={<LinkAccountPage />} />
```

**Step 3: Verify frontend builds**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc/web && npm run build
```

**Step 4: Commit**

```bash
git add web/src/pages/LinkAccountPage.tsx web/src/App.tsx
git commit -m "feat(oidc): add account linking page"
```

---

### Task 13: Frontend — OIDC settings (admin)

**Files:**
- Create: `web/src/pages/settings/OIDCSettingsTab.tsx`
- Modify: `web/src/pages/SettingsPage.tsx`

**Step 1: Create OIDCSettingsTab component**

This is an admin-only tab in the settings page for configuring the OIDC provider.

```tsx
import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client.ts'
import type { OIDCProvider } from '@/api/types.ts'
import { useAuth } from '@/hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface OIDCConfigResponse {
  configured: boolean
  provider?: OIDCProvider
}

export default function OIDCSettingsTab() {
  const { user } = useAuth()
  const queryClient = useQueryClient()

  const configQuery = useQuery({
    queryKey: ['oidc', 'config'],
    queryFn: () => api.get<OIDCConfigResponse>('/auth/oidc/config'),
    enabled: user?.role === 'admin',
  })

  const [name, setName] = useState('')
  const [issuerUrl, setIssuerUrl] = useState('')
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [scopes, setScopes] = useState('openid email profile')
  const [defaultRole, setDefaultRole] = useState<'admin' | 'member'>('member')
  const [enabled, setEnabled] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    if (configQuery.data?.provider) {
      const p = configQuery.data.provider
      setName(p.name)
      setIssuerUrl(p.issuer_url)
      setClientId(p.client_id)
      setScopes(p.scopes)
      setDefaultRole(p.default_role)
      setEnabled(p.enabled)
    }
  }, [configQuery.data])

  const saveMutation = useMutation({
    mutationFn: (data: {
      name: string
      issuer_url: string
      client_id: string
      client_secret: string
      scopes: string
      default_role: string
      enabled: boolean
    }) => api.put<OIDCProvider>('/auth/oidc/config', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oidc', 'config'] })
      queryClient.invalidateQueries({ queryKey: ['auth', 'status'] })
      setSuccess('OIDC configuration saved')
      setError('')
      setClientSecret('')
    },
    onError: (err: Error) => {
      setError(err.message)
      setSuccess('')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.delete('/auth/oidc/config'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oidc', 'config'] })
      queryClient.invalidateQueries({ queryKey: ['auth', 'status'] })
      setName('')
      setIssuerUrl('')
      setClientId('')
      setClientSecret('')
      setScopes('openid email profile')
      setDefaultRole('member')
      setEnabled(true)
      setSuccess('OIDC configuration removed')
      setError('')
    },
    onError: (err: Error) => {
      setError(err.message)
      setSuccess('')
    },
  })

  if (user?.role !== 'admin') {
    return <div className="text-sm text-muted-foreground">Admin access required.</div>
  }

  const callbackUrl = `${window.location.origin}/api/v1/auth/oidc/callback`

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    if (!clientSecret && !configQuery.data?.configured) {
      setError('Client secret is required for initial setup')
      return
    }
    saveMutation.mutate({
      name,
      issuer_url: issuerUrl,
      client_id: clientId,
      client_secret: clientSecret || '',
      scopes,
      default_role: defaultRole,
      enabled,
    })
  }

  return (
    <div className="max-w-2xl">
      <h2 className="text-sm font-medium mb-1">OpenID Connect (OIDC)</h2>
      <p className="text-xs text-muted-foreground mb-6">
        Configure SSO with your identity provider (Okta, Azure AD, Google Workspace, etc.)
      </p>

      <Card className="mb-5">
        <CardContent className="p-6">
          <div className="text-xs font-mono text-muted-foreground mb-1">Callback URL</div>
          <div className="flex items-center gap-2">
            <code className="text-xs bg-muted px-2.5 py-1.5 rounded font-mono flex-1 break-all">
              {callbackUrl}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => navigator.clipboard.writeText(callbackUrl)}
            >
              Copy
            </Button>
          </div>
          <p className="text-[11px] text-muted-foreground/60 mt-2">
            Add this URL as the redirect URI in your identity provider configuration.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-6">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Provider Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Okta, Azure AD" required />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Issuer URL</Label>
              <Input value={issuerUrl} onChange={(e) => setIssuerUrl(e.target.value)} placeholder="https://accounts.google.com" required />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Client ID</Label>
              <Input value={clientId} onChange={(e) => setClientId(e.target.value)} placeholder="your-client-id" required />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Client Secret</Label>
              <Input
                type="password"
                value={clientSecret}
                onChange={(e) => setClientSecret(e.target.value)}
                placeholder={configQuery.data?.configured ? '(unchanged)' : 'your-client-secret'}
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Scopes</Label>
              <Input value={scopes} onChange={(e) => setScopes(e.target.value)} placeholder="openid email profile" />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Default Role for New Users</Label>
              <Select value={defaultRole} onValueChange={(v) => setDefaultRole(v as 'admin' | 'member')}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="member">Member</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center gap-3">
              <Switch checked={enabled} onCheckedChange={setEnabled} />
              <Label className="text-sm">Enabled</Label>
            </div>

            {error && <div className="text-[13px] text-destructive">{error}</div>}
            {success && <div className="text-[13px] text-emerald-500">{success}</div>}

            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={saveMutation.isPending}>
                {saveMutation.isPending ? 'Saving...' : 'Save Configuration'}
              </Button>
              {configQuery.data?.configured && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => deleteMutation.mutate()}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending ? 'Removing...' : 'Remove OIDC'}
                </Button>
              )}
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
```

**Step 2: Add OIDC tab to SettingsPage**

The current SettingsPage only shows a theme picker. We need to restructure it to support tabs, OR add OIDC settings as a separate page.

Looking at the current `SettingsPage.tsx`, it's a standalone page under `OrgLayout`. The simplest approach: add the OIDC settings tab within the existing `OrgLayout` `/settings` structure.

Since `/settings` is already a top-level route, and the current `SettingsPage` just shows theme, let's add OIDC config below the theme section for admin users:

Modify `SettingsPage.tsx` to import and render `OIDCSettingsTab`:

```tsx
import OIDCSettingsTab from './settings/OIDCSettingsTab.tsx'
import { useAuth } from '@/hooks/useAuth'
```

After the theme section closing `</div>`, add:

```tsx
{user?.role === 'admin' && (
  <div className="mt-10">
    <OIDCSettingsTab />
  </div>
)}
```

And add `const { user } = useAuth()` alongside the existing `useTheme()` call. Also remove the early return for `!isThemeToggleEnabled` (or adjust it to still show the page for admin OIDC settings).

Actually, looking more carefully, the SettingsPage redirects to `/projects` if theme toggle is not enabled. We need to change this so the page is always accessible (for OIDC settings at minimum for admins). Change the condition:

```tsx
if (!isThemeToggleEnabled && user?.role !== 'admin') {
  return <Navigate to="/projects" replace />
}
```

**Step 3: Verify frontend builds**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc/web && npm run build
```

**Step 4: Commit**

```bash
git add web/src/pages/settings/OIDCSettingsTab.tsx web/src/pages/SettingsPage.tsx
git commit -m "feat(oidc): add OIDC settings tab for admin configuration"
```

---

### Task 14: Frontend — Account page SSO identity section

**Files:**
- Modify: `web/src/pages/AccountPage.tsx`

**Step 1: Add OIDC identity query**

Add after the existing mutations:

```tsx
const identitiesQuery = useQuery({
  queryKey: ['oidc', 'identities'],
  queryFn: () => api.get<OIDCIdentity[]>('/auth/oidc/identities'),
})
```

Import `useQuery` (already imported), `OIDCIdentity` from types.

**Step 2: Add SSO identity Card**

After the "Change Password" Card and before the "Account Info" Card, add:

```tsx
{/* SSO Identity */}
<Card className="mb-5">
  <CardContent className="p-6">
    <div className="text-sm font-semibold text-foreground mb-4">
      SSO Identity
    </div>
    {identitiesQuery.data && identitiesQuery.data.length > 0 ? (
      <div className="flex flex-col gap-2">
        {identitiesQuery.data.map((ident) => (
          <div key={ident.id} className="flex items-center gap-3">
            <Badge variant="secondary" className="font-mono text-[11px]">SSO</Badge>
            <span className="text-[13px] text-muted-foreground">{ident.email || ident.subject}</span>
          </div>
        ))}
      </div>
    ) : (
      <div className="text-[13px] text-muted-foreground/60">
        No SSO identity linked to this account.
      </div>
    )}
  </CardContent>
</Card>
```

**Step 3: Add imports**

Add `useQuery` to the existing import from `@tanstack/react-query` and `OIDCIdentity` from `../api/types.ts`.

**Step 4: Verify frontend builds**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc/web && npm run build
```

**Step 5: Commit**

```bash
git add web/src/pages/AccountPage.tsx
git commit -m "feat(oidc): show linked SSO identity on account page"
```

---

### Task 15: Frontend lint check

**Step 1: Run ESLint**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc/web && npm run lint
```

Fix any lint errors that appear.

**Step 2: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve lint errors from OIDC frontend changes"
```

---

### Task 16: Run all tests

**Step 1: Run Go tests (excluding store)**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc
go test $(go list ./... | grep -v /store)
```
Expected: all pass including new `internal/oidc/` tests.

**Step 2: Build full binary**

```bash
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc/web && npm run build
cd /Users/jonascurth/Documents/git/togglerino/.worktrees/oidc && go build -o /dev/null ./cmd/togglerino
```
Expected: builds successfully.

**Step 3: Fix any issues and commit**

---

### Task 17: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add OIDC to environment variables section**

Add:

```
- `OIDC_ISSUER_URL` — OIDC provider issuer URL (optional, overrides DB config)
- `OIDC_CLIENT_ID` — OIDC client ID (optional, overrides DB config)
- `OIDC_CLIENT_SECRET` — OIDC client secret (optional, overrides DB config)
- `OIDC_DEFAULT_ROLE` — Default role for OIDC-provisioned users: `admin` or `member` (default: `member`)
- `SESSION_SECRET` — HMAC key for signing OIDC state cookies (auto-generated if not set)
- `BASE_URL` — External base URL (e.g. `https://flags.example.com`), required for OIDC env var configuration
```

**Step 2: Add OIDC to key patterns section**

Add a bullet:

```
- **OIDC SSO**: Single provider OIDC authentication via `coreos/go-oidc`. Authorization code flow with HMAC-signed state/nonce cookies. Auto-provisions new users or requires password confirmation for account linking by email. Config via admin UI (DB) or env vars (override). Provider hot-reloaded on config change.
```

**Step 3: Add OIDC package to the packages table**

Add row:

```
| `oidc` | OIDC provider wrapper (`go-oidc` + `oauth2`) and HMAC-signed cookie state management |
```

**Step 4: Add OIDC routes to API routes section**

Add under "Public":

```
- `GET /api/v1/auth/oidc/authorize` — start OIDC flow (redirect to IdP)
- `GET /api/v1/auth/oidc/callback` — handle IdP callback (rate-limited)
- `POST /api/v1/auth/oidc/link` — confirm account linking with password (rate-limited)
```

Add under "Session-authed":

```
- `GET /api/v1/auth/oidc/config` — get OIDC config (admin-only, secret redacted)
- `PUT /api/v1/auth/oidc/config` — create/update OIDC config (admin-only)
- `DELETE /api/v1/auth/oidc/config` — delete OIDC config (admin-only)
- `GET /api/v1/auth/oidc/identities` — list current user's OIDC identities
```

**Step 5: Update database section**

Add `oidc_providers`, `oidc_identities` to the tables list, and update migration range to `001_initial_schema` through `012_oidc`.

**Step 6: Add /link-account route to frontend routes section**

**Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with OIDC SSO documentation"
```
