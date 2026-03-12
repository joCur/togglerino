# MCP Server & Personal Access Tokens Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Personal Access Tokens (PATs) for programmatic management API access, then build an MCP server that uses PATs to let AI assistants manage feature flags.

**Architecture:** PATs are a new auth path in the Go backend — migration, store, handler, combined `SessionOrPATAuth` middleware. The MCP server is a standalone TypeScript package (`mcp/`) using `@modelcontextprotocol/sdk` for stdio transport, acting as a thin HTTP client to the management API.

**Tech Stack:** Go 1.25 (stdlib net/http, pgx/v5, log/slog), PostgreSQL 16, React 19 + TypeScript + TanStack Query + shadcn/ui, TypeScript + @modelcontextprotocol/sdk + tsup + vitest

**Spec:** `docs/superpowers/specs/2026-03-12-mcp-server-design.md`

---

## Review Corrections (Apply During Implementation)

The following corrections were identified during plan review and MUST be applied when implementing:

### Go Backend Corrections

1. **Migration number**: The plan uses `028` as the next migration number. Before creating the migration, verify this is correct by checking `ls migrations/` — if another branch has landed `028`, use the next available number.

2. **`SessionOrPATAuth` mock/concrete type mismatch**: The existing `SessionAuth` uses concrete types (`*store.SessionStore`, `*store.UserStore`). The plan's middleware tests use mocks. To resolve this, `SessionOrPATAuth` should compose with the existing `SessionAuth` internally: try PAT auth first (using a `PATFinder` interface), and if no PAT header is present, delegate to the existing `SessionAuth` middleware. This avoids needing to mock `SessionStore`/`UserStore`. The test for the session fallback path can be an integration test or simply verify that the middleware delegates correctly.

3. **Missing imports in middleware**: The `SessionOrPATAuth` implementation needs `"strings"` in addition to `"crypto/sha256"`, `"encoding/hex"`, `"sync"`, and `"time"`. The test file needs `"fmt"`.

4. **PAT handler test (Task 6) needs full test code**: The plan provides only a description of what to test. The implementer should follow the pattern in `internal/handler/sdk_key_handler_test.go` (or similar handler tests) using `testPool(t)`, real database, and actual HTTP requests via `httptest`.

5. **`uniqueKey` helper**: The PAT store tests use `uniqueKey()` from `internal/store/project_store_test.go`. This works because both are in `package store_test`. Ensure test files use `package store_test` (not `package store`).

6. **Routes to keep session-only**: When replacing `sessionAuth` with `sessionOrPATAuth`, keep these routes session-only:
   - PAT CRUD routes (`/api/v1/auth/tokens*`) — cannot create tokens with tokens
   - `GET /PUT /api/v1/auth/me` — profile management is session-only
   - `POST /api/v1/auth/change-password` — password changes require session

### MCP Server Corrections

7. **Top-level await requires ESM**: The entry point uses `await server.connect(transport)` at the top level. Since `tsup` outputs CJS (`dist/index.js`) and the `bin` field points there, this will crash. Fix: wrap in an async IIFE (`(async () => { ... })()`) or only output ESM and update the bin entry to `dist/index.mjs`.

8. **`tsup` banner should only apply to CJS**: The shebang `#!/usr/bin/env node` should only be on the CJS output. Use format-specific banner or only build one format for the CLI.

9. **Error handling wrapper for MCP tools**: The spec defines detailed HTTP-to-MCP error mapping. Each tool handler should be wrapped with try/catch that catches `TogglerinoError` and returns proper MCP tool error responses with `isError: true`. Example:
   ```typescript
   try { ... } catch (e) {
     const msg = e instanceof TogglerinoError ? e.message : 'Unexpected error'
     return { content: [{ type: 'text', text: msg }], isError: true }
   }
   ```

10. **`create_flag` parameter names**: Verify exact field names against the actual `FlagHandler.Create` request body. The Go backend likely uses `flag_type`, `value_type`, `default_value` (snake_case in JSON). Check `internal/handler/flag_handler.go` for the struct tags.

11. **Missing `.gitignore` for `mcp/`**: Check if root `.gitignore` covers `node_modules/` and `dist/`. If not, create `mcp/.gitignore` with those entries.

12. **MCP server version**: The `McpServer` constructor hardcodes `version: '0.1.0'`. Consider reading from `package.json` or adding a note to update on release.

13. **Documentation**: In addition to updating `configuration.md` and `CLAUDE.md`, create a dedicated MCP server setup page under `docs-site/docs/sdks/` per the docs maintenance rules.

---

## Chunk 1: PAT Backend (Database, Store, Model)

### Task 1: Database Migration

**Files:**
- Create: `migrations/028_personal_access_tokens.up.sql`
- Create: `migrations/028_personal_access_tokens.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
CREATE TABLE personal_access_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (length(name) <= 100),
    token_hash TEXT NOT NULL,
    token_prefix VARCHAR(12) NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_personal_access_tokens_hash ON personal_access_tokens (token_hash);
CREATE INDEX idx_personal_access_tokens_user ON personal_access_tokens (user_id);
```

- [ ] **Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS personal_access_tokens;
```

- [ ] **Step 3: Verify migration applies cleanly**

Run: `docker compose up -d` (if not already running), then:
```bash
PGPASSWORD=togglerino psql -h localhost -U togglerino -d togglerino -f migrations/028_personal_access_tokens.up.sql
```
Expected: `CREATE TABLE`, `CREATE INDEX` x2, no errors.

- [ ] **Step 4: Commit**

```bash
git add migrations/028_personal_access_tokens.up.sql migrations/028_personal_access_tokens.down.sql
git commit -m "feat: add personal_access_tokens migration (#124)"
```

---

### Task 2: PAT Model

**Files:**
- Create: `internal/model/personal_access_token.go`

- [ ] **Step 1: Create the model**

```go
package model

import "time"

// PersonalAccessToken represents a user's API token for programmatic access.
type PersonalAccessToken struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id,omitempty"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// PersonalAccessTokenWithValue includes the plaintext token (only returned on creation).
type PersonalAccessTokenWithValue struct {
	PersonalAccessToken
	Token string `json:"token"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/personal_access_token.go
git commit -m "feat: add PersonalAccessToken model (#124)"
```

---

### Task 3: PAT Store — Create and FindByHash

**Files:**
- Create: `internal/store/pat_store.go`
- Create: `internal/store/pat_store_test.go`

- [ ] **Step 1: Write failing test for Create**

In `internal/store/pat_store_test.go`:

```go
package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func createTestUser(t *testing.T, us *store.UserStore) *model.User {
	t.Helper()
	ctx := context.Background()
	email := uniqueKey("pat") + "@test.com"
	user, err := us.Create(ctx, email, "hashedpw", model.RoleAdmin)
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return user
}

func TestPATStore_Create(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	pat, err := ps.Create(ctx, user.ID, "My Token", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if pat.ID == "" {
		t.Error("expected non-empty ID")
	}
	if !strings.HasPrefix(pat.Token, "pat_") {
		t.Errorf("Token should start with 'pat_', got %q", pat.Token)
	}
	if len(pat.Token) != 44 { // "pat_" (4) + 40 hex chars
		t.Errorf("Token length: got %d, want 44", len(pat.Token))
	}
	if pat.Name != "My Token" {
		t.Errorf("Name: got %q, want %q", pat.Name, "My Token")
	}
	if pat.TokenPrefix != pat.Token[:12] {
		t.Errorf("TokenPrefix: got %q, want %q", pat.TokenPrefix, pat.Token[:12])
	}
	if pat.ExpiresAt != nil {
		t.Error("expected nil ExpiresAt")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestPATStore_Create -v`
Expected: FAIL — `NewPATStore` not found.

- [ ] **Step 3: Write failing test for FindByHash**

Append to `internal/store/pat_store_test.go`:

```go
func TestPATStore_FindByHash(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	created, err := ps.Create(ctx, user.ID, "Findable", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Hash the token the same way the store does
	h := sha256.Sum256([]byte(created.Token))
	hash := hex.EncodeToString(h[:])

	found, err := ps.FindByHash(ctx, hash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}

	if found.ID != created.ID {
		t.Errorf("ID: got %q, want %q", found.ID, created.ID)
	}
	if found.UserID != user.ID {
		t.Errorf("UserID: got %q, want %q", found.UserID, user.ID)
	}
}

func TestPATStore_FindByHash_NotFound(t *testing.T) {
	pool := testPool(t)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	h := sha256.Sum256([]byte("pat_nonexistent0000000000000000000000000000"))
	hash := hex.EncodeToString(h[:])

	_, err := ps.FindByHash(ctx, hash)
	if err == nil {
		t.Fatal("expected error for non-existent token, got nil")
	}
}
```

- [ ] **Step 4: Implement PATStore with Create and FindByHash**

Create `internal/store/pat_store.go`:

```go
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type PATStore struct {
	pool *pgxpool.Pool
}

func NewPATStore(pool *pgxpool.Pool) *PATStore {
	return &PATStore{pool: pool}
}

// Create generates a new personal access token for a user.
// The plaintext token is returned only in this response.
func (s *PATStore) Create(ctx context.Context, userID, name string, expiresAt *time.Time) (*model.PersonalAccessTokenWithValue, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generating random token: %w", err)
	}
	token := "pat_" + hex.EncodeToString(b)
	prefix := token[:12]

	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	var pat model.PersonalAccessToken
	err := s.pool.QueryRow(ctx,
		`INSERT INTO personal_access_tokens (user_id, name, token_hash, token_prefix, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, name, token_prefix, expires_at, last_used_at, created_at`,
		userID, name, hash, prefix, expiresAt,
	).Scan(&pat.ID, &pat.UserID, &pat.Name, &pat.TokenPrefix, &pat.ExpiresAt, &pat.LastUsedAt, &pat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating personal access token: %w", err)
	}

	return &model.PersonalAccessTokenWithValue{
		PersonalAccessToken: pat,
		Token:               token,
	}, nil
}

// FindByHash looks up a token by its SHA-256 hash.
func (s *PATStore) FindByHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error) {
	var pat model.PersonalAccessToken
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, token_prefix, expires_at, last_used_at, created_at
		 FROM personal_access_tokens
		 WHERE token_hash = $1`,
		hash,
	).Scan(&pat.ID, &pat.UserID, &pat.Name, &pat.TokenPrefix, &pat.ExpiresAt, &pat.LastUsedAt, &pat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding personal access token: %w", err)
	}
	return &pat, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestPATStore -v`
Expected: All 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/pat_store.go internal/store/pat_store_test.go
git commit -m "feat: add PATStore with Create and FindByHash (#124)"
```

---

### Task 4: PAT Store — ListByUser, Delete, UpdateLastUsed

**Files:**
- Modify: `internal/store/pat_store.go`
- Modify: `internal/store/pat_store_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/store/pat_store_test.go`:

```go
func TestPATStore_ListByUser(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	_, err := ps.Create(ctx, user.ID, "Token 1", nil)
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	_, err = ps.Create(ctx, user.ID, "Token 2", nil)
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	tokens, err := ps.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}

	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	// Should be ordered by created_at DESC
	if tokens[0].Name != "Token 2" {
		t.Errorf("first token: got %q, want %q", tokens[0].Name, "Token 2")
	}
}

func TestPATStore_Delete(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	created, err := ps.Create(ctx, user.ID, "To Delete", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.Delete(ctx, created.ID, user.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	tokens, err := ps.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens after delete, got %d", len(tokens))
	}
}

func TestPATStore_Delete_WrongUser(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user1 := createTestUser(t, us)
	user2 := createTestUser(t, us)

	created, err := ps.Create(ctx, user1.ID, "User1 Token", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.Delete(ctx, created.ID, user2.ID)
	if err == nil {
		t.Fatal("expected error when deleting another user's token")
	}
}

func TestPATStore_UpdateLastUsed(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	created, err := ps.Create(ctx, user.ID, "Track Usage", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = ps.UpdateLastUsed(ctx, created.ID)
	if err != nil {
		t.Fatalf("UpdateLastUsed: %v", err)
	}

	tokens, err := ps.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].LastUsedAt == nil {
		t.Error("expected LastUsedAt to be set after UpdateLastUsed")
	}
}

func TestPATStore_Create_WithExpiry(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ps := store.NewPATStore(pool)
	ctx := context.Background()

	user := createTestUser(t, us)

	expiry := time.Now().Add(24 * time.Hour)
	pat, err := ps.Create(ctx, user.ID, "Expiring Token", &expiry)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if pat.ExpiresAt == nil {
		t.Fatal("expected non-nil ExpiresAt")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestPATStore -v`
Expected: FAIL — `ListByUser`, `Delete`, `UpdateLastUsed` not found.

- [ ] **Step 3: Implement ListByUser, Delete, UpdateLastUsed**

Add to `internal/store/pat_store.go`:

```go
// ListByUser returns all tokens for a user, ordered by created_at DESC.
func (s *PATStore) ListByUser(ctx context.Context, userID string) ([]model.PersonalAccessToken, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, token_prefix, expires_at, last_used_at, created_at
		 FROM personal_access_tokens
		 WHERE user_id = $1
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing personal access tokens: %w", err)
	}
	defer rows.Close()

	var tokens []model.PersonalAccessToken
	for rows.Next() {
		var pat model.PersonalAccessToken
		if err := rows.Scan(&pat.ID, &pat.UserID, &pat.Name, &pat.TokenPrefix, &pat.ExpiresAt, &pat.LastUsedAt, &pat.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning personal access token: %w", err)
		}
		tokens = append(tokens, pat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating personal access tokens: %w", err)
	}
	return tokens, nil
}

// Delete removes a token. Only succeeds if the token belongs to the given user.
func (s *PATStore) Delete(ctx context.Context, id, userID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM personal_access_tokens WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting personal access token: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("token not found or not owned by user")
	}
	return nil
}

// UpdateLastUsed sets last_used_at to now.
func (s *PATStore) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE personal_access_tokens SET last_used_at = now() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("updating last_used_at: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestPATStore -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/pat_store.go internal/store/pat_store_test.go
git commit -m "feat: add PATStore ListByUser, Delete, UpdateLastUsed (#124)"
```

---

## Chunk 2: PAT Auth Middleware & Handler

### Task 5: SessionOrPATAuth Middleware

**Files:**
- Modify: `internal/auth/middleware.go`
- Create: `internal/auth/pat_auth_test.go`

- [ ] **Step 1: Write failing test for PAT auth — valid token**

Create `internal/auth/pat_auth_test.go`. Note: handler tests in this codebase use `httptest`. Check how existing middleware tests work:

```go
package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
)

// mockPATStore implements the interface needed by SessionOrPATAuth.
type mockPATStore struct {
	tokens map[string]*model.PersonalAccessToken // keyed by token_hash
}

func (m *mockPATStore) FindByHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error) {
	pat, ok := m.tokens[hash]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return pat, nil
}

func (m *mockPATStore) UpdateLastUsed(ctx context.Context, id string) error {
	return nil
}

// mockSessionStore and mockUserStore should match existing test patterns.
// Check internal/auth/ for existing mock patterns and adapt accordingly.

func TestSessionOrPATAuth_ValidPAT(t *testing.T) {
	token := "pat_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	user := &model.User{ID: "user-1", Email: "test@test.com", Role: model.RoleAdmin}

	patStore := &mockPATStore{
		tokens: map[string]*model.PersonalAccessToken{
			hash: {ID: "pat-1", UserID: user.ID, Name: "Test", TokenPrefix: "pat_aaaaaaaa"},
		},
	}

	// The middleware needs a UserStore to load the user from the PAT's user_id.
	// Create a mock that returns the user for the given ID.
	userStore := &mockUserStore{users: map[string]*model.User{user.ID: user}}
	sessionStore := &mockSessionStore{} // empty — no sessions

	middleware := auth.SessionOrPATAuth(sessionStore, userStore, patStore)

	called := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		u := auth.UserFromContext(r.Context())
		if u == nil {
			t.Fatal("expected user in context")
		}
		if u.ID != "user-1" {
			t.Errorf("user ID: got %q, want %q", u.ID, "user-1")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSessionOrPATAuth_ExpiredPAT(t *testing.T) {
	token := "pat_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	h := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(h[:])

	expired := time.Now().Add(-1 * time.Hour)
	patStore := &mockPATStore{
		tokens: map[string]*model.PersonalAccessToken{
			hash: {ID: "pat-2", UserID: "user-1", Name: "Expired", TokenPrefix: "pat_bbbbbbbb", ExpiresAt: &expired},
		},
	}

	userStore := &mockUserStore{}
	sessionStore := &mockSessionStore{}

	middleware := auth.SessionOrPATAuth(sessionStore, userStore, patStore)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for expired token")
	}))

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSessionOrPATAuth_FallsBackToSession(t *testing.T) {
	// When no Authorization header is present, should fall back to session auth.
	// This test verifies the fallback path. Use existing session mock patterns.
	// A valid session cookie should authenticate the user.
	// Implementation depends on existing mock patterns in the auth package — adapt as needed.
}
```

Note: The exact mock types depend on what interfaces `SessionOrPATAuth` requires. When implementing, define interfaces for the stores it needs (`PATFinder`, `UserFinder`, `SessionFinder`) and create mocks accordingly. Check existing test files in `internal/auth/` for mock patterns to follow.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run TestSessionOrPATAuth -v`
Expected: FAIL — `SessionOrPATAuth` not found.

- [ ] **Step 3: Implement SessionOrPATAuth middleware**

Add to `internal/auth/middleware.go`:

```go
// PATFinder looks up a PAT by its hash.
type PATFinder interface {
	FindByHash(ctx context.Context, hash string) (*model.PersonalAccessToken, error)
	UpdateLastUsed(ctx context.Context, id string) error
}

// SessionOrPATAuth tries PAT auth first (if Authorization header with pat_ prefix present),
// then falls back to session cookie auth.
func SessionOrPATAuth(sessions *store.SessionStore, users *store.UserStore, pats PATFinder) func(http.Handler) http.Handler {
	// In-memory map for last_used_at debouncing (token ID → last update time)
	var lastUsedMu sync.Mutex
	lastUsedTimes := make(map[string]time.Time)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try PAT auth if Authorization header has pat_ prefix
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer pat_") {
				token := strings.TrimPrefix(authHeader, "Bearer ")

				h := sha256.Sum256([]byte(token))
				hash := hex.EncodeToString(h[:])

				pat, err := pats.FindByHash(r.Context(), hash)
				if err != nil {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}

				// Check expiry
				if pat.ExpiresAt != nil && pat.ExpiresAt.Before(time.Now()) {
					http.Error(w, `{"error":"token expired"}`, http.StatusUnauthorized)
					return
				}

				// Debounced last_used_at update (at most once per minute per token)
				lastUsedMu.Lock()
				lastUpdate, exists := lastUsedTimes[pat.ID]
				shouldUpdate := !exists || time.Since(lastUpdate) > time.Minute
				if shouldUpdate {
					lastUsedTimes[pat.ID] = time.Now()
				}
				lastUsedMu.Unlock()

				if shouldUpdate {
					// Fire and forget — don't block the request
					go pats.UpdateLastUsed(context.Background(), pat.ID)
				}

				user, err := users.FindByID(r.Context(), pat.UserID)
				if err != nil {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}

				ctx := ContextWithUser(r.Context(), user)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall back to session auth
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessions.FindByID(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := users.FindByID(r.Context(), session.UserID)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

Add these imports at the top: `"crypto/sha256"`, `"encoding/hex"`, `"sync"`, `"time"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -run TestSessionOrPATAuth -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/middleware.go internal/auth/pat_auth_test.go
git commit -m "feat: add SessionOrPATAuth combined middleware (#124)"
```

---

### Task 6: PAT Handler (CRUD)

**Files:**
- Create: `internal/handler/pat_handler.go`
- Create: `internal/handler/pat_handler_test.go`

- [ ] **Step 1: Write failing tests for Create, List, Delete handlers**

Create `internal/handler/pat_handler_test.go`. Tests should:
- Test `POST /api/v1/auth/tokens` — creates token, response includes `token` field
- Test `GET /api/v1/auth/tokens` — lists tokens, response does NOT include `token` field
- Test `DELETE /api/v1/auth/tokens/{id}` — deletes token, returns 204
- Test `DELETE /api/v1/auth/tokens/{id}` with wrong user — returns error
- Test `POST /api/v1/auth/tokens` with empty name — returns 400

Follow the patterns in existing handler tests. Use `testPool(t)`, create a real user, authenticate via session, and test the handler end-to-end.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestPATHandler -v`
Expected: FAIL — `PATHandler` not found.

- [ ] **Step 3: Implement PATHandler**

Create `internal/handler/pat_handler.go`:

```go
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/store"
)

type PATHandler struct {
	pats *store.PATStore
}

func NewPATHandler(pats *store.PATStore) *PATHandler {
	return &PATHandler{pats: pats}
}

// Create handles POST /api/v1/auth/tokens
func (h *PATHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name must be 100 characters or fewer")
		return
	}

	pat, err := h.pats.Create(r.Context(), user.ID, req.Name, req.ExpiresAt)
	if err != nil {
		slog.Error("failed to create personal access token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, pat)
}

// List handles GET /api/v1/auth/tokens
func (h *PATHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	tokens, err := h.pats.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("failed to list personal access tokens", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	if tokens == nil {
		tokens = []model.PersonalAccessToken{}
	}

	writeJSON(w, http.StatusOK, tokens)
}

// Delete handles DELETE /api/v1/auth/tokens/{id}
func (h *PATHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "token id is required")
		return
	}

	if err := h.pats.Delete(r.Context(), id, user.ID); err != nil {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

Add the `model` import: `"github.com/togglerino/togglerino/internal/model"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestPATHandler -v`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/pat_handler.go internal/handler/pat_handler_test.go
git commit -m "feat: add PAT handler for create, list, delete (#124)"
```

---

### Task 7: Wire PATs into main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

- [ ] **Step 1: Add PATStore creation, PATHandler, and SessionOrPATAuth to main.go**

In the dependency creation section of `main.go` (near the other store creations):

```go
patStore := store.NewPATStore(pool)
```

In the handler creation section:

```go
patHandler := handler.NewPATHandler(patStore)
```

Replace the `sessionAuth` middleware definition:

```go
sessionOrPATAuth := auth.SessionOrPATAuth(sessionStore, userStore, patStore)
```

Then replace all occurrences of `sessionAuth` in route registrations with `sessionOrPATAuth`. Keep `sessionAuth` only for the PAT management endpoints themselves (create/list/delete tokens must use session auth only — you shouldn't be able to create a token using a token).

Add the PAT management routes (session-authed only):

```go
// PAT management (session-only, not PAT-authed)
sessionAuth := auth.SessionAuth(sessionStore, userStore)
mux.Handle("POST /api/v1/auth/tokens", wrap(patHandler.Create, sessionAuth))
mux.Handle("GET /api/v1/auth/tokens", wrap(patHandler.List, sessionAuth))
mux.Handle("DELETE /api/v1/auth/tokens/{id}", wrap(patHandler.Delete, sessionAuth))
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cmd/togglerino`
Expected: Builds successfully.

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: wire PATs into main.go, replace sessionAuth with sessionOrPATAuth (#124)"
```

---

## Chunk 3: Dashboard UI for PAT Management

### Task 8: API Client & Types for Tokens

**Files:**
- Modify: `web/src/api/types.ts` — add PAT types
- Modify: `web/src/api/client.ts` — add token API methods

- [ ] **Step 1: Add types**

In `web/src/api/types.ts`, add:

```typescript
export interface PersonalAccessToken {
  id: string
  name: string
  token_prefix: string
  expires_at: string | null
  last_used_at: string | null
  created_at: string
}

export interface PersonalAccessTokenWithValue extends PersonalAccessToken {
  token: string
}
```

- [ ] **Step 2: Add API methods**

In `web/src/api/client.ts`, add a `tokens` namespace:

```typescript
tokens: {
  list: () => request<PersonalAccessToken[]>('/auth/tokens'),
  create: (data: { name: string; expires_at?: string }) =>
    request<PersonalAccessTokenWithValue>('/auth/tokens', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  delete: (id: string) =>
    request<void>(`/auth/tokens/${id}`, { method: 'DELETE' }),
},
```

- [ ] **Step 3: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat: add PAT types and API client methods (#124)"
```

---

### Task 9: API Tokens UI on Account Page

**Files:**
- Modify: `web/src/pages/AccountPage.tsx`

- [ ] **Step 1: Add token management section to AccountPage**

Add after the existing cards on the account page. The section should include:

1. A "API Tokens" card with:
   - A list/table of existing tokens (name, prefix, last used, expires, created, revoke button)
   - A "Create Token" button that opens a dialog
2. A create token dialog with:
   - Name input (required)
   - Expiry date input (optional)
   - Submit button
3. After creation, a one-time token display dialog with:
   - The full token in a copyable code block
   - A warning that it won't be shown again
   - A "Done" button that closes the dialog

Use these TanStack Query patterns:
```typescript
const tokensQuery = useQuery({
  queryKey: ['auth', 'tokens'],
  queryFn: () => api.tokens.list(),
})

const createToken = useMutation({
  mutationFn: (data: { name: string; expires_at?: string }) => api.tokens.create(data),
  onSuccess: (data) => {
    setNewToken(data.token)
    queryClient.invalidateQueries({ queryKey: ['auth', 'tokens'] })
  },
})

const deleteToken = useMutation({
  mutationFn: (id: string) => api.tokens.delete(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['auth', 'tokens'] })
  },
})
```

Use shadcn/ui components: `Card`, `CardContent`, `Button`, `Input`, `Label`, `Dialog`, `DialogContent`, `DialogHeader`, `DialogTitle`, `Table`, `TableBody`, `TableCell`, `TableHead`, `TableHeader`, `TableRow`, `Badge`.

If any shadcn component is not yet installed (check `web/src/components/ui/`), install it:
```bash
cd web && npx shadcn@latest add <component>
```

- [ ] **Step 2: Verify the UI works**

Run: `cd web && npm run dev`
Navigate to `/account`. Verify:
- Token list renders (empty initially)
- Create token dialog opens, submits, shows token once
- Token appears in list after creation
- Revoke button works

- [ ] **Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/AccountPage.tsx
git commit -m "feat: add API Tokens management UI to account page (#124)"
```

---

## Chunk 4: MCP Server

### Task 10: MCP Server Project Setup

**Files:**
- Create: `mcp/package.json`
- Create: `mcp/tsconfig.json`
- Create: `mcp/tsup.config.ts`
- Create: `mcp/vitest.config.ts`
- Create: `mcp/src/index.ts` (placeholder)

- [ ] **Step 1: Create package.json**

```json
{
  "name": "@togglerino/mcp",
  "version": "0.1.0",
  "description": "MCP server for Togglerino feature flag management",
  "main": "dist/index.js",
  "module": "dist/index.mjs",
  "types": "dist/index.d.ts",
  "bin": {
    "togglerino-mcp": "dist/index.js"
  },
  "files": [
    "dist"
  ],
  "scripts": {
    "build": "tsup",
    "test": "vitest run",
    "test:watch": "vitest",
    "dev": "tsx src/index.ts"
  },
  "keywords": [
    "mcp",
    "feature-flags",
    "togglerino"
  ],
  "license": "MIT",
  "repository": {
    "type": "git",
    "url": "https://github.com/joCur/togglerino",
    "directory": "mcp"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.12.1"
  },
  "devDependencies": {
    "tsup": "^8.5.1",
    "tsx": "^4.19.4",
    "typescript": "^5.9.3",
    "vitest": "^4.0.18"
  }
}
```

Note: Check the latest `@modelcontextprotocol/sdk` version on npm before installing. The version above is a placeholder — use whatever is current.

- [ ] **Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ES2022",
    "moduleResolution": "bundler",
    "esModuleInterop": true,
    "strict": true,
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

- [ ] **Step 3: Create tsup.config.ts**

```typescript
import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['cjs', 'esm'],
  dts: true,
  clean: true,
  banner: {
    js: '#!/usr/bin/env node',
  },
})
```

The `banner` makes the built output executable via `npx`.

- [ ] **Step 4: Create vitest.config.ts**

```typescript
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    globals: true,
  },
})
```

- [ ] **Step 5: Create placeholder index.ts**

Create `mcp/src/index.ts`:

```typescript
console.log('togglerino-mcp placeholder')
```

- [ ] **Step 6: Install dependencies and verify build**

```bash
cd mcp && npm install && npm run build
```
Expected: Builds successfully, creates `dist/`.

- [ ] **Step 7: Commit**

```bash
git add mcp/
git commit -m "feat: scaffold MCP server project (#124)"
```

---

### Task 11: HTTP Client

**Files:**
- Create: `mcp/src/client.ts`
- Create: `mcp/tests/client.test.ts`

- [ ] **Step 1: Write failing tests for the HTTP client**

Create `mcp/tests/client.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { TogglerinoClient } from '../src/client'

describe('TogglerinoClient', () => {
  let client: TogglerinoClient

  beforeEach(() => {
    client = new TogglerinoClient('http://localhost:8080', 'pat_testtoken123')
  })

  it('sends authorization header', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
    })
    vi.stubGlobal('fetch', mockFetch)

    await client.get('/projects')

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/projects',
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer pat_testtoken123',
        }),
      }),
    )

    vi.unstubAllGlobals()
  })

  it('throws on non-ok response with error message', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: 'forbidden' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    await expect(client.get('/projects')).rejects.toThrow('forbidden')

    vi.unstubAllGlobals()
  })

  it('constructs POST requests with JSON body', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 201,
      json: () => Promise.resolve({ id: '1' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    await client.post('/projects/test/flags', { name: 'my-flag', key: 'my-flag' })

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/v1/projects/test/flags',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ name: 'my-flag', key: 'my-flag' }),
      }),
    )

    vi.unstubAllGlobals()
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mcp && npm test`
Expected: FAIL — `TogglerinoClient` not found.

- [ ] **Step 3: Implement the client**

Create `mcp/src/client.ts`:

```typescript
export class TogglerinoError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'TogglerinoError'
    this.status = status
  }
}

export class TogglerinoClient {
  private baseUrl: string
  private apiKey: string

  constructor(baseUrl: string, apiKey: string) {
    // Remove trailing slash
    this.baseUrl = baseUrl.replace(/\/+$/, '')
    this.apiKey = apiKey
  }

  async get<T>(path: string): Promise<T> {
    return this.request<T>(path, { method: 'GET' })
  }

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async put<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>(path, {
      method: 'PUT',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  private async request<T>(path: string, options: RequestInit): Promise<T> {
    const url = `${this.baseUrl}/api/v1${path}`
    const res = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${this.apiKey}`,
        ...options.headers,
      },
    })

    if (!res.ok) {
      const error = await res.json().catch(() => ({ error: res.statusText }))
      throw new TogglerinoError(res.status, error.error || res.statusText)
    }

    if (res.status === 204) {
      return undefined as T
    }

    return res.json()
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mcp && npm test`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/src/client.ts mcp/tests/client.test.ts
git commit -m "feat: add MCP HTTP client for Togglerino API (#124)"
```

---

### Task 12: MCP Tool Definitions — Projects & Environments

**Files:**
- Create: `mcp/src/tools/projects.ts`
- Create: `mcp/src/tools/environments.ts`
- Create: `mcp/tests/tools/projects.test.ts`
- Create: `mcp/tests/tools/environments.test.ts`

- [ ] **Step 1: Write failing tests for list_projects tool**

Create `mcp/tests/tools/projects.test.ts`:

```typescript
import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listProjects } from '../../src/tools/projects'

describe('listProjects', () => {
  it('calls GET /projects and returns results', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue([
        { key: 'proj-1', name: 'Project One', description: 'First project' },
      ]),
    } as unknown as TogglerinoClient

    const result = await listProjects(mockClient)

    expect(mockClient.get).toHaveBeenCalledWith('/projects')
    expect(result).toEqual([
      { key: 'proj-1', name: 'Project One', description: 'First project' },
    ])
  })
})
```

- [ ] **Step 2: Write failing tests for list_environments tool**

Create `mcp/tests/tools/environments.test.ts`:

```typescript
import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listEnvironments } from '../../src/tools/environments'

describe('listEnvironments', () => {
  it('calls GET /projects/{key}/environments', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue([
        { key: 'development', name: 'Development' },
        { key: 'production', name: 'Production' },
      ]),
    } as unknown as TogglerinoClient

    const result = await listEnvironments(mockClient, 'my-project')

    expect(mockClient.get).toHaveBeenCalledWith('/projects/my-project/environments')
    expect(result).toHaveLength(2)
  })
})
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd mcp && npm test`
Expected: FAIL.

- [ ] **Step 4: Implement tools**

Create `mcp/src/tools/projects.ts`:

```typescript
import { TogglerinoClient } from '../client'

export async function listProjects(client: TogglerinoClient): Promise<unknown[]> {
  return client.get<unknown[]>('/projects')
}
```

Create `mcp/src/tools/environments.ts`:

```typescript
import { TogglerinoClient } from '../client'

export async function listEnvironments(client: TogglerinoClient, projectKey: string): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/environments`)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd mcp && npm test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add mcp/src/tools/projects.ts mcp/src/tools/environments.ts mcp/tests/tools/projects.test.ts mcp/tests/tools/environments.test.ts
git commit -m "feat: add MCP tools for projects and environments (#124)"
```

---

### Task 13: MCP Tool Definitions — Flags

**Files:**
- Create: `mcp/src/tools/flags.ts`
- Create: `mcp/tests/tools/flags.test.ts`

- [ ] **Step 1: Write failing tests for flag tools**

Create `mcp/tests/tools/flags.test.ts`. Test each function:

```typescript
import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listFlags, getFlag, createFlag, updateFlag, toggleFlag, updateFlagConfig } from '../../src/tools/flags'

describe('listFlags', () => {
  it('calls GET /projects/{key}/flags with query params', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ items: [], total: 0 }),
    } as unknown as TogglerinoClient

    await listFlags(mockClient, 'proj', { search: 'feat', tag: 'beta' })

    expect(mockClient.get).toHaveBeenCalledWith('/projects/proj/flags?search=feat&tag=beta')
  })

  it('calls without query params when none provided', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ items: [], total: 0 }),
    } as unknown as TogglerinoClient

    await listFlags(mockClient, 'proj')

    expect(mockClient.get).toHaveBeenCalledWith('/projects/proj/flags')
  })
})

describe('getFlag', () => {
  it('calls GET /projects/{key}/flags/{flag}', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ key: 'my-flag', name: 'My Flag' }),
    } as unknown as TogglerinoClient

    await getFlag(mockClient, 'proj', 'my-flag')

    expect(mockClient.get).toHaveBeenCalledWith('/projects/proj/flags/my-flag')
  })
})

describe('createFlag', () => {
  it('calls POST /projects/{key}/flags', async () => {
    const mockClient = {
      post: vi.fn().mockResolvedValue({ key: 'new-flag' }),
    } as unknown as TogglerinoClient

    const params = {
      name: 'New Flag',
      key: 'new-flag',
      flag_type: 'release',
      value_type: 'boolean',
      default_value: false,
    }
    await createFlag(mockClient, 'proj', params)

    expect(mockClient.post).toHaveBeenCalledWith('/projects/proj/flags', params)
  })
})

describe('updateFlag', () => {
  it('calls PUT /projects/{key}/flags/{flag}', async () => {
    const mockClient = {
      put: vi.fn().mockResolvedValue({ key: 'my-flag' }),
    } as unknown as TogglerinoClient

    await updateFlag(mockClient, 'proj', 'my-flag', { description: 'Updated' })

    expect(mockClient.put).toHaveBeenCalledWith('/projects/proj/flags/my-flag', { description: 'Updated' })
  })
})

describe('toggleFlag', () => {
  it('GETs current config then PUTs with enabled changed', async () => {
    const currentConfig = {
      environments: {
        development: { enabled: false, default_variant: 'off', variants: [], targeting_rules: [] },
      },
    }
    const mockClient = {
      get: vi.fn().mockResolvedValue(currentConfig),
      put: vi.fn().mockResolvedValue({}),
    } as unknown as TogglerinoClient

    await toggleFlag(mockClient, 'proj', 'my-flag', 'development', true)

    expect(mockClient.get).toHaveBeenCalledWith('/projects/proj/flags/my-flag')
    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/proj/flags/my-flag/environments/development',
      expect.objectContaining({ enabled: true }),
    )
  })
})

describe('updateFlagConfig', () => {
  it('GETs current config then PUTs with merged changes', async () => {
    const currentConfig = {
      environments: {
        development: { enabled: true, default_variant: 'off', variants: [], targeting_rules: [] },
      },
    }
    const mockClient = {
      get: vi.fn().mockResolvedValue(currentConfig),
      put: vi.fn().mockResolvedValue({}),
    } as unknown as TogglerinoClient

    await updateFlagConfig(mockClient, 'proj', 'my-flag', 'development', { default_variant: 'on' })

    expect(mockClient.put).toHaveBeenCalledWith(
      '/projects/proj/flags/my-flag/environments/development',
      expect.objectContaining({ enabled: true, default_variant: 'on' }),
    )
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mcp && npm test`
Expected: FAIL.

- [ ] **Step 3: Implement flag tools**

Create `mcp/src/tools/flags.ts`:

```typescript
import { TogglerinoClient } from '../client'

export async function listFlags(
  client: TogglerinoClient,
  projectKey: string,
  params?: { search?: string; tag?: string },
): Promise<unknown> {
  const qs = new URLSearchParams()
  if (params?.search) qs.set('search', params.search)
  if (params?.tag) qs.set('tag', params.tag)
  const query = qs.toString()
  return client.get(`/projects/${projectKey}/flags${query ? `?${query}` : ''}`)
}

export async function getFlag(client: TogglerinoClient, projectKey: string, flagKey: string): Promise<unknown> {
  return client.get(`/projects/${projectKey}/flags/${flagKey}`)
}

export async function createFlag(client: TogglerinoClient, projectKey: string, params: Record<string, unknown>): Promise<unknown> {
  return client.post(`/projects/${projectKey}/flags`, params)
}

export async function updateFlag(
  client: TogglerinoClient,
  projectKey: string,
  flagKey: string,
  params: Record<string, unknown>,
): Promise<unknown> {
  return client.put(`/projects/${projectKey}/flags/${flagKey}`, params)
}

export async function toggleFlag(
  client: TogglerinoClient,
  projectKey: string,
  flagKey: string,
  environmentKey: string,
  enabled: boolean,
): Promise<unknown> {
  // Read-modify-write: GET current flag, merge enabled, PUT back
  const flag = (await client.get(`/projects/${projectKey}/flags/${flagKey}`)) as {
    environments: Record<string, Record<string, unknown>>
  }
  const envConfig = flag.environments?.[environmentKey] || {}
  return client.put(`/projects/${projectKey}/flags/${flagKey}/environments/${environmentKey}`, {
    ...envConfig,
    enabled,
  })
}

export async function updateFlagConfig(
  client: TogglerinoClient,
  projectKey: string,
  flagKey: string,
  environmentKey: string,
  updates: Record<string, unknown>,
): Promise<unknown> {
  // Read-modify-write: GET current flag, merge updates, PUT back
  const flag = (await client.get(`/projects/${projectKey}/flags/${flagKey}`)) as {
    environments: Record<string, Record<string, unknown>>
  }
  const envConfig = flag.environments?.[environmentKey] || {}
  return client.put(`/projects/${projectKey}/flags/${flagKey}/environments/${environmentKey}`, {
    ...envConfig,
    ...updates,
  })
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mcp && npm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/src/tools/flags.ts mcp/tests/tools/flags.test.ts
git commit -m "feat: add MCP tools for flag operations (#124)"
```

---

### Task 14: MCP Tool Definitions — Segments

**Files:**
- Create: `mcp/src/tools/segments.ts`
- Create: `mcp/tests/tools/segments.test.ts`

- [ ] **Step 1: Write failing tests**

Create `mcp/tests/tools/segments.test.ts`:

```typescript
import { describe, it, expect, vi } from 'vitest'
import { TogglerinoClient } from '../../src/client'
import { listSegments, getSegment } from '../../src/tools/segments'

describe('listSegments', () => {
  it('calls GET /projects/{key}/segments', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue([]),
    } as unknown as TogglerinoClient

    await listSegments(mockClient, 'proj')

    expect(mockClient.get).toHaveBeenCalledWith('/projects/proj/segments')
  })
})

describe('getSegment', () => {
  it('calls GET /projects/{key}/segments/{segmentKey}', async () => {
    const mockClient = {
      get: vi.fn().mockResolvedValue({ key: 'beta-users' }),
    } as unknown as TogglerinoClient

    await getSegment(mockClient, 'proj', 'beta-users')

    expect(mockClient.get).toHaveBeenCalledWith('/projects/proj/segments/beta-users')
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd mcp && npm test`
Expected: FAIL.

- [ ] **Step 3: Implement segment tools**

Create `mcp/src/tools/segments.ts`:

```typescript
import { TogglerinoClient } from '../client'

export async function listSegments(client: TogglerinoClient, projectKey: string): Promise<unknown[]> {
  return client.get<unknown[]>(`/projects/${projectKey}/segments`)
}

export async function getSegment(client: TogglerinoClient, projectKey: string, segmentKey: string): Promise<unknown> {
  return client.get(`/projects/${projectKey}/segments/${segmentKey}`)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd mcp && npm test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/src/tools/segments.ts mcp/tests/tools/segments.test.ts
git commit -m "feat: add MCP tools for segments (#124)"
```

---

### Task 15: MCP Server Entry Point

**Files:**
- Modify: `mcp/src/index.ts` — replace placeholder with full MCP server

- [ ] **Step 1: Implement the MCP server entry point**

Replace `mcp/src/index.ts` with the full server. This registers all 10 tools with the MCP SDK, validates config, creates the client, and starts the stdio transport.

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'
import { z } from 'zod'
import { TogglerinoClient } from './client.js'
import { listProjects } from './tools/projects.js'
import { listFlags, getFlag, createFlag, updateFlag, toggleFlag, updateFlagConfig } from './tools/flags.js'
import { listEnvironments } from './tools/environments.js'
import { listSegments, getSegment } from './tools/segments.js'

const TOGGLERINO_URL = process.env.TOGGLERINO_URL
const TOGGLERINO_API_KEY = process.env.TOGGLERINO_API_KEY
const TOGGLERINO_PROJECT = process.env.TOGGLERINO_PROJECT

if (!TOGGLERINO_URL) {
  console.error('TOGGLERINO_URL environment variable is required')
  process.exit(1)
}
if (!TOGGLERINO_API_KEY) {
  console.error('TOGGLERINO_API_KEY environment variable is required')
  process.exit(1)
}

const client = new TogglerinoClient(TOGGLERINO_URL, TOGGLERINO_API_KEY)

function resolveProject(projectKey?: string): string {
  const key = projectKey || TOGGLERINO_PROJECT
  if (!key) {
    throw new Error('projectKey is required — either pass it as a parameter or set TOGGLERINO_PROJECT environment variable. Use list_projects to discover available projects.')
  }
  return key
}

const server = new McpServer({
  name: 'togglerino',
  version: '0.1.0',
})

// --- Tool registrations ---

server.tool('list_projects', 'List all projects you have access to', {}, async () => {
  const projects = await listProjects(client)
  return { content: [{ type: 'text', text: JSON.stringify(projects, null, 2) }] }
})

server.tool(
  'list_flags',
  'List flags in a project, with optional search and tag filters',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    search: z.string().optional().describe('Search flags by name or key'),
    tag: z.string().optional().describe('Filter flags by tag'),
  },
  async ({ projectKey, search, tag }) => {
    const key = resolveProject(projectKey)
    const result = await listFlags(client, key, { search, tag })
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'get_flag',
  'Get full details of a flag including variants, environment configs, and targeting rules',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    flagKey: z.string().describe('The flag key'),
  },
  async ({ projectKey, flagKey }) => {
    const key = resolveProject(projectKey)
    const result = await getFlag(client, key, flagKey)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'create_flag',
  'Create a new feature flag',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    name: z.string().describe('Display name for the flag'),
    key: z.string().describe('Unique key for the flag (kebab-case)'),
    flag_type: z.enum(['release', 'experiment', 'operational', 'kill-switch', 'permission']).describe('Flag type'),
    value_type: z.enum(['boolean', 'string', 'number', 'json']).describe('Value type'),
    default_value: z.unknown().describe('Default value for the flag'),
    description: z.string().optional().describe('Flag description'),
    tags: z.array(z.string()).optional().describe('Tags for the flag'),
    environment_overrides: z.record(z.unknown()).optional().describe('Per-environment config overrides (e.g., enable in development)'),
  },
  async ({ projectKey, ...params }) => {
    const key = resolveProject(projectKey)
    const result = await createFlag(client, key, params)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'update_flag',
  'Update flag metadata (name, description, tags)',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    flagKey: z.string().describe('The flag key'),
    name: z.string().optional().describe('New display name'),
    description: z.string().optional().describe('New description'),
    tags: z.array(z.string()).optional().describe('New tags'),
  },
  async ({ projectKey, flagKey, ...params }) => {
    const key = resolveProject(projectKey)
    const result = await updateFlag(client, key, flagKey, params)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'toggle_flag',
  'Enable or disable a flag in a specific environment',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    flagKey: z.string().describe('The flag key'),
    environmentKey: z.string().describe('The environment key (e.g., development, staging, production)'),
    enabled: z.boolean().describe('Whether to enable (true) or disable (false) the flag'),
  },
  async ({ projectKey, flagKey, environmentKey, enabled }) => {
    const key = resolveProject(projectKey)
    const result = await toggleFlag(client, key, flagKey, environmentKey, enabled)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'update_flag_config',
  'Update per-environment flag config (default variant, targeting rules, rollout percentage)',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    flagKey: z.string().describe('The flag key'),
    environmentKey: z.string().describe('The environment key'),
    default_variant: z.string().optional().describe('Default variant key'),
    targeting_rules: z.array(z.unknown()).optional().describe('Targeting rules'),
    rollout_percentage: z.number().optional().describe('Rollout percentage (0-100)'),
  },
  async ({ projectKey, flagKey, environmentKey, ...updates }) => {
    const key = resolveProject(projectKey)
    const result = await updateFlagConfig(client, key, flagKey, environmentKey, updates)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'list_environments',
  'List environments in a project',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
  },
  async ({ projectKey }) => {
    const key = resolveProject(projectKey)
    const result = await listEnvironments(client, key)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'list_segments',
  'List segments in a project',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
  },
  async ({ projectKey }) => {
    const key = resolveProject(projectKey)
    const result = await listSegments(client, key)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

server.tool(
  'get_segment',
  'Get segment details including conditions',
  {
    projectKey: z.string().optional().describe('Project key (uses default if not set)'),
    segmentKey: z.string().describe('The segment key'),
  },
  async ({ projectKey, segmentKey }) => {
    const key = resolveProject(projectKey)
    const result = await getSegment(client, key, segmentKey)
    return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
  },
)

// --- Start server ---

const transport = new StdioServerTransport()
await server.connect(transport)
```

Note: The `@modelcontextprotocol/sdk` import paths and `zod` usage follow the MCP SDK patterns. Check the actual SDK docs/examples for the latest API surface when implementing. The MCP SDK includes `zod` as a dependency — no separate install needed.

- [ ] **Step 2: Verify build**

Run: `cd mcp && npm run build`
Expected: Builds successfully.

- [ ] **Step 3: Run all MCP tests**

Run: `cd mcp && npm test`
Expected: All tests PASS.

- [ ] **Step 4: Commit**

```bash
git add mcp/src/index.ts
git commit -m "feat: implement MCP server entry point with all 10 tools (#124)"
```

---

## Chunk 5: CI, Release, and Documentation

### Task 16: CI Configuration

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add test-mcp job**

Add a new job in `ci.yml` after the existing `test-sdks` job:

```yaml
  test-mcp:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
      - name: Test MCP server
        run: cd mcp && npm ci && npm test
```

- [ ] **Step 2: Add test-mcp to the build job's needs**

Update the `build` job's `needs` array to include `test-mcp`:

```yaml
  build:
    needs: [test-go, test-sdks, test-dotnet-sdk, test-go-sdk, lint-frontend, build-docs, test-mcp]
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add test-mcp job for MCP server tests (#124)"
```

---

### Task 17: Release Configuration

**Files:**
- Modify: `release-please-config.json`
- Modify: `.release-please-manifest.json`
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Add MCP package to release-please config**

In `release-please-config.json`, add to `packages`:

```json
"mcp": {
  "release-type": "node",
  "component": "mcp",
  "changelog-path": "CHANGELOG.md",
  "changelog-sections": [
    {"type": "feat", "section": "Features"},
    {"type": "fix", "section": "Bug Fixes"},
    {"type": "refactor", "section": "Code Refactoring"}
  ]
}
```

- [ ] **Step 2: Add MCP to manifest**

In `.release-please-manifest.json`, add:

```json
"mcp": "0.1.0"
```

- [ ] **Step 3: Add MCP publish job to release.yml**

Add output to `release-please` job:

```yaml
mcp--release_created: ${{ steps.release.outputs['mcp--release_created'] }}
mcp--tag_name: ${{ steps.release.outputs['mcp--tag_name'] }}
```

Add the MCP package to the `publish-npm-sdks` job condition, or create a separate publish job:

```yaml
  publish-mcp:
    runs-on: ubuntu-latest
    needs: release-please
    if: needs.release-please.outputs['mcp--release_created'] == 'true'
    permissions:
      contents: read
      id-token: write
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "22"
          registry-url: "https://registry.npmjs.org"
      - name: Update npm for OIDC trusted publishing
        run: npm install -g npm@latest
      - name: Publish @togglerino/mcp
        working-directory: mcp
        run: |
          npm ci
          npm run build
          npm publish --access public --provenance
```

- [ ] **Step 4: Commit**

```bash
git add release-please-config.json .release-please-manifest.json .github/workflows/release.yml
git commit -m "ci: add release-please config for @togglerino/mcp (#124)"
```

---

### Task 18: Documentation Updates

**Files:**
- Modify: `docs-site/docs/self-hosting/configuration.md` — add PAT-related info
- Modify: `CLAUDE.md` — add PAT and MCP server info to relevant sections

- [ ] **Step 1: Update configuration docs**

Add a section about Personal Access Tokens to the self-hosting configuration docs. Document:
- How to create tokens via the dashboard
- MCP server configuration example
- Environment variables (`TOGGLERINO_URL`, `TOGGLERINO_API_KEY`, `TOGGLERINO_PROJECT`)

- [ ] **Step 2: Update CLAUDE.md**

Add to the API Routes section:
```
- `POST /api/v1/auth/tokens` — create personal access token (session-authed)
- `GET /api/v1/auth/tokens` — list current user's tokens (session-authed)
- `DELETE /api/v1/auth/tokens/{id}` — revoke token (session-authed)
```

Add to the Key Patterns section:
```
- **Personal Access Tokens**: PATs provide programmatic access to the management API. Token format `pat_<40 hex>`, stored as SHA-256 hash. `SessionOrPATAuth` middleware accepts either session cookie or PAT Bearer token. PATs inherit the user's full permissions (RBAC, project roles, environment locks).
```

Add to the Architecture section a note about the MCP server in `mcp/`.

- [ ] **Step 3: Commit**

```bash
git add docs-site/docs/self-hosting/configuration.md CLAUDE.md
git commit -m "docs: add PAT and MCP server documentation (#124)"
```

---

### Task 19: Final Verification

- [ ] **Step 1: Run full Go test suite**

Run: `go test ./...`
Expected: All tests PASS.

- [ ] **Step 2: Run MCP tests**

Run: `cd mcp && npm test`
Expected: All tests PASS.

- [ ] **Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors.

- [ ] **Step 4: Build Go binary**

Run: `cd web && npm ci && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: Builds successfully.

- [ ] **Step 5: Build MCP server**

Run: `cd mcp && npm run build`
Expected: Builds successfully.
