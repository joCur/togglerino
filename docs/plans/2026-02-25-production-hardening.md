# Production Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make togglerino production-ready for small team use with graceful shutdown, user management, CORS config, structured logging, password reset, SDK reconnection, and rate limiting.

**Architecture:** Seven independent hardening features layered onto the existing Go backend and React frontend. Structured logging goes first (everything else benefits from it), then config/CORS/shutdown, then the user management system (largest piece), then SDK improvements.

**Tech Stack:** Go stdlib (`log/slog`, `os/signal`), pgx/v5, React 19, TypeScript, TanStack Query

---

### Task 1: Structured Logging — slog Setup

**Files:**
- Create: `internal/logging/logging.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Create logging package**

Create `internal/logging/logging.go` — a thin setup function that configures `slog.Default()`:

```go
package logging

import (
	"log/slog"
	"os"
)

func Setup(format string) {
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, nil)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))
}
```

**Step 2: Add LOG_FORMAT to config**

In `internal/config/config.go`, add `LogFormat string` to the Config struct and load from env:

```go
type Config struct {
	Port        string
	DatabaseURL string
	LogFormat   string
}
```

In `Load()`, add: `LogFormat: getEnv("LOG_FORMAT", "json")`

**Step 3: Wire into main.go**

At the top of `main()` (before anything else), add:

```go
logging.Setup(cfg.LogFormat)
slog.Info("starting togglerino", "port", cfg.Port)
```

Replace all `fmt.Printf("warning:` calls in handler files with `slog.Warn(` or `slog.Error(` calls.

**Step 4: Commit**

```
feat: add structured logging with slog
```

---

### Task 2: Request Logging Middleware

**Files:**
- Create: `internal/logging/middleware.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Create request logging middleware**

```go
package logging

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
```

**Step 2: Wire into main.go**

Wrap the CORS middleware with the logging middleware in the `ListenAndServe` call:

```go
http.ListenAndServe(cfg.Addr(), logging.Middleware(corsMiddleware(mux)))
```

**Step 3: Test manually** — start server, make a request, verify JSON log lines appear.

**Step 4: Commit**

```
feat: add request logging middleware
```

---

### Task 3: CORS Configuration

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Add CORS_ORIGINS to config**

Add to Config struct:

```go
CORSOrigins []string
```

In `Load()`:

```go
CORSOrigins: parseCORSOrigins(getEnv("CORS_ORIGINS", "*")),
```

Add helper:

```go
func parseCORSOrigins(s string) []string {
	if s == "*" {
		return []string{"*"}
	}
	parts := strings.Split(s, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			origins = append(origins, t)
		}
	}
	return origins
}
```

**Step 2: Update CORS middleware in main.go**

Replace the hardcoded `"*"` in `corsMiddleware`. Accept `origins []string` as parameter. Check if `"*"` is in the list (allow all), otherwise check the request's `Origin` header against the whitelist:

```go
func corsMiddleware(origins []string, next http.Handler) http.Handler {
	allowAll := len(origins) == 1 && origins[0] == "*"
	originSet := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		originSet[o] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if _, ok := originSet[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Update the `ListenAndServe` call to pass `cfg.CORSOrigins`.

**Step 3: Add startup log**

```go
slog.Info("cors configured", "origins", cfg.CORSOrigins)
```

**Step 4: Commit**

```
feat: make CORS origins configurable via CORS_ORIGINS env var
```

---

### Task 4: Graceful Shutdown

**Files:**
- Modify: `cmd/togglerino/main.go`
- Modify: `internal/stream/hub.go`

**Step 1: Add Close method to Hub**

In `internal/stream/hub.go`, add a method that closes all subscriber channels:

```go
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, subs := range h.subscribers {
		for ch := range subs {
			close(ch)
		}
		delete(h.subscribers, key)
	}
}
```

**Step 2: Replace ListenAndServe with graceful shutdown in main.go**

Replace the current `http.ListenAndServe` (line ~153) with:

```go
srv := &http.Server{
	Addr:    cfg.Addr(),
	Handler: logging.Middleware(corsMiddleware(cfg.CORSOrigins, mux)),
}

go func() {
	slog.Info("server listening", "addr", cfg.Addr())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

slog.Info("shutting down server")
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
	slog.Error("server shutdown error", "error", err)
}
hub.Close()
pool.Close()
slog.Info("server stopped")
```

Add imports: `"os/signal"`, `"syscall"`.

**Step 3: Test manually** — start server, send SIGTERM, verify clean shutdown log.

**Step 4: Commit**

```
feat: add graceful shutdown with signal handling
```

---

### Task 5: Rate Limiting Middleware

**Files:**
- Create: `internal/ratelimit/ratelimit.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Write failing test**

Create `internal/ratelimit/ratelimit_test.go`:

```go
package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := New(5, 60) // 5 per 60 seconds
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := New(2, 60)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if i < 2 && rec.Code != 200 {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
		if i == 2 && rec.Code != 429 {
			t.Fatalf("request %d: expected 429, got %d", i, rec.Code)
		}
	}
}

func TestRateLimiter_SeparateIPs(t *testing.T) {
	rl := New(1, 60)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req1 := httptest.NewRequest("POST", "/", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != 200 {
		t.Fatal("IP1 first request should be 200")
	}

	req2 := httptest.NewRequest("POST", "/", nil)
	req2.RemoteAddr = "5.6.7.8:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatal("IP2 first request should be 200")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/ratelimit/... -v
```

**Step 3: Implement rate limiter**

Create `internal/ratelimit/ratelimit.go`:

```go
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type entry struct {
	count    int
	windowStart time.Time
}

type Limiter struct {
	mu       sync.Mutex
	entries  map[string]*entry
	limit    int
	windowSec int
}

func New(limit, windowSeconds int) *Limiter {
	return &Limiter{
		entries:   make(map[string]*entry),
		limit:     limit,
		windowSec: windowSeconds,
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}

		l.mu.Lock()
		now := time.Now()
		e, ok := l.entries[ip]
		if !ok || now.Sub(e.windowStart) > time.Duration(l.windowSec)*time.Second {
			l.entries[ip] = &entry{count: 1, windowStart: now}
			l.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}
		e.count++
		if e.count > l.limit {
			l.mu.Unlock()
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		l.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/ratelimit/... -v
```

**Step 5: Wire into main.go**

Create a rate limiter instance and wrap the auth endpoints:

```go
authLimiter := ratelimit.New(10, 60)
```

Apply to login, setup, and (later) invite-accept routes by wrapping them:

```go
mux.Handle("POST /api/auth/login", authLimiter.Middleware(http.HandlerFunc(authHandler.Login)))
mux.Handle("POST /api/auth/setup", authLimiter.Middleware(http.HandlerFunc(authHandler.Setup)))
```

**Step 6: Commit**

```
feat: add rate limiting middleware for auth endpoints
```

---

### Task 6: Database Migration for Invites

**Files:**
- Create: `migrations/002_invites.up.sql`
- Create: `migrations/002_invites.down.sql`

**Step 1: Write the up migration**

```sql
CREATE TABLE invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invites_token ON invites(token);
CREATE INDEX idx_invites_email ON invites(email);
```

**Step 2: Write the down migration**

```sql
DROP TABLE IF EXISTS invites;
```

**Step 3: Verify migration runs**

Start the server (or run tests that trigger migration). Check the table exists:

```bash
docker compose exec postgres psql -U togglerino -c '\d invites'
```

**Step 4: Commit**

```
feat: add invites table migration
```

---

### Task 7: Invite Store

**Files:**
- Create: `internal/store/invite_store.go`
- Create: `internal/store/invite_store_test.go`
- Modify: `internal/model/user.go`

**Step 1: Add Invite model**

Add to `internal/model/user.go`:

```go
type Invite struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       Role       `json:"role"`
	Token      string     `json:"token"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	InvitedBy  *string    `json:"invited_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}
```

**Step 2: Write failing tests**

Create `internal/store/invite_store_test.go` — test Create, FindByToken, MarkAccepted, and that expired tokens are rejected. Follow the existing store test patterns (use `testPool()`).

**Step 3: Implement InviteStore**

Create `internal/store/invite_store.go`:

```go
package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"togglerino/internal/model"
)

type InviteStore struct {
	pool *pgxpool.Pool
}

func NewInviteStore(pool *pgxpool.Pool) *InviteStore {
	return &InviteStore{pool: pool}
}

func (s *InviteStore) Create(ctx context.Context, invite *model.Invite) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO invites (email, role, token, expires_at, invited_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		invite.Email, invite.Role, invite.Token, invite.ExpiresAt, invite.InvitedBy,
	).Scan(&invite.ID, &invite.CreatedAt)
}

func (s *InviteStore) FindByToken(ctx context.Context, token string) (*model.Invite, error) {
	var inv model.Invite
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, role, token, expires_at, accepted_at, invited_by, created_at
		 FROM invites WHERE token = $1`, token,
	).Scan(&inv.ID, &inv.Email, &inv.Role, &inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.InvitedBy, &inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (s *InviteStore) MarkAccepted(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE invites SET accepted_at = $1 WHERE id = $2`,
		time.Now(), id,
	)
	return err
}

func (s *InviteStore) ListPending(ctx context.Context) ([]model.Invite, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, role, token, expires_at, accepted_at, invited_by, created_at
		 FROM invites WHERE accepted_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invites []model.Invite
	for rows.Next() {
		var inv model.Invite
		if err := rows.Scan(&inv.ID, &inv.Email, &inv.Role, &inv.Token, &inv.ExpiresAt, &inv.AcceptedAt, &inv.InvitedBy, &inv.CreatedAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/store/... -v -run Invite
```

**Step 5: Commit**

```
feat: add invite store with create, find, accept, list
```

---

### Task 8: User Management Handler — Invite, List, Delete Users

**Files:**
- Create: `internal/handler/user_handler.go`
- Modify: `internal/store/user_store.go` (add List and Delete methods)
- Modify: `cmd/togglerino/main.go`

**Step 1: Add List and Delete to UserStore**

In `internal/store/user_store.go`, add:

```go
func (s *UserStore) List(ctx context.Context) ([]model.User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, role, created_at, updated_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *UserStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
```

**Step 2: Create UserHandler**

Create `internal/handler/user_handler.go`:

```go
package handler

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"togglerino/internal/auth"
	"togglerino/internal/model"
	"togglerino/internal/store"
)

type UserHandler struct {
	users   *store.UserStore
	invites *store.InviteStore
}

func NewUserHandler(users *store.UserStore, invites *store.InviteStore) *UserHandler {
	return &UserHandler{users: users, invites: invites}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		slog.Error("failed to list users", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	// Strip password hashes from response
	type safeUser struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		CreatedAt time.Time `json:"created_at"`
	}
	safe := make([]safeUser, len(users))
	for i, u := range users {
		safe[i] = safeUser{ID: u.ID, Email: u.Email, Role: u.Role, CreatedAt: u.CreatedAt}
	}
	writeJSON(w, http.StatusOK, safe)
}

func (h *UserHandler) Invite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	if req.Role != string(model.RoleAdmin) && req.Role != string(model.RoleMember) {
		writeError(w, http.StatusBadRequest, "role must be admin or member")
		return
	}

	token := generateToken()
	user := auth.UserFromContext(r.Context())
	invite := &model.Invite{
		Email:     req.Email,
		Role:      model.Role(req.Role),
		Token:     token,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		InvitedBy: &user.ID,
	}
	if err := h.invites.Create(r.Context(), invite); err != nil {
		slog.Error("failed to create invite", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":         invite.ID,
		"token":      invite.Token,
		"expires_at": invite.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user := auth.UserFromContext(r.Context())
	if user.ID == id {
		writeError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}
	if err := h.users.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := h.invites.ListPending(r.Context())
	if err != nil {
		slog.Error("failed to list invites", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

**Step 3: Wire routes in main.go**

Create the stores and handler:

```go
inviteStore := store.NewInviteStore(pool)
userHandler := handler.NewUserHandler(userStore, inviteStore)
```

Add routes (admin-only via RequireRole):

```go
mux.Handle("GET /api/management/users", wrap(http.HandlerFunc(userHandler.List), sessionAuth, requireAdmin))
mux.Handle("POST /api/management/users/invite", wrap(http.HandlerFunc(userHandler.Invite), sessionAuth, requireAdmin))
mux.Handle("GET /api/management/users/invites", wrap(http.HandlerFunc(userHandler.ListInvites), sessionAuth, requireAdmin))
mux.Handle("DELETE /api/management/users/{id}", wrap(http.HandlerFunc(userHandler.Delete), sessionAuth, requireAdmin))
```

Where `requireAdmin` is:

```go
requireAdmin := auth.RequireRole(model.RoleAdmin)
```

**Step 4: Commit**

```
feat: add user management endpoints (list, invite, delete)
```

---

### Task 9: Accept Invite Endpoint

**Files:**
- Modify: `internal/handler/auth_handler.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Add AcceptInvite to AuthHandler**

The AuthHandler needs access to the InviteStore. Add it as a field, update the constructor, and update main.go accordingly.

Add the `AcceptInvite` method:

```go
func (h *AuthHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "token and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	invite, err := h.invites.FindByToken(r.Context(), req.Token)
	if err != nil {
		writeError(w, http.StatusNotFound, "invalid or expired invite")
		return
	}
	if invite.AcceptedAt != nil {
		writeError(w, http.StatusBadRequest, "invite already accepted")
		return
	}
	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "invite has expired")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user := &model.User{
		Email:        invite.Email,
		PasswordHash: hash,
		Role:         string(invite.Role),
	}
	if err := h.users.Create(r.Context(), user); err != nil {
		slog.Error("failed to create user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	if err := h.invites.MarkAccepted(r.Context(), invite.ID); err != nil {
		slog.Warn("failed to mark invite accepted", "error", err)
	}

	writeJSON(w, http.StatusCreated, map[string]string{"email": user.Email})
}
```

**Step 2: Add route in main.go** (public, no auth required, but rate limited):

```go
mux.Handle("POST /api/auth/accept-invite", authLimiter.Middleware(http.HandlerFunc(authHandler.AcceptInvite)))
```

**Step 3: Commit**

```
feat: add accept-invite endpoint for new user onboarding
```

---

### Task 10: Admin-Initiated Password Reset

**Files:**
- Modify: `internal/handler/user_handler.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Add ResetPassword to UserHandler**

Reuses the invite infrastructure — generates a token, stores it as an invite with a "reset" marker (same table, same flow). The accept-invite endpoint already handles creating credentials.

Actually, simpler approach: add a dedicated reset token flow. The admin generates a reset link. The user visits it, provides a new password.

Add to `user_handler.go`:

```go
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Verify user exists
	_, err := h.users.FindByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	token := generateToken()
	user := auth.UserFromContext(r.Context())
	invite := &model.Invite{
		Email:     "", // Will be filled from existing user
		Role:      "",
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		InvitedBy: &user.ID,
	}
	// Store reset token in invites table (reuse infrastructure)
	// We need the target user's email
	targetUser, _ := h.users.FindByID(r.Context(), id)
	invite.Email = targetUser.Email
	invite.Role = model.Role(targetUser.Role)

	if err := h.invites.Create(r.Context(), invite); err != nil {
		slog.Error("failed to create reset token", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create reset token")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"token":      invite.Token,
		"expires_at": invite.ExpiresAt.Format(time.RFC3339),
	})
}
```

Add a separate `ResetAccept` endpoint on AuthHandler that takes token + new password, finds the user by email, and updates their password hash. Add `UpdatePassword` method to UserStore.

**Step 2: Add UpdatePassword to UserStore**

```go
func (s *UserStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		passwordHash, id,
	)
	return err
}
```

**Step 3: Add ResetAccept to AuthHandler**

```go
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "token and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	invite, err := h.invites.FindByToken(r.Context(), req.Token)
	if err != nil || invite.AcceptedAt != nil || time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "invalid or expired reset token")
		return
	}

	user, err := h.users.FindByEmail(r.Context(), invite.Email)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.users.UpdatePassword(r.Context(), user.ID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	h.invites.MarkAccepted(r.Context(), invite.ID)
	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Add routes**

```go
mux.Handle("POST /api/management/users/{id}/reset-password", wrap(http.HandlerFunc(userHandler.ResetPassword), sessionAuth, requireAdmin))
mux.Handle("POST /api/auth/reset-password", authLimiter.Middleware(http.HandlerFunc(authHandler.ResetPassword)))
```

**Step 5: Commit**

```
feat: add admin-initiated password reset flow
```

---

### Task 11: Frontend — Team Page (Member List + Invite)

**Files:**
- Modify: `web/src/pages/TeamPage.tsx`
- Modify: `web/src/api/client.ts` (or add to existing API calls)

**Step 1: Add API functions**

Add to the API layer (wherever other API calls live):

```typescript
export const getUsers = () => api.get<User[]>('/management/users');
export const inviteUser = (email: string, role: string) =>
  api.post<{ id: string; token: string; expires_at: string }>('/management/users/invite', { email, role });
export const deleteUser = (id: string) => api.delete(`/management/users/${id}`);
export const getPendingInvites = () => api.get<Invite[]>('/management/users/invites');
```

**Step 2: Rewrite TeamPage**

Replace the placeholder TeamPage with:
- A table/list of current team members (email, role, joined date)
- An invite form (email + role dropdown)
- On invite success, show the invite link for copying
- A list of pending invites
- Delete user button (with confirmation, can't delete self)

Use TanStack Query (`useQuery`, `useMutation`) following existing patterns in the codebase.

**Step 3: Test manually** — verify member list loads, invite generates a link, delete works.

**Step 4: Commit**

```
feat: replace team page placeholder with member management UI
```

---

### Task 12: Frontend — Accept Invite Page

**Files:**
- Create: `web/src/pages/AcceptInvitePage.tsx`
- Modify: `web/src/App.tsx` (add route)

**Step 1: Create AcceptInvitePage**

A simple page at `/invite/:token` that:
- Shows a "Set your password" form (password + confirm password)
- Calls `POST /api/auth/accept-invite` with token from URL params + password
- On success, redirects to login page
- On error, shows the error message (expired, already used, etc.)

This route should be **outside** the auth wrapper (user isn't logged in yet). Add it alongside the SetupPage/LoginPage routes in App.tsx.

**Step 2: Add route to App.tsx**

Add before the auth-checking routes:

```tsx
<Route path="/invite/:token" element={<AcceptInvitePage />} />
```

Also add a similar page/route for password reset: `/reset-password/:token`.

**Step 3: Commit**

```
feat: add accept-invite and reset-password pages
```

---

### Task 13: SDK SSE Reconnection

**Files:**
- Modify: `sdks/javascript/src/client.ts`

**Step 1: Add reconnection logic to startSSE**

Modify the `startSSE()` method in the SDK client. When the stream ends or errors:

```typescript
private sseRetryCount = 0;
private maxRetryDelay = 30000;

private getRetryDelay(): number {
  const delay = Math.min(1000 * Math.pow(2, this.sseRetryCount), this.maxRetryDelay);
  this.sseRetryCount++;
  return delay;
}

private scheduleSSEReconnect(): void {
  const delay = this.getRetryDelay();
  this.emit('reconnecting', { attempt: this.sseRetryCount, delay });

  setTimeout(() => {
    if (!this.abortController?.signal.aborted) {
      this.startSSE();
    }
  }, delay);
}
```

In the existing `startSSE()` method:
- On stream end/error, call `scheduleSSEReconnect()` instead of just falling back to polling
- On successful reconnection, reset `sseRetryCount = 0` and emit `'reconnected'`
- Keep polling as fallback while SSE retries happen

**Step 2: Add reconnecting/reconnected event types**

Update the event types to include `'reconnecting'` and `'reconnected'`.

**Step 3: Write tests**

Add tests in `sdks/javascript/src/__tests__/` verifying:
- SSE failure triggers reconnection attempt
- Backoff delay increases exponentially
- Reconnection resets on success
- Close() stops reconnection attempts

**Step 4: Run SDK tests**

```bash
cd sdks/javascript && npm test
```

**Step 5: Commit**

```
feat: add SSE reconnection with exponential backoff to JS SDK
```

---

### Task 14: Role Enforcement on Existing Routes

**Files:**
- Modify: `cmd/togglerino/main.go`

**Step 1: Add RequireRole to admin-only routes**

Apply `requireAdmin` middleware to destructive operations:
- `DELETE /api/management/projects/{key}` — admin only
- Project settings updates could remain member-accessible

This is a judgement call. For a small team, making delete operations admin-only is sufficient. Keep everything else accessible to all authenticated users.

**Step 2: Commit**

```
feat: enforce admin role on destructive operations
```

---

### Task 15: Final Integration — docker-compose & Env Vars

**Files:**
- Modify: `docker-compose.yml`
- Modify: `CLAUDE.md`

**Step 1: Update docker-compose.yml**

Add the new env vars with sensible defaults:

```yaml
environment:
  - DATABASE_URL=postgres://togglerino:togglerino@postgres:5432/togglerino?sslmode=disable
  - PORT=8080
  - CORS_ORIGINS=*
  - LOG_FORMAT=json
```

**Step 2: Update CLAUDE.md**

Add the new env vars to the Environment Variables section:

```
- `CORS_ORIGINS` — Comma-separated allowed origins (default: `*`)
- `LOG_FORMAT` — Log format: `json` or `text` (default: `json`)
```

**Step 3: End-to-end smoke test**

```bash
docker compose up --build
```

Verify:
1. Server starts with JSON log output
2. Login works
3. Create a project, flag, environment
4. Invite a user, copy the link
5. Accept invite in incognito
6. New user can log in and see flags
7. SIGTERM gracefully shuts down

**Step 4: Commit**

```
docs: update config docs and docker-compose with new env vars
```
