package handler_test

import (
	"bytes"
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

func newPATHandler(t *testing.T) (*handler.PATHandler, *store.PATStore) {
	t.Helper()
	pool := testPool(t)
	pats := store.NewPATStore(pool)
	h := handler.NewPATHandler(pats)
	return h, pats
}

func patRequest(t *testing.T, method, path string, user *model.User, body any) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != nil {
		req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	}
	return req
}

// TestPATHandler_Create verifies that a valid POST returns 201 with the token field.
func TestPATHandler_Create(t *testing.T) {
	h, _ := newPATHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user := createTestUser(t, testPool(t), "pat-create-"+suffix+"@test.dev", model.RoleMember)

	req := patRequest(t, http.MethodPost, "/api/v1/auth/tokens", user, map[string]string{
		"name": "my-token",
	})
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp["token"] == "" || resp["token"] == nil {
		t.Errorf("expected token field in response, got: %v", resp)
	}
	if resp["name"] != "my-token" {
		t.Errorf("expected name 'my-token', got %v", resp["name"])
	}
}

// TestPATHandler_Create_EmptyName verifies that an empty name returns 400.
func TestPATHandler_Create_EmptyName(t *testing.T) {
	h, _ := newPATHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user := createTestUser(t, testPool(t), "pat-empty-"+suffix+"@test.dev", model.RoleMember)

	req := patRequest(t, http.MethodPost, "/api/v1/auth/tokens", user, map[string]string{
		"name": "",
	})
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestPATHandler_Create_PastExpiry verifies that an expires_at in the past returns 400.
func TestPATHandler_Create_PastExpiry(t *testing.T) {
	h, _ := newPATHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user := createTestUser(t, testPool(t), "pat-past-"+suffix+"@test.dev", model.RoleMember)

	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	req := patRequest(t, http.MethodPost, "/api/v1/auth/tokens", user, map[string]string{
		"name":       "expired-token",
		"expires_at": past,
	})
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestPATHandler_List verifies that GET returns tokens without the `token` field.
func TestPATHandler_List(t *testing.T) {
	pool := testPool(t)
	h, pats := newPATHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user := createTestUser(t, pool, "pat-list-"+suffix+"@test.dev", model.RoleMember)

	// Create a token first
	created, err := pats.Create(t.Context(), user.ID, "list-token", nil)
	if err != nil {
		t.Fatalf("creating token: %v", err)
	}
	_ = created

	req := patRequest(t, http.MethodGet, "/api/v1/auth/tokens", user, nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var tokens []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&tokens); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(tokens) == 0 {
		t.Fatal("expected at least one token in list")
	}

	// The list should not include the raw token value
	for _, tok := range tokens {
		if tok["token"] != nil {
			t.Errorf("list response should not include 'token' field, got: %v", tok)
		}
	}
}

// TestPATHandler_Delete verifies that DELETE returns 204.
func TestPATHandler_Delete(t *testing.T) {
	pool := testPool(t)
	h, pats := newPATHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user := createTestUser(t, pool, "pat-delete-"+suffix+"@test.dev", model.RoleMember)

	// Create a token
	created, err := pats.Create(t.Context(), user.ID, "delete-me", nil)
	if err != nil {
		t.Fatalf("creating token: %v", err)
	}

	req := patRequest(t, http.MethodDelete, "/api/v1/auth/tokens/"+created.ID, user, nil)
	req.SetPathValue("id", created.ID)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestPATHandler_Delete_NotFound verifies that deleting a non-existent token returns an error.
func TestPATHandler_Delete_NotFound(t *testing.T) {
	pool := testPool(t)
	h, _ := newPATHandler(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	user := createTestUser(t, pool, "pat-notfound-"+suffix+"@test.dev", model.RoleMember)

	req := patRequest(t, http.MethodDelete, "/api/v1/auth/tokens/00000000-0000-0000-0000-000000000000", user, nil)
	req.SetPathValue("id", "00000000-0000-0000-0000-000000000000")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code == http.StatusNoContent {
		t.Errorf("expected error status for non-existent token, got 204")
	}
}
