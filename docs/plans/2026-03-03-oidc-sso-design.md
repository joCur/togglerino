# OIDC SSO Design

Issue: #35

## Decisions

| Decision | Choice |
|----------|--------|
| Providers | Single provider first (multi-provider later) |
| Default role | Configurable (defaults to `member`) |
| Login modes | Both password + OIDC coexist |
| Config storage | Database + env var override |
| Account linking | Require password confirmation |
| Library | `coreos/go-oidc` + `golang.org/x/oauth2` |

## Database Schema

### `oidc_providers`

```sql
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
```

### `oidc_identities`

```sql
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

## OIDC Authorization Code Flow

```
Browser                  Togglerino               Identity Provider
  |                          |                          |
  |  GET /api/v1/auth/oidc/authorize                    |
  |------------------------->|                          |
  |                          | Generate state + nonce   |
  |                          | Store in cookie           |
  |  302 Redirect            |                          |
  |<-------------------------|                          |
  |                          |                          |
  |  Follow redirect to IdP  |                          |
  |------------------------------------------------------->
  |                          |        User authenticates |
  |  302 callback with code  |                          |
  |<-------------------------------------------------------|
  |                          |                          |
  |  GET /api/v1/auth/oidc/callback?code=...&state=...  |
  |------------------------->|                          |
  |                          | Validate state            |
  |                          | Exchange code for tokens  |
  |                          |------------------------->|
  |                          |<-------------------------|
  |                          | Verify ID token           |
  |                          | Extract sub + email       |
  |                          |                          |
  |                          | Lookup oidc_identities    |
  |                          | - Found? Login (session)  |
  |                          | - Email match? Link page  |
  |                          | - No match? Provision     |
  |  Set session cookie      |                          |
  |<-------------------------|                          |
  |  Redirect to /           |                          |
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/v1/auth/oidc/authorize` | Public | Start OIDC flow, redirect to IdP |
| `GET` | `/api/v1/auth/oidc/callback` | Public | Handle IdP callback, create session |
| `POST` | `/api/v1/auth/oidc/link` | Public (state-token) | Confirm account linking with password |
| `GET` | `/api/v1/auth/oidc/config` | Session (admin) | Get OIDC config (secret redacted) |
| `PUT` | `/api/v1/auth/oidc/config` | Session (admin) | Create/update OIDC config |
| `DELETE` | `/api/v1/auth/oidc/config` | Session (admin) | Delete OIDC config |

`GET /api/v1/auth/status` extended with `oidc_enabled` boolean field.

## Account Linking Flow

When callback finds email match but no linked identity:

1. Store OIDC claims in HMAC-signed HttpOnly cookie (`oidc_pending`, 5-min TTL, contains provider_id + subject + email)
2. Redirect to `/link-account` in frontend
3. User enters existing password
4. `POST /api/v1/auth/oidc/link` validates password, creates `oidc_identities` row, creates session

## Go Package Structure

### `internal/oidc/`

- `provider.go` — Wraps `coreos/go-oidc` verifier + `oauth2.Config`. Methods: `AuthURL(state, nonce)`, `Exchange(code)`, `Verify(idToken)`. Rebuilt on config change.
- `state.go` — Cookie helpers for `oidc_state` (state + nonce, HMAC-signed, 10min TTL) and `oidc_pending` (provider_id + subject + email, HMAC-signed, 5min TTL).

### `internal/store/oidc_store.go`

```go
type OIDCStore struct { pool *pgxpool.Pool }

// Provider config
func (s *OIDCStore) GetProvider(ctx) (*model.OIDCProvider, error)
func (s *OIDCStore) UpsertProvider(ctx, *model.OIDCProvider) error
func (s *OIDCStore) DeleteProvider(ctx, id) error

// Identities
func (s *OIDCStore) FindIdentity(ctx, providerID, subject) (*model.OIDCIdentity, error)
func (s *OIDCStore) CreateIdentity(ctx, *model.OIDCIdentity) error
func (s *OIDCStore) FindIdentitiesByUser(ctx, userID) ([]model.OIDCIdentity, error)
```

### `internal/handler/oidc_handler.go`

```go
type OIDCHandler struct {
    oidcStore    *store.OIDCStore
    userStore    *store.UserStore
    sessionStore *store.SessionStore
    provider     atomic.Pointer[oidc.Provider]
}
```

Methods: `Authorize`, `Callback`, `Link`, `GetConfig`, `UpdateConfig`, `DeleteConfig`.

### Models (`internal/model/`)

```go
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

### Config additions

| Env var | Purpose |
|---------|---------|
| `OIDC_ISSUER_URL` | Provider issuer URL |
| `OIDC_CLIENT_ID` | Client ID |
| `OIDC_CLIENT_SECRET` | Client secret |
| `OIDC_DEFAULT_ROLE` | Default role for new users |
| `SESSION_SECRET` | HMAC key for signing state cookies |

## Frontend Changes

**Login page** — "Sign in with SSO" button, shown when `oidc_enabled: true`. Full page redirect to `/api/v1/auth/oidc/authorize`.

**Account linking page** (`/link-account`) — New route. Password confirmation form. Calls `POST /api/v1/auth/oidc/link`.

**OIDC settings** — New tab in `/settings` (admin-only). Config form with callback URL display.

**Account page** (`/account`) — "Linked SSO identity" section.

## Security

- **State parameter**: HMAC-signed cookie, 10-min TTL, prevents CSRF/code injection
- **Nonce**: In authorization request, verified in ID token, prevents replay
- **Client secret**: `json:"-"`, never returned from API. Env var override avoids DB storage
- **SESSION_SECRET**: HMAC key for signing cookies. Auto-generated if not set
- **Account linking**: Password confirmation prevents rogue IdP hijack
- **Auto-provisioned users**: Empty `password_hash`, cannot password-login without reset
- **Rate limiting**: callback + link endpoints get existing 10 req/60s limiter
- **Token handling**: Only ID token used, no access/refresh tokens stored
