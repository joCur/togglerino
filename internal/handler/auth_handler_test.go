package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/store"
)

func newAuthHandler(t *testing.T, baseURL string) *handler.AuthHandler {
	t.Helper()
	pool := testPool(t)
	return handler.NewAuthHandler(
		store.NewUserStore(pool),
		store.NewSessionStore(pool),
		store.NewInviteStore(pool),
		baseURL,
	)
}

func TestSetup_SecureCookie_HTTPS(t *testing.T) {
	h := newAuthHandler(t, "https://example.com")
	body, _ := json.Marshal(map[string]string{
		"email":    "setup-secure@test.com",
		"password": "testpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Setup(rr, req)

	if rr.Code == http.StatusConflict {
		t.Skip("users already exist in database, cannot test setup cookie")
	}
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status %d", rr.Code)
	}
	for _, c := range rr.Result().Cookies() {
		if c.Name == "session_id" {
			if !c.Secure {
				t.Error("expected Secure=true on session cookie with HTTPS base URL")
			}
			return
		}
	}
	t.Error("session_id cookie not found")
}

func TestLogin_SecureCookie_HTTPS(t *testing.T) {
	h := newAuthHandler(t, "https://example.com")

	// First create a user via setup (ignore if already exists)
	setupBody, _ := json.Marshal(map[string]string{
		"email":    "login-secure@test.com",
		"password": "testpassword123",
	})
	setupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewReader(setupBody))
	setupRR := httptest.NewRecorder()
	h.Setup(setupRR, setupReq)

	body, _ := json.Marshal(map[string]string{
		"email":    "login-secure@test.com",
		"password": "testpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code == http.StatusOK {
		for _, c := range rr.Result().Cookies() {
			if c.Name == "session_id" {
				if !c.Secure {
					t.Error("expected Secure=true on session cookie with HTTPS base URL")
				}
				return
			}
		}
		t.Error("session_id cookie not found")
	}
}

func TestLogin_SecureCookie_HTTP(t *testing.T) {
	h := newAuthHandler(t, "http://localhost:8080")

	setupBody, _ := json.Marshal(map[string]string{
		"email":    "login-http@test.com",
		"password": "testpassword123",
	})
	setupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup", bytes.NewReader(setupBody))
	setupRR := httptest.NewRecorder()
	h.Setup(setupRR, setupReq)

	body, _ := json.Marshal(map[string]string{
		"email":    "login-http@test.com",
		"password": "testpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code == http.StatusOK {
		for _, c := range rr.Result().Cookies() {
			if c.Name == "session_id" {
				if c.Secure {
					t.Error("expected Secure=false on session cookie with HTTP base URL")
				}
				return
			}
		}
		t.Error("session_id cookie not found")
	}
}

func TestLogout_SecureCookie(t *testing.T) {
	h := newAuthHandler(t, "https://example.com")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "fake-session"})
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	for _, c := range rr.Result().Cookies() {
		if c.Name == "session_id" {
			if !c.Secure {
				t.Error("expected Secure=true on logout cookie with HTTPS base URL")
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Error("expected SameSite=Lax on logout cookie")
			}
			return
		}
	}
	t.Error("session_id cookie not found")
}
