package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	limiter := New(5, 60)
	handler := limiter.Middleware(okHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	limiter := New(2, 60)
	handler := limiter.Middleware(okHandler())

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, rr.Code)
		}
	}

	// Third request should be blocked
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("request 3: expected status 429, got %d", rr.Code)
	}

	// Verify Retry-After header is set
	if rr.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header to be set")
	}

	// Verify JSON error response body
	body := rr.Body.String()
	expected := `{"error":"too many requests"}`
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

func TestRateLimiter_SeparateIPs(t *testing.T) {
	limiter := New(1, 60)
	handler := limiter.Middleware(okHandler())

	// First IP — first request should pass
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req1.RemoteAddr = "1.2.3.4:1111"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("IP 1, request 1: expected status 200, got %d", rr1.Code)
	}

	// Second IP — first request should also pass
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req2.RemoteAddr = "5.6.7.8:2222"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("IP 2, request 1: expected status 200, got %d", rr2.Code)
	}

	// First IP — second request should be blocked (limit is 1)
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req3.RemoteAddr = "1.2.3.4:3333"
	rr3 := httptest.NewRecorder()
	handler.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusTooManyRequests {
		t.Errorf("IP 1, request 2: expected status 429, got %d", rr3.Code)
	}

	// Second IP — second request should also be blocked (limit is 1)
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req4.RemoteAddr = "5.6.7.8:4444"
	rr4 := httptest.NewRecorder()
	handler.ServeHTTP(rr4, req4)

	if rr4.Code != http.StatusTooManyRequests {
		t.Errorf("IP 2, request 2: expected status 429, got %d", rr4.Code)
	}
}
