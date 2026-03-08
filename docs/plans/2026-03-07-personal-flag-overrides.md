# Personal Flag Overrides Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Allow dashboard users to set personal flag value overrides that take effect during SDK evaluation, scoped per-flag, per-environment, with automatic expiry.

**Architecture:** Two new tables (`user_app_identities`, `flag_overrides`), two new stores, one new handler, override cache layer in the evaluation cache, override check injected into the evaluate handler (before engine evaluation), and a periodic cleanup goroutine. Frontend adds override controls to the flag detail page and a "My Overrides" page.

**Tech Stack:** Go (stdlib net/http, pgx/v5, slog), React 19 + TypeScript + TanStack Query + shadcn/ui + Tailwind CSS v4

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/021_personal_overrides.up.sql`
- Create: `migrations/021_personal_overrides.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/021_personal_overrides.up.sql

-- App identity: maps dashboard user to their application user ID per project
CREATE TABLE user_app_identities (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    app_user_id   TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, project_id),
    UNIQUE (project_id, app_user_id)
);

CREATE INDEX idx_user_app_identities_project_app_user
    ON user_app_identities(project_id, app_user_id);

-- Personal flag overrides
CREATE TABLE flag_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    flag_id         UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    environment_id  UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    value           JSONB NOT NULL,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, flag_id, environment_id)
);

CREATE INDEX idx_flag_overrides_expires_at
    ON flag_overrides(expires_at) WHERE expires_at IS NOT NULL;
```

**Step 2: Write the down migration**

```sql
-- migrations/021_personal_overrides.down.sql
DROP TABLE IF EXISTS flag_overrides;
DROP TABLE IF EXISTS user_app_identities;
```

**Step 3: Verify migration applies**

Run: `./dev.sh` (rebuilds Go binary + runs migrations)
Expected: Server starts without migration errors, new tables visible in psql.

**Step 4: Commit**

```bash
git add migrations/021_personal_overrides.up.sql migrations/021_personal_overrides.down.sql
git commit -m "feat: add migration for personal flag overrides (#48)"
```

---

### Task 2: Models

**Files:**
- Create: `internal/model/override.go`

**Step 1: Define the model types**

```go
// internal/model/override.go
package model

import (
    "encoding/json"
    "time"
)

// AppIdentity maps a dashboard user to their application user ID within a project.
type AppIdentity struct {
    UserID    string    `json:"user_id"`
    ProjectID string    `json:"project_id"`
    AppUserID string    `json:"app_user_id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// FlagOverride is a personal flag value override set by a dashboard user.
type FlagOverride struct {
    ID            string          `json:"id"`
    UserID        string          `json:"user_id"`
    FlagID        string          `json:"flag_id"`
    FlagKey       string          `json:"flag_key,omitempty"`
    EnvironmentID string          `json:"environment_id"`
    EnvironmentKey string         `json:"environment_key,omitempty"`
    ProjectKey    string          `json:"project_key,omitempty"`
    Value         json.RawMessage `json:"value"`
    ExpiresAt     *time.Time      `json:"expires_at"`
    CreatedAt     time.Time       `json:"created_at"`
}
```

**Step 2: Commit**

```bash
git add internal/model/override.go
git commit -m "feat: add AppIdentity and FlagOverride models (#48)"
```

---

### Task 3: App Identity Store

**Files:**
- Create: `internal/store/app_identity_store.go`
- Create: `internal/store/app_identity_store_test.go`

**Step 1: Write the failing tests**

```go
// internal/store/app_identity_store_test.go
package store_test

import (
    "context"
    "testing"

    "github.com/togglerino/togglerino/internal/store"
)

func TestAppIdentityStore_SetAndGet(t *testing.T) {
    pool := testPool(t)
    s := store.NewAppIdentityStore(pool)
    ctx := context.Background()

    userID, projectID := createTestUser(t, pool), createTestProject(t, pool)

    // Set identity
    identity, err := s.Set(ctx, userID, projectID, "app-user-42")
    if err != nil {
        t.Fatalf("Set: %v", err)
    }
    if identity.AppUserID != "app-user-42" {
        t.Fatalf("expected app-user-42, got %s", identity.AppUserID)
    }

    // Get identity
    got, err := s.Get(ctx, userID, projectID)
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if got.AppUserID != "app-user-42" {
        t.Fatalf("expected app-user-42, got %s", got.AppUserID)
    }

    // Update (upsert)
    updated, err := s.Set(ctx, userID, projectID, "app-user-99")
    if err != nil {
        t.Fatalf("Set update: %v", err)
    }
    if updated.AppUserID != "app-user-99" {
        t.Fatalf("expected app-user-99, got %s", updated.AppUserID)
    }
}

func TestAppIdentityStore_Delete(t *testing.T) {
    pool := testPool(t)
    s := store.NewAppIdentityStore(pool)
    ctx := context.Background()

    userID, projectID := createTestUser(t, pool), createTestProject(t, pool)

    _, err := s.Set(ctx, userID, projectID, "app-user-42")
    if err != nil {
        t.Fatalf("Set: %v", err)
    }

    err = s.Delete(ctx, userID, projectID)
    if err != nil {
        t.Fatalf("Delete: %v", err)
    }

    _, err = s.Get(ctx, userID, projectID)
    if err == nil {
        t.Fatal("expected error after delete, got nil")
    }
}

func TestAppIdentityStore_UniqueAppUserPerProject(t *testing.T) {
    pool := testPool(t)
    s := store.NewAppIdentityStore(pool)
    ctx := context.Background()

    projectID := createTestProject(t, pool)
    user1 := createTestUser(t, pool)
    user2 := createTestUserWithEmail(t, pool, "user2@test.com")

    _, err := s.Set(ctx, user1, projectID, "same-app-user")
    if err != nil {
        t.Fatalf("Set user1: %v", err)
    }

    _, err = s.Set(ctx, user2, projectID, "same-app-user")
    if err == nil {
        t.Fatal("expected uniqueness error, got nil")
    }
}
```

Note: The test helper functions `createTestUser`, `createTestProject`, and `createTestUserWithEmail` may need to be created or adapted from existing test helpers. Check `internal/store/` for existing `_test.go` files to find the test helper patterns (e.g., `testPool(t)`, user/project creation helpers).

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestAppIdentityStore -v`
Expected: FAIL — `NewAppIdentityStore` not defined

**Step 3: Implement the store**

```go
// internal/store/app_identity_store.go
package store

import (
    "context"
    "fmt"
    "strings"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/togglerino/togglerino/internal/model"
)

type AppIdentityStore struct {
    pool *pgxpool.Pool
}

func NewAppIdentityStore(pool *pgxpool.Pool) *AppIdentityStore {
    return &AppIdentityStore{pool: pool}
}

func (s *AppIdentityStore) Set(ctx context.Context, userID, projectID, appUserID string) (*model.AppIdentity, error) {
    var identity model.AppIdentity
    err := s.pool.QueryRow(ctx,
        `INSERT INTO user_app_identities (user_id, project_id, app_user_id)
         VALUES ($1, $2, $3)
         ON CONFLICT (user_id, project_id)
         DO UPDATE SET app_user_id = EXCLUDED.app_user_id, updated_at = NOW()
         RETURNING user_id, project_id, app_user_id, created_at, updated_at`,
        userID, projectID, appUserID,
    ).Scan(&identity.UserID, &identity.ProjectID, &identity.AppUserID, &identity.CreatedAt, &identity.UpdatedAt)
    if err != nil {
        if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate key") {
            return nil, fmt.Errorf("app user ID already claimed by another user")
        }
        return nil, fmt.Errorf("setting app identity: %w", err)
    }
    return &identity, nil
}

func (s *AppIdentityStore) Get(ctx context.Context, userID, projectID string) (*model.AppIdentity, error) {
    var identity model.AppIdentity
    err := s.pool.QueryRow(ctx,
        `SELECT user_id, project_id, app_user_id, created_at, updated_at
         FROM user_app_identities
         WHERE user_id = $1 AND project_id = $2`,
        userID, projectID,
    ).Scan(&identity.UserID, &identity.ProjectID, &identity.AppUserID, &identity.CreatedAt, &identity.UpdatedAt)
    if err != nil {
        return nil, fmt.Errorf("getting app identity: %w", err)
    }
    return &identity, nil
}

func (s *AppIdentityStore) GetByProjectKey(ctx context.Context, userID, projectKey string) (*model.AppIdentity, error) {
    var identity model.AppIdentity
    err := s.pool.QueryRow(ctx,
        `SELECT uai.user_id, uai.project_id, uai.app_user_id, uai.created_at, uai.updated_at
         FROM user_app_identities uai
         JOIN projects p ON p.id = uai.project_id
         WHERE uai.user_id = $1 AND p.key = $2`,
        userID, projectKey,
    ).Scan(&identity.UserID, &identity.ProjectID, &identity.AppUserID, &identity.CreatedAt, &identity.UpdatedAt)
    if err != nil {
        return nil, fmt.Errorf("getting app identity by project key: %w", err)
    }
    return &identity, nil
}

func (s *AppIdentityStore) Delete(ctx context.Context, userID, projectID string) error {
    _, err := s.pool.Exec(ctx,
        `DELETE FROM user_app_identities WHERE user_id = $1 AND project_id = $2`,
        userID, projectID,
    )
    if err != nil {
        return fmt.Errorf("deleting app identity: %w", err)
    }
    return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestAppIdentityStore -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/app_identity_store.go internal/store/app_identity_store_test.go
git commit -m "feat: add AppIdentityStore (#48)"
```

---

### Task 4: Override Store

**Files:**
- Create: `internal/store/override_store.go`
- Create: `internal/store/override_store_test.go`

**Step 1: Write the failing tests**

```go
// internal/store/override_store_test.go
package store_test

import (
    "context"
    "encoding/json"
    "testing"
    "time"

    "github.com/togglerino/togglerino/internal/store"
)

func TestOverrideStore_SetAndGet(t *testing.T) {
    pool := testPool(t)
    s := store.NewOverrideStore(pool)
    ctx := context.Background()

    userID, projectID := createTestUser(t, pool), createTestProject(t, pool)
    flagID := createTestFlag(t, pool, projectID)
    envID := getTestEnvironmentID(t, pool, projectID)

    value := json.RawMessage(`true`)
    expiresAt := time.Now().Add(24 * time.Hour)

    override, err := s.Set(ctx, userID, flagID, envID, value, &expiresAt)
    if err != nil {
        t.Fatalf("Set: %v", err)
    }
    if override.FlagID != flagID {
        t.Fatalf("expected flag ID %s, got %s", flagID, override.FlagID)
    }

    got, err := s.Get(ctx, userID, flagID, envID)
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if string(got.Value) != `true` {
        t.Fatalf("expected true, got %s", got.Value)
    }
}

func TestOverrideStore_Delete(t *testing.T) {
    pool := testPool(t)
    s := store.NewOverrideStore(pool)
    ctx := context.Background()

    userID, projectID := createTestUser(t, pool), createTestProject(t, pool)
    flagID := createTestFlag(t, pool, projectID)
    envID := getTestEnvironmentID(t, pool, projectID)

    value := json.RawMessage(`true`)
    _, err := s.Set(ctx, userID, flagID, envID, value, nil)
    if err != nil {
        t.Fatalf("Set: %v", err)
    }

    err = s.Delete(ctx, userID, flagID, envID)
    if err != nil {
        t.Fatalf("Delete: %v", err)
    }

    _, err = s.Get(ctx, userID, flagID, envID)
    if err == nil {
        t.Fatal("expected error after delete, got nil")
    }
}

func TestOverrideStore_ListByUser(t *testing.T) {
    pool := testPool(t)
    s := store.NewOverrideStore(pool)
    ctx := context.Background()

    userID, projectID := createTestUser(t, pool), createTestProject(t, pool)
    flagID := createTestFlag(t, pool, projectID)
    envID := getTestEnvironmentID(t, pool, projectID)

    value := json.RawMessage(`"on"`)
    _, err := s.Set(ctx, userID, flagID, envID, value, nil)
    if err != nil {
        t.Fatalf("Set: %v", err)
    }

    overrides, err := s.ListByUser(ctx, userID)
    if err != nil {
        t.Fatalf("ListByUser: %v", err)
    }
    if len(overrides) != 1 {
        t.Fatalf("expected 1 override, got %d", len(overrides))
    }
}

func TestOverrideStore_DeleteExpired(t *testing.T) {
    pool := testPool(t)
    s := store.NewOverrideStore(pool)
    ctx := context.Background()

    userID, projectID := createTestUser(t, pool), createTestProject(t, pool)
    flagID := createTestFlag(t, pool, projectID)
    envID := getTestEnvironmentID(t, pool, projectID)

    // Create an already-expired override
    pastTime := time.Now().Add(-1 * time.Hour)
    value := json.RawMessage(`true`)
    _, err := s.Set(ctx, userID, flagID, envID, value, &pastTime)
    if err != nil {
        t.Fatalf("Set: %v", err)
    }

    count, err := s.DeleteExpired(ctx)
    if err != nil {
        t.Fatalf("DeleteExpired: %v", err)
    }
    if count != 1 {
        t.Fatalf("expected 1 deleted, got %d", count)
    }
}
```

Note: `createTestFlag` and `getTestEnvironmentID` helpers may need to be created. Check existing test files for similar patterns. `createTestProject` already creates default environments (development, staging, production).

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -run TestOverrideStore -v`
Expected: FAIL — `NewOverrideStore` not defined

**Step 3: Implement the store**

```go
// internal/store/override_store.go
package store

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/togglerino/togglerino/internal/model"
)

type OverrideStore struct {
    pool *pgxpool.Pool
}

func NewOverrideStore(pool *pgxpool.Pool) *OverrideStore {
    return &OverrideStore{pool: pool}
}

func (s *OverrideStore) Set(ctx context.Context, userID, flagID, environmentID string, value json.RawMessage, expiresAt *time.Time) (*model.FlagOverride, error) {
    var o model.FlagOverride
    err := s.pool.QueryRow(ctx,
        `INSERT INTO flag_overrides (user_id, flag_id, environment_id, value, expires_at)
         VALUES ($1, $2, $3, $4, $5)
         ON CONFLICT (user_id, flag_id, environment_id)
         DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at
         RETURNING id, user_id, flag_id, environment_id, value, expires_at, created_at`,
        userID, flagID, environmentID, value, expiresAt,
    ).Scan(&o.ID, &o.UserID, &o.FlagID, &o.EnvironmentID, &o.Value, &o.ExpiresAt, &o.CreatedAt)
    if err != nil {
        return nil, fmt.Errorf("setting override: %w", err)
    }
    return &o, nil
}

func (s *OverrideStore) Get(ctx context.Context, userID, flagID, environmentID string) (*model.FlagOverride, error) {
    var o model.FlagOverride
    err := s.pool.QueryRow(ctx,
        `SELECT id, user_id, flag_id, environment_id, value, expires_at, created_at
         FROM flag_overrides
         WHERE user_id = $1 AND flag_id = $2 AND environment_id = $3`,
        userID, flagID, environmentID,
    ).Scan(&o.ID, &o.UserID, &o.FlagID, &o.EnvironmentID, &o.Value, &o.ExpiresAt, &o.CreatedAt)
    if err != nil {
        return nil, fmt.Errorf("getting override: %w", err)
    }
    return &o, nil
}

func (s *OverrideStore) Delete(ctx context.Context, userID, flagID, environmentID string) error {
    _, err := s.pool.Exec(ctx,
        `DELETE FROM flag_overrides WHERE user_id = $1 AND flag_id = $2 AND environment_id = $3`,
        userID, flagID, environmentID,
    )
    if err != nil {
        return fmt.Errorf("deleting override: %w", err)
    }
    return nil
}

func (s *OverrideStore) DeleteByUserAndFlag(ctx context.Context, userID, flagID string) error {
    _, err := s.pool.Exec(ctx,
        `DELETE FROM flag_overrides WHERE user_id = $1 AND flag_id = $2`,
        userID, flagID,
    )
    if err != nil {
        return fmt.Errorf("deleting overrides for flag: %w", err)
    }
    return nil
}

func (s *OverrideStore) ListByUser(ctx context.Context, userID string) ([]model.FlagOverride, error) {
    rows, err := s.pool.Query(ctx,
        `SELECT fo.id, fo.user_id, fo.flag_id, f.key, fo.environment_id, e.key, p.key, fo.value, fo.expires_at, fo.created_at
         FROM flag_overrides fo
         JOIN flags f ON f.id = fo.flag_id
         JOIN environments e ON e.id = fo.environment_id
         JOIN projects p ON p.id = f.project_id
         WHERE fo.user_id = $1 AND (fo.expires_at IS NULL OR fo.expires_at > NOW())
         ORDER BY fo.created_at DESC`,
        userID,
    )
    if err != nil {
        return nil, fmt.Errorf("listing overrides: %w", err)
    }
    defer rows.Close()

    var overrides []model.FlagOverride
    for rows.Next() {
        var o model.FlagOverride
        if err := rows.Scan(&o.ID, &o.UserID, &o.FlagID, &o.FlagKey, &o.EnvironmentID, &o.EnvironmentKey, &o.ProjectKey, &o.Value, &o.ExpiresAt, &o.CreatedAt); err != nil {
            return nil, fmt.Errorf("scanning override: %w", err)
        }
        overrides = append(overrides, o)
    }
    if overrides == nil {
        overrides = []model.FlagOverride{}
    }
    return overrides, rows.Err()
}

// ListByProjectEnv returns all non-expired overrides for a project+environment,
// keyed by app_user_id. Used for cache loading.
func (s *OverrideStore) ListByProjectEnv(ctx context.Context, projectKey, envKey string) ([]model.FlagOverride, error) {
    rows, err := s.pool.Query(ctx,
        `SELECT fo.id, fo.user_id, fo.flag_id, f.key, fo.environment_id, fo.value, fo.expires_at, fo.created_at
         FROM flag_overrides fo
         JOIN flags f ON f.id = fo.flag_id
         JOIN environments e ON e.id = fo.environment_id
         JOIN projects p ON p.id = f.project_id
         WHERE p.key = $1 AND e.key = $2
           AND (fo.expires_at IS NULL OR fo.expires_at > NOW())`,
        projectKey, envKey,
    )
    if err != nil {
        return nil, fmt.Errorf("listing overrides by project env: %w", err)
    }
    defer rows.Close()

    var overrides []model.FlagOverride
    for rows.Next() {
        var o model.FlagOverride
        if err := rows.Scan(&o.ID, &o.UserID, &o.FlagID, &o.FlagKey, &o.EnvironmentID, &o.Value, &o.ExpiresAt, &o.CreatedAt); err != nil {
            return nil, fmt.Errorf("scanning override: %w", err)
        }
        overrides = append(overrides, o)
    }
    if overrides == nil {
        overrides = []model.FlagOverride{}
    }
    return overrides, rows.Err()
}

// ListAllOverrides loads all non-expired overrides with app_user_id resolved.
// Used for cache LoadAll at startup.
func (s *OverrideStore) ListAllOverrides(ctx context.Context) ([]OverrideCacheEntry, error) {
    rows, err := s.pool.Query(ctx,
        `SELECT p.key, e.key, f.key, uai.app_user_id, fo.value, fo.expires_at
         FROM flag_overrides fo
         JOIN flags f ON f.id = fo.flag_id
         JOIN environments e ON e.id = fo.environment_id
         JOIN projects p ON p.id = f.project_id
         JOIN user_app_identities uai ON uai.user_id = fo.user_id AND uai.project_id = f.project_id
         WHERE fo.expires_at IS NULL OR fo.expires_at > NOW()`)
    if err != nil {
        return nil, fmt.Errorf("listing all overrides: %w", err)
    }
    defer rows.Close()

    var entries []OverrideCacheEntry
    for rows.Next() {
        var e OverrideCacheEntry
        if err := rows.Scan(&e.ProjectKey, &e.EnvironmentKey, &e.FlagKey, &e.AppUserID, &e.Value, &e.ExpiresAt); err != nil {
            return nil, fmt.Errorf("scanning override cache entry: %w", err)
        }
        entries = append(entries, e)
    }
    return entries, rows.Err()
}

func (s *OverrideStore) DeleteExpired(ctx context.Context) (int64, error) {
    tag, err := s.pool.Exec(ctx,
        `DELETE FROM flag_overrides WHERE expires_at IS NOT NULL AND expires_at <= NOW()`)
    if err != nil {
        return 0, fmt.Errorf("deleting expired overrides: %w", err)
    }
    return tag.RowsAffected(), nil
}

func (s *OverrideStore) DeleteAllByUser(ctx context.Context, userID string) error {
    _, err := s.pool.Exec(ctx, `DELETE FROM flag_overrides WHERE user_id = $1`, userID)
    if err != nil {
        return fmt.Errorf("deleting all overrides for user: %w", err)
    }
    return nil
}

// OverrideCacheEntry is used for bulk-loading overrides into the cache.
type OverrideCacheEntry struct {
    ProjectKey     string
    EnvironmentKey string
    FlagKey        string
    AppUserID      string
    Value          json.RawMessage
    ExpiresAt      *time.Time
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestOverrideStore -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/override_store.go internal/store/override_store_test.go
git commit -m "feat: add OverrideStore (#48)"
```

---

### Task 5: Override Cache

**Files:**
- Modify: `internal/evaluation/cache.go`
- Create: `internal/evaluation/cache_test.go` (or add to existing)

**Step 1: Write the failing tests**

```go
// Test that override cache stores and retrieves overrides correctly
func TestCache_Overrides(t *testing.T) {
    c := evaluation.NewCache()

    // Set an override
    c.SetOverride("myproject", "production", "app-user-1", "feature-x", json.RawMessage(`true`), nil)

    // Get override — should find it
    val, ok := c.GetOverride("myproject", "production", "app-user-1", "feature-x")
    if !ok {
        t.Fatal("expected to find override")
    }
    if string(val) != "true" {
        t.Fatalf("expected true, got %s", string(val))
    }

    // Get override for different user — should not find it
    _, ok = c.GetOverride("myproject", "production", "other-user", "feature-x")
    if ok {
        t.Fatal("expected no override for other user")
    }

    // Delete override
    c.DeleteOverride("myproject", "production", "app-user-1", "feature-x")
    _, ok = c.GetOverride("myproject", "production", "app-user-1", "feature-x")
    if ok {
        t.Fatal("expected override to be deleted")
    }
}

func TestCache_OverrideExpiry(t *testing.T) {
    c := evaluation.NewCache()

    past := time.Now().Add(-1 * time.Hour)
    c.SetOverride("proj", "dev", "user-1", "flag-a", json.RawMessage(`true`), &past)

    // Expired override should not be returned
    _, ok := c.GetOverride("proj", "dev", "user-1", "flag-a")
    if ok {
        t.Fatal("expected expired override to not be found")
    }
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/evaluation/ -run TestCache_Override -v`
Expected: FAIL — methods not defined

**Step 3: Add override support to Cache**

Add these fields and methods to `internal/evaluation/cache.go`:

Add a new field to the `Cache` struct:
```go
// Key: "projectKey:envKey", Value: map of "appUserID:flagKey" -> OverrideEntry
overrides map[string]map[string]OverrideEntry
```

Add the `OverrideEntry` type:
```go
type OverrideEntry struct {
    Value     json.RawMessage
    ExpiresAt *time.Time
}
```

Initialize in `NewCache()`:
```go
overrides: make(map[string]map[string]OverrideEntry),
```

Add override key helper:
```go
func overrideKey(appUserID, flagKey string) string {
    return appUserID + ":" + flagKey
}
```

Add methods:
```go
func (c *Cache) SetOverride(projectKey, envKey, appUserID, flagKey string, value json.RawMessage, expiresAt *time.Time) {
    key := cacheKey(projectKey, envKey)
    oKey := overrideKey(appUserID, flagKey)
    c.mu.Lock()
    if c.overrides[key] == nil {
        c.overrides[key] = make(map[string]OverrideEntry)
    }
    c.overrides[key][oKey] = OverrideEntry{Value: value, ExpiresAt: expiresAt}
    c.mu.Unlock()
}

func (c *Cache) GetOverride(projectKey, envKey, appUserID, flagKey string) (json.RawMessage, bool) {
    key := cacheKey(projectKey, envKey)
    oKey := overrideKey(appUserID, flagKey)
    c.mu.RLock()
    defer c.mu.RUnlock()
    overrides := c.overrides[key]
    if overrides == nil {
        return nil, false
    }
    entry, ok := overrides[oKey]
    if !ok {
        return nil, false
    }
    // Check expiry
    if entry.ExpiresAt != nil && entry.ExpiresAt.Before(time.Now()) {
        return nil, false
    }
    return entry.Value, true
}

func (c *Cache) DeleteOverride(projectKey, envKey, appUserID, flagKey string) {
    key := cacheKey(projectKey, envKey)
    oKey := overrideKey(appUserID, flagKey)
    c.mu.Lock()
    if c.overrides[key] != nil {
        delete(c.overrides[key], oKey)
    }
    c.mu.Unlock()
}

func (c *Cache) DeleteOverridesForUser(projectKey, envKey, appUserID string) {
    key := cacheKey(projectKey, envKey)
    prefix := appUserID + ":"
    c.mu.Lock()
    if c.overrides[key] != nil {
        for k := range c.overrides[key] {
            if strings.HasPrefix(k, prefix) {
                delete(c.overrides[key], k)
            }
        }
    }
    c.mu.Unlock()
}
```

Also add override loading in `LoadAll()` — after loading segments, add a comment placeholder:
```go
// Overrides are loaded separately via LoadOverrides() after LoadAll()
```

Add a separate `LoadOverrides` method that accepts `[]store.OverrideCacheEntry`:
```go
func (c *Cache) LoadOverrides(entries []OverrideCacheEntryData) {
    newOverrides := make(map[string]map[string]OverrideEntry)
    for _, e := range entries {
        key := cacheKey(e.ProjectKey, e.EnvironmentKey)
        oKey := overrideKey(e.AppUserID, e.FlagKey)
        if newOverrides[key] == nil {
            newOverrides[key] = make(map[string]OverrideEntry)
        }
        newOverrides[key][oKey] = OverrideEntry{Value: e.Value, ExpiresAt: e.ExpiresAt}
    }
    c.mu.Lock()
    c.overrides = newOverrides
    c.mu.Unlock()
}

// OverrideCacheEntryData is the data needed to populate the override cache.
type OverrideCacheEntryData struct {
    ProjectKey     string
    EnvironmentKey string
    FlagKey        string
    AppUserID      string
    Value          json.RawMessage
    ExpiresAt      *time.Time
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/evaluation/ -run TestCache_Override -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/evaluation/cache.go internal/evaluation/cache_test.go
git commit -m "feat: add override support to evaluation cache (#48)"
```

---

### Task 6: Override Handler (Backend API)

**Files:**
- Create: `internal/handler/override_handler.go`
- Create: `internal/handler/override_handler_test.go`

**Step 1: Write the failing tests**

Write HTTP handler tests following the pattern in other `*_handler_test.go` files. Key test cases:

- `TestOverrideHandler_SetAppIdentity` — PUT app identity, verify 200
- `TestOverrideHandler_GetAppIdentity` — GET app identity after setting, verify returned
- `TestOverrideHandler_SetOverride` — PUT override for a flag+env, verify 200
- `TestOverrideHandler_SetOverride_NoIdentity` — PUT override without app identity, verify 400
- `TestOverrideHandler_DeleteOverride` — DELETE override, verify 204
- `TestOverrideHandler_SetOverrideAllEnvs` — PUT override for all envs, verify creates per-env
- `TestOverrideHandler_ListMyOverrides` — GET /overrides/me, verify list returned

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestOverrideHandler -v`
Expected: FAIL

**Step 3: Implement the handler**

```go
// internal/handler/override_handler.go
package handler

import (
    "encoding/json"
    "log/slog"
    "net/http"
    "time"

    "github.com/togglerino/togglerino/internal/auth"
    "github.com/togglerino/togglerino/internal/evaluation"
    "github.com/togglerino/togglerino/internal/store"
)

type OverrideHandler struct {
    overrides    *store.OverrideStore
    identities   *store.AppIdentityStore
    projects     *store.ProjectStore
    flags        *store.FlagStore
    environments *store.EnvironmentStore
    cache        *evaluation.Cache
}

func NewOverrideHandler(
    overrides *store.OverrideStore,
    identities *store.AppIdentityStore,
    projects *store.ProjectStore,
    flags *store.FlagStore,
    environments *store.EnvironmentStore,
    cache *evaluation.Cache,
) *OverrideHandler {
    return &OverrideHandler{
        overrides:    overrides,
        identities:   identities,
        projects:     projects,
        flags:        flags,
        environments: environments,
        cache:        cache,
    }
}

// SetAppIdentity handles PUT /api/v1/projects/{key}/app-identity
func (h *OverrideHandler) SetAppIdentity(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    var req struct {
        AppUserID string `json:"app_user_id"`
    }
    if err := readJSON(r, &req); err != nil || req.AppUserID == "" {
        writeError(w, http.StatusBadRequest, "app_user_id is required")
        return
    }

    identity, err := h.identities.Set(r.Context(), user.ID, project.ID, req.AppUserID)
    if err != nil {
        if isUniqueViolation(err) {
            writeError(w, http.StatusConflict, "app user ID already claimed by another user")
            return
        }
        slog.Error("setting app identity", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to set app identity")
        return
    }

    writeJSON(w, http.StatusOK, identity)
}

// GetAppIdentity handles GET /api/v1/projects/{key}/app-identity
func (h *OverrideHandler) GetAppIdentity(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "app identity not configured")
        return
    }

    writeJSON(w, http.StatusOK, identity)
}

// DeleteAppIdentity handles DELETE /api/v1/projects/{key}/app-identity
func (h *OverrideHandler) DeleteAppIdentity(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    if err := h.identities.Delete(r.Context(), user.ID, project.ID); err != nil {
        slog.Error("deleting app identity", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to delete app identity")
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

var durationMap = map[string]time.Duration{
    "1h":  1 * time.Hour,
    "8h":  8 * time.Hour,
    "24h": 24 * time.Hour,
    "7d":  7 * 24 * time.Hour,
}

// SetOverride handles PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/override
func (h *OverrideHandler) SetOverride(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    flagKey := r.PathValue("flag")
    envKey := r.PathValue("env")

    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    // Check app identity exists
    identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey)
    if err != nil {
        writeError(w, http.StatusBadRequest, "app identity not configured for this project")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "flag not found")
        return
    }

    env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "environment not found")
        return
    }

    var req struct {
        Value    json.RawMessage `json:"value"`
        Duration *string         `json:"duration"`
    }
    if err := readJSON(r, &req); err != nil || req.Value == nil {
        writeError(w, http.StatusBadRequest, "value is required")
        return
    }

    // Calculate expiry
    var expiresAt *time.Time
    if req.Duration == nil {
        // Default: 24h
        t := time.Now().Add(24 * time.Hour)
        expiresAt = &t
    } else if *req.Duration != "" {
        dur, ok := durationMap[*req.Duration]
        if !ok {
            writeError(w, http.StatusBadRequest, "invalid duration, use: 1h, 8h, 24h, 7d, or null")
            return
        }
        t := time.Now().Add(dur)
        expiresAt = &t
    }
    // If duration is explicitly "" or null JSON, expiresAt stays nil (no expiry)

    override, err := h.overrides.Set(r.Context(), user.ID, flag.ID, env.ID, req.Value, expiresAt)
    if err != nil {
        slog.Error("setting override", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to set override")
        return
    }

    // Update cache
    h.cache.SetOverride(projectKey, envKey, identity.AppUserID, flagKey, req.Value, expiresAt)

    writeJSON(w, http.StatusOK, override)
}

// DeleteOverride handles DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/override
func (h *OverrideHandler) DeleteOverride(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    flagKey := r.PathValue("flag")
    envKey := r.PathValue("env")

    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "flag not found")
        return
    }

    env, err := h.environments.FindByKey(r.Context(), project.ID, envKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "environment not found")
        return
    }

    if err := h.overrides.Delete(r.Context(), user.ID, flag.ID, env.ID); err != nil {
        slog.Error("deleting override", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to delete override")
        return
    }

    // Update cache
    if identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey); err == nil {
        h.cache.DeleteOverride(projectKey, envKey, identity.AppUserID, flagKey)
    }

    w.WriteHeader(http.StatusNoContent)
}

// SetOverrideAllEnvs handles PUT /api/v1/projects/{key}/flags/{flag}/override
func (h *OverrideHandler) SetOverrideAllEnvs(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    flagKey := r.PathValue("flag")

    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey)
    if err != nil {
        writeError(w, http.StatusBadRequest, "app identity not configured for this project")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "flag not found")
        return
    }

    envs, err := h.environments.ListByProjectID(r.Context(), project.ID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to list environments")
        return
    }

    var req struct {
        Value    json.RawMessage `json:"value"`
        Duration *string         `json:"duration"`
    }
    if err := readJSON(r, &req); err != nil || req.Value == nil {
        writeError(w, http.StatusBadRequest, "value is required")
        return
    }

    var expiresAt *time.Time
    if req.Duration == nil {
        t := time.Now().Add(24 * time.Hour)
        expiresAt = &t
    } else if *req.Duration != "" {
        dur, ok := durationMap[*req.Duration]
        if !ok {
            writeError(w, http.StatusBadRequest, "invalid duration")
            return
        }
        t := time.Now().Add(dur)
        expiresAt = &t
    }

    for _, env := range envs {
        if _, err := h.overrides.Set(r.Context(), user.ID, flag.ID, env.ID, req.Value, expiresAt); err != nil {
            slog.Error("setting override for env", "env", env.Key, "error", err)
            continue
        }
        h.cache.SetOverride(projectKey, env.Key, identity.AppUserID, flagKey, req.Value, expiresAt)
    }

    w.WriteHeader(http.StatusNoContent)
}

// DeleteOverrideAllEnvs handles DELETE /api/v1/projects/{key}/flags/{flag}/override
func (h *OverrideHandler) DeleteOverrideAllEnvs(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    flagKey := r.PathValue("flag")

    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "flag not found")
        return
    }

    if err := h.overrides.DeleteByUserAndFlag(r.Context(), user.ID, flag.ID); err != nil {
        slog.Error("deleting all overrides for flag", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to delete overrides")
        return
    }

    // Clear from cache for all environments
    if identity, err := h.identities.GetByProjectKey(r.Context(), user.ID, projectKey); err == nil {
        envs, _ := h.environments.ListByProjectID(r.Context(), project.ID)
        for _, env := range envs {
            h.cache.DeleteOverride(projectKey, env.Key, identity.AppUserID, flagKey)
        }
    }

    w.WriteHeader(http.StatusNoContent)
}

// ListMyOverrides handles GET /api/v1/overrides/me
func (h *OverrideHandler) ListMyOverrides(w http.ResponseWriter, r *http.Request) {
    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    overrides, err := h.overrides.ListByUser(r.Context(), user.ID)
    if err != nil {
        slog.Error("listing overrides", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to list overrides")
        return
    }

    writeJSON(w, http.StatusOK, overrides)
}

// GetFlagOverrides handles GET /api/v1/projects/{key}/flags/{flag}/overrides/me
// Returns the user's overrides for a specific flag across all environments.
func (h *OverrideHandler) GetFlagOverrides(w http.ResponseWriter, r *http.Request) {
    projectKey := r.PathValue("key")
    flagKey := r.PathValue("flag")

    user := auth.UserFromContext(r.Context())
    if user == nil {
        writeError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    project, err := h.projects.FindByKey(r.Context(), projectKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "project not found")
        return
    }

    flag, err := h.flags.FindByKey(r.Context(), project.ID, flagKey)
    if err != nil {
        writeError(w, http.StatusNotFound, "flag not found")
        return
    }

    // Get all overrides for this user, then filter by flag
    allOverrides, err := h.overrides.ListByUser(r.Context(), user.ID)
    if err != nil {
        slog.Error("listing overrides", "error", err)
        writeError(w, http.StatusInternalServerError, "failed to list overrides")
        return
    }

    var flagOverrides []model.FlagOverride
    for _, o := range allOverrides {
        if o.FlagID == flag.ID {
            flagOverrides = append(flagOverrides, o)
        }
    }
    if flagOverrides == nil {
        flagOverrides = []model.FlagOverride{}
    }

    writeJSON(w, http.StatusOK, flagOverrides)
}

func isUniqueViolation(err error) bool {
    s := err.Error()
    return strings.Contains(s, "unique") || strings.Contains(s, "duplicate key")
}
```

Note: Add `"strings"` to the import block. Also add missing `"github.com/togglerino/togglerino/internal/model"` import for `GetFlagOverrides`.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestOverrideHandler -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/handler/override_handler.go internal/handler/override_handler_test.go
git commit -m "feat: add override handler with API endpoints (#48)"
```

---

### Task 7: Integrate Overrides into Evaluation

**Files:**
- Modify: `internal/handler/evaluate_handler.go`
- Create or modify: `internal/handler/evaluate_handler_test.go`

**Step 1: Write the failing test**

```go
func TestEvaluateHandler_PersonalOverride(t *testing.T) {
    // Set up cache with a flag that is enabled with default variant "off"
    cache := evaluation.NewCache()
    cache.SetFlag("proj", "dev", "my-flag", evaluation.FlagData{
        Flag: model.Flag{
            Key:             "my-flag",
            LifecycleStatus: model.LifecycleActive,
            DefaultValue:    json.RawMessage(`false`),
        },
        Config: model.FlagEnvironmentConfig{
            Enabled:        true,
            DefaultVariant: "off",
            Variants:       []model.Variant{{Key: "off", Value: json.RawMessage(`false`)}},
        },
    })

    // Set a personal override for app-user-1
    cache.SetOverride("proj", "dev", "app-user-1", "my-flag", json.RawMessage(`true`), nil)

    engine := evaluation.NewEngine()
    // ... set up handler, make request with context.userId = "app-user-1"
    // Verify response has value: true and reason: "override"
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/ -run TestEvaluateHandler_PersonalOverride -v`
Expected: FAIL

**Step 3: Modify EvaluateHandler to check overrides**

In `internal/handler/evaluate_handler.go`, modify `EvaluateAll` and `EvaluateSingle`:

```go
// In EvaluateAll, after getting flags and segments:
for flagKey, fd := range flags {
    // Check for personal override first
    if evalCtx.UserID != "" {
        if overrideVal, ok := h.cache.GetOverride(sdkKey.ProjectKey, sdkKey.EnvironmentKey, evalCtx.UserID, flagKey); ok {
            results[flagKey] = &model.EvaluationResult{
                Value:  rawToAny(overrideVal),
                Reason: "override",
            }
            continue
        }
    }
    results[flagKey] = h.engine.EvaluateWithSegments(&fd.Flag, &fd.Config, evalCtx, segments)
}
```

```go
// In EvaluateSingle, after getting the flag:
if evalCtx.UserID != "" {
    if overrideVal, ok := h.cache.GetOverride(sdkKey.ProjectKey, sdkKey.EnvironmentKey, evalCtx.UserID, flagKey); ok {
        writeJSON(w, http.StatusOK, &model.EvaluationResult{
            Value:  rawToAny(overrideVal),
            Reason: "override",
        })
        return
    }
}
```

Add the `rawToAny` helper import or define locally (it's in the evaluation package — use `evaluation.RawToAny` if exported, or duplicate the simple helper).

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestEvaluateHandler_PersonalOverride -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/handler/evaluate_handler.go internal/handler/evaluate_handler_test.go
git commit -m "feat: check personal overrides during SDK evaluation (#48)"
```

---

### Task 8: Wire Everything in main.go

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add stores, handler, and routes**

In the stores section (after `lifecycleSnapshotStore`):
```go
appIdentityStore := store.NewAppIdentityStore(pool)
overrideStore := store.NewOverrideStore(pool)
```

In the cache loading section (after `cache.LoadAll`):
```go
// Load overrides into cache
overrideEntries, err := overrideStore.ListAllOverrides(ctx)
if err != nil {
    slog.Warn("failed to load overrides into cache", "error", err)
} else {
    cacheEntries := make([]evaluation.OverrideCacheEntryData, len(overrideEntries))
    for i, e := range overrideEntries {
        cacheEntries[i] = evaluation.OverrideCacheEntryData{
            ProjectKey:     e.ProjectKey,
            EnvironmentKey: e.EnvironmentKey,
            FlagKey:        e.FlagKey,
            AppUserID:      e.AppUserID,
            Value:          e.Value,
            ExpiresAt:      e.ExpiresAt,
        }
    }
    cache.LoadOverrides(cacheEntries)
}
```

In the handler section:
```go
overrideHandler := handler.NewOverrideHandler(overrideStore, appIdentityStore, projectStore, flagStore, environmentStore, cache)
```

In the route registration section:
```go
// App identity
mux.Handle("PUT /api/v1/projects/{key}/app-identity", wrap(overrideHandler.SetAppIdentity, sessionAuth, requireFlagsRead))
mux.Handle("GET /api/v1/projects/{key}/app-identity", wrap(overrideHandler.GetAppIdentity, sessionAuth, requireFlagsRead))
mux.Handle("DELETE /api/v1/projects/{key}/app-identity", wrap(overrideHandler.DeleteAppIdentity, sessionAuth, requireFlagsRead))

// Personal overrides
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/override", wrap(overrideHandler.SetOverride, sessionAuth, requireFlagsRead))
mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/override", wrap(overrideHandler.DeleteOverride, sessionAuth, requireFlagsRead))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/override", wrap(overrideHandler.SetOverrideAllEnvs, sessionAuth, requireFlagsRead))
mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/override", wrap(overrideHandler.DeleteOverrideAllEnvs, sessionAuth, requireFlagsRead))
mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/overrides/me", wrap(overrideHandler.GetFlagOverrides, sessionAuth, requireFlagsRead))
mux.Handle("GET /api/v1/overrides/me", wrap(overrideHandler.ListMyOverrides, sessionAuth))
```

Note: Override endpoints use `requireFlagsRead` (not write) since overrides are personal and don't modify flag config.

**Step 2: Build and verify**

Run: `go build ./cmd/togglerino`
Expected: Compiles without errors

**Step 3: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: wire override stores, handler, and routes in main.go (#48)"
```

---

### Task 9: Expired Override Cleanup Goroutine

**Files:**
- Create: `internal/override/cleaner.go`
- Create: `internal/override/cleaner_test.go`

**Step 1: Write the failing test**

```go
func TestCleaner_Run(t *testing.T) {
    // Test that the cleaner calls DeleteExpired on the store
}
```

**Step 2: Implement the cleaner**

```go
// internal/override/cleaner.go
package override

import (
    "context"
    "log/slog"
    "time"

    "github.com/togglerino/togglerino/internal/store"
)

type Cleaner struct {
    overrides *store.OverrideStore
    interval  time.Duration
}

func NewCleaner(overrides *store.OverrideStore, interval time.Duration) *Cleaner {
    return &Cleaner{overrides: overrides, interval: interval}
}

func (c *Cleaner) Run(ctx context.Context) {
    ticker := time.NewTicker(c.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            count, err := c.overrides.DeleteExpired(ctx)
            if err != nil {
                slog.Error("cleaning expired overrides", "error", err)
                continue
            }
            if count > 0 {
                slog.Info("cleaned expired overrides", "count", count)
            }
        }
    }
}
```

**Step 3: Wire in main.go**

After the schedule checker initialization:
```go
overrideCleaner := override.NewCleaner(overrideStore, 15*time.Minute)
```

After `go scheduleChecker.Run(ctx)`:
```go
go overrideCleaner.Run(ctx)
```

Add import: `"github.com/togglerino/togglerino/internal/override"`

**Step 4: Build and verify**

Run: `go build ./cmd/togglerino`
Expected: Compiles without errors

**Step 5: Commit**

```bash
git add internal/override/cleaner.go internal/override/cleaner_test.go cmd/togglerino/main.go
git commit -m "feat: add periodic cleanup for expired overrides (#48)"
```

---

### Task 10: Frontend Types and API Client

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

**Step 1: Add TypeScript types**

In `web/src/api/types.ts`:

```typescript
export interface AppIdentity {
  user_id: string
  project_id: string
  app_user_id: string
  created_at: string
  updated_at: string
}

export interface FlagOverrideEntry {
  id: string
  user_id: string
  flag_id: string
  flag_key?: string
  environment_id: string
  environment_key?: string
  project_key?: string
  value: unknown
  expires_at: string | null
  created_at: string
}
```

**Step 2: Add API client methods**

In `web/src/api/client.ts`, add to the `api` object:

```typescript
appIdentity: {
  get: (projectKey: string) =>
    request<AppIdentity>(`/projects/${projectKey}/app-identity`),
  set: (projectKey: string, appUserID: string) =>
    request<AppIdentity>(`/projects/${projectKey}/app-identity`, {
      method: 'PUT',
      body: JSON.stringify({ app_user_id: appUserID }),
    }),
  delete: (projectKey: string) =>
    request<void>(`/projects/${projectKey}/app-identity`, { method: 'DELETE' }),
},

overrides: {
  listMine: () => request<FlagOverrideEntry[]>('/overrides/me'),
  getForFlag: (projectKey: string, flagKey: string) =>
    request<FlagOverrideEntry[]>(`/projects/${projectKey}/flags/${flagKey}/overrides/me`),
  set: (projectKey: string, flagKey: string, envKey: string, value: unknown, duration?: string | null) =>
    request<FlagOverrideEntry>(
      `/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/override`,
      { method: 'PUT', body: JSON.stringify({ value, duration }) },
    ),
  delete: (projectKey: string, flagKey: string, envKey: string) =>
    request<void>(
      `/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/override`,
      { method: 'DELETE' },
    ),
  setAll: (projectKey: string, flagKey: string, value: unknown, duration?: string | null) =>
    request<void>(`/projects/${projectKey}/flags/${flagKey}/override`, {
      method: 'PUT',
      body: JSON.stringify({ value, duration }),
    }),
  deleteAll: (projectKey: string, flagKey: string) =>
    request<void>(`/projects/${projectKey}/flags/${flagKey}/override`, { method: 'DELETE' }),
},
```

**Step 3: Verify lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat: add frontend types and API client for overrides (#48)"
```

---

### Task 11: Override Controls on Flag Detail Page

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx` (or the environment config section component)

This task adds the "Override for me" toggle to each environment's config section on the flag detail page.

**Step 1: Read the current FlagDetailPage**

Read `web/src/pages/FlagDetailPage.tsx` to understand the existing structure before making changes.

**Step 2: Add the override UI component**

Create a new component `web/src/components/FlagOverrideControl.tsx`:

```typescript
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import type { FlagOverrideEntry, ValueType } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

interface FlagOverrideControlProps {
  projectKey: string
  flagKey: string
  envKey: string
  valueType: ValueType
  override?: FlagOverrideEntry
}

export function FlagOverrideControl({ projectKey, flagKey, envKey, valueType, override }: FlagOverrideControlProps) {
  const queryClient = useQueryClient()
  const [showSetDialog, setShowSetDialog] = useState(false)
  const [showIdentityDialog, setShowIdentityDialog] = useState(false)
  const [overrideValue, setOverrideValue] = useState('')
  const [duration, setDuration] = useState('24h')
  const [appUserID, setAppUserID] = useState('')

  const identityQuery = useQuery({
    queryKey: ['app-identity', projectKey],
    queryFn: () => api.appIdentity.get(projectKey),
    retry: false,
  })

  const setIdentityMutation = useMutation({
    mutationFn: (id: string) => api.appIdentity.set(projectKey, id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app-identity', projectKey] })
      setShowIdentityDialog(false)
      setShowSetDialog(true)
    },
  })

  const setOverrideMutation = useMutation({
    mutationFn: () => {
      let parsedValue: unknown = overrideValue
      if (valueType === 'boolean') parsedValue = overrideValue === 'true'
      else if (valueType === 'number') parsedValue = Number(overrideValue)
      else if (valueType === 'json') parsedValue = JSON.parse(overrideValue)
      return api.overrides.set(projectKey, flagKey, envKey, parsedValue, duration || null)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flag-overrides', projectKey, flagKey] })
      setShowSetDialog(false)
    },
  })

  const deleteOverrideMutation = useMutation({
    mutationFn: () => api.overrides.delete(projectKey, flagKey, envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flag-overrides', projectKey, flagKey] })
    },
  })

  const handleOverrideClick = () => {
    if (override) {
      deleteOverrideMutation.mutate()
    } else if (identityQuery.error) {
      setShowIdentityDialog(true)
    } else {
      setShowSetDialog(true)
    }
  }

  return (
    <>
      <div className="flex items-center gap-2">
        {override ? (
          <>
            <Badge variant="outline" className="text-amber-500 border-amber-500/30">
              Override: {JSON.stringify(override.value)}
            </Badge>
            {override.expires_at && (
              <span className="text-xs text-muted-foreground">
                expires {new Date(override.expires_at).toLocaleString()}
              </span>
            )}
            <Button variant="ghost" size="sm" onClick={handleOverrideClick}>
              Remove
            </Button>
          </>
        ) : (
          <Button variant="outline" size="sm" onClick={handleOverrideClick}>
            Override for me
          </Button>
        )}
      </div>

      {/* Identity setup dialog */}
      <Dialog open={showIdentityDialog} onOpenChange={setShowIdentityDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Set your app identity</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Enter the user ID your application uses to identify you in SDK evaluation context.
          </p>
          <Input
            placeholder="Your app user ID"
            value={appUserID}
            onChange={(e) => setAppUserID(e.target.value)}
          />
          <DialogFooter>
            <Button onClick={() => setIdentityMutation.mutate(appUserID)} disabled={!appUserID}>
              Save & Continue
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Set override dialog */}
      <Dialog open={showSetDialog} onOpenChange={setShowSetDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Set personal override</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">Value</label>
              {valueType === 'boolean' ? (
                <Select value={overrideValue || 'true'} onValueChange={setOverrideValue}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">true</SelectItem>
                    <SelectItem value="false">false</SelectItem>
                  </SelectContent>
                </Select>
              ) : (
                <Input
                  value={overrideValue}
                  onChange={(e) => setOverrideValue(e.target.value)}
                  placeholder={valueType === 'number' ? '0' : valueType === 'json' ? '{}' : 'value'}
                />
              )}
            </div>
            <div>
              <label className="text-sm font-medium">Duration</label>
              <Select value={duration} onValueChange={setDuration}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1h">1 hour</SelectItem>
                  <SelectItem value="8h">8 hours</SelectItem>
                  <SelectItem value="24h">24 hours</SelectItem>
                  <SelectItem value="7d">7 days</SelectItem>
                  <SelectItem value="">No expiry</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setOverrideMutation.mutate()}>
              Set Override
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
```

**Step 3: Integrate into FlagDetailPage**

In the environment config section of the flag detail page, add the `FlagOverrideControl` component. Query for the user's overrides for this flag:

```typescript
const overridesQuery = useQuery({
  queryKey: ['flag-overrides', projectKey, flagKey],
  queryFn: () => api.overrides.getForFlag(projectKey!, flagKey!),
  enabled: !!projectKey && !!flagKey,
})
```

Then for each environment config section, find the matching override and pass it:

```typescript
const envOverride = overridesQuery.data?.find(o => o.environment_key === env.key)
<FlagOverrideControl
  projectKey={projectKey!}
  flagKey={flagKey!}
  envKey={env.key}
  valueType={flag.value_type}
  override={envOverride}
/>
```

**Step 4: Verify lint and dev server**

Run: `cd web && npm run lint`
Run: `cd web && npm run dev` and test the UI manually

**Step 5: Commit**

```bash
git add web/src/components/FlagOverrideControl.tsx web/src/pages/FlagDetailPage.tsx
git commit -m "feat: add override controls to flag detail page (#48)"
```

---

### Task 12: My Overrides Page

**Files:**
- Create: `web/src/pages/MyOverridesPage.tsx`
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create the page component**

```typescript
// web/src/pages/MyOverridesPage.tsx
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Link } from 'react-router-dom'

export default function MyOverridesPage() {
  const queryClient = useQueryClient()

  const { data: overrides, isLoading } = useQuery({
    queryKey: ['my-overrides'],
    queryFn: () => api.overrides.listMine(),
  })

  const deleteMutation = useMutation({
    mutationFn: (o: { projectKey: string; flagKey: string; envKey: string }) =>
      api.overrides.delete(o.projectKey, o.flagKey, o.envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['my-overrides'] })
    },
  })

  if (isLoading) return <div className="p-6">Loading...</div>

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">My Overrides</h1>
      </div>

      {overrides?.length === 0 ? (
        <p className="text-muted-foreground">No active overrides.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Project</TableHead>
              <TableHead>Flag</TableHead>
              <TableHead>Environment</TableHead>
              <TableHead>Value</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {overrides?.map((o) => (
              <TableRow key={o.id}>
                <TableCell>
                  <Link to={`/projects/${o.project_key}`} className="text-amber-500 hover:underline">
                    {o.project_key}
                  </Link>
                </TableCell>
                <TableCell>
                  <Link
                    to={`/projects/${o.project_key}/flags/${o.flag_key}`}
                    className="font-mono text-sm text-amber-500 hover:underline"
                  >
                    {o.flag_key}
                  </Link>
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{o.environment_key}</Badge>
                </TableCell>
                <TableCell className="font-mono text-sm">{JSON.stringify(o.value)}</TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {o.expires_at ? new Date(o.expires_at).toLocaleString() : 'Never'}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      deleteMutation.mutate({
                        projectKey: o.project_key!,
                        flagKey: o.flag_key!,
                        envKey: o.environment_key!,
                      })
                    }
                  >
                    Remove
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
```

**Step 2: Add route**

In `web/src/App.tsx`, add inside the `<Route element={<OrgLayout />}>` section:

```typescript
<Route path="/overrides" element={<MyOverridesPage />} />
```

Add import:
```typescript
import MyOverridesPage from './pages/MyOverridesPage'
```

**Step 3: Add navigation link**

Add "My Overrides" to the user menu/navigation. Check where the user dropdown or sidebar is defined and add a link to `/overrides`.

**Step 4: Verify lint and dev server**

Run: `cd web && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/pages/MyOverridesPage.tsx web/src/App.tsx
git commit -m "feat: add My Overrides page (#48)"
```

---

### Task 13: Integration Testing and Final Verification

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: All tests pass

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 3: Build the full binary**

Run: `cd web && npm install && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: Build succeeds

**Step 4: Manual smoke test**

1. Start with `./dev.sh` and `cd web && npm run dev`
2. Create a project with a boolean flag
3. Set your app identity for the project
4. Set a personal override on the flag
5. Verify via the playground that the override takes effect for your app user ID
6. Check the My Overrides page shows the override
7. Remove the override and verify it's gone

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: personal flag overrides (#48)"
```
