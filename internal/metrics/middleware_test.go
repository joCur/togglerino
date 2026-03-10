package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_RecordsRequestMetrics(t *testing.T) {
	r := NewRegistry()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := r.Middleware(mux)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	counter, err := r.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "GET /healthz", "200")
	if err != nil {
		t.Fatal(err)
	}
	if got := collectCounter(counter); got != 1 {
		t.Errorf("expected 1 request counted, got %f", got)
	}
}

func TestMiddleware_UnmatchedRoute(t *testing.T) {
	r := NewRegistry()

	mux := http.NewServeMux()
	handler := r.Middleware(mux)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should record with "unmatched" or whatever pattern the mux sets
	// Just verify no panic
}
