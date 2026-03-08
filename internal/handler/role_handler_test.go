package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/store"
)

type countingRefresher struct {
	count atomic.Int64
}

func (c *countingRefresher) Refresh() { c.count.Add(1) }

func jsonBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func parseError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp struct{ Error string }
	json.NewDecoder(rec.Body).Decode(&resp)
	return resp.Error
}

func setupRoleHandler(t *testing.T) (*handler.RoleHandler, *countingRefresher, func()) {
	t.Helper()
	pool := testPool(t)
	rs := store.NewRoleStore(pool)
	cr := &countingRefresher{}
	h := handler.NewRoleHandler(rs, cr)

	cleanup := func() {
		pool.Exec(context.Background(), `DELETE FROM roles WHERE name LIKE 'test-handler-%'`)
	}
	cleanup() // clean before
	t.Cleanup(cleanup)
	return h, cr, cleanup
}

// serve wires up the handler with a real mux so PathValue works.
func serve(method, pattern string, handlerFunc http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(method+" "+pattern, handlerFunc)
	return mux
}

func TestRoleHandler_Create_Validation(t *testing.T) {
	h, _, _ := setupRoleHandler(t)

	tests := []struct {
		name   string
		body   map[string]any
		status int
		errMsg string
	}{
		{
			name:   "empty name",
			body:   map[string]any{"name": "", "permissions": []string{"flags:read"}},
			status: 400,
			errMsg: "name must be 2-50 lowercase alphanumeric characters or hyphens",
		},
		{
			name:   "invalid name chars",
			body:   map[string]any{"name": "UPPER CASE!", "permissions": []string{"flags:read"}},
			status: 400,
			errMsg: "name must be 2-50 lowercase alphanumeric characters or hyphens",
		},
		{
			name:   "empty permissions",
			body:   map[string]any{"name": "test-handler-empty", "permissions": []string{}},
			status: 400,
			errMsg: "at least one permission is required",
		},
		{
			name:   "invalid permission",
			body:   map[string]any{"name": "test-handler-badperm", "permissions": []string{"invalid:perm"}},
			status: 400,
			errMsg: "invalid permission: invalid:perm",
		},
	}

	mux := serve("POST", "/roles", h.Create)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/roles", jsonBody(t, tt.body))
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			if msg := parseError(t, rec); msg != tt.errMsg {
				t.Errorf("error = %q, want %q", msg, tt.errMsg)
			}
		})
	}
}

func TestRoleHandler_Create_Success(t *testing.T) {
	h, cr, _ := setupRoleHandler(t)
	mux := serve("POST", "/roles", h.Create)

	body := map[string]any{
		"name":        "test-handler-create",
		"description": "A test role",
		"permissions": []string{"flags:read", "flags:write"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/roles", jsonBody(t, body))
	mux.ServeHTTP(rec, req)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	if cr.count.Load() != 1 {
		t.Errorf("cache refresh count = %d, want 1", cr.count.Load())
	}
}

func TestRoleHandler_Create_Duplicate(t *testing.T) {
	h, _, _ := setupRoleHandler(t)
	mux := serve("POST", "/roles", h.Create)

	body := map[string]any{
		"name":        "test-handler-dup",
		"permissions": []string{"flags:read"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/roles", jsonBody(t, body))
	mux.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("first create: status = %d", rec.Code)
	}

	// Second create should return 409.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/roles", jsonBody(t, body))
	mux.ServeHTTP(rec, req)
	if rec.Code != 409 {
		t.Errorf("duplicate create: status = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRoleHandler_Update_NotFound(t *testing.T) {
	h, _, _ := setupRoleHandler(t)
	mux := serve("PUT", "/roles/{name}", h.Update)

	body := map[string]any{
		"description": "updated",
		"permissions": []string{"flags:read"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/roles/test-handler-nonexistent", jsonBody(t, body))
	mux.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRoleHandler_Update_BuiltIn(t *testing.T) {
	h, _, _ := setupRoleHandler(t)
	mux := serve("PUT", "/roles/{name}", h.Update)

	body := map[string]any{
		"description": "hacked",
		"permissions": []string{"flags:read"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/roles/admin", jsonBody(t, body))
	mux.ServeHTTP(rec, req)

	// Built-in roles return no rows from the UPDATE WHERE is_built_in=false,
	// so this is a 404 ("role not found or is built-in").
	if rec.Code != 404 {
		t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRoleHandler_Update_EmptyPermissions(t *testing.T) {
	h, _, _ := setupRoleHandler(t)
	mux := serve("PUT", "/roles/{name}", h.Update)

	body := map[string]any{
		"permissions": []string{},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/roles/test-handler-noperm", jsonBody(t, body))
	mux.ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRoleHandler_Delete_BuiltIn(t *testing.T) {
	h, _, _ := setupRoleHandler(t)
	mux := serve("DELETE", "/roles/{name}", h.Delete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/roles/admin", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != 403 {
		t.Errorf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRoleHandler_Delete_NotFound(t *testing.T) {
	h, _, _ := setupRoleHandler(t)
	mux := serve("DELETE", "/roles/{name}", h.Delete)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/roles/test-handler-nonexistent", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404; body: %s", rec.Code, rec.Body.String())
	}
}

func TestRoleHandler_Delete_Success(t *testing.T) {
	h, cr, _ := setupRoleHandler(t)

	// Create a role to delete.
	createMux := serve("POST", "/roles", h.Create)
	body := map[string]any{
		"name":        "test-handler-del",
		"permissions": []string{"flags:read"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/roles", jsonBody(t, body))
	createMux.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create: status = %d", rec.Code)
	}
	cr.count.Store(0) // reset

	deleteMux := serve("DELETE", "/roles/{name}", h.Delete)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/roles/test-handler-del", nil)
	deleteMux.ServeHTTP(rec, req)

	if rec.Code != 204 {
		t.Errorf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
	if cr.count.Load() != 1 {
		t.Errorf("cache refresh count = %d, want 1", cr.count.Load())
	}
}
