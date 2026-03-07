package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d@test.togglerino.dev", prefix, time.Now().UnixNano())
}

// createTestUser inserts a user directly via SQL and returns the user model.
func createTestUser(t *testing.T, pool *pgxpool.Pool, email string, role model.Role) *model.User {
	t.Helper()
	var u model.User
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash, role) VALUES ($1, 'hash', $2)
		 RETURNING id, email, display_name, password_hash, role, created_at, updated_at`,
		email, role,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		t.Fatalf("creating test user %s: %v", email, err)
	}
	return &u
}

// createTestProject inserts a project directly via SQL and returns its ID.
func createTestProject(t *testing.T, pool *pgxpool.Pool, key, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO projects (key, name, description) VALUES ($1, $2, '') RETURNING id`,
		key, name,
	).Scan(&id)
	if err != nil {
		t.Fatalf("creating test project %s: %v", key, err)
	}
	return id
}

// newProjectHandler creates a ProjectHandler wired to real stores.
func newProjectHandler(pool *pgxpool.Pool) *handler.ProjectHandler {
	return handler.NewProjectHandler(
		store.NewProjectStore(pool),
		store.NewEnvironmentStore(pool),
		store.NewAuditStore(pool),
		store.NewOrgSettingsStore(pool),
		store.NewProjectMemberStore(pool),
	)
}

// requestWithUser creates a GET request with the given user set in the context.
func requestWithUser(t *testing.T, method, path string, user *model.User) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if user != nil {
		req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	}
	return req
}

// decodeProjects parses the paginated JSON response body into a slice of projects.
func decodeProjects(t *testing.T, rr *httptest.ResponseRecorder) []model.Project {
	t.Helper()
	var resp struct {
		Data       []model.Project `json:"data"`
		TotalCount int             `json:"total_count"`
		Limit      int             `json:"limit"`
		Offset     int             `json:"offset"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding projects response: %v", err)
	}
	return resp.Data
}

// TestProjectList_AdminSeesAll verifies that an org admin sees all projects
// regardless of base role or project memberships.
func TestProjectList_AdminSeesAll(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	h := newProjectHandler(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create an admin user
	admin := createTestUser(t, pool, "admin-list-"+suffix+"@test.dev", model.RoleAdmin)

	// Create two projects
	createTestProject(t, pool, "proj-admin-a-"+suffix, "Admin A")
	createTestProject(t, pool, "proj-admin-b-"+suffix, "Admin B")

	// Set base role to "none" — admin should still see everything
	orgSettings := store.NewOrgSettingsStore(pool)
	if err := orgSettings.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("setting base project role: %v", err)
	}

	req := requestWithUser(t, http.MethodGet, "/api/v1/projects", admin)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	projects := decodeProjects(t, rr)
	// Admin sees all projects — at least the two we created (there may be others from other tests)
	projectKeys := make(map[string]bool)
	for _, p := range projects {
		projectKeys[p.Key] = true
	}
	if !projectKeys["proj-admin-a-"+suffix] {
		t.Errorf("admin should see proj-admin-a-%s", suffix)
	}
	if !projectKeys["proj-admin-b-"+suffix] {
		t.Errorf("admin should see proj-admin-b-%s", suffix)
	}
}

// TestProjectList_BaseRoleNotNone_MemberSeesAll verifies that when the base
// project role is not "none", a member sees all projects.
func TestProjectList_BaseRoleNotNone_MemberSeesAll(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	h := newProjectHandler(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	member := createTestUser(t, pool, "member-all-"+suffix+"@test.dev", model.RoleMember)

	createTestProject(t, pool, "proj-base-a-"+suffix, "Base A")
	createTestProject(t, pool, "proj-base-b-"+suffix, "Base B")

	// Set base role to "editor" — non-admin members should see all projects
	orgSettings := store.NewOrgSettingsStore(pool)
	if err := orgSettings.SetBaseProjectRole(ctx, "editor"); err != nil {
		t.Fatalf("setting base project role: %v", err)
	}

	req := requestWithUser(t, http.MethodGet, "/api/v1/projects", member)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	projects := decodeProjects(t, rr)
	projectKeys := make(map[string]bool)
	for _, p := range projects {
		projectKeys[p.Key] = true
	}
	if !projectKeys["proj-base-a-"+suffix] {
		t.Errorf("member should see proj-base-a-%s when base role is editor", suffix)
	}
	if !projectKeys["proj-base-b-"+suffix] {
		t.Errorf("member should see proj-base-b-%s when base role is editor", suffix)
	}
}

// TestProjectList_BaseRoleNone_MemberSeesOnlyAssigned verifies that when
// the base project role is "none", a member only sees projects they are
// explicitly assigned to.
func TestProjectList_BaseRoleNone_MemberSeesOnlyAssigned(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	h := newProjectHandler(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	member := createTestUser(t, pool, "member-restricted-"+suffix+"@test.dev", model.RoleMember)

	assignedID := createTestProject(t, pool, "proj-assigned-"+suffix, "Assigned")
	createTestProject(t, pool, "proj-unassigned-"+suffix, "Unassigned")

	// Assign the member to only one project
	members := store.NewProjectMemberStore(pool)
	if _, err := members.Add(ctx, assignedID, member.ID, model.ProjectRoleEditor); err != nil {
		t.Fatalf("adding project member: %v", err)
	}

	// Set base role to "none"
	orgSettings := store.NewOrgSettingsStore(pool)
	if err := orgSettings.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("setting base project role: %v", err)
	}

	req := requestWithUser(t, http.MethodGet, "/api/v1/projects", member)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	projects := decodeProjects(t, rr)
	projectKeys := make(map[string]bool)
	for _, p := range projects {
		projectKeys[p.Key] = true
	}

	if !projectKeys["proj-assigned-"+suffix] {
		t.Errorf("member should see proj-assigned-%s (explicitly assigned)", suffix)
	}
	if projectKeys["proj-unassigned-"+suffix] {
		t.Errorf("member should NOT see proj-unassigned-%s (not assigned, base role is none)", suffix)
	}
}

// TestProjectList_BaseRoleNone_MemberWithNoAssignments verifies that a member
// with no project assignments sees an empty list when base role is "none".
func TestProjectList_BaseRoleNone_MemberWithNoAssignments(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	h := newProjectHandler(pool)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	member := createTestUser(t, pool, "member-none-"+suffix+"@test.dev", model.RoleMember)
	createTestProject(t, pool, "proj-invisible-"+suffix, "Invisible")

	// Set base role to "none"
	orgSettings := store.NewOrgSettingsStore(pool)
	if err := orgSettings.SetBaseProjectRole(ctx, "none"); err != nil {
		t.Fatalf("setting base project role: %v", err)
	}

	req := requestWithUser(t, http.MethodGet, "/api/v1/projects", member)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	projects := decodeProjects(t, rr)
	// With no assignments, the member should see zero projects (from our test)
	for _, p := range projects {
		if p.Key == "proj-invisible-"+suffix {
			t.Errorf("member with no assignments should NOT see proj-invisible-%s", suffix)
		}
	}
}
