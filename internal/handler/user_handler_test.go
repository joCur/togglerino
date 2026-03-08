package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

// newUserHandler creates a UserHandler wired to real stores and a real pool.
func newUserHandler(t *testing.T) (*handler.UserHandler, *store.ProjectMemberStore) {
	t.Helper()
	pool := testPool(t)
	members := store.NewProjectMemberStore(pool)
	h := handler.NewUserHandler(
		store.NewUserStore(pool),
		store.NewInviteStore(pool),
		members,
		store.NewRoleStore(pool),
		pool,
		store.NewAuditStore(pool),
	)
	return h, members
}

// updateAssignmentsRequest builds a PUT request for UpdateProjectAssignments.
func updateAssignmentsRequest(t *testing.T, userID string, assignments []map[string]string, caller *model.User) *http.Request {
	t.Helper()
	body := map[string]any{"assignments": assignments}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/management/users/"+userID+"/projects", bytes.NewReader(b))
	req.SetPathValue("id", userID)
	if caller != nil {
		req = req.WithContext(auth.ContextWithUser(req.Context(), caller))
	}
	return req
}

// decodeAssignments parses the JSON response body into a slice of assignments.
func decodeAssignments(t *testing.T, rr *httptest.ResponseRecorder) []model.UserProjectAssignment {
	t.Helper()
	var assignments []model.UserProjectAssignment
	if err := json.NewDecoder(rr.Body).Decode(&assignments); err != nil {
		t.Fatalf("decoding assignments response: %v (body: %s)", err, rr.Body.String())
	}
	return assignments
}

// TestUpdateProjectAssignments_AddNew verifies that new assignments are created.
func TestUpdateProjectAssignments_AddNew(t *testing.T) {
	pool := testPool(t)
	h, _ := newUserHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "upa-admin-add-"+suffix+"@test.dev", model.RoleAdmin)
	target := createTestUser(t, pool, "upa-target-add-"+suffix+"@test.dev", model.RoleMember)
	projID := createTestProject(t, pool, "upa-add-proj-"+suffix, "UPA Add Project")

	req := updateAssignmentsRequest(t, target.ID, []map[string]string{
		{"project_id": projID, "role": "editor"},
	}, admin)
	rr := httptest.NewRecorder()
	h.UpdateProjectAssignments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	assignments := decodeAssignments(t, rr)
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	if assignments[0].ProjectID != projID {
		t.Errorf("project_id: got %q, want %q", assignments[0].ProjectID, projID)
	}
	if assignments[0].Role != model.ProjectRoleEditor {
		t.Errorf("role: got %q, want %q", assignments[0].Role, model.ProjectRoleEditor)
	}
}

// TestUpdateProjectAssignments_RemoveExisting verifies that omitting a
// previously assigned project removes that assignment.
func TestUpdateProjectAssignments_RemoveExisting(t *testing.T) {
	pool := testPool(t)
	h, members := newUserHandler(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "upa-admin-rm-"+suffix+"@test.dev", model.RoleAdmin)
	target := createTestUser(t, pool, "upa-target-rm-"+suffix+"@test.dev", model.RoleMember)
	projID := createTestProject(t, pool, "upa-rm-proj-"+suffix, "UPA Remove Project")

	// Pre-create assignment
	if _, err := members.Add(ctx, projID, target.ID, model.ProjectRoleEditor); err != nil {
		t.Fatalf("pre-creating assignment: %v", err)
	}

	// Send empty assignments to remove all
	req := updateAssignmentsRequest(t, target.ID, []map[string]string{}, admin)
	rr := httptest.NewRecorder()
	h.UpdateProjectAssignments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	assignments := decodeAssignments(t, rr)
	if len(assignments) != 0 {
		t.Errorf("expected 0 assignments after removal, got %d", len(assignments))
	}
}

// TestUpdateProjectAssignments_UpdateRole verifies that changing the role
// of an existing assignment updates it in place.
func TestUpdateProjectAssignments_UpdateRole(t *testing.T) {
	pool := testPool(t)
	h, members := newUserHandler(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "upa-admin-upd-"+suffix+"@test.dev", model.RoleAdmin)
	target := createTestUser(t, pool, "upa-target-upd-"+suffix+"@test.dev", model.RoleMember)
	projID := createTestProject(t, pool, "upa-upd-proj-"+suffix, "UPA Update Project")

	// Pre-create assignment with viewer role
	if _, err := members.Add(ctx, projID, target.ID, model.ProjectRoleViewer); err != nil {
		t.Fatalf("pre-creating assignment: %v", err)
	}

	// Update to admin role
	req := updateAssignmentsRequest(t, target.ID, []map[string]string{
		{"project_id": projID, "role": "admin"},
	}, admin)
	rr := httptest.NewRecorder()
	h.UpdateProjectAssignments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	assignments := decodeAssignments(t, rr)
	if len(assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(assignments))
	}
	if assignments[0].Role != model.ProjectRoleAdmin {
		t.Errorf("role: got %q, want %q", assignments[0].Role, model.ProjectRoleAdmin)
	}
}

// TestUpdateProjectAssignments_MixedAddRemoveUpdate verifies that a single
// request can add, remove, and update assignments atomically.
func TestUpdateProjectAssignments_MixedAddRemoveUpdate(t *testing.T) {
	pool := testPool(t)
	h, members := newUserHandler(t)
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "upa-admin-mix-"+suffix+"@test.dev", model.RoleAdmin)
	target := createTestUser(t, pool, "upa-target-mix-"+suffix+"@test.dev", model.RoleMember)

	projKeep := createTestProject(t, pool, "upa-mix-keep-"+suffix, "Keep")
	projRemove := createTestProject(t, pool, "upa-mix-rm-"+suffix, "Remove")
	projAdd := createTestProject(t, pool, "upa-mix-add-"+suffix, "Add")

	// Pre-create: keep (viewer) and remove (editor)
	if _, err := members.Add(ctx, projKeep, target.ID, model.ProjectRoleViewer); err != nil {
		t.Fatalf("adding keep assignment: %v", err)
	}
	if _, err := members.Add(ctx, projRemove, target.ID, model.ProjectRoleEditor); err != nil {
		t.Fatalf("adding remove assignment: %v", err)
	}

	// Desired: keep (upgrade to admin) + add (editor) — remove is omitted
	req := updateAssignmentsRequest(t, target.ID, []map[string]string{
		{"project_id": projKeep, "role": "admin"},
		{"project_id": projAdd, "role": "editor"},
	}, admin)
	rr := httptest.NewRecorder()
	h.UpdateProjectAssignments(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	assignments := decodeAssignments(t, rr)
	if len(assignments) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(assignments))
	}

	byProject := make(map[string]model.ProjectRole)
	for _, a := range assignments {
		byProject[a.ProjectID] = a.Role
	}

	// projKeep should be upgraded to admin
	if role, ok := byProject[projKeep]; !ok {
		t.Error("keep project not in assignments")
	} else if role != model.ProjectRoleAdmin {
		t.Errorf("keep project role: got %q, want %q", role, model.ProjectRoleAdmin)
	}

	// projAdd should be editor
	if role, ok := byProject[projAdd]; !ok {
		t.Error("add project not in assignments")
	} else if role != model.ProjectRoleEditor {
		t.Errorf("add project role: got %q, want %q", role, model.ProjectRoleEditor)
	}

	// projRemove should be gone
	if _, ok := byProject[projRemove]; ok {
		t.Error("removed project should not be in assignments")
	}
}

// TestUpdateProjectAssignments_InvalidRole verifies that an invalid role
// returns a 400 error.
func TestUpdateProjectAssignments_InvalidRole(t *testing.T) {
	pool := testPool(t)
	h, _ := newUserHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "upa-admin-bad-"+suffix+"@test.dev", model.RoleAdmin)
	target := createTestUser(t, pool, "upa-target-bad-"+suffix+"@test.dev", model.RoleMember)
	projID := createTestProject(t, pool, "upa-bad-proj-"+suffix, "Bad Role Project")

	req := updateAssignmentsRequest(t, target.ID, []map[string]string{
		{"project_id": projID, "role": "superadmin"},
	}, admin)
	rr := httptest.NewRecorder()
	h.UpdateProjectAssignments(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestUpdateProjectAssignments_UserNotFound verifies that updating assignments
// for a non-existent user returns 404.
func TestUpdateProjectAssignments_UserNotFound(t *testing.T) {
	pool := testPool(t)
	h, _ := newUserHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	admin := createTestUser(t, pool, "upa-admin-nf-"+suffix+"@test.dev", model.RoleAdmin)

	req := updateAssignmentsRequest(t, "00000000-0000-0000-0000-000000000000", []map[string]string{}, admin)
	rr := httptest.NewRecorder()
	h.UpdateProjectAssignments(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
