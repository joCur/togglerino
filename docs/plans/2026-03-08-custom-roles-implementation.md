# Custom Roles Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace hardcoded project role definitions with a database-driven model supporting admin-defined custom roles.

**Architecture:** New `roles` table stores both built-in and custom roles with their permissions as `TEXT[]`. The `project_members.role` column becomes a FK to `roles.name`. Permission checking moves from static Go maps to in-memory role cache loaded from DB.

**Tech Stack:** Go 1.25 (stdlib net/http, pgx/v5), React 19, TypeScript, TanStack Query, shadcn/ui, Tailwind CSS v4

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/022_custom_roles.up.sql`
- Create: `migrations/022_custom_roles.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/022_custom_roles.up.sql

-- Create roles table
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    permissions TEXT[] NOT NULL,
    is_built_in BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed built-in roles
INSERT INTO roles (name, description, permissions, is_built_in) VALUES
    ('admin', 'Full project access including settings', ARRAY['flags:read','flags:write','environments:read','environments:write','sdk_keys:manage','segments:write','templates:manage','project:settings'], true),
    ('editor', 'Can modify flags, environments, and segments', ARRAY['flags:read','flags:write','environments:read','environments:write','sdk_keys:manage','segments:write','templates:manage'], true),
    ('viewer', 'Read-only access to flags and environments', ARRAY['flags:read','environments:read'], true);

-- Drop the old CHECK constraint on project_members.role
ALTER TABLE project_members DROP CONSTRAINT IF EXISTS project_members_role_check;

-- Add FK to roles table
ALTER TABLE project_members
    ADD CONSTRAINT project_members_role_fkey FOREIGN KEY (role) REFERENCES roles(name) ON UPDATE CASCADE;
```

**Step 2: Write the down migration**

```sql
-- migrations/022_custom_roles.down.sql

ALTER TABLE project_members DROP CONSTRAINT IF EXISTS project_members_role_fkey;
ALTER TABLE project_members ADD CONSTRAINT project_members_role_check CHECK (role IN ('admin', 'editor', 'viewer'));
DROP TABLE IF EXISTS roles;
```

**Step 3: Verify migration compiles**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go build ./migrations/...`
Expected: No errors (embed.FS picks up new files automatically)

**Step 4: Commit**

```bash
git add migrations/022_custom_roles.up.sql migrations/022_custom_roles.down.sql
git commit -m "feat: add roles table migration (#83)"
```

---

### Task 2: Role Model

**Files:**
- Create: `internal/model/role.go`

**Step 1: Write the model**

```go
package model

import "time"

// Role definition for project-level access control.
type RoleDefinition struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	IsBuiltIn   bool      `json:"is_built_in"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AllProjectPermissions is the canonical list of valid project-level permissions.
var AllProjectPermissions = []Permission{
	PermFlagsRead,
	PermFlagsWrite,
	PermEnvironmentsRead,
	PermEnvironmentsWrite,
	PermSDKKeysManage,
	PermSegmentsWrite,
	PermTemplatesManage,
	PermProjectSettings,
}

// ValidPermission returns true if p is a known project-level permission.
func ValidPermission(p string) bool {
	for _, perm := range AllProjectPermissions {
		if string(perm) == p {
			return true
		}
	}
	return false
}
```

**Step 2: Write tests for ValidPermission**

Create `internal/model/role_test.go`:

```go
package model

import "testing"

func TestValidPermission(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"flags:read", true},
		{"flags:write", true},
		{"environments:read", true},
		{"environments:write", true},
		{"sdk_keys:manage", true},
		{"segments:write", true},
		{"templates:manage", true},
		{"project:settings", true},
		{"", false},
		{"unknown", false},
		{"org:users:manage", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ValidPermission(tt.input); got != tt.want {
				t.Errorf("ValidPermission(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
```

**Step 3: Run tests to verify they pass**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/model/...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/model/role.go internal/model/role_test.go
git commit -m "feat: add RoleDefinition model and ValidPermission (#83)"
```

---

### Task 3: Role Store

**Files:**
- Create: `internal/store/role_store.go`
- Create: `internal/store/role_store_test.go`

**Step 1: Write the failing test for List**

```go
package store

import (
	"context"
	"testing"
)

func TestRoleStore_List(t *testing.T) {
	pool := testPool(t)
	s := NewRoleStore(pool)
	ctx := context.Background()

	roles, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Migration seeds 3 built-in roles
	if len(roles) < 3 {
		t.Fatalf("expected at least 3 built-in roles, got %d", len(roles))
	}

	// Verify built-in roles exist
	found := map[string]bool{}
	for _, r := range roles {
		found[r.Name] = true
		if !r.IsBuiltIn {
			continue
		}
		if len(r.Permissions) == 0 {
			t.Errorf("built-in role %q has no permissions", r.Name)
		}
	}
	for _, name := range []string{"admin", "editor", "viewer"} {
		if !found[name] {
			t.Errorf("built-in role %q not found", name)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/store/ -run TestRoleStore_List -v`
Expected: FAIL (NewRoleStore not defined)

**Step 3: Write the store implementation**

```go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

// RoleStore manages role definitions in the database.
type RoleStore struct {
	pool *pgxpool.Pool
}

// NewRoleStore creates a new RoleStore.
func NewRoleStore(pool *pgxpool.Pool) *RoleStore {
	return &RoleStore{pool: pool}
}

// List returns all role definitions ordered by built-in first, then name.
func (s *RoleStore) List(ctx context.Context) ([]model.RoleDefinition, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, description, permissions, is_built_in, created_at, updated_at
		 FROM roles ORDER BY is_built_in DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing roles: %w", err)
	}
	defer rows.Close()

	var roles []model.RoleDefinition
	for rows.Next() {
		var r model.RoleDefinition
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning role: %w", err)
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

// GetByName returns a single role by name.
func (s *RoleStore) GetByName(ctx context.Context, name string) (*model.RoleDefinition, error) {
	var r model.RoleDefinition
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, permissions, is_built_in, created_at, updated_at
		 FROM roles WHERE name = $1`, name,
	).Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("role not found: %w", err)
	}
	return &r, nil
}

// Create inserts a new custom role.
func (s *RoleStore) Create(ctx context.Context, name, description string, permissions []string) (*model.RoleDefinition, error) {
	var r model.RoleDefinition
	err := s.pool.QueryRow(ctx,
		`INSERT INTO roles (name, description, permissions, is_built_in)
		 VALUES ($1, $2, $3, false)
		 RETURNING id, name, description, permissions, is_built_in, created_at, updated_at`,
		name, description, permissions,
	).Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating role: %w", err)
	}
	return &r, nil
}

// Update modifies an existing custom role.
func (s *RoleStore) Update(ctx context.Context, name, description string, permissions []string) (*model.RoleDefinition, error) {
	var r model.RoleDefinition
	err := s.pool.QueryRow(ctx,
		`UPDATE roles SET description = $2, permissions = $3, updated_at = NOW()
		 WHERE name = $1 AND is_built_in = false
		 RETURNING id, name, description, permissions, is_built_in, created_at, updated_at`,
		name, description, permissions,
	).Scan(&r.ID, &r.Name, &r.Description, &r.Permissions, &r.IsBuiltIn, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("updating role: %w", err)
	}
	return &r, nil
}

// Delete removes a custom role. Returns an error if the role is built-in or in use.
func (s *RoleStore) Delete(ctx context.Context, name string) error {
	// Check if built-in
	var isBuiltIn bool
	err := s.pool.QueryRow(ctx,
		`SELECT is_built_in FROM roles WHERE name = $1`, name,
	).Scan(&isBuiltIn)
	if err != nil {
		return fmt.Errorf("role not found: %w", err)
	}
	if isBuiltIn {
		return fmt.Errorf("cannot delete built-in role")
	}

	// Check if in use by project_members
	var memberCount int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM project_members WHERE role = $1`, name,
	).Scan(&memberCount)
	if err != nil {
		return fmt.Errorf("checking role usage: %w", err)
	}
	if memberCount > 0 {
		return ErrRoleInUse
	}

	// Check if in use as base project role
	var baseRole string
	err = s.pool.QueryRow(ctx,
		`SELECT value FROM org_settings WHERE key = 'base_project_role'`,
	).Scan(&baseRole)
	if err == nil && baseRole == name {
		return ErrRoleInUse
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM roles WHERE name = $1 AND is_built_in = false`, name)
	if err != nil {
		return fmt.Errorf("deleting role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

// Exists returns true if a role with the given name exists.
func (s *RoleStore) Exists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, name,
	).Scan(&exists)
	return exists, err
}
```

Add the sentinel error to `internal/store/errors.go` (or wherever `ErrLastAdmin` is defined). Find the file first:

Look for `ErrLastAdmin` — it's likely in `internal/store/project_member_store.go` or a shared errors file. Add `ErrRoleInUse` next to it.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/store/ -run TestRoleStore -v`
Expected: PASS

**Step 5: Write additional tests (CRUD)**

Add to `role_store_test.go`:

```go
func TestRoleStore_CreateAndGet(t *testing.T) {
	pool := testPool(t)
	s := NewRoleStore(pool)
	ctx := context.Background()

	role, err := s.Create(ctx, "qa-engineer", "QA role", []string{"flags:read", "environments:read", "sdk_keys:manage"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if role.Name != "qa-engineer" {
		t.Errorf("got name %q, want %q", role.Name, "qa-engineer")
	}
	if role.IsBuiltIn {
		t.Error("custom role should not be built-in")
	}
	if len(role.Permissions) != 3 {
		t.Errorf("got %d permissions, want 3", len(role.Permissions))
	}

	got, err := s.GetByName(ctx, "qa-engineer")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name != role.Name {
		t.Errorf("got name %q, want %q", got.Name, role.Name)
	}

	// Cleanup
	_ = s.Delete(ctx, "qa-engineer")
}

func TestRoleStore_Update(t *testing.T) {
	pool := testPool(t)
	s := NewRoleStore(pool)
	ctx := context.Background()

	_, err := s.Create(ctx, "test-update-role", "before", []string{"flags:read"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := s.Update(ctx, "test-update-role", "after", []string{"flags:read", "flags:write"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "after" {
		t.Errorf("description = %q, want %q", updated.Description, "after")
	}
	if len(updated.Permissions) != 2 {
		t.Errorf("got %d permissions, want 2", len(updated.Permissions))
	}

	// Cannot update built-in role
	_, err = s.Update(ctx, "admin", "hacked", []string{"flags:read"})
	if err == nil {
		t.Error("expected error updating built-in role")
	}

	// Cleanup
	_ = s.Delete(ctx, "test-update-role")
}

func TestRoleStore_Delete(t *testing.T) {
	pool := testPool(t)
	s := NewRoleStore(pool)
	ctx := context.Background()

	// Cannot delete built-in role
	err := s.Delete(ctx, "admin")
	if err == nil {
		t.Error("expected error deleting built-in role")
	}

	// Create and delete a custom role
	_, err = s.Create(ctx, "delete-me", "temp", []string{"flags:read"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete(ctx, "delete-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify it's gone
	_, err = s.GetByName(ctx, "delete-me")
	if err == nil {
		t.Error("expected error getting deleted role")
	}
}

func TestRoleStore_Exists(t *testing.T) {
	pool := testPool(t)
	s := NewRoleStore(pool)
	ctx := context.Background()

	exists, err := s.Exists(ctx, "admin")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("admin role should exist")
	}

	exists, err = s.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("nonexistent role should not exist")
	}
}
```

**Step 6: Run all role store tests**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/store/ -run TestRoleStore -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/store/role_store.go internal/store/role_store_test.go
git commit -m "feat: add RoleStore with CRUD operations (#83)"
```

---

### Task 4: Role Cache

**Files:**
- Create: `internal/auth/role_cache.go`
- Create: `internal/auth/role_cache_test.go`

**Step 1: Write the failing test**

```go
package auth

import (
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func TestRoleCache(t *testing.T) {
	cache := NewRoleCache()

	// Load roles
	roles := []model.RoleDefinition{
		{Name: "admin", Permissions: []string{"flags:read", "flags:write", "project:settings"}, IsBuiltIn: true},
		{Name: "viewer", Permissions: []string{"flags:read"}, IsBuiltIn: true},
	}
	cache.Load(roles)

	// Check permission
	if !cache.HasPermission("admin", model.PermFlagsRead) {
		t.Error("admin should have flags:read")
	}
	if !cache.HasPermission("admin", model.PermProjectSettings) {
		t.Error("admin should have project:settings")
	}
	if cache.HasPermission("viewer", model.PermFlagsWrite) {
		t.Error("viewer should not have flags:write")
	}
	if cache.HasPermission("unknown", model.PermFlagsRead) {
		t.Error("unknown role should not have any permissions")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/auth/ -run TestRoleCache -v`
Expected: FAIL

**Step 3: Write the implementation**

```go
package auth

import (
	"sync"

	"github.com/togglerino/togglerino/internal/model"
)

// RoleCache provides fast in-memory permission lookups for project roles.
type RoleCache struct {
	mu    sync.RWMutex
	perms map[string]map[model.Permission]bool // role name -> permission set
}

// NewRoleCache creates a new empty RoleCache.
func NewRoleCache() *RoleCache {
	return &RoleCache{
		perms: make(map[string]map[model.Permission]bool),
	}
}

// Load replaces all cached roles with the given definitions.
func (c *RoleCache) Load(roles []model.RoleDefinition) {
	m := make(map[string]map[model.Permission]bool, len(roles))
	for _, r := range roles {
		perms := make(map[model.Permission]bool, len(r.Permissions))
		for _, p := range r.Permissions {
			perms[model.Permission(p)] = true
		}
		m[r.Name] = perms
	}
	c.mu.Lock()
	c.perms = m
	c.mu.Unlock()
}

// HasPermission returns true if the named role grants the given permission.
func (c *RoleCache) HasPermission(roleName string, perm model.Permission) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	perms, ok := c.perms[roleName]
	if !ok {
		return false
	}
	return perms[perm]
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/auth/ -run TestRoleCache -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/role_cache.go internal/auth/role_cache_test.go
git commit -m "feat: add in-memory role permission cache (#83)"
```

---

### Task 5: Refactor Permission Checking to Use RoleCache

This task replaces the hardcoded `projectRolePermissions` map with the dynamic `RoleCache`.

**Files:**
- Modify: `internal/model/permission.go` (lines 35-78)
- Modify: `internal/model/permission_test.go`
- Modify: `internal/auth/middleware.go` (line 99)
- Modify: `internal/auth/resolver.go` (lines 13-46)

**Step 1: Update `internal/model/permission.go`**

Keep the `ProjectRole` type and constants (they're useful as string constants for the built-in names). Remove `ValidProjectRole()`, `projectRolePermissions`, and `HasPermission()`. These are now handled by the cache.

Replace lines 35-78 with:

```go
// ValidProjectRole is deprecated — use RoleStore.Exists() for validation.
// Kept temporarily for backward compatibility during migration.
// TODO: remove after all callers are updated.
```

Actually, since we're doing this properly: remove `ValidProjectRole` and `projectRolePermissions` and `HasPermission` entirely. The callers that need these will be updated in subsequent steps.

**Step 2: Update the RoleResolver signature**

Change `internal/auth/middleware.go` line 39:

```go
// RoleResolver resolves a user's effective project role for a given project key.
type RoleResolver func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error)
```

This stays the same — it still returns a `ProjectRole` (string). The permission check moves to the middleware using the cache.

**Step 3: Update RequireProjectPermission to accept RoleCache**

In `internal/auth/middleware.go`, change `RequireProjectPermission` signature to also take a `*RoleCache`:

```go
func RequireProjectPermission(perm model.Permission, resolve RoleResolver, roleCache *RoleCache, projects ...*store.ProjectStore) func(http.Handler) http.Handler {
```

Replace line 99 (`if !role.HasPermission(perm)`) with:

```go
if !roleCache.HasPermission(string(role), perm) {
```

**Step 4: Update permission_test.go**

Remove `TestValidProjectRole` and `TestProjectRoleHasPermission` tests (the functionality moved to `RoleCache` which is already tested). Keep `TestRoleHasOrgPermission`.

**Step 5: Update all callers in `cmd/togglerino/main.go`**

Every call to `auth.RequireProjectPermission` needs the role cache argument. This will be wired in Task 7.

**Step 6: Verify compilation**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go build ./...`
Note: This may fail until main.go is updated in Task 7. That's OK — verify model and auth packages compile:
Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go build ./internal/model/... && go build ./internal/auth/...`

**Step 7: Run surviving tests**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/model/... ./internal/auth/...`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/model/permission.go internal/model/permission_test.go internal/auth/middleware.go
git commit -m "refactor: replace hardcoded permission maps with RoleCache (#83)"
```

---

### Task 6: Update OrgSettingsStore and ProjectMemberHandler Validation

**Files:**
- Modify: `internal/store/org_settings_store.go` (lines 36-41)
- Modify: `internal/handler/org_settings_handler.go` (lines 41-47)
- Modify: `internal/handler/project_member_handler.go` (lines 83-84, 142-143)

**Step 1: Update OrgSettingsStore.SetBaseProjectRole**

Replace the hardcoded switch (lines 36-41) with a DB check:

```go
func (s *OrgSettingsStore) SetBaseProjectRole(ctx context.Context, role string) error {
	if role != "none" {
		// Validate role exists in roles table
		var exists bool
		err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, role,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking role: %w", err)
		}
		if !exists {
			return fmt.Errorf("invalid base project role: %q", role)
		}
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO org_settings (key, value) VALUES ('base_project_role', $1)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		role,
	)
	if err != nil {
		return fmt.Errorf("setting base project role: %w", err)
	}
	return nil
}
```

**Step 2: Update OrgSettingsHandler.SetBaseProjectRole**

Replace the hardcoded switch (lines 41-47) — remove it entirely. The store now validates against the DB. Keep only a check for empty string:

```go
if req.BaseProjectRole == "" {
	writeError(w, http.StatusBadRequest, "base_project_role is required")
	return
}
```

**Step 3: Update ProjectMemberHandler role validation**

In `Add()` (line 83-84) and `Update()` (lines 142-143), replace `model.ValidProjectRole(req.Role)` with a DB check via the role store.

The handler needs a `*store.RoleStore` dependency. Update the struct and constructor:

```go
type ProjectMemberHandler struct {
	members  *store.ProjectMemberStore
	projects *store.ProjectStore
	users    *store.UserStore
	roles    *store.RoleStore
	audit    *store.AuditStore
}

func NewProjectMemberHandler(members *store.ProjectMemberStore, projects *store.ProjectStore, users *store.UserStore, roles *store.RoleStore, audit *store.AuditStore) *ProjectMemberHandler {
	return &ProjectMemberHandler{members: members, projects: projects, users: users, roles: roles, audit: audit}
}
```

Replace `model.ValidProjectRole(req.Role)` calls with:

```go
exists, err := h.roles.Exists(r.Context(), req.Role)
if err != nil || !exists {
	writeError(w, http.StatusBadRequest, "invalid role")
	return
}
```

**Step 4: Verify tests compile**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go vet ./internal/handler/... ./internal/store/...`
Expected: No errors (or only main.go wiring issues, fixed in Task 7)

**Step 5: Commit**

```bash
git add internal/store/org_settings_store.go internal/handler/org_settings_handler.go internal/handler/project_member_handler.go
git commit -m "refactor: validate roles against database instead of hardcoded list (#83)"
```

---

### Task 7: Role Handler and Main Wiring

**Files:**
- Create: `internal/handler/role_handler.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Write the role handler**

```go
package handler

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

var roleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,48}[a-z0-9]$`)

// RoleHandler manages role definition endpoints.
type RoleHandler struct {
	roles     *store.RoleStore
	roleCache *RoleCacheRefresher
}

// RoleCacheRefresher is called after role mutations to reload the cache.
type RoleCacheRefresher interface {
	Refresh()
}

// NewRoleHandler creates a new RoleHandler.
func NewRoleHandler(roles *store.RoleStore, refresher RoleCacheRefresher) *RoleHandler {
	return &RoleHandler{roles: roles, roleCache: refresher}
}

// List returns all roles.
// GET /api/v1/roles
func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	roles, err := h.roles.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list roles")
		return
	}
	if roles == nil {
		roles = []model.RoleDefinition{}
	}
	writeJSON(w, http.StatusOK, roles)
}

// Get returns a single role by name.
// GET /api/v1/roles/{name}
func (h *RoleHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	role, err := h.roles.GetByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	writeJSON(w, http.StatusOK, role)
}

// Create creates a new custom role.
// POST /api/v1/roles
func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(strings.ToLower(req.Name))

	if !roleNamePattern.MatchString(req.Name) {
		writeError(w, http.StatusBadRequest, "name must be 2-50 lowercase alphanumeric characters or hyphens")
		return
	}
	if len(req.Permissions) == 0 {
		writeError(w, http.StatusBadRequest, "at least one permission is required")
		return
	}
	for _, p := range req.Permissions {
		if !model.ValidPermission(p) {
			writeError(w, http.StatusBadRequest, "invalid permission: "+p)
			return
		}
	}

	role, err := h.roles.Create(r.Context(), req.Name, req.Description, req.Permissions)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			writeError(w, http.StatusConflict, "role name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create role")
		return
	}

	h.refreshCache()
	writeJSON(w, http.StatusCreated, role)
}

// Update modifies a custom role.
// PUT /api/v1/roles/{name}
func (h *RoleHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var req struct {
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check if built-in
	existing, err := h.roles.GetByName(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}
	if existing.IsBuiltIn {
		writeError(w, http.StatusForbidden, "cannot modify built-in role")
		return
	}

	if len(req.Permissions) == 0 {
		writeError(w, http.StatusBadRequest, "at least one permission is required")
		return
	}
	for _, p := range req.Permissions {
		if !model.ValidPermission(p) {
			writeError(w, http.StatusBadRequest, "invalid permission: "+p)
			return
		}
	}

	role, err := h.roles.Update(r.Context(), name, req.Description, req.Permissions)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	h.refreshCache()
	writeJSON(w, http.StatusOK, role)
}

// Delete removes a custom role.
// DELETE /api/v1/roles/{name}
func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	err := h.roles.Delete(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "built-in") {
			writeError(w, http.StatusForbidden, "cannot delete built-in role")
			return
		}
		if err == store.ErrRoleInUse {
			writeError(w, http.StatusConflict, "role is in use and cannot be deleted")
			return
		}
		writeError(w, http.StatusNotFound, "role not found")
		return
	}

	h.refreshCache()
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoleHandler) refreshCache() {
	if h.roleCache != nil {
		h.roleCache.Refresh()
	}
}
```

Note: The `RoleCacheRefresher` interface decouples the handler from the cache. We'll implement a concrete refresher in main.go.

**Step 2: Wire everything in main.go**

Add to store init section (after line 77):

```go
roleStore := store.NewRoleStore(pool)
```

Create and load the role cache (after the cache.LoadAll block, ~line 118):

```go
// Load role definitions into cache
roleCache := auth.NewRoleCache()
allRoles, err := roleStore.List(ctx)
if err != nil {
	log.Fatalf("failed to load roles: %v", err)
}
roleCache.Load(allRoles)
```

Create a refresher adapter:

```go
type roleCacheRefresher struct {
	store *store.RoleStore
	cache *auth.RoleCache
}

func (r *roleCacheRefresher) Refresh() {
	roles, err := r.store.List(context.Background())
	if err != nil {
		slog.Error("failed to refresh role cache", "error", err)
		return
	}
	r.cache.Load(roles)
}
```

Update handler init:

```go
roleHandler := handler.NewRoleHandler(roleStore, &roleCacheRefresher{store: roleStore, cache: roleCache})
```

Update `NewProjectMemberHandler` call to pass `roleStore`:

```go
projectMemberHandler := handler.NewProjectMemberHandler(projectMemberStore, projectStore, userStore, roleStore, auditStore)
```

Update all `auth.RequireProjectPermission` calls to pass `roleCache`:

```go
requireFlagsRead := auth.RequireProjectPermission(model.PermFlagsRead, roleResolver, roleCache, projectStore)
// ... same for all other RequireProjectPermission calls
```

Add role routes (after org settings routes, ~line 333):

```go
// Roles (admin-only)
mux.Handle("GET /api/v1/roles", wrap(roleHandler.List, sessionAuth, requireOrgUsersManage))
mux.Handle("POST /api/v1/roles", wrap(roleHandler.Create, sessionAuth, requireOrgUsersManage))
mux.Handle("GET /api/v1/roles/{name}", wrap(roleHandler.Get, sessionAuth, requireOrgUsersManage))
mux.Handle("PUT /api/v1/roles/{name}", wrap(roleHandler.Update, sessionAuth, requireOrgUsersManage))
mux.Handle("DELETE /api/v1/roles/{name}", wrap(roleHandler.Delete, sessionAuth, requireOrgUsersManage))
```

**Step 3: Verify everything compiles**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go build ./...`
Expected: No errors

**Step 4: Run all non-DB tests**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./internal/model/... ./internal/auth/... ./internal/evaluation/...`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/handler/role_handler.go cmd/togglerino/main.go
git commit -m "feat: add role handler and wire custom roles into main (#83)"
```

---

### Task 8: Update `usePermissions` Hook and Role Fetching

**Files:**
- Modify: `web/src/hooks/usePermissions.ts`
- Create: `web/src/hooks/useRoles.ts`

**Step 1: Create useRoles hook**

```typescript
// web/src/hooks/useRoles.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

export interface RoleDefinition {
  id: string
  name: string
  description: string
  permissions: string[]
  is_built_in: boolean
  created_at: string
  updated_at: string
}

export function useRoles() {
  return useQuery({
    queryKey: ['roles'],
    queryFn: () => api.get<RoleDefinition[]>('/roles'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useRole(name: string) {
  return useQuery({
    queryKey: ['roles', name],
    queryFn: () => api.get<RoleDefinition>(`/roles/${name}`),
    enabled: !!name,
  })
}

export function useCreateRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description: string; permissions: string[] }) =>
      api.post<RoleDefinition>('/roles', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}

export function useUpdateRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, ...data }: { name: string; description: string; permissions: string[] }) =>
      api.put<RoleDefinition>(`/roles/${name}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}

export function useDeleteRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.delete(`/roles/${name}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}
```

**Step 2: Update usePermissions.ts**

Change `ProjectRole` type from union to string:

```typescript
export type ProjectRole = string
```

Update `useCanWrite` and `useIsProjectAdmin` — these can no longer check role names directly since custom roles may also have write/admin permissions. Instead, add a server-side permissions endpoint. For now, keep checking the role name for built-in roles but also accept roles with the relevant permissions:

Actually, since the server already resolves the role and the middleware checks permissions, the simplest approach is: `useCanWrite` and `useIsProjectAdmin` should query the server for the user's effective permissions. But `useProjectRole` already fetches the role name. We need the permissions too.

Update the server endpoint `GET /api/v1/auth/me/project-role/{key}` to also return permissions. Then:

```typescript
interface ProjectRoleResponse {
  role: string
  permissions: string[]
}

export function useProjectRole(projectKey: string | undefined): ProjectRole | null {
  const { data } = useQuery({
    queryKey: ['my-project-role', projectKey],
    queryFn: () => api.get<ProjectRoleResponse>(`/auth/me/project-role/${projectKey}`),
    enabled: !!projectKey,
    staleTime: 5 * 60 * 1000,
  })

  if (!data) return null
  if (data.role === 'none') return null
  return data.role
}

export function useProjectPermissions(projectKey: string | undefined): string[] {
  const { data } = useQuery({
    queryKey: ['my-project-role', projectKey],
    queryFn: () => api.get<ProjectRoleResponse>(`/auth/me/project-role/${projectKey}`),
    enabled: !!projectKey,
    staleTime: 5 * 60 * 1000,
  })

  return data?.permissions ?? []
}

export function useCanWrite(projectKey: string | undefined): boolean {
  const perms = useProjectPermissions(projectKey)
  return perms.includes('flags:write')
}

export function useIsProjectAdmin(projectKey: string | undefined): boolean {
  const perms = useProjectPermissions(projectKey)
  return perms.includes('project:settings')
}
```

**Step 3: Update MyRoleHandler to return permissions**

Find and modify the `MyRoleHandler` (in `internal/handler/`) to also return the role's permissions from the cache:

The handler needs access to `RoleCache`. Update its constructor and response to include permissions.

**Step 4: Verify frontend compiles**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles/web && npx tsc --noEmit`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/hooks/useRoles.ts web/src/hooks/usePermissions.ts
git commit -m "feat: add useRoles hook and permission-based access checks (#83)"
```

---

### Task 9: Update Role Selectors (MembersTab + TeamPage)

**Files:**
- Modify: `web/src/pages/settings/MembersTab.tsx` (line 66, lines 236-240, lines 327-331)
- Modify: `web/src/pages/TeamPage.tsx` (line 54)

**Step 1: Update MembersTab.tsx**

Replace hardcoded `roleOptions` (line 66) with dynamic query:

```typescript
// Remove: const roleOptions: ProjectRole[] = ['admin', 'editor', 'viewer']
// Add inside the component:
const { data: roles } = useRoles()
const roleOptions = (roles ?? []).map(r => r.name)
```

Import `useRoles` from `@/hooks/useRoles`.

Update the `roleBadgeVariant` to handle custom roles:

```typescript
function roleBadgeVariant(role: string): 'secondary' | 'outline' | 'default' {
  if (role === 'admin') return 'secondary'
  return 'outline'
}
```

(This already works — custom roles get 'outline' by default.)

**Step 2: Update TeamPage.tsx**

Replace hardcoded `projectRoleOptions` (line 54) the same way:

```typescript
// Remove: const projectRoleOptions: ProjectRole[] = ['admin', 'editor', 'viewer']
// Add inside the component or as a hook call:
const { data: roles } = useRoles()
const projectRoleOptions = (roles ?? []).map(r => r.name)
```

Import `useRoles`.

**Step 3: Verify frontend compiles**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles/web && npx tsc --noEmit`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/pages/settings/MembersTab.tsx web/src/pages/TeamPage.tsx
git commit -m "feat: use dynamic role list in member and team role selectors (#83)"
```

---

### Task 10: Roles Management Page

**Files:**
- Create: `web/src/pages/RolesPage.tsx`
- Modify: `web/src/App.tsx` (add route)
- Modify: `web/src/components/OrgLayout.tsx` (add nav link)

**Step 1: Create RolesPage**

Create `web/src/pages/RolesPage.tsx` — a dedicated admin page with:
- Table listing all roles (name, description, permission count, built-in badge)
- "Create Role" button opening a dialog
- Role builder dialog: name input, description textarea, 8 permission checkboxes
- Edit button on custom roles (opens same dialog in edit mode)
- Delete button on custom roles (confirmation dialog, handles 409)
- Built-in roles shown as read-only

The page should follow the same patterns as other admin pages (Card layout, shadcn components, font-mono labels, accent color).

Permission checkboxes should display user-friendly labels:

```typescript
const permissionLabels: Record<string, string> = {
  'flags:read': 'View flags',
  'flags:write': 'Create & edit flags',
  'environments:read': 'View environments',
  'environments:write': 'Create environments',
  'sdk_keys:manage': 'Manage SDK keys',
  'segments:write': 'Create & edit segments',
  'templates:manage': 'Manage templates',
  'project:settings': 'Project settings',
}
```

Use `useRoles`, `useCreateRole`, `useUpdateRole`, `useDeleteRole` hooks from `useRoles.ts`.

**Step 2: Add route in App.tsx**

After line 93 (`<Route path="/settings" end ...>`), add:

```tsx
<Route path="/settings/roles" element={<RolesPage />} />
```

Import `RolesPage`.

**Step 3: Add nav link in OrgLayout.tsx**

After the Settings NavLink (line 23), add:

```tsx
{isAdmin && (
  <NavLink to="/settings/roles" className={navLinkClass} onClick={onNavigate}>Roles</NavLink>
)}
```

**Step 4: Verify frontend compiles and lint**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles/web && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/pages/RolesPage.tsx web/src/App.tsx web/src/components/OrgLayout.tsx
git commit -m "feat: add roles management page with permission matrix (#83)"
```

---

### Task 11: Update OrgSettingsHandler for Dynamic Role Validation

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx` (or wherever base-project-role selector lives)

**Step 1: Update base project role selector**

The base project role selector currently likely uses a hardcoded list. Update it to fetch from `GET /api/v1/roles` and include "none" as a special option.

```typescript
const { data: roles } = useRoles()
const baseRoleOptions = [...(roles ?? []).map(r => r.name), 'none']
```

**Step 2: Verify frontend compiles**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles/web && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat: use dynamic role list in base project role selector (#83)"
```

---

### Task 12: Integration Testing and Final Verification

**Step 1: Start dev environment**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && ./dev.sh`

This runs migrations (including 022_custom_roles) and starts the backend.

**Step 2: Run all Go tests**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles && go test ./...`
Expected: PASS (all tests including store tests with DB)

**Step 3: Run frontend lint and type check**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles/web && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 4: Build the full binary**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/custom-roles/web && npm run build && cd .. && go build -o togglerino ./cmd/togglerino`
Expected: Binary builds successfully

**Step 5: Commit any remaining fixes**

```bash
git add -A
git commit -m "fix: integration fixes for custom roles (#83)"
```

---

## Task Dependency Graph

```
Task 1 (migration) ─────────────────────────────────┐
Task 2 (model) ──────────┐                          │
Task 3 (store) ──────────┤ ← depends on 1, 2       │
Task 4 (cache) ──────────┤ ← depends on 2           │
Task 5 (refactor perms) ←┴ depends on 4             │
Task 6 (validation) ───── depends on 3, 5           │
Task 7 (handler+wiring)── depends on 3, 4, 5, 6    │
Task 8 (frontend hooks)── depends on 7              │
Task 9 (role selectors)── depends on 8              │
Task 10 (roles page) ──── depends on 8              │
Task 11 (settings page)── depends on 8              │
Task 12 (integration) ─── depends on all            │
```

Tasks 9, 10, 11 can run in parallel after Task 8.
