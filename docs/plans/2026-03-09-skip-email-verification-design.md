# Skip Email Verification Toggle — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an optional `skip_email_verification` boolean to the OIDC provider config so admins can allow login from providers that don't emit the `email_verified` claim.

**Architecture:** New boolean column on `oidc_providers` table, checked in the OIDC callback handler before rejecting unverified emails. Configurable via admin UI toggle and `OIDC_SKIP_EMAIL_VERIFICATION` env var.

**Tech Stack:** Go 1.25 (stdlib net/http, pgx/v5), React 19, TypeScript, TanStack Query, Tailwind/shadcn

**Worktree:** `.worktrees/skip-email-verification` (branch `feat/skip-email-verification`)

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/024_oidc_skip_email_verification.up.sql`
- Create: `migrations/024_oidc_skip_email_verification.down.sql`

**Step 1: Write the up migration**

```sql
ALTER TABLE oidc_providers ADD COLUMN skip_email_verification BOOLEAN NOT NULL DEFAULT FALSE;
```

**Step 2: Write the down migration**

```sql
ALTER TABLE oidc_providers DROP COLUMN skip_email_verification;
```

**Step 3: Commit**

```bash
git add migrations/024_oidc_skip_email_verification.up.sql migrations/024_oidc_skip_email_verification.down.sql
git commit -m "feat: add skip_email_verification column to oidc_providers"
```

---

### Task 2: Model Update

**Files:**
- Modify: `internal/model/oidc.go:5-16`

**Step 1: Add field to OIDCProvider struct**

Add `SkipEmailVerification bool` field after `Enabled`:

```go
type OIDCProvider struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	IssuerURL             string    `json:"issuer_url"`
	ClientID              string    `json:"client_id"`
	ClientSecret          string    `json:"-"`
	Scopes                string    `json:"scopes"`
	DefaultRole           Role      `json:"default_role"`
	Enabled               bool      `json:"enabled"`
	SkipEmailVerification bool      `json:"skip_email_verification"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
```

**Step 2: Commit**

```bash
git add internal/model/oidc.go
git commit -m "feat: add SkipEmailVerification to OIDCProvider model"
```

---

### Task 3: Store — Update SQL queries (TDD)

**Files:**
- Modify: `internal/store/oidc_store.go:22-54`
- Test: `internal/store/oidc_store_test.go` (create if not exists)

**Step 1: Write the failing test**

Create `internal/store/oidc_store_test.go`:

```go
package store

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func TestOIDCStore_SkipEmailVerification(t *testing.T) {
	pool := testPool(t)
	store := NewOIDCStore(pool)
	ctx := context.Background()

	// Clean up any existing provider
	existing, _ := store.GetProvider(ctx)
	if existing != nil {
		store.DeleteProvider(ctx, existing.ID)
	}

	// Upsert with skip_email_verification = true
	p := &model.OIDCProvider{
		Name:                  "Test Provider",
		IssuerURL:             "https://example.com",
		ClientID:              "client-id",
		ClientSecret:          "client-secret",
		Scopes:                "openid email",
		DefaultRole:           model.RoleMember,
		Enabled:               true,
		SkipEmailVerification: true,
	}
	if err := store.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("UpsertProvider: %v", err)
	}

	got, err := store.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if !got.SkipEmailVerification {
		t.Error("expected SkipEmailVerification=true, got false")
	}

	// Update to false
	p.SkipEmailVerification = false
	if err := store.UpsertProvider(ctx, p); err != nil {
		t.Fatalf("UpsertProvider (update): %v", err)
	}

	got, err = store.GetProvider(ctx)
	if err != nil {
		t.Fatalf("GetProvider (after update): %v", err)
	}
	if got.SkipEmailVerification {
		t.Error("expected SkipEmailVerification=false, got true")
	}

	// Clean up
	store.DeleteProvider(ctx, got.ID)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestOIDCStore_SkipEmailVerification -v`
Expected: FAIL — the SELECT and INSERT don't include the new column yet.

**Step 3: Update GetProvider SELECT**

In `internal/store/oidc_store.go`, update the `GetProvider` query to include `skip_email_verification` in both the SELECT and Scan:

```go
func (s *OIDCStore) GetProvider(ctx context.Context) (*model.OIDCProvider, error) {
	var p model.OIDCProvider
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, issuer_url, client_id, client_secret, scopes, default_role, enabled, skip_email_verification, created_at, updated_at
		 FROM oidc_providers ORDER BY created_at LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.IssuerURL, &p.ClientID, &p.ClientSecret, &p.Scopes, &p.DefaultRole, &p.Enabled, &p.SkipEmailVerification, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting oidc provider: %w", err)
	}
	return &p, nil
}
```

**Step 4: Update UpsertProvider INSERT/ON CONFLICT**

```go
func (s *OIDCStore) UpsertProvider(ctx context.Context, p *model.OIDCProvider) error {
	err := s.pool.QueryRow(ctx,
		`INSERT INTO oidc_providers (name, issuer_url, client_id, client_secret, scopes, default_role, enabled, skip_email_verification)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (singleton) DO UPDATE SET
		   name = EXCLUDED.name, issuer_url = EXCLUDED.issuer_url, client_id = EXCLUDED.client_id,
		   client_secret = EXCLUDED.client_secret, scopes = EXCLUDED.scopes, default_role = EXCLUDED.default_role,
		   enabled = EXCLUDED.enabled, skip_email_verification = EXCLUDED.skip_email_verification, updated_at = NOW()
		 RETURNING id, created_at, updated_at`,
		p.Name, p.IssuerURL, p.ClientID, p.ClientSecret, p.Scopes, p.DefaultRole, p.Enabled, p.SkipEmailVerification,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting oidc provider: %w", err)
	}
	return nil
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestOIDCStore_SkipEmailVerification -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/store/oidc_store.go internal/store/oidc_store_test.go
git commit -m "feat: store skip_email_verification in oidc_providers"
```

---

### Task 4: Config — Add env var (TDD)

**Files:**
- Modify: `internal/config/config.go:10-42`

**Step 1: Write the failing test**

Create or append to `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
)

func TestLoad_OIDCSkipEmailVerification(t *testing.T) {
	// Default: false
	os.Unsetenv("OIDC_SKIP_EMAIL_VERIFICATION")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.OIDCSkipEmailVerification {
		t.Error("expected OIDCSkipEmailVerification=false by default")
	}

	// Explicit true
	os.Setenv("OIDC_SKIP_EMAIL_VERIFICATION", "true")
	defer os.Unsetenv("OIDC_SKIP_EMAIL_VERIFICATION")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.OIDCSkipEmailVerification {
		t.Error("expected OIDCSkipEmailVerification=true when env var is 'true'")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoad_OIDCSkipEmailVerification -v`
Expected: FAIL — field doesn't exist yet.

**Step 3: Add the field to Config struct and Load()**

In `internal/config/config.go`, add to Config struct:

```go
OIDCSkipEmailVerification bool
```

In `Load()`, add to the `cfg` initialization:

```go
OIDCSkipEmailVerification: os.Getenv("OIDC_SKIP_EMAIL_VERIFICATION") == "true",
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestLoad_OIDCSkipEmailVerification -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add OIDC_SKIP_EMAIL_VERIFICATION env var"
```

---

### Task 5: Handler — Callback email verification bypass (TDD)

**Files:**
- Modify: `internal/handler/oidc_handler.go:54-121` (InitProvider), `238-242` (Callback), `391-451` (UpdateConfig)

**Step 1: Update InitProvider to accept and sync the new field**

Change `InitProvider` signature to accept the new parameter:

```go
func (h *OIDCHandler) InitProvider(ctx context.Context, callbackURL string, envIssuer, envClientID, envClientSecret, envScopes, envDefaultRole string, envSkipEmailVerification bool) {
```

In the env-var sync block (around line 103), add `SkipEmailVerification` to the `dbP`:

```go
dbP := &model.OIDCProvider{
	Name:                  "Environment",
	IssuerURL:             issuer,
	ClientID:              clientID,
	ClientSecret:          clientSecret,
	Scopes:                effectiveScopes,
	DefaultRole:           role,
	Enabled:               true,
	SkipEmailVerification: envSkipEmailVerification,
}
```

**Step 2: Update Callback to check the toggle**

Replace lines 238-242 in `oidc_handler.go`:

```go
	if !claims.EmailVerified && !dbProvider.SkipEmailVerification {
		slog.Warn("oidc email not verified, rejecting login", "email", claims.Email, "subject", claims.Subject, "email_verified_present", claims.EmailVerifiedPresent)
		http.Redirect(w, r, "/?error=oidc_email_not_verified", http.StatusFound)
		return
	}
```

**Step 3: Update UpdateConfig to accept the new field**

In the `UpdateConfig` request struct, add:

```go
SkipEmailVerification bool `json:"skip_email_verification"`
```

And include it when constructing the provider (around line 443):

```go
provider := &model.OIDCProvider{
	Name:                  req.Name,
	IssuerURL:             req.IssuerURL,
	ClientID:              req.ClientID,
	ClientSecret:          req.ClientSecret,
	Scopes:                req.Scopes,
	DefaultRole:           req.DefaultRole,
	Enabled:               req.Enabled,
	SkipEmailVerification: req.SkipEmailVerification,
}
```

**Step 4: Update main.go call site**

In `cmd/togglerino/main.go` line 177, update the `InitProvider` call:

```go
oidcHandler.InitProvider(ctx, callbackURL, cfg.OIDCIssuerURL, cfg.OIDCClientID, cfg.OIDCClientSecret, "", cfg.OIDCDefaultRole, cfg.OIDCSkipEmailVerification)
```

**Step 5: Run all Go tests**

Run: `go test ./...`
Expected: PASS (no OIDC handler-specific tests exist yet, but compilation and existing tests must pass)

**Step 6: Commit**

```bash
git add internal/handler/oidc_handler.go cmd/togglerino/main.go
git commit -m "feat: skip email verification in OIDC callback when configured"
```

---

### Task 6: Frontend — Add toggle to OIDC settings form

**Files:**
- Modify: `web/src/api/types.ts:195-205`
- Modify: `web/src/pages/settings/OIDCSettingsTab.tsx:32-103`

**Step 1: Update TypeScript type**

In `web/src/api/types.ts`, add to `OIDCProvider`:

```typescript
export interface OIDCProvider {
  id: string
  name: string
  issuer_url: string
  client_id: string
  scopes: string
  default_role: 'admin' | 'member'
  enabled: boolean
  skip_email_verification: boolean
  created_at: string
  updated_at: string
}
```

**Step 2: Add state and toggle to OIDCForm**

In `web/src/pages/settings/OIDCSettingsTab.tsx`, add state in OIDCForm:

```typescript
const [skipEmailVerification, setSkipEmailVerification] = useState(provider?.skip_email_verification ?? false)
```

Add to the mutation payload in `handleSubmit`:

```typescript
saveMutation.mutate({
  name,
  issuer_url: issuerUrl,
  client_id: clientId,
  client_secret: clientSecret || '',
  scopes,
  default_role: defaultRole,
  enabled,
  skip_email_verification: skipEmailVerification,
})
```

Add a Switch toggle in the form, after the Enabled toggle:

```tsx
<div className="flex flex-col gap-1.5">
  <div className="flex items-center gap-3">
    <Switch checked={skipEmailVerification} onCheckedChange={setSkipEmailVerification} />
    <Label className="text-sm">Skip email verification</Label>
  </div>
  <p className="text-[11px] text-muted-foreground/60">
    When enabled, users can log in via SSO even if the identity provider does not return a verified email address. Only enable this if you trust your identity provider to return accurate email addresses.
  </p>
</div>
```

**Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 4: Commit**

```bash
git add web/src/api/types.ts web/src/pages/settings/OIDCSettingsTab.tsx
git commit -m "feat: add skip email verification toggle to OIDC settings UI"
```

---

### Task 7: Documentation

**Files:**
- Modify: `docs-site/docs/dashboard/sso-oidc.md:76-86`
- Modify: `docs-site/docs/self-hosting/configuration.md:23`

**Step 1: Update OIDC docs**

Replace the "Email Verification Requirement" section in `docs-site/docs/dashboard/sso-oidc.md` with:

```markdown
## Email Verification Requirement

Togglerino requires the OIDC provider to return an `email_verified: true` claim in the ID token. If the claim is missing or set to `false`, the login is rejected with an `oidc_email_not_verified` error.

This prevents account linking to the wrong user when an identity provider returns an unverified email address.

**Providers known to include `email_verified`:** Google Workspace, Okta, Auth0, Azure AD (Entra ID).

**Providers that may omit the claim:** Some enterprise SAML-to-OIDC bridges and self-hosted identity providers may not include `email_verified` in the ID token.

### Skipping Email Verification

If your provider does not emit the `email_verified` claim, you can enable the **Skip email verification** toggle in the OIDC settings. When enabled, Togglerino treats missing or unverified email addresses as verified.

**Dashboard:** Go to **Settings → SSO/OIDC** and enable the **Skip email verification** toggle.

**Environment variable:** Set `OIDC_SKIP_EMAIL_VERIFICATION=true`.

:::caution
Only enable this if you trust your identity provider to return accurate email addresses. Disabling email verification removes a layer of protection against account takeover via unverified email claims.
:::
```

**Step 2: Update configuration docs**

Add a row to the env var table in `docs-site/docs/self-hosting/configuration.md`:

```markdown
| `OIDC_SKIP_EMAIL_VERIFICATION` | `false` | Skip `email_verified` claim check for OIDC login. Set to `true` only if your provider doesn't emit this claim. |
```

**Step 3: Commit**

```bash
git add docs-site/docs/dashboard/sso-oidc.md docs-site/docs/self-hosting/configuration.md
git commit -m "docs: document skip_email_verification toggle and env var"
```

---

### Task 8: Update CLAUDE.md env var list

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add env var to the Environment Variables section**

Add after the `OIDC_DEFAULT_ROLE` line:

```markdown
- `OIDC_SKIP_EMAIL_VERIFICATION` — Skip `email_verified` claim check for OIDC login (default: `false`)
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add OIDC_SKIP_EMAIL_VERIFICATION to CLAUDE.md"
```
