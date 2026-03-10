package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ServesPrometheusMetrics(t *testing.T) {
	r := NewRegistry()
	r.SetCacheFlagCount(10)

	handler := r.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "togglerino_cache_flags_total 10") {
		t.Errorf("expected cache_flags_total metric in output, got:\n%s", body)
	}
	if !strings.Contains(body, "go_goroutines") {
		t.Errorf("expected go runtime metrics in output")
	}
}
