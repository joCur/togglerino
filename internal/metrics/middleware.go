package metrics

import (
	"net/http"
	"strconv"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, req)

		pattern := req.Pattern
		if pattern == "" {
			pattern = "unmatched"
		}

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(sw.status)

		r.HTTPRequestsTotal.WithLabelValues(req.Method, pattern, status).Inc()
		r.HTTPRequestDurationSeconds.WithLabelValues(req.Method, pattern).Observe(duration)
	})
}
