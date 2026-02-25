package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type entry struct {
	count       int
	windowStart time.Time
}

// Limiter implements a fixed-window rate limiter keyed by client IP.
type Limiter struct {
	mu            sync.Mutex
	entries       map[string]*entry
	limit         int
	windowSeconds int
}

// New creates a new Limiter that allows limit requests per windowSeconds
// from a single IP address.
func New(limit, windowSeconds int) *Limiter {
	return &Limiter{
		entries:       make(map[string]*entry),
		limit:         limit,
		windowSeconds: windowSeconds,
	}
}

// Middleware returns an http.Handler that enforces the rate limit before
// passing the request to next. If the limit is exceeded, it responds with
// HTTP 429 and a JSON error body.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// If we can't parse the address, use RemoteAddr as-is.
			ip = r.RemoteAddr
		}

		l.mu.Lock()

		now := time.Now()
		window := time.Duration(l.windowSeconds) * time.Second

		e, exists := l.entries[ip]
		if !exists || now.Sub(e.windowStart) >= window {
			// New window: create or reset the entry.
			l.entries[ip] = &entry{
				count:       1,
				windowStart: now,
			}
			l.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		if e.count >= l.limit {
			remaining := window - now.Sub(e.windowStart)
			retryAfter := int(remaining.Seconds()) + 1
			l.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many requests"}`))
			return
		}

		e.count++
		l.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}
