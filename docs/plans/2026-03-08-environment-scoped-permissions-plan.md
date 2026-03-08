# Environment-Scoped Permissions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restrict which roles can update flag environment configs on a per-project, per-environment basis.

**Architecture:** New `project_environment_access` table stores allow-lists per project+role. A check in `UpdateEnvironmentConfig` (and `PromoteEnvironmentConfig`) verifies the user's resolved role has access to the target environment. No rows = unrestricted (backwards-compatible). Dedicated `GET/PUT` endpoint manages the matrix. Frontend adds an "Environment Access" tab in Project Settings.

**Tech Stack:** Go (stdlib net/http, pgx/v5), PostgreSQL, React 19 + TypeScript + TanStack Query + shadcn/ui

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/023_environment_access.up.sql`
- Create: `migrations/023_environment_access.down.sql`

**Step 1: Write the up migration**

```sql
CREATE TABLE project_environment_access (
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role_name      TEXT NOT NULL REFERENCES roles(name) ON UPDATE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, role_name, environment_id)
);
```

**Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS project_environment_access;
```

**Step 3: Verify migration applies**

Run: `./dev.sh` (starts PostgreSQL + runs migrations)
Expected: Clean startup, no migration errors in logs

**Step 4: Commit**

```bash
git add migrations/023_environment_access.up.sql migrations/023_environment_access.down.sql
git commit -m "feat: add project_environment_access migration (#99)"
```

---

### Task 2: Environment Access Store

**Files:**
- Create: `internal/store/environment_access_store.go`
- Create: `internal/store/environment_access_store_test.go`

**Step 1: Write failing tests**

```go
package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestEnvironmentAccessStore_NoRestrictions(t *testing.T) {
	pool := testPool(t)
	s := store.NewEnvironmentAccessStore(pool)
	ctx := context.Background()

	// With no rows, ListByProjectAndRole should return empty slice
	envIDs, err := s.ListByProjectAndRole(ctx, "some-project-id", "editor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envIDs) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(envIDs))
	}
}

func TestEnvironmentAccessStore_ReplaceForProject(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Setup: create project, environments, and role (use helper fixtures)
	projectID := createTestProject(t, pool, ctx)
	envID1 := createTestEnvironment(t, pool, ctx, projectID, "development")
	envID2 := createTestEnvironment(t, pool, ctx, projectID, "production")

	s := store.NewEnvironmentAccessStore(pool)

	// Replace with restrictions for editor
	restrictions := []store.EnvironmentAccessRestriction{
		{RoleName: "editor", EnvironmentIDs: []string{envID1}},
	}
	err := s.ReplaceForProject(ctx, projectID, restrictions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// editor should only see envID1
	envIDs, err := s.ListByProjectAndRole(ctx, projectID, "editor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envIDs) != 1 || envIDs[0] != envID1 {
		t.Fatalf("expected [%s], got %v", envID1, envIDs)
	}

	// admin should have no restrictions (no rows)
	envIDs, err = s.ListByProjectAndRole(ctx, projectID, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envIDs) != 0 {
		t.Fatalf("expected empty for admin, got %v", envIDs)
	}
}

func TestEnvironmentAccessStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	projectID := createTestProject(t, pool, ctx)
	envID1 := createTestEnvironment(t, pool, ctx, projectID, "development")

	s := store.NewEnvironmentAccessStore(pool)

	restrictions := []store.EnvironmentAccessRestriction{
		{RoleName: "editor", EnvironmentIDs: []string{envID1}},
	}
	_ = s.ReplaceForProject(ctx, projectID, restrictions)

	result, err := s.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 restriction, got %d", len(result))
	}
	if result[0].RoleName != "editor" || len(result[0].EnvironmentIDs) != 1 {
		t.Fatalf("unexpected restriction: %+v", result[0])
	}
}
```

Note: The test helpers `testPool`, `createTestProject`, `createTestEnvironment` follow the patterns already used in the codebase's store tests. Check `internal/store/project_member_store_test.go` for the exact helper signatures and adapt accordingly.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/... -run TestEnvironmentAccess -v`
Expected: FAIL — `NewEnvironmentAccessStore` undefined

**Step 3: Write the store implementation**

```go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvironmentAccessRestriction represents the allowed environments for a role in a project.
type EnvironmentAccessRestriction struct {
	RoleName       string   `json:"role_name"`
	EnvironmentIDs []string `json:"environment_ids"`
}

// EnvironmentAccessStore manages per-project, per-role environment write access.
type EnvironmentAccessStore struct {
	pool *pgxpool.Pool
}

// NewEnvironmentAccessStore creates a new EnvironmentAccessStore.
func NewEnvironmentAccessStore(pool *pgxpool.Pool) *EnvironmentAccessStore {
	return &EnvironmentAccessStore{pool: pool}
}

// ListByProjectAndRole returns the allowed environment IDs for a role in a project.
// An empty slice means the role has no restrictions configured (access to all environments).
func (s *EnvironmentAccessStore) ListByProjectAndRole(ctx context.Context, projectID, roleName string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT environment_id FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2`,
		projectID, roleName,
	)
	if err != nil {
		return nil, fmt.Errorf("listing environment access: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning environment access: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environment access: %w", err)
	}
	return ids, nil
}

// ListByProject returns all environment access restrictions for a project,
// grouped by role name.
func (s *EnvironmentAccessStore) ListByProject(ctx context.Context, projectID string) ([]EnvironmentAccessRestriction, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT role_name, environment_id FROM project_environment_access
		 WHERE project_id = $1
		 ORDER BY role_name, environment_id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing project environment access: %w", err)
	}
	defer rows.Close()

	byRole := map[string][]string{}
	var roleOrder []string
	for rows.Next() {
		var roleName, envID string
		if err := rows.Scan(&roleName, &envID); err != nil {
			return nil, fmt.Errorf("scanning project environment access: %w", err)
		}
		if _, exists := byRole[roleName]; !exists {
			roleOrder = append(roleOrder, roleName)
		}
		byRole[roleName] = append(byRole[roleName], envID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating project environment access: %w", err)
	}

	var result []EnvironmentAccessRestriction
	for _, roleName := range roleOrder {
		result = append(result, EnvironmentAccessRestriction{
			RoleName:       roleName,
			EnvironmentIDs: byRole[roleName],
		})
	}
	return result, nil
}

// ReplaceForProject atomically replaces all environment access restrictions
// for a project. Pass an empty slice to remove all restrictions.
func (s *EnvironmentAccessStore) ReplaceForProject(ctx context.Context, projectID string, restrictions []EnvironmentAccessRestriction) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete all existing restrictions for this project.
	_, err = tx.Exec(ctx,
		`DELETE FROM project_environment_access WHERE project_id = $1`,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("deleting old restrictions: %w", err)
	}

	// Insert new restrictions.
	for _, r := range restrictions {
		for _, envID := range r.EnvironmentIDs {
			_, err = tx.Exec(ctx,
				`INSERT INTO project_environment_access (project_id, role_name, environment_id)
				 VALUES ($1, $2, $3)`,
				projectID, r.RoleName, envID,
			)
			if err != nil {
				return fmt.Errorf("inserting environment access for role %s: %w", r.RoleName, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// HasAccess checks whether a role has write access to a specific environment
// in a project. Returns true if unrestricted (no rows) or if the environment
// is in the allow-list.
func (s *EnvironmentAccessStore) HasAccess(ctx context.Context, projectID, roleName, environmentID string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2`,
		projectID, roleName,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking environment access count: %w", err)
	}

	// No restrictions configured for this role = unrestricted access.
	if count == 0 {
		return true, nil
	}

	// Check if the specific environment is in the allow-list.
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2 AND environment_id = $3`,
		projectID, roleName, environmentID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking environment access: %w", err)
	}
	return count > 0, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/... -run TestEnvironmentAccess -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/environment_access_store.go internal/store/environment_access_store_test.go
git commit -m "feat: add EnvironmentAccessStore (#99)"
```

---

### Task 3: Enforce Environment Access in Middleware

**Files:**
- Modify: `internal/auth/middleware.go` — store resolved role in context, add `CheckEnvironmentAccess`
- Create: `internal/auth/middleware_test.go` (or add to existing) — test the new check

**Step 1: Write failing tests**

Test that `RequireProjectPermission` stores the resolved role in context, and that `CheckEnvironmentAccess` blocks/allows correctly.

```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
)

func TestCheckEnvironmentAccess_Unrestricted(t *testing.T) {
	// When HasAccess returns true, the handler should be called
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envID string) (bool, error) {
			return true, nil // unrestricted
		},
	)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	// Build request with user (org member, not admin), project, and role in context
	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag/environments/production", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleMember})
	ctx = auth.ContextWithProject(ctx, &model.Project{ID: "p1"})
	ctx = auth.ContextWithResolvedRole(ctx, "editor")
	r = r.WithContext(ctx)
	r.SetPathValue("env", "production")

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestCheckEnvironmentAccess_Blocked(t *testing.T) {
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envID string) (bool, error) {
			return false, nil // restricted
		},
	)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag/environments/production", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleMember})
	ctx = auth.ContextWithProject(ctx, &model.Project{ID: "p1"})
	ctx = auth.ContextWithResolvedRole(ctx, "editor")
	r = r.WithContext(ctx)
	r.SetPathValue("env", "production")

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestCheckEnvironmentAccess_OrgAdminBypasses(t *testing.T) {
	checker := auth.CheckEnvironmentAccess(
		func(ctx context.Context, projectID, roleName, envID string) (bool, error) {
			t.Fatal("should not be called for org admin")
			return false, nil
		},
	)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	r := httptest.NewRequest("PUT", "/api/v1/projects/myproj/flags/myflag/environments/production", nil)
	ctx := auth.ContextWithUser(r.Context(), &model.User{ID: "u1", Role: model.RoleAdmin})
	r = r.WithContext(ctx)

	w := httptest.NewRecorder()
	checker(inner).ServeHTTP(w, r)

	if !called {
		t.Fatal("expected handler to be called for org admin")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/... -run TestCheckEnvironmentAccess -v`
Expected: FAIL — `CheckEnvironmentAccess` undefined, `ContextWithResolvedRole` undefined

**Step 3: Implement context helpers and middleware**

Add to `internal/auth/middleware.go`:

```go
const resolvedRoleContextKey contextKey = "resolved_role"

// ResolvedRoleFromContext returns the project role stored by RequireProjectPermission.
func ResolvedRoleFromContext(ctx context.Context) string {
	r, _ := ctx.Value(resolvedRoleContextKey).(string)
	return r
}

// ContextWithResolvedRole returns a new context with the resolved role set.
func ContextWithResolvedRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, resolvedRoleContextKey, role)
}
```

Modify `RequireProjectPermission` to store the resolved role in context (after line 101, before `next.ServeHTTP`):

```go
// Store the resolved role in context for downstream middleware.
ctx := context.WithValue(r.Context(), resolvedRoleContextKey, string(role))
r = r.WithContext(ctx)
```

Add the `CheckEnvironmentAccess` middleware:

```go
// AccessChecker is a function that checks if a role has write access to an environment.
type AccessChecker func(ctx context.Context, projectID, roleName, environmentID string) (bool, error)

// CheckEnvironmentAccess returns middleware that verifies the user's resolved role
// has write access to the target environment. Org admins bypass the check.
// The "env" path value is used to look up the environment.
func CheckEnvironmentAccess(hasAccess AccessChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Org admins bypass environment access checks.
			if user.Role == model.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			project := ProjectFromContext(r.Context())
			if project == nil {
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
				return
			}

			roleName := ResolvedRoleFromContext(r.Context())
			if roleName == "" {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			envKey := r.PathValue("env")
			if envKey == "" {
				// No environment in path — skip check (not an env-scoped route).
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := hasAccess(r.Context(), project.ID, roleName, envKey)
			if err != nil {
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, `{"error":"forbidden: no write access to this environment"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

Note: The `hasAccess` function takes `envKey` (not `envID`) from the path. The store's `HasAccess` method needs the environment ID. We need to resolve envKey→envID. Two approaches:

**Option A (recommended):** Change `HasAccess` to accept envKey and do the join in SQL.
**Option B:** Look up the environment in the middleware.

For simplicity, update the `HasAccess` store method to work with environment keys via a join:

```go
// HasAccessByEnvKey checks whether a role has write access to a specific environment
// (identified by key) in a project. Returns true if unrestricted (no rows) or if the
// environment is in the allow-list.
func (s *EnvironmentAccessStore) HasAccessByEnvKey(ctx context.Context, projectID, roleName, envKey string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access
		 WHERE project_id = $1 AND role_name = $2`,
		projectID, roleName,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking environment access count: %w", err)
	}

	// No restrictions configured for this role = unrestricted access.
	if count == 0 {
		return true, nil
	}

	// Check if the specific environment is in the allow-list.
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_environment_access pea
		 JOIN environments e ON e.id = pea.environment_id
		 WHERE pea.project_id = $1 AND pea.role_name = $2 AND e.key = $3 AND e.project_id = $1`,
		projectID, roleName, envKey,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking environment access: %w", err)
	}
	return count > 0, nil
}
```

Update tests to use `envKey` instead of `envID` in the `AccessChecker` function signature.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/... -run TestCheckEnvironmentAccess -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go internal/store/environment_access_store.go
git commit -m "feat: add CheckEnvironmentAccess middleware (#99)"
```

---

### Task 4: Wire Environment Access Check into Routes

**Files:**
- Modify: `cmd/togglerino/main.go` — create store, build checker, add middleware to env-config routes

**Step 1: Wire the store and middleware in main.go**

After the existing store initialization (around line 78), add:

```go
environmentAccessStore := store.NewEnvironmentAccessStore(pool)
```

After the middleware creation (around line 197), add:

```go
checkEnvAccess := auth.CheckEnvironmentAccess(environmentAccessStore.HasAccessByEnvKey)
```

Update the `UpdateEnvironmentConfig` and `PromoteEnvironmentConfig` routes (lines 263-264) to include the new middleware:

```go
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}",
    wrap(flagHandler.UpdateEnvironmentConfig, sessionAuth, requireFlagsWrite, checkEnvAccess))
mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/promote",
    wrap(flagHandler.PromoteEnvironmentConfig, sessionAuth, requireFlagsWrite, checkEnvAccess))
```

Also add to scheduled changes routes that write to environments:

```go
mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules",
    wrap(scheduleHandler.Create, sessionAuth, requireFlagsWrite, checkEnvAccess))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}",
    wrap(scheduleHandler.Update, sessionAuth, requireFlagsWrite, checkEnvAccess))
mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}",
    wrap(scheduleHandler.Cancel, sessionAuth, requireFlagsWrite, checkEnvAccess))
```

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS (no regressions — no restrictions configured means everything passes)

**Step 3: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: wire environment access check into flag config routes (#99)"
```

---

### Task 5: Environment Access Handler (GET/PUT API)

**Files:**
- Create: `internal/handler/environment_access_handler.go`
- Create: `internal/handler/environment_access_handler_test.go`

**Step 1: Write failing tests**

```go
package handler_test

// Test GET returns restrictions + environments
// Test PUT replaces restrictions
// Test PUT with invalid role returns 400
// Test PUT with invalid environment ID returns 400
```

Follow the patterns in existing handler tests. Key test scenarios:
- GET with no restrictions returns empty restrictions + all environments
- PUT with valid restrictions updates and returns 200
- PUT with non-existent role name returns 400
- PUT with environment ID not belonging to project returns 400
- GET after PUT reflects the changes

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/handler/... -run TestEnvironmentAccess -v`
Expected: FAIL

**Step 3: Write the handler**

```go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type EnvironmentAccessHandler struct {
	envAccess    *store.EnvironmentAccessStore
	environments *store.EnvironmentStore
	projects     *store.ProjectStore
	roles        *store.RoleStore
	audit        *store.AuditStore
}

func NewEnvironmentAccessHandler(
	envAccess *store.EnvironmentAccessStore,
	environments *store.EnvironmentStore,
	projects *store.ProjectStore,
	roles *store.RoleStore,
	audit *store.AuditStore,
) *EnvironmentAccessHandler {
	return &EnvironmentAccessHandler{
		envAccess:    envAccess,
		environments: environments,
		projects:     projects,
		roles:        roles,
		audit:        audit,
	}
}

type environmentAccessResponse struct {
	Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
	Environments []environmentSummary                 `json:"environments"`
}

type environmentSummary struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Get returns the environment access restrictions for a project.
// GET /api/v1/projects/{key}/environment-access
func (h *EnvironmentAccessHandler) Get(w http.ResponseWriter, r *http.Request) {
	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	restrictions, err := h.envAccess.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environment access")
		return
	}
	if restrictions == nil {
		restrictions = []store.EnvironmentAccessRestriction{}
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}

	envSummaries := make([]environmentSummary, len(envs))
	for i, e := range envs {
		envSummaries[i] = environmentSummary{ID: e.ID, Key: e.Key, Name: e.Name}
	}

	writeJSON(w, http.StatusOK, environmentAccessResponse{
		Restrictions: restrictions,
		Environments: envSummaries,
	})
}

// Update replaces the environment access restrictions for a project.
// PUT /api/v1/projects/{key}/environment-access
func (h *EnvironmentAccessHandler) Update(w http.ResponseWriter, r *http.Request) {
	project := auth.ProjectFromContext(r.Context())
	if project == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Restrictions []store.EnvironmentAccessRestriction `json:"restrictions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate role names exist
	for _, restriction := range req.Restrictions {
		_, err := h.roles.GetByName(r.Context(), restriction.RoleName)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid role: "+restriction.RoleName)
			return
		}
	}

	// Validate environment IDs belong to the project
	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	envIDSet := map[string]bool{}
	for _, e := range envs {
		envIDSet[e.ID] = true
	}
	for _, restriction := range req.Restrictions {
		for _, envID := range restriction.EnvironmentIDs {
			if !envIDSet[envID] {
				writeError(w, http.StatusBadRequest, "environment not found in project: "+envID)
				return
			}
		}
	}

	// Fetch old restrictions for audit
	oldRestrictions, _ := h.envAccess.ListByProject(r.Context(), project.ID)

	if err := h.envAccess.ReplaceForProject(r.Context(), project.ID, req.Restrictions); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update environment access")
		return
	}

	// Best-effort audit logging
	if user := auth.UserFromContext(r.Context()); user != nil {
		oldVal, _ := json.Marshal(oldRestrictions)
		newVal, _ := json.Marshal(req.Restrictions)
		if err := h.audit.Record(r.Context(), model.AuditEntry{
			ProjectID: &project.ID,
			UserID:    &user.ID,
			UserEmail: &user.Email,
			Action:    "update",
			EntityType: "environment_access",
			EntityID:  project.Key,
			OldValue:  oldVal,
			NewValue:  newVal,
		}); err != nil {
			slog.Warn("failed to record audit log", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

Note: Check if `RoleStore` has a `GetByName` method. If not, you may need to add one or use `List` and filter. Look at `internal/store/role_store.go` for available methods.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/... -run TestEnvironmentAccess -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/handler/environment_access_handler.go internal/handler/environment_access_handler_test.go
git commit -m "feat: add environment access handler (#99)"
```

---

### Task 6: Register Environment Access Routes

**Files:**
- Modify: `cmd/togglerino/main.go` — create handler, register routes

**Step 1: Add handler creation and routes**

In handler initialization section (around line 163):

```go
environmentAccessHandler := handler.NewEnvironmentAccessHandler(
    environmentAccessStore, environmentStore, projectStore, roleStore, auditStore,
)
```

In route registration (after the project settings routes, around line 279):

```go
// Environment access
mux.Handle("GET /api/v1/projects/{key}/environment-access",
    wrap(environmentAccessHandler.Get, sessionAuth, requireProjectSettings))
mux.Handle("PUT /api/v1/projects/{key}/environment-access",
    wrap(environmentAccessHandler.Update, sessionAuth, requireProjectSettings))
```

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat: register environment access API routes (#99)"
```

---

### Task 7: Frontend — API Client and Types

**Files:**
- Modify: `web/src/api/client.ts` — add environment access API methods
- Modify: `web/src/api/types.ts` — add types (if types file exists, otherwise define inline)

**Step 1: Add types**

Check if `web/src/api/types.ts` exists. Add:

```typescript
export interface EnvironmentAccessRestriction {
  role_name: string
  environment_ids: string[]
}

export interface EnvironmentSummary {
  id: string
  key: string
  name: string
}

export interface EnvironmentAccessResponse {
  restrictions: EnvironmentAccessRestriction[]
  environments: EnvironmentSummary[]
}
```

**Step 2: Add API methods to client.ts**

Add to the `api` object:

```typescript
environmentAccess: {
  get: (projectKey: string) =>
    request<EnvironmentAccessResponse>(`/projects/${projectKey}/environment-access`),
  update: (projectKey: string, restrictions: EnvironmentAccessRestriction[]) =>
    request<{ status: string }>(`/projects/${projectKey}/environment-access`, {
      method: 'PUT',
      body: JSON.stringify({ restrictions }),
    }),
},
```

**Step 3: Commit**

```bash
git add web/src/api/client.ts web/src/api/types.ts
git commit -m "feat: add environment access API client methods (#99)"
```

---

### Task 8: Frontend — Environment Access Settings Tab

**Files:**
- Create: `web/src/pages/settings/EnvironmentAccessTab.tsx`
- Modify: `web/src/pages/ProjectSettingsPage.tsx` — add tab
- Modify: `web/src/App.tsx` — add route

**Step 1: Create the tab component**

Create `web/src/pages/settings/EnvironmentAccessTab.tsx` following the pattern of `MembersTab.tsx`:

- Use `useQuery` to fetch environment access from `api.environmentAccess.get(key)`
- Use `useMutation` + `useQueryClient` for saving
- Render a matrix grid: rows = environments, columns = roles (from `useRoles` hook)
- Each cell is a `Switch` component from shadcn/ui
- Default state: all switches on (unrestricted). When a role has restrictions, only allowed environments are on
- "Save" button triggers PUT with the current restriction state
- Show loading/error states

The UI logic:
- If a role has NO entry in `restrictions`, it's unrestricted (all switches on)
- When user toggles OFF an environment for a role, add that role to restrictions with remaining on environments
- When user toggles ON all environments for a role, remove that role from restrictions
- When user toggles OFF all environments for a role, that role's `environment_ids` is empty (fully restricted)

```tsx
import { useState, useEffect, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useRoles } from '@/hooks/useRoles'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import type { EnvironmentAccessRestriction } from '@/api/types'

export default function EnvironmentAccessTab() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const { data: roles } = useRoles()

  const { data, isLoading } = useQuery({
    queryKey: ['projects', key, 'environment-access'],
    queryFn: () => api.environmentAccess.get(key!),
  })

  // State: map of roleName -> set of allowed environment IDs
  // null means unrestricted
  const [accessMap, setAccessMap] = useState<Record<string, string[] | null>>({})
  const [isDirty, setIsDirty] = useState(false)

  // Initialize state from server data
  useEffect(() => {
    if (!data) return
    const map: Record<string, string[] | null> = {}
    // Roles with restrictions
    for (const r of data.restrictions) {
      map[r.role_name] = r.environment_ids
    }
    setAccessMap(map)
    setIsDirty(false)
  }, [data])

  const saveMutation = useMutation({
    mutationFn: () => {
      const restrictions: EnvironmentAccessRestriction[] = []
      for (const [roleName, envIds] of Object.entries(accessMap)) {
        if (envIds !== null) {
          restrictions.push({ role_name: roleName, environment_ids: envIds })
        }
      }
      return api.environmentAccess.update(key!, restrictions)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environment-access'] })
      setIsDirty(false)
    },
  })

  const handleToggle = (roleName: string, envId: string, checked: boolean) => {
    setAccessMap(prev => {
      const allEnvIds = data!.environments.map(e => e.id)
      const current = prev[roleName] ?? allEnvIds // null means all

      let next: string[]
      if (checked) {
        next = [...current, envId]
      } else {
        next = current.filter(id => id !== envId)
      }

      // If all environments are allowed, remove the restriction (set to null)
      const isUnrestricted = allEnvIds.every(id => next.includes(id))

      return {
        ...prev,
        [roleName]: isUnrestricted ? null : next,
      }
    })
    setIsDirty(true)
  }

  const isChecked = (roleName: string, envId: string): boolean => {
    const allowed = accessMap[roleName]
    if (allowed === null || allowed === undefined) return true // unrestricted
    return allowed.includes(envId)
  }

  if (isLoading || !data) return <div className="text-sm text-muted-foreground">Loading...</div>

  const projectRoles = roles?.filter(r => !['admin', 'member'].includes(r.name) || ['admin', 'editor', 'viewer'].includes(r.name))
    .filter(r => r.permissions?.includes('flags:write')) ?? []

  return (
    <div className="space-y-6">
      <div>
        <h3 className="text-sm font-medium mb-1">Environment Write Access</h3>
        <p className="text-xs text-muted-foreground/60">
          Control which roles can update flag configurations per environment.
          Unrestricted roles can write to all environments.
        </p>
      </div>

      <Card>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border/40">
                  <th className="text-left p-3 font-medium text-muted-foreground">Environment</th>
                  {projectRoles.map(role => (
                    <th key={role.name} className="text-center p-3 font-medium text-muted-foreground">
                      {role.name}
                      {accessMap[role.name] === undefined || accessMap[role.name] === null ? (
                        <Badge variant="outline" className="ml-1.5 text-[10px]">all</Badge>
                      ) : null}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {data.environments.map(env => (
                  <tr key={env.id} className="border-b border-border/20">
                    <td className="p-3 font-mono text-xs">{env.key}</td>
                    {projectRoles.map(role => (
                      <td key={role.name} className="text-center p-3">
                        <Switch
                          checked={isChecked(role.name, env.id)}
                          onCheckedChange={(checked) => handleToggle(role.name, env.id, checked)}
                        />
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button
          onClick={() => saveMutation.mutate()}
          disabled={!isDirty || saveMutation.isPending}
        >
          {saveMutation.isPending ? 'Saving...' : 'Save'}
        </Button>
      </div>
    </div>
  )
}
```

**Step 2: Add tab to ProjectSettingsPage.tsx**

In `web/src/pages/ProjectSettingsPage.tsx`, add to `allSettingsTabs`:

```typescript
{ to: 'environment-access', label: 'Environment Access', adminOnly: true },
```

**Step 3: Add route to App.tsx**

Import and add route inside the `settings` route group:

```tsx
import EnvironmentAccessTab from './pages/settings/EnvironmentAccessTab.tsx'

// Inside <Route path="settings" ...>:
<Route path="environment-access" element={<EnvironmentAccessTab />} />
```

**Step 4: Test manually**

Run: `cd web && npm run dev`
Navigate to: `/projects/{key}/settings/environment-access`
Expected: Matrix grid shows environments × roles with switches. All switches on by default. Toggling and saving works.

**Step 5: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 6: Commit**

```bash
git add web/src/pages/settings/EnvironmentAccessTab.tsx web/src/pages/ProjectSettingsPage.tsx web/src/App.tsx
git commit -m "feat: add environment access settings UI (#99)"
```

---

### Task 9: Integration Testing

**Files:**
- Modify or create: `internal/handler/environment_access_handler_test.go` — end-to-end test

**Step 1: Write integration test**

Test the full flow:
1. Create a project with environments
2. Verify unrestricted user can update any environment config
3. Add restrictions (editor can only write to development)
4. Verify editor can update development config
5. Verify editor cannot update production config (403)
6. Verify org admin can still update production config
7. Remove restrictions
8. Verify editor can update production config again

**Step 2: Run tests**

Run: `go test ./... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/handler/environment_access_handler_test.go
git commit -m "test: add environment access integration tests (#99)"
```

---

### Task 10: Final Verification

**Step 1: Run all backend tests**

Run: `go test ./...`
Expected: All PASS

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 3: Build the full binary**

Run: `cd web && npm install && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: Build succeeds

**Step 4: Final commit (if any remaining changes)**

```bash
git status  # should be clean
```
