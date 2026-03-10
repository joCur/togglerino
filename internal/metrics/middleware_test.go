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

	// Go's ServeMux returns 404 for unmatched routes but still sets a pattern.
	// Verify the request was recorded (not panicking) with some path label.
	counter, err := r.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "unmatched", "404")
	if err != nil {
		t.Fatal(err)
	}
	got := collectCounter(counter)
	if got == 1 {
		return // "unmatched" label used as expected
	}
	// ServeMux may set a catch-all pattern — verify at least one counter was incremented
	counter2, err := r.HTTPRequestsTotal.GetMetricWithLabelValues("GET", "/", "404")
	if err != nil {
		t.Fatal(err)
	}
	if collectCounter(counter2) != 1 && got != 1 {
		t.Error("expected unmatched request to be recorded with either 'unmatched' or '/' path label")
	}
}
