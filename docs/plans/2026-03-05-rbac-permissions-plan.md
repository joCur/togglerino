# RBAC: Built-in Roles & Project-Scoped Permissions — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add granular permissions with built-in project roles (admin/editor/viewer), org-wide base project role setting, and per-project role overrides.

**Architecture:** New permission model with two scopes: org-level (derived from global admin/member role) and project-level (derived from project member assignment or base project role). Middleware-based enforcement extracts project key from URL, resolves effective role, checks permission. Frontend gates UI elements based on permissions returned from `/auth/me`.

**Tech Stack:** Go stdlib `net/http`, pgx/v5, React 19 + TanStack Query + shadcn/ui

**Design doc:** `docs/plans/2026-03-05-rbac-permissions-design.md`

---

## Task 1: Permission Model & Project Role Types

**Files:**
- Create: `internal/model/permission.go`
- Test: `internal/model/permission_test.go`

**Step 1: Write the test**

```go
// internal/model/permission_test.go
package model_test

import (
	"testing"

	"github.com/togglerino/togglerino/internal/model"
)

func TestProjectRolePermissions(t *testing.T) {
	tests := []struct {
		role       model.ProjectRole
		perm       model.Permission
		wantAllow  bool
	}{
		{model.ProjectRoleAdmin, model.PermFlagsRead, true},
		{model.ProjectRoleAdmin, model.PermFlagsWrite, true},
		{model.ProjectRoleAdmin, model.PermProjectSettings, true},
		{model.ProjectRoleEditor, model.PermFlagsRead, true},
		{model.ProjectRoleEditor, model.PermFlagsWrite, true},
		{model.ProjectRoleEditor, model.PermProjectSettings, false},
		{model.ProjectRoleViewer, model.PermFlagsRead, true},
		{model.ProjectRoleViewer, model.PermFlagsWrite, false},
		{model.ProjectRoleViewer, model.PermProjectSettings, false},
	}
	for _, tt := range tests {
		got := tt.role.HasPermission(tt.perm)
		if got != tt.wantAllow {
			t.Errorf("role %q permission %q: got %v, want %v", tt.role, tt.perm, got, tt.wantAllow)
		}
	}
}

func TestOrgPermissions(t *testing.T) {
	tests := []struct {
		role      model.Role
		perm      model.Permission
		wantAllow bool
	}{
		{model.RoleAdmin, model.PermOrgUsersManage, true},
		{model.RoleAdmin, model.PermOrgOIDCManage, true},
		{model.RoleAdmin, model.PermOrgProjectsCreate, true},
		{model.RoleAdmin, model.PermOrgProjectsDelete, true},
		{model.RoleMember, model.PermOrgUsersManage, false},
		{model.RoleMember, model.PermOrgProjectsCreate, false},
	}
	for _, tt := range tests {
		got := tt.role.HasOrgPermission(tt.perm)
		if got != tt.wantAllow {
			t.Errorf("role %q permission %q: got %v, want %v", tt.role, tt.perm, got, tt.wantAllow)
		}
	}
}

func TestValidProjectRole(t *testing.T) {
	if !model.ValidProjectRole("admin") {
		t.Error("admin should be valid")
	}
	if !model.ValidProjectRole("editor") {
		t.Error("editor should be valid")
	}
	if !model.ValidProjectRole("viewer") {
		t.Error("viewer should be valid")
	}
	if model.ValidProjectRole("superadmin") {
		t.Error("superadmin should be invalid")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/model/... -run TestProjectRolePermissions -v`
Expected: FAIL — `permission.go` doesn't exist yet.

**Step 3: Write minimal implementation**

```go
// internal/model/permission.go
package model

type Permission string

// Organization-level permissions
const (
	PermOrgUsersManage   Permission = "org:users:manage"
	PermOrgOIDCManage    Permission = "org:oidc:manage"
	PermOrgProjectsCreate Permission = "org:projects:create"
	PermOrgProjectsDelete Permission = "org:projects:delete"
)

// Project-level permissions
const (
	PermFlagsRead        Permission = "flags:read"
	PermFlagsWrite       Permission = "flags:write"
	PermEnvironmentsRead Permission = "environments:read"
	PermEnvironmentsWrite Permission = "environments:write"
	PermSDKKeysManage    Permission = "sdk_keys:manage"
	PermSegmentsWrite    Permission = "segments:write"
	PermTemplatesManage  Permission = "templates:manage"
	PermProjectSettings  Permission = "project:settings"
)

type ProjectRole string

const (
	ProjectRoleAdmin  ProjectRole = "admin"
	ProjectRoleEditor ProjectRole = "editor"
	ProjectRoleViewer ProjectRole = "viewer"
)

func ValidProjectRole(s string) bool {
	switch ProjectRole(s) {
	case ProjectRoleAdmin, ProjectRoleEditor, ProjectRoleViewer:
		return true
	}
	return false
}

var projectRolePermissions = map[ProjectRole]map[Permission]bool{
	ProjectRoleAdmin: {
		PermFlagsRead:         true,
		PermFlagsWrite:        true,
		PermEnvironmentsRead:  true,
		PermEnvironmentsWrite: true,
		PermSDKKeysManage:     true,
		PermSegmentsWrite:     true,
		PermTemplatesManage:   true,
		PermProjectSettings:   true,
	},
	ProjectRoleEditor: {
		PermFlagsRead:         true,
		PermFlagsWrite:        true,
		PermEnvironmentsRead:  true,
		PermEnvironmentsWrite: true,
		PermSDKKeysManage:     true,
		PermSegmentsWrite:     true,
		PermTemplatesManage:   true,
	},
	ProjectRoleViewer: {
		PermFlagsRead:        true,
		PermEnvironmentsRead: true,
	},
}

func (r ProjectRole) HasPermission(p Permission) bool {
	perms, ok := projectRolePermissions[r]
	if !ok {
		return false
	}
	return perms[p]
}

var orgRolePermissions = map[Role]map[Permission]bool{
	RoleAdmin: {
		PermOrgUsersManage:    true,
		PermOrgOIDCManage:     true,
		PermOrgProjectsCreate: true,
		PermOrgProjectsDelete: true,
	},
}

func (r Role) HasOrgPermission(p Permission) bool {
	perms, ok := orgRolePermissions[r]
	if !ok {
		return false
	}
	return perms[p]
}
```

**Step 4: Run tests**

Run: `go test ./internal/model/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/model/permission.go internal/model/permission_test.go
git commit -m "feat(rbac): add permission model and project role types"
```

---

## Task 2: Database Migration — `org_settings` and `project_members` Tables

**Files:**
- Create: `migrations/016_rbac.up.sql`
- Create: `migrations/016_rbac.down.sql`

**Step 1: Write the migration**

```sql
-- migrations/016_rbac.up.sql
CREATE TABLE org_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO org_settings (key, value) VALUES ('base_project_role', 'editor');

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'viewer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_members_user_id ON project_members(user_id);
```

```sql
-- migrations/016_rbac.down.sql
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS org_settings;
```

**Step 2: Run migration**

Run: `go run ./cmd/togglerino` (migrations run on startup)
Expected: Server starts, new tables exist. Verify: `psql postgres://togglerino:togglerino@localhost:5432/togglerino -c "SELECT * FROM org_settings;"` returns `base_project_role | editor`.

**Step 3: Commit**

```bash
git add migrations/016_rbac.up.sql migrations/016_rbac.down.sql
git commit -m "feat(rbac): add org_settings and project_members tables"
```

---

## Task 3: Org Settings Store

**Files:**
- Create: `internal/store/org_settings_store.go`
- Create: `internal/store/org_settings_store_test.go`

**Step 1: Write the test**

```go
// internal/store/org_settings_store_test.go
package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/store"
)

func TestOrgSettingsStore_GetBaseProjectRole(t *testing.T) {
	pool := testPool(t)
	s := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	role, err := s.GetBaseProjectRole(ctx)
	if err != nil {
		t.Fatalf("GetBaseProjectRole: %v", err)
	}
	if role != "editor" {
		t.Errorf("default base project role: got %q, want %q", role, "editor")
	}
}

func TestOrgSettingsStore_SetBaseProjectRole(t *testing.T) {
	pool := testPool(t)
	s := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	if err := s.SetBaseProjectRole(ctx, "viewer"); err != nil {
		t.Fatalf("SetBaseProjectRole: %v", err)
	}

	role, err := s.GetBaseProjectRole(ctx)
	if err != nil {
		t.Fatalf("GetBaseProjectRole: %v", err)
	}
	if role != "viewer" {
		t.Errorf("got %q, want %q", role, "viewer")
	}

	// Reset for other tests
	if err := s.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("reset: %v", err)
	}
}

func TestOrgSettingsStore_SetBaseProjectRole_None(t *testing.T) {
	pool := testPool(t)
	s := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	if err := s.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("SetBaseProjectRole none: %v", err)
	}

	role, err := s.GetBaseProjectRole(ctx)
	if err != nil {
		t.Fatalf("GetBaseProjectRole: %v", err)
	}
	if role != "none" {
		t.Errorf("got %q, want %q", role, "none")
	}

	// Reset
	if err := s.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("reset: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestOrgSettingsStore -v`
Expected: FAIL — store doesn't exist.

**Step 3: Write implementation**

```go
// internal/store/org_settings_store.go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrgSettingsStore struct {
	pool *pgxpool.Pool
}

func NewOrgSettingsStore(pool *pgxpool.Pool) *OrgSettingsStore {
	return &OrgSettingsStore{pool: pool}
}

func (s *OrgSettingsStore) GetBaseProjectRole(ctx context.Context) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx,
		`SELECT value FROM org_settings WHERE key = 'base_project_role'`,
	).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get base project role: %w", err)
	}
	return value, nil
}

func (s *OrgSettingsStore) SetBaseProjectRole(ctx context.Context, role string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO org_settings (key, value) VALUES ('base_project_role', $1)
		 ON CONFLICT (key) DO UPDATE SET value = $1`,
		role,
	)
	if err != nil {
		return fmt.Errorf("set base project role: %w", err)
	}
	return nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/store/... -run TestOrgSettingsStore -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/org_settings_store.go internal/store/org_settings_store_test.go
git commit -m "feat(rbac): add org settings store for base project role"
```

---

## Task 4: Project Members Store

**Files:**
- Create: `internal/store/project_member_store.go`
- Create: `internal/store/project_member_store_test.go`
- Create: `internal/model/project_member.go`

**Step 1: Write the model**

```go
// internal/model/project_member.go
package model

import "time"

type ProjectMember struct {
	ProjectID string      `json:"project_id"`
	UserID    string      `json:"user_id"`
	Role      ProjectRole `json:"role"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ProjectMemberWithUser is returned when listing members for a project
type ProjectMemberWithUser struct {
	ProjectID   string      `json:"project_id"`
	UserID      string      `json:"user_id"`
	Role        ProjectRole `json:"role"`
	Email       string      `json:"email"`
	DisplayName *string     `json:"display_name,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// UserProjectAssignment is returned when listing project assignments for a user
type UserProjectAssignment struct {
	ProjectID  string      `json:"project_id"`
	ProjectKey string      `json:"project_key"`
	ProjectName string     `json:"project_name"`
	Role       ProjectRole `json:"role"`
}
```

**Step 2: Write the test**

```go
// internal/store/project_member_store_test.go
package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func uniqueProjectKey(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func setupProjectAndUser(t *testing.T, pool interface{ Exec(ctx context.Context, sql string, args ...any) (interface{}, error) }, us *store.UserStore, ps *store.ProjectStore) (string, string) {
	t.Helper()
	ctx := context.Background()
	email := uniqueEmail("pm")
	user, err := us.Create(ctx, email, "hashedpw", model.RoleMember)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	key := uniqueProjectKey("pm")
	project, err := ps.Create(ctx, key, key+" Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project.ID, user.ID
}

func TestProjectMemberStore_AddAndGet(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	us := store.NewUserStore(pool)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	projectID, userID := setupProjectAndUser(t, pool, us, ps)

	member, err := pms.Add(ctx, projectID, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if member.Role != model.ProjectRoleEditor {
		t.Errorf("role: got %q, want %q", member.Role, model.ProjectRoleEditor)
	}

	got, err := pms.GetRole(ctx, projectID, userID)
	if err != nil {
		t.Fatalf("GetRole: %v", err)
	}
	if got != model.ProjectRoleEditor {
		t.Errorf("GetRole: got %q, want %q", got, model.ProjectRoleEditor)
	}
}

func TestProjectMemberStore_GetRole_NotFound(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	ctx := context.Background()

	_, err := pms.GetRole(ctx, "00000000-0000-0000-0000-000000000000", "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Error("expected error for nonexistent member")
	}
}

func TestProjectMemberStore_Update(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	us := store.NewUserStore(pool)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	projectID, userID := setupProjectAndUser(t, pool, us, ps)

	_, err := pms.Add(ctx, projectID, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := pms.Update(ctx, projectID, userID, model.ProjectRoleViewer)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Role != model.ProjectRoleViewer {
		t.Errorf("role: got %q, want %q", updated.Role, model.ProjectRoleViewer)
	}
}

func TestProjectMemberStore_Remove(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	us := store.NewUserStore(pool)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	projectID, userID := setupProjectAndUser(t, pool, us, ps)

	_, err := pms.Add(ctx, projectID, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := pms.Remove(ctx, projectID, userID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	_, err = pms.GetRole(ctx, projectID, userID)
	if err == nil {
		t.Error("expected error after removal")
	}
}

func TestProjectMemberStore_ListByProject(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	us := store.NewUserStore(pool)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	projectID, userID := setupProjectAndUser(t, pool, us, ps)

	_, err := pms.Add(ctx, projectID, userID, model.ProjectRoleEditor)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	members, err := pms.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(members) == 0 {
		t.Error("expected at least one member")
	}
	found := false
	for _, m := range members {
		if m.UserID == userID {
			found = true
			if m.Role != model.ProjectRoleEditor {
				t.Errorf("role: got %q, want %q", m.Role, model.ProjectRoleEditor)
			}
		}
	}
	if !found {
		t.Error("added user not found in member list")
	}
}

func TestProjectMemberStore_ListAccessibleProjectIDs(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	us := store.NewUserStore(pool)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	projectID, userID := setupProjectAndUser(t, pool, us, ps)

	_, err := pms.Add(ctx, projectID, userID, model.ProjectRoleViewer)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	ids, err := pms.ListAccessibleProjectIDs(ctx, userID)
	if err != nil {
		t.Fatalf("ListAccessibleProjectIDs: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == projectID {
			found = true
		}
	}
	if !found {
		t.Error("expected project in accessible list")
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestProjectMemberStore -v`
Expected: FAIL

**Step 4: Write implementation**

```go
// internal/store/project_member_store.go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type ProjectMemberStore struct {
	pool *pgxpool.Pool
}

func NewProjectMemberStore(pool *pgxpool.Pool) *ProjectMemberStore {
	return &ProjectMemberStore{pool: pool}
}

func (s *ProjectMemberStore) Add(ctx context.Context, projectID, userID string, role model.ProjectRole) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := s.pool.QueryRow(ctx,
		`INSERT INTO project_members (project_id, user_id, role)
		 VALUES ($1, $2, $3)
		 RETURNING project_id, user_id, role, created_at, updated_at`,
		projectID, userID, string(role),
	).Scan(&m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("add project member: %w", err)
	}
	return &m, nil
}

func (s *ProjectMemberStore) GetRole(ctx context.Context, projectID, userID string) (model.ProjectRole, error) {
	var role model.ProjectRole
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("project member not found")
		}
		return "", fmt.Errorf("get project member role: %w", err)
	}
	return role, nil
}

func (s *ProjectMemberStore) Update(ctx context.Context, projectID, userID string, role model.ProjectRole) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := s.pool.QueryRow(ctx,
		`UPDATE project_members SET role = $3, updated_at = NOW()
		 WHERE project_id = $1 AND user_id = $2
		 RETURNING project_id, user_id, role, created_at, updated_at`,
		projectID, userID, string(role),
	).Scan(&m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update project member: %w", err)
	}
	return &m, nil
}

func (s *ProjectMemberStore) Remove(ctx context.Context, projectID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove project member: %w", err)
	}
	return nil
}

func (s *ProjectMemberStore) ListByProject(ctx context.Context, projectID string) ([]model.ProjectMemberWithUser, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT pm.project_id, pm.user_id, pm.role, u.email, u.display_name, pm.created_at, pm.updated_at
		 FROM project_members pm
		 JOIN users u ON u.id = pm.user_id
		 WHERE pm.project_id = $1
		 ORDER BY u.email`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer rows.Close()

	var members []model.ProjectMemberWithUser
	for rows.Next() {
		var m model.ProjectMemberWithUser
		if err := rows.Scan(&m.ProjectID, &m.UserID, &m.Role, &m.Email, &m.DisplayName, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		members = append(members, m)
	}
	return members, nil
}

func (s *ProjectMemberStore) ListByUser(ctx context.Context, userID string) ([]model.UserProjectAssignment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT pm.project_id, p.key, p.name, pm.role
		 FROM project_members pm
		 JOIN projects p ON p.id = pm.project_id
		 WHERE pm.user_id = $1
		 ORDER BY p.name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list user project assignments: %w", err)
	}
	defer rows.Close()

	var assignments []model.UserProjectAssignment
	for rows.Next() {
		var a model.UserProjectAssignment
		if err := rows.Scan(&a.ProjectID, &a.ProjectKey, &a.ProjectName, &a.Role); err != nil {
			return nil, fmt.Errorf("scan user project assignment: %w", err)
		}
		assignments = append(assignments, a)
	}
	return assignments, nil
}

func (s *ProjectMemberStore) ListAccessibleProjectIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT project_id FROM project_members WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list accessible project IDs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project ID: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/store/... -run TestProjectMemberStore -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/model/project_member.go internal/store/project_member_store.go internal/store/project_member_store_test.go
git commit -m "feat(rbac): add project member store and model"
```

---

## Task 5: Permission Middleware

**Files:**
- Modify: `internal/auth/middleware.go`
- Create: `internal/auth/middleware_test.go`

This task adds two new middleware functions: `RequireOrgPermission` and `RequireProjectPermission`. The project permission middleware needs to resolve the effective project role by: (1) checking if user is global admin (bypass), (2) looking up project-specific override, (3) falling back to base project role.

**Step 1: Write the test**

```go
// internal/auth/middleware_test.go
package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
)

func requestWithUser(role model.Role) *http.Request {
	user := &model.User{ID: "user-1", Email: "test@test.com", Role: role}
	ctx := auth.ContextWithUser(context.Background(), user)
	return httptest.NewRequest("GET", "/", nil).WithContext(ctx)
}

func TestRequireOrgPermission_AdminAllowed(t *testing.T) {
	handler := auth.RequireOrgPermission(model.PermOrgUsersManage)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, requestWithUser(model.RoleAdmin))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireOrgPermission_MemberDenied(t *testing.T) {
	handler := auth.RequireOrgPermission(model.PermOrgUsersManage)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, requestWithUser(model.RoleMember))

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireProjectPermission_AdminBypasses(t *testing.T) {
	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return "", fmt.Errorf("should not be called for admin")
	}

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	w := httptest.NewRecorder()
	req := requestWithUser(model.RoleAdmin)
	req.SetPathValue("key", "my-project")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireProjectPermission_EditorAllowedWrite(t *testing.T) {
	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return model.ProjectRoleEditor, nil
	}

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	w := httptest.NewRecorder()
	req := requestWithUser(model.RoleMember)
	req.SetPathValue("key", "my-project")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireProjectPermission_ViewerDeniedWrite(t *testing.T) {
	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return model.ProjectRoleViewer, nil
	}

	handler := auth.RequireProjectPermission(model.PermFlagsWrite, resolver)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	w := httptest.NewRecorder()
	req := requestWithUser(model.RoleMember)
	req.SetPathValue("key", "my-project")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireProjectPermission_NoAccess(t *testing.T) {
	resolver := func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		return "", fmt.Errorf("no access")
	}

	handler := auth.RequireProjectPermission(model.PermFlagsRead, resolver)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	w := httptest.NewRecorder()
	req := requestWithUser(model.RoleMember)
	req.SetPathValue("key", "my-project")
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}
```

Note: The test uses a `resolver` function to decouple the middleware from the store. The resolver is a `func(ctx, projectKey, userID) (ProjectRole, error)` that encapsulates the role resolution logic. The middleware also needs a `ContextWithUser` export — add this to `middleware.go`.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestRequireOrgPermission -v`
Expected: FAIL

**Step 3: Write implementation**

Add to `internal/auth/middleware.go`:

```go
// Add exported function to set user in context (needed for tests and handlers)
func ContextWithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// RoleResolver resolves the effective project role for a user in a project.
// Returns the role if the user has access, or an error if no access.
type RoleResolver func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error)

func RequireOrgPermission(perm model.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || !user.Role.HasOrgPermission(perm) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireProjectPermission(perm model.Permission, resolve RoleResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Global admins bypass project permission checks
			if user.Role == model.RoleAdmin {
				next.ServeHTTP(w, r)
				return
			}

			projectKey := r.PathValue("key")
			if projectKey == "" {
				http.Error(w, `{"error":"project key required"}`, http.StatusBadRequest)
				return
			}

			role, err := resolve(r.Context(), projectKey, user.ID)
			if err != nil {
				// No access — return 404 to avoid leaking project existence
				http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
				return
			}

			if !role.HasPermission(perm) {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/auth/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go
git commit -m "feat(rbac): add org and project permission middleware"
```

---

## Task 6: Role Resolver — Wiring Store to Middleware

**Files:**
- Create: `internal/auth/resolver.go`
- Create: `internal/auth/resolver_test.go`

The resolver connects the middleware to the stores. It looks up the user's project-specific role, falls back to base project role, and returns the effective role.

**Step 1: Write the test**

```go
// internal/auth/resolver_test.go
package auth_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestBuildRoleResolver_ExplicitMembership(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	ps := store.NewProjectStore(pool)
	us := store.NewUserStore(pool)
	oss := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	// Setup
	email := uniqueEmail("resolver")
	user, err := us.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	project, err := ps.Create(ctx, uniqueProjectKey("resolver"), "Resolver Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = pms.Add(ctx, project.ID, user.ID, model.ProjectRoleAdmin)
	if err != nil {
		t.Fatalf("add member: %v", err)
	}

	resolve := auth.BuildRoleResolver(pms, ps, oss)
	role, err := resolve(ctx, project.Key, user.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if role != model.ProjectRoleAdmin {
		t.Errorf("role: got %q, want %q", role, model.ProjectRoleAdmin)
	}
}

func TestBuildRoleResolver_FallbackToBaseRole(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	ps := store.NewProjectStore(pool)
	us := store.NewUserStore(pool)
	oss := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	email := uniqueEmail("resolver-base")
	user, err := us.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	project, err := ps.Create(ctx, uniqueProjectKey("resolver-base"), "Base Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Ensure base role is editor
	if err := oss.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("set base role: %v", err)
	}

	resolve := auth.BuildRoleResolver(pms, ps, oss)
	role, err := resolve(ctx, project.Key, user.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if role != model.ProjectRoleEditor {
		t.Errorf("role: got %q, want %q", role, model.ProjectRoleEditor)
	}
}

func TestBuildRoleResolver_BaseRoleNone_NoAccess(t *testing.T) {
	pool := testPool(t)
	pms := store.NewProjectMemberStore(pool)
	ps := store.NewProjectStore(pool)
	us := store.NewUserStore(pool)
	oss := store.NewOrgSettingsStore(pool)
	ctx := context.Background()

	email := uniqueEmail("resolver-none")
	user, err := us.Create(ctx, email, "hash", model.RoleMember)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	project, err := ps.Create(ctx, uniqueProjectKey("resolver-none"), "None Project")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if err := oss.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("set base role: %v", err)
	}

	resolve := auth.BuildRoleResolver(pms, ps, oss)
	_, err = resolve(ctx, project.Key, user.ID)
	if err == nil {
		t.Error("expected error for no-access user")
	}

	// Reset
	if err := oss.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("reset: %v", err)
	}
}
```

Note: The resolver tests need a `testPool` helper and `uniqueEmail`/`uniqueProjectKey` in the auth_test package. Create `internal/auth/testhelper_test.go` with the same pattern as store's.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/... -run TestBuildRoleResolver -v`
Expected: FAIL

**Step 3: Write implementation**

```go
// internal/auth/resolver.go
package auth

import (
	"context"
	"fmt"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func BuildRoleResolver(members *store.ProjectMemberStore, projects *store.ProjectStore, orgSettings *store.OrgSettingsStore) RoleResolver {
	return func(ctx context.Context, projectKey, userID string) (model.ProjectRole, error) {
		// Look up project by key
		project, err := projects.FindByKey(ctx, projectKey)
		if err != nil {
			return "", fmt.Errorf("project not found: %w", err)
		}

		// Check for explicit project membership
		role, err := members.GetRole(ctx, project.ID, userID)
		if err == nil {
			return role, nil
		}

		// Fall back to org base project role
		baseRole, err := orgSettings.GetBaseProjectRole(ctx)
		if err != nil {
			return "", fmt.Errorf("get base project role: %w", err)
		}

		if baseRole == "none" {
			return "", fmt.Errorf("no access to project %q", projectKey)
		}

		return model.ProjectRole(baseRole), nil
	}
}
```

**Step 4: Run tests**

Run: `go test ./internal/auth/... -run TestBuildRoleResolver -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/auth/resolver.go internal/auth/resolver_test.go internal/auth/testhelper_test.go
git commit -m "feat(rbac): add role resolver connecting stores to middleware"
```

---

## Task 7: Wire Permissions into `main.go`

**Files:**
- Modify: `cmd/togglerino/main.go`

This task wires up the new stores, resolver, and permission middleware into the existing route definitions. Replace the current `requireAdmin` with the new permission-based middleware.

**Step 1: Add stores and resolver initialization**

Add after existing store initialization in `main.go`:

```go
orgSettingsStore := store.NewOrgSettingsStore(pool)
projectMemberStore := store.NewProjectMemberStore(pool)
```

Create resolver and permission middleware helpers:

```go
roleResolver := auth.BuildRoleResolver(projectMemberStore, projectStore, orgSettingsStore)

// Org-level permission shortcuts
requireOrgUsersManage := auth.RequireOrgPermission(model.PermOrgUsersManage)
requireOrgOIDCManage := auth.RequireOrgPermission(model.PermOrgOIDCManage)
requireOrgProjectsCreate := auth.RequireOrgPermission(model.PermOrgProjectsCreate)
requireOrgProjectsDelete := auth.RequireOrgPermission(model.PermOrgProjectsDelete)

// Project-level permission shortcuts
requireFlagsRead := auth.RequireProjectPermission(model.PermFlagsRead, roleResolver)
requireFlagsWrite := auth.RequireProjectPermission(model.PermFlagsWrite, roleResolver)
requireEnvsRead := auth.RequireProjectPermission(model.PermEnvironmentsRead, roleResolver)
requireEnvsWrite := auth.RequireProjectPermission(model.PermEnvironmentsWrite, roleResolver)
requireSDKKeysManage := auth.RequireProjectPermission(model.PermSDKKeysManage, roleResolver)
requireSegmentsWrite := auth.RequireProjectPermission(model.PermSegmentsWrite, roleResolver)
requireTemplatesManage := auth.RequireProjectPermission(model.PermTemplatesManage, roleResolver)
requireProjectSettings := auth.RequireProjectPermission(model.PermProjectSettings, roleResolver)
```

**Step 2: Update route definitions**

Replace existing route middleware. The key changes:

```go
// OIDC config — was requireAdmin, now requireOrgOIDCManage
mux.Handle("GET /api/v1/auth/oidc/config", wrap(oidcHandler.GetConfig, sessionAuth, requireOrgOIDCManage))
mux.Handle("PUT /api/v1/auth/oidc/config", wrap(oidcHandler.UpdateConfig, sessionAuth, requireOrgOIDCManage))
mux.Handle("DELETE /api/v1/auth/oidc/config", wrap(oidcHandler.DeleteConfig, sessionAuth, requireOrgOIDCManage))

// User management — was requireAdmin, now requireOrgUsersManage
mux.Handle("GET /api/v1/management/users", wrap(userHandler.List, sessionAuth, requireOrgUsersManage))
mux.Handle("POST /api/v1/management/users/invite", wrap(userHandler.Invite, sessionAuth, requireOrgUsersManage))
mux.Handle("GET /api/v1/management/users/invites", wrap(userHandler.ListInvites, sessionAuth, requireOrgUsersManage))
mux.Handle("DELETE /api/v1/management/users/{id}", wrap(userHandler.Delete, sessionAuth, requireOrgUsersManage))
mux.Handle("POST /api/v1/management/users/{id}/reset-password", wrap(http.HandlerFunc(userHandler.ResetPassword), sessionAuth, requireOrgUsersManage))

// Projects
mux.Handle("POST /api/v1/projects", wrap(projectHandler.Create, sessionAuth, requireOrgProjectsCreate))
mux.Handle("GET /api/v1/projects", wrap(projectHandler.List, sessionAuth))  // filtered in handler
mux.Handle("GET /api/v1/projects/{key}", wrap(projectHandler.Get, sessionAuth, requireFlagsRead))
mux.Handle("PUT /api/v1/projects/{key}", wrap(projectHandler.Update, sessionAuth, requireProjectSettings))
mux.Handle("DELETE /api/v1/projects/{key}", wrap(projectHandler.Delete, sessionAuth, requireOrgProjectsDelete))

// Environments
mux.Handle("POST /api/v1/projects/{key}/environments", wrap(environmentHandler.Create, sessionAuth, requireEnvsWrite))
mux.Handle("GET /api/v1/projects/{key}/environments", wrap(environmentHandler.List, sessionAuth, requireEnvsRead))

// SDK keys
mux.Handle("POST /api/v1/projects/{key}/environments/{env}/sdk-keys", wrap(sdkKeyHandler.Create, sessionAuth, requireSDKKeysManage))
mux.Handle("GET /api/v1/projects/{key}/environments/{env}/sdk-keys", wrap(sdkKeyHandler.List, sessionAuth, requireSDKKeysManage))
mux.Handle("DELETE /api/v1/projects/{key}/environments/{env}/sdk-keys/{id}", wrap(sdkKeyHandler.Revoke, sessionAuth, requireSDKKeysManage))

// Flags
mux.Handle("POST /api/v1/projects/{key}/flags", wrap(flagHandler.Create, sessionAuth, requireFlagsWrite))
mux.Handle("GET /api/v1/projects/{key}/flags", wrap(flagHandler.List, sessionAuth, requireFlagsRead))
mux.Handle("POST /api/v1/projects/{key}/flags/bulk", wrap(flagHandler.BulkAction, sessionAuth, requireFlagsWrite))
mux.Handle("GET /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Get, sessionAuth, requireFlagsRead))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Update, sessionAuth, requireFlagsWrite))
mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}", wrap(flagHandler.Delete, sessionAuth, requireFlagsWrite))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/archive", wrap(flagHandler.Archive, sessionAuth, requireFlagsWrite))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/staleness", wrap(flagHandler.SetStaleness, sessionAuth, requireFlagsWrite))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}", wrap(flagHandler.UpdateEnvironmentConfig, sessionAuth, requireFlagsWrite))

// Schedules
mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules", wrap(scheduleHandler.List, sessionAuth, requireFlagsRead))
mux.Handle("POST /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules", wrap(scheduleHandler.Create, sessionAuth, requireFlagsWrite))
mux.Handle("PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}", wrap(scheduleHandler.Update, sessionAuth, requireFlagsWrite))
mux.Handle("DELETE /api/v1/projects/{key}/flags/{flag}/environments/{env}/schedules/{id}", wrap(scheduleHandler.Cancel, sessionAuth, requireFlagsWrite))

// History (read-only)
mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/history", wrap(historyHandler.List, sessionAuth, requireFlagsRead))
mux.Handle("GET /api/v1/projects/{key}/flags/{flag}/history/{id}", wrap(historyHandler.Get, sessionAuth, requireFlagsRead))

// Unknown flags
mux.Handle("GET /api/v1/projects/{key}/unknown-flags", wrap(unknownFlagHandler.List, sessionAuth, requireFlagsRead))
mux.Handle("DELETE /api/v1/projects/{key}/unknown-flags/{id}", wrap(unknownFlagHandler.Dismiss, sessionAuth, requireFlagsWrite))

// Audit log
mux.Handle("GET /api/v1/projects/{key}/audit-log", wrap(auditHandler.List, sessionAuth, requireFlagsRead))

// Project settings
mux.Handle("GET /api/v1/projects/{key}/settings/flags", wrap(projectSettingsHandler.Get, sessionAuth, requireFlagsRead))
mux.Handle("PUT /api/v1/projects/{key}/settings/flags", wrap(projectSettingsHandler.Update, sessionAuth, requireProjectSettings))
mux.Handle("GET /api/v1/projects/{key}/settings/environments", wrap(projectSettingsHandler.GetEnvironmentDefaults, sessionAuth, requireFlagsRead))
mux.Handle("PUT /api/v1/projects/{key}/settings/environments", wrap(projectSettingsHandler.UpdateEnvironmentDefaults, sessionAuth, requireProjectSettings))

// Context attributes (read-only)
mux.Handle("GET /api/v1/projects/{key}/context-attributes", wrap(contextAttributeHandler.List, sessionAuth, requireFlagsRead))

// Segments
mux.Handle("GET /api/v1/projects/{key}/segments", wrap(segmentHandler.List, sessionAuth, requireFlagsRead))
mux.Handle("POST /api/v1/projects/{key}/segments", wrap(segmentHandler.Create, sessionAuth, requireSegmentsWrite))
mux.Handle("GET /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Get, sessionAuth, requireFlagsRead))
mux.Handle("PUT /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Update, sessionAuth, requireSegmentsWrite))
mux.Handle("DELETE /api/v1/projects/{key}/segments/{segmentKey}", wrap(segmentHandler.Delete, sessionAuth, requireSegmentsWrite))
mux.Handle("GET /api/v1/projects/{key}/segments/{segmentKey}/usage", wrap(segmentHandler.Usage, sessionAuth, requireFlagsRead))

// Global templates — requireAdmin stays for org-level
mux.Handle("GET /api/v1/templates", wrap(templateHandler.ListGlobal, sessionAuth))
mux.Handle("POST /api/v1/templates", wrap(templateHandler.CreateGlobal, sessionAuth, requireOrgUsersManage))
mux.Handle("PUT /api/v1/templates/{key}", wrap(templateHandler.UpdateGlobal, sessionAuth, requireOrgUsersManage))
mux.Handle("DELETE /api/v1/templates/{key}", wrap(templateHandler.DeleteGlobal, sessionAuth, requireOrgUsersManage))

// Project templates
mux.Handle("GET /api/v1/projects/{key}/templates", wrap(templateHandler.ListForProject, sessionAuth, requireFlagsRead))
mux.Handle("POST /api/v1/projects/{key}/templates", wrap(templateHandler.CreateForProject, sessionAuth, requireTemplatesManage))
mux.Handle("PUT /api/v1/projects/{key}/templates/{templateKey}", wrap(templateHandler.UpdateForProject, sessionAuth, requireTemplatesManage))
mux.Handle("DELETE /api/v1/projects/{key}/templates/{templateKey}", wrap(templateHandler.DeleteForProject, sessionAuth, requireTemplatesManage))
```

**Step 3: Remove old `requireAdmin` variable** (now replaced by specific org permissions)

**Step 4: Run existing tests**

Run: `go test ./...`
Expected: PASS (existing tests should work since admin users bypass all project checks)

**Step 5: Commit**

```bash
git add cmd/togglerino/main.go
git commit -m "feat(rbac): wire permission middleware into all routes"
```

---

## Task 8: Project Members Handler — API Endpoints

**Files:**
- Create: `internal/handler/project_member_handler.go`
- Modify: `cmd/togglerino/main.go` (add routes)

**Step 1: Write the handler**

```go
// internal/handler/project_member_handler.go
package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type ProjectMemberHandler struct {
	members  *store.ProjectMemberStore
	projects *store.ProjectStore
	users    *store.UserStore
}

func NewProjectMemberHandler(members *store.ProjectMemberStore, projects *store.ProjectStore, users *store.UserStore) *ProjectMemberHandler {
	return &ProjectMemberHandler{members: members, projects: projects, users: users}
}

func (h *ProjectMemberHandler) List(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	members, err := h.members.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	if members == nil {
		members = []model.ProjectMemberWithUser{}
	}
	writeJSON(w, http.StatusOK, members)
}

func (h *ProjectMemberHandler) Add(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Role == "" {
		writeError(w, http.StatusBadRequest, "user_id and role are required")
		return
	}
	if !model.ValidProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role: must be admin, editor, or viewer")
		return
	}

	// Verify user exists
	if _, err := h.users.FindByID(r.Context(), req.UserID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	member, err := h.members.Add(r.Context(), project.ID, req.UserID, model.ProjectRole(req.Role))
	if err != nil {
		writeError(w, http.StatusConflict, "user is already a member of this project")
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (h *ProjectMemberHandler) Update(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	userID := r.PathValue("userId")

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !model.ValidProjectRole(req.Role) {
		writeError(w, http.StatusBadRequest, "invalid role: must be admin, editor, or viewer")
		return
	}

	member, err := h.members.Update(r.Context(), project.ID, userID, model.ProjectRole(req.Role))
	if err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (h *ProjectMemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	userID := r.PathValue("userId")

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := h.members.Remove(r.Context(), project.ID, userID); err != nil {
		writeError(w, http.StatusNotFound, "member not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Add routes in `main.go`**

```go
// Project members
projectMemberHandler := handler.NewProjectMemberHandler(projectMemberStore, projectStore, userStore)
mux.Handle("GET /api/v1/projects/{key}/members", wrap(projectMemberHandler.List, sessionAuth, requireProjectSettings))
mux.Handle("POST /api/v1/projects/{key}/members", wrap(projectMemberHandler.Add, sessionAuth, requireProjectSettings))
mux.Handle("PUT /api/v1/projects/{key}/members/{userId}", wrap(projectMemberHandler.Update, sessionAuth, requireProjectSettings))
mux.Handle("DELETE /api/v1/projects/{key}/members/{userId}", wrap(projectMemberHandler.Remove, sessionAuth, requireProjectSettings))
```

**Step 3: Run tests and verify build**

Run: `go build ./cmd/togglerino && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/handler/project_member_handler.go cmd/togglerino/main.go
git commit -m "feat(rbac): add project members API endpoints"
```

---

## Task 9: Org Settings Handler — Base Project Role API

**Files:**
- Create: `internal/handler/org_settings_handler.go`
- Modify: `cmd/togglerino/main.go` (add routes)

**Step 1: Write the handler**

```go
// internal/handler/org_settings_handler.go
package handler

import (
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type OrgSettingsHandler struct {
	orgSettings *store.OrgSettingsStore
}

func NewOrgSettingsHandler(orgSettings *store.OrgSettingsStore) *OrgSettingsHandler {
	return &OrgSettingsHandler{orgSettings: orgSettings}
}

func (h *OrgSettingsHandler) GetBaseProjectRole(w http.ResponseWriter, r *http.Request) {
	role, err := h.orgSettings.GetBaseProjectRole(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get base project role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_project_role": role})
}

func (h *OrgSettingsHandler) SetBaseProjectRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Role string `json:"base_project_role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !model.ValidProjectRole(req.Role) && req.Role != "none" {
		writeError(w, http.StatusBadRequest, "invalid role: must be admin, editor, viewer, or none")
		return
	}

	if err := h.orgSettings.SetBaseProjectRole(r.Context(), req.Role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set base project role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_project_role": req.Role})
}
```

**Step 2: Add routes in `main.go`**

```go
orgSettingsHandler := handler.NewOrgSettingsHandler(orgSettingsStore)
mux.Handle("GET /api/v1/settings/base-project-role", wrap(orgSettingsHandler.GetBaseProjectRole, sessionAuth, requireOrgUsersManage))
mux.Handle("PUT /api/v1/settings/base-project-role", wrap(orgSettingsHandler.SetBaseProjectRole, sessionAuth, requireOrgUsersManage))
```

**Step 3: Run tests**

Run: `go build ./cmd/togglerino && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/handler/org_settings_handler.go cmd/togglerino/main.go
git commit -m "feat(rbac): add base project role API endpoint"
```

---

## Task 10: User Project Assignments API (Team Page)

**Files:**
- Modify: `internal/handler/user_handler.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Add methods to UserHandler**

Add `ListProjectAssignments` and `UpdateProjectAssignments` methods to `UserHandler`. The handler needs a `ProjectMemberStore` dependency added.

```go
func (h *UserHandler) ListProjectAssignments(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	assignments, err := h.members.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list assignments")
		return
	}
	if assignments == nil {
		assignments = []model.UserProjectAssignment{}
	}
	writeJSON(w, http.StatusOK, assignments)
}

func (h *UserHandler) UpdateProjectAssignments(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")

	var req struct {
		Assignments []struct {
			ProjectID string `json:"project_id"`
			Role      string `json:"role"`
		} `json:"assignments"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify user exists
	if _, err := h.users.FindByID(r.Context(), userID); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Remove all existing assignments and add new ones
	// (Simple approach: clear and re-add)
	for _, a := range req.Assignments {
		if !model.ValidProjectRole(a.Role) {
			writeError(w, http.StatusBadRequest, "invalid role: "+a.Role)
			return
		}
	}

	// Get current assignments to diff
	current, err := h.members.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list current assignments")
		return
	}

	// Remove assignments not in new list
	newMap := make(map[string]string)
	for _, a := range req.Assignments {
		newMap[a.ProjectID] = a.Role
	}
	for _, c := range current {
		if _, exists := newMap[c.ProjectID]; !exists {
			h.members.Remove(r.Context(), c.ProjectID, userID)
		}
	}

	// Add or update assignments
	currentMap := make(map[string]bool)
	for _, c := range current {
		currentMap[c.ProjectID] = true
	}
	for _, a := range req.Assignments {
		if currentMap[a.ProjectID] {
			h.members.Update(r.Context(), a.ProjectID, userID, model.ProjectRole(a.Role))
		} else {
			h.members.Add(r.Context(), a.ProjectID, userID, model.ProjectRole(a.Role))
		}
	}

	// Return updated assignments
	assignments, err := h.members.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list assignments")
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}
```

**Step 2: Add routes**

```go
mux.Handle("GET /api/v1/management/users/{id}/projects", wrap(userHandler.ListProjectAssignments, sessionAuth, requireOrgUsersManage))
mux.Handle("PUT /api/v1/management/users/{id}/projects", wrap(userHandler.UpdateProjectAssignments, sessionAuth, requireOrgUsersManage))
```

**Step 3: Run tests**

Run: `go build ./cmd/togglerino && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/handler/user_handler.go cmd/togglerino/main.go
git commit -m "feat(rbac): add user project assignments API for team page"
```

---

## Task 11: Filter Project Listing by Access

**Files:**
- Modify: `internal/handler/project_handler.go`
- Modify: `internal/store/project_store.go`

The `GET /api/v1/projects` endpoint must filter results based on the user's access. Global admins see all projects. Members with base role != "none" see all. Members with base role "none" see only explicitly assigned projects.

**Step 1: Add `ListAccessible` method to ProjectStore**

```go
func (s *ProjectStore) ListAccessible(ctx context.Context, projectIDs []string) ([]Project, error) {
	// If projectIDs is nil, return all projects (admin/base-role access)
	// If projectIDs is empty slice, return nothing
	// If projectIDs has entries, return only those projects
	// ...
}
```

**Step 2: Update ProjectHandler.List** to use auth context

The handler needs access to `OrgSettingsStore` and `ProjectMemberStore` to determine filtering. Inject these as dependencies. Check if user is admin → return all. Check base role → if not "none", return all. Otherwise, get accessible IDs from project_members and filter.

**Step 3: Run tests**

Run: `go build ./cmd/togglerino && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/handler/project_handler.go internal/store/project_store.go
git commit -m "feat(rbac): filter project listing by user access"
```

---

## Task 12: Extend `/auth/me` with Permissions

**Files:**
- Modify: `internal/handler/auth_handler.go`

The `/auth/me` response should include the user's org-level permissions so the frontend can gate UI elements without making additional API calls.

**Step 1: Update Me handler response**

Add `permissions` field to the response:

```go
type MeResponse struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName *string  `json:"display_name,omitempty"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}
```

For org-level permissions, derive from the user's global role. The frontend will resolve project-level permissions per project using the project member data.

**Step 2: Run tests**

Run: `go build ./cmd/togglerino && go test ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/handler/auth_handler.go
git commit -m "feat(rbac): include org permissions in /auth/me response"
```

---

## Task 13: Frontend — Permission Context & Hook

**Files:**
- Create: `web/src/hooks/usePermissions.ts`
- Modify: `web/src/api/client.ts` (types)

**Step 1: Add permission types and hook**

```typescript
// web/src/hooks/usePermissions.ts
import { useAuth } from '@/hooks/useAuth'  // or wherever auth context lives
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export type ProjectRole = 'admin' | 'editor' | 'viewer'

export interface ProjectMember {
  project_id: string
  user_id: string
  role: ProjectRole
  email: string
  display_name?: string
  created_at: string
  updated_at: string
}

export function useProjectPermission(projectKey: string | undefined, permission: string): boolean {
  const { user } = useAuth()

  // Global admins have all permissions
  if (user?.role === 'admin') return true

  // For project-level, we need the project member data
  // This is resolved via the API — if a 403/404 comes back, the UI handles it
  return true  // optimistic, server enforces
}

export function useIsOrgAdmin(): boolean {
  const { user } = useAuth()
  return user?.role === 'admin'
}
```

**Step 2: Commit**

```bash
git add web/src/hooks/usePermissions.ts
git commit -m "feat(rbac): add frontend permission hooks"
```

---

## Task 14: Frontend — Project Members Tab

**Files:**
- Modify: `web/src/pages/settings/MembersTab.tsx`

**Step 1: Implement the members tab**

Replace the placeholder with a full member management UI:
- List project members with role badges
- Add member form (user search + role selector)
- Change role dropdown
- Remove member button
- Show "inherited from base" indicator for users without explicit override

Use existing shadcn/ui components: Card, Table, Select, Button, Badge, Dialog.

**Step 2: Test in browser**

Navigate to project settings → Members tab. Verify:
- Members list loads
- Can add a member with a role
- Can change a member's role
- Can remove a member

**Step 3: Commit**

```bash
git add web/src/pages/settings/MembersTab.tsx
git commit -m "feat(rbac): implement project members management tab"
```

---

## Task 15: Frontend — Team Page User Assignments

**Files:**
- Modify: `web/src/pages/TeamPage.tsx`

**Step 1: Add project assignments section**

For admin users, add a section showing a user's project-specific role overrides. Clicking a user (or expanding) shows their project assignments with role selectors.

Use: `GET /api/v1/management/users/{id}/projects` and `PUT /api/v1/management/users/{id}/projects`

**Step 2: Test in browser**

Navigate to team page. Verify:
- Can view a user's project assignments
- Can add/change/remove project assignments

**Step 3: Commit**

```bash
git add web/src/pages/TeamPage.tsx
git commit -m "feat(rbac): add project assignments to team page"
```

---

## Task 16: Frontend — Org Settings Base Project Role

**Files:**
- Modify: `web/src/pages/SettingsPage.tsx`

**Step 1: Add base project role setting**

Add a card/section to the org settings page with a select dropdown for the base project role (admin/editor/viewer/none) and a save button. Explanation text: "Default project access for all members. Set to 'none' to require explicit project membership."

Use: `GET /PUT /api/v1/settings/base-project-role`

**Step 2: Test in browser**

Navigate to settings page. Verify selector and save work.

**Step 3: Commit**

```bash
git add web/src/pages/SettingsPage.tsx
git commit -m "feat(rbac): add base project role setting to org settings"
```

---

## Task 17: Frontend — UI Gating for Viewer Role

**Files:**
- Modify: various page components

**Step 1: Gate mutation UI elements**

Across all project pages, hide or disable mutation controls for users without write access:
- Flag list: hide "Create Flag" button for viewers
- Flag detail: disable toggle, hide edit/delete for viewers
- Environment list: hide "Create Environment" for viewers
- Segments: hide create/edit/delete for viewers
- SDK keys: hide create/revoke for viewers
- Project settings: hide settings tabs for non-project-admins

The server enforces permissions — frontend gating is UX-only. Use the `useProjectPermission` hook or check `user.role` + project member data.

**Step 2: Test in browser**

Log in as a member, set base role to viewer, verify mutation controls are hidden.

**Step 3: Commit**

```bash
git add web/src/
git commit -m "feat(rbac): gate mutation UI elements by project role"
```

---

## Task 18: Filter Sidebar Projects by Access

**Files:**
- Modify: `web/src/components/OrgLayout.tsx` (or wherever project list is in sidebar)

When base role is "none", the project list API already filters. Ensure the sidebar project list uses the same API and doesn't show inaccessible projects.

**Step 1: Verify sidebar uses filtered project list**

The sidebar should already work if it calls `GET /api/v1/projects` (which is now filtered). Verify and adjust if needed.

**Step 2: Commit if changes needed**

```bash
git add web/src/components/OrgLayout.tsx
git commit -m "feat(rbac): ensure sidebar shows only accessible projects"
```

---

## Task 19: Update CLAUDE.md & Clean Up

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update CLAUDE.md**

Update the following sections:
- **Key Patterns**: Add RBAC description (global roles, project roles, base project role, permission middleware)
- **API Routes**: Add new endpoints (project members, org settings, user assignments)
- **Database**: Add new tables to the list
- Remove the `TODO(#36)` reference from any code comments

**Step 2: Run full test suite**

Run: `go test ./... && cd web && npm run lint`
Expected: All pass

**Step 3: Commit**

```bash
git add CLAUDE.md cmd/togglerino/main.go
git commit -m "docs: update CLAUDE.md with RBAC documentation"
```

---

## Summary

| Task | Description | Type |
|------|-------------|------|
| 1 | Permission model & project role types | Model + Tests |
| 2 | Database migration | SQL |
| 3 | Org settings store | Store + Tests |
| 4 | Project members store | Store + Model + Tests |
| 5 | Permission middleware | Middleware + Tests |
| 6 | Role resolver | Auth + Tests |
| 7 | Wire permissions into main.go | Wiring |
| 8 | Project members handler | Handler |
| 9 | Org settings handler | Handler |
| 10 | User project assignments API | Handler |
| 11 | Filter project listing | Handler + Store |
| 12 | Extend /auth/me with permissions | Handler |
| 13 | Frontend permission hooks | React |
| 14 | Project members tab | React |
| 15 | Team page user assignments | React |
| 16 | Org settings base project role | React |
| 17 | UI gating for viewer role | React |
| 18 | Filter sidebar projects | React |
| 19 | Update docs & clean up | Docs |
