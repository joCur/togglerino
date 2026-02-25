# Togglerino Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a self-hosted feature flag management platform with a Go backend, React dashboard, and JS/TS SDK.

**Architecture:** Single Go monolith serving management API, evaluation API, SSE streaming, and an embedded React dashboard. PostgreSQL for storage. Clean internal package boundaries.

**Tech Stack:** Go, PostgreSQL, React (Vite + TypeScript), chi router, pgx (Postgres driver), bcrypt, crypto/rand

---

## Phase 1: Foundation

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/togglerino/main.go`
- Create: `internal/config/config.go`
- Create: `docker-compose.yml`
- Create: `.gitignore`

**Step 1: Initialize Go module**

Run: `go mod init github.com/togglerino/togglerino`

**Step 2: Create .gitignore**

```gitignore
# Binaries
/togglerino
*.exe

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Environment
.env

# Web build output (committed separately)
web/node_modules/
```

**Step 3: Create docker-compose.yml**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: togglerino
      POSTGRES_PASSWORD: togglerino
      POSTGRES_DB: togglerino
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

**Step 4: Create config package**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        envOr("PORT", "8080"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"),
	}
	return cfg, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

**Step 5: Create main.go (minimal — starts HTTP server)**

```go
// cmd/togglerino/main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/togglerino/togglerino/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Printf("togglerino starting on %s\n", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), mux); err != nil {
		log.Fatal(err)
	}
}
```

**Step 6: Verify it compiles and runs**

Run: `go build ./cmd/togglerino && ./togglerino &`
Run: `curl http://localhost:8080/healthz`
Expected: `{"status":"ok"}`
Kill the process after verification.

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: project scaffolding with Go module, config, health endpoint, docker-compose"
```

---

### Task 2: Database Connection & Migration Framework

**Files:**
- Create: `internal/store/db.go`
- Create: `internal/store/migrate.go`
- Create: `migrations/001_initial_schema.up.sql`
- Create: `migrations/001_initial_schema.down.sql`
- Modify: `cmd/togglerino/main.go`
- Modify: `go.mod` (add pgx dependency)

**Step 1: Add pgx dependency**

Run: `go get github.com/jackc/pgx/v5/pgxpool`

**Step 2: Create database connection helper**

```go
// internal/store/db.go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}
```

**Step 3: Create migration runner**

Use a simple embedded-SQL approach — read `.sql` files from an embedded filesystem and execute them in order. Track applied migrations in a `schema_migrations` table.

```go
// internal/store/migrate.go
package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// BUT we actually embed from the top-level migrations/ dir.
// We'll pass it in instead.

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrations embed.FS) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("reading migrations directory: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		version := strings.TrimSuffix(name, ".up.sql")

		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		sql, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("beginning transaction for %s: %w", version, err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("executing migration %s: %w", version, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing migration %s: %w", version, err)
		}

		fmt.Printf("Applied migration: %s\n", version)
	}

	return nil
}
```

**Step 4: Create initial schema migration**

```sql
-- migrations/001_initial_schema.up.sql

-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Projects
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Environments
CREATE TABLE environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key)
);

-- SDK Keys
CREATE TABLE sdk_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key TEXT NOT NULL UNIQUE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Flags
CREATE TABLE flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    flag_type TEXT NOT NULL DEFAULT 'boolean' CHECK (flag_type IN ('boolean', 'string', 'number', 'json')),
    default_value JSONB NOT NULL DEFAULT 'false',
    tags TEXT[] NOT NULL DEFAULT '{}',
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key)
);

-- Flag Environment Configs
CREATE TABLE flag_environment_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    flag_id UUID NOT NULL REFERENCES flags(id) ON DELETE CASCADE,
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    default_variant TEXT NOT NULL DEFAULT 'off',
    variants JSONB NOT NULL DEFAULT '[]',
    targeting_rules JSONB NOT NULL DEFAULT '[]',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(flag_id, environment_id)
);

-- Audit Log
CREATE TABLE audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    old_value JSONB,
    new_value JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_log_project_id ON audit_log(project_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC);
```

```sql
-- migrations/001_initial_schema.down.sql
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS flag_environment_configs;
DROP TABLE IF EXISTS flags;
DROP TABLE IF EXISTS sdk_keys;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
```

**Step 5: Wire database + migrations into main.go**

Update `cmd/togglerino/main.go` to connect to Postgres on startup, run migrations, then start the server. Add an `embed.FS` in main for the migrations directory.

```go
// Add to cmd/togglerino/main.go
import "embed"

//go:embed all:../../migrations
var migrations embed.FS
```

In `main()`, after loading config:

```go
ctx := context.Background()
pool, err := store.NewPool(ctx, cfg.DatabaseURL)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

if err := store.RunMigrations(ctx, pool, migrations); err != nil {
    log.Fatal(err)
}
```

**Step 6: Start Postgres, verify migrations run**

Run: `docker compose up -d`
Run: `go run ./cmd/togglerino`
Expected: See "Applied migration: 001_initial_schema" in output and server starts.

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: database connection, migration framework, initial schema"
```

---

### Task 3: Domain Models

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/model/project.go`
- Create: `internal/model/environment.go`
- Create: `internal/model/flag.go`
- Create: `internal/model/audit.go`

**Step 1: Create all model files**

```go
// internal/model/user.go
package model

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
```

```go
// internal/model/project.go
package model

import "time"

type Project struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
```

```go
// internal/model/environment.go
package model

import "time"

type Environment struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type SDKKey struct {
	ID            string    `json:"id"`
	Key           string    `json:"key"`
	EnvironmentID string    `json:"environment_id"`
	Name          string    `json:"name"`
	Revoked       bool      `json:"revoked"`
	CreatedAt     time.Time `json:"created_at"`
}
```

```go
// internal/model/flag.go
package model

import (
	"encoding/json"
	"time"
)

type FlagType string

const (
	FlagTypeBoolean FlagType = "boolean"
	FlagTypeString  FlagType = "string"
	FlagTypeNumber  FlagType = "number"
	FlagTypeJSON    FlagType = "json"
)

type Flag struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"project_id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	FlagType     FlagType        `json:"flag_type"`
	DefaultValue json.RawMessage `json:"default_value"`
	Tags         []string        `json:"tags"`
	Archived     bool            `json:"archived"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type FlagEnvironmentConfig struct {
	ID             string          `json:"id"`
	FlagID         string          `json:"flag_id"`
	EnvironmentID  string          `json:"environment_id"`
	Enabled        bool            `json:"enabled"`
	DefaultVariant string          `json:"default_variant"`
	Variants       []Variant       `json:"variants"`
	TargetingRules []TargetingRule `json:"targeting_rules"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Variant struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type TargetingRule struct {
	Conditions        []Condition `json:"conditions"`
	Variant           string      `json:"variant"`
	PercentageRollout *int        `json:"percentage_rollout,omitempty"`
}

type Condition struct {
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Value     any    `json:"value"`
}

type Operator string

const (
	OpEquals      Operator = "equals"
	OpNotEquals   Operator = "not_equals"
	OpContains    Operator = "contains"
	OpNotContains Operator = "not_contains"
	OpStartsWith  Operator = "starts_with"
	OpEndsWith    Operator = "ends_with"
	OpGreaterThan Operator = "greater_than"
	OpLessThan    Operator = "less_than"
	OpGTE         Operator = "gte"
	OpLTE         Operator = "lte"
	OpIn          Operator = "in"
	OpNotIn       Operator = "not_in"
	OpExists      Operator = "exists"
	OpNotExists   Operator = "not_exists"
	OpMatches     Operator = "matches"
)

// EvaluationContext is what clients send when requesting flag evaluation.
type EvaluationContext struct {
	UserID     string         `json:"user_id"`
	Attributes map[string]any `json:"attributes"`
}

// EvaluationResult is the response for a single flag evaluation.
type EvaluationResult struct {
	Value   any    `json:"value"`
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}
```

```go
// internal/model/audit.go
package model

import (
	"encoding/json"
	"time"
)

type AuditEntry struct {
	ID         string          `json:"id"`
	ProjectID  *string         `json:"project_id,omitempty"`
	UserID     *string         `json:"user_id,omitempty"`
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	OldValue   json.RawMessage `json:"old_value,omitempty"`
	NewValue   json.RawMessage `json:"new_value,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 3: Commit**

```bash
git add -A
git commit -m "feat: domain model types for users, projects, environments, flags, audit"
```

---

## Phase 2: Auth & Store Layer

### Task 4: User & Session Store

**Files:**
- Create: `internal/store/user_store.go`
- Create: `internal/store/user_store_test.go`

**Step 1: Write failing tests for user store**

Test creating a user, finding by email, and finding by ID. Use a test helper that connects to a real Postgres (from `DATABASE_URL` env var or default docker-compose URL). Each test should run in a transaction that gets rolled back.

```go
// internal/store/user_store_test.go
package store_test

import (
	"context"
	"testing"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestUserStore_Create(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := store.NewUserStore(pool)

	user, err := s.Create(ctx, "test@example.com", "hashedpw", model.RoleAdmin)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("got email %q, want test@example.com", user.Email)
	}
	if user.Role != model.RoleAdmin {
		t.Errorf("got role %q, want admin", user.Role)
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestUserStore_FindByEmail(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := store.NewUserStore(pool)

	_, err := s.Create(ctx, "find@example.com", "hashedpw", model.RoleMember)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	user, err := s.FindByEmail(ctx, "find@example.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if user.Email != "find@example.com" {
		t.Errorf("got email %q, want find@example.com", user.Email)
	}
}

func TestUserStore_Count(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	s := store.NewUserStore(pool)

	count, err := s.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	// Count should be >= 0 (exact value depends on test isolation)
	if count < 0 {
		t.Errorf("got count %d, want >= 0", count)
	}
}
```

Also create a test helper file:

```go
// internal/store/testhelper_test.go
package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/ -v -run TestUserStore`
Expected: FAIL — `store.NewUserStore` doesn't exist.

**Step 3: Implement UserStore**

```go
// internal/store/user_store.go
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) Create(ctx context.Context, email, passwordHash string, role model.Role) (*model.User, error) {
	var user model.User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, role, created_at, updated_at`,
		email, passwordHash, role,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}
	return &user, nil
}

func (s *UserStore) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding user by email: %w", err)
	}
	return &user, nil
}

func (s *UserStore) FindByID(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, created_at, updated_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding user by id: %w", err)
	}
	return &user, nil
}

func (s *UserStore) Count(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/store/ -v -run TestUserStore`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: user store with create, find by email/id, count"
```

---

### Task 5: Session Store

**Files:**
- Create: `internal/store/session_store.go`
- Create: `internal/store/session_store_test.go`

**Step 1: Write failing tests**

```go
// internal/store/session_store_test.go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

func TestSessionStore_CreateAndFind(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ss := store.NewSessionStore(pool)

	user, err := us.Create(ctx, "session@example.com", "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, err := ss.Create(ctx, user.ID, 24*time.Hour)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}
	if session.UserID != user.ID {
		t.Errorf("got user_id %q, want %q", session.UserID, user.ID)
	}

	found, err := ss.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.UserID != user.ID {
		t.Errorf("got user_id %q, want %q", found.UserID, user.ID)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ss := store.NewSessionStore(pool)

	user, _ := us.Create(ctx, "delete-session@example.com", "hash", model.RoleAdmin)
	session, _ := ss.Create(ctx, user.ID, 24*time.Hour)

	err := ss.Delete(ctx, session.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = ss.FindByID(ctx, session.ID)
	if err == nil {
		t.Error("expected error after deleting session, got nil")
	}
}
```

**Step 2: Run tests, verify fail**

Run: `go test ./internal/store/ -v -run TestSessionStore`
Expected: FAIL

**Step 3: Implement SessionStore**

```go
// internal/store/session_store.go
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/model"
)

type SessionStore struct {
	pool *pgxpool.Pool
}

func NewSessionStore(pool *pgxpool.Pool) *SessionStore {
	return &SessionStore{pool: pool}
}

func (s *SessionStore) Create(ctx context.Context, userID string, duration time.Duration) (*model.Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("generating session id: %w", err)
	}

	expiresAt := time.Now().Add(duration)
	var session model.Session
	err = s.pool.QueryRow(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)
		 RETURNING id, user_id, expires_at, created_at`,
		id, userID, expiresAt,
	).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return &session, nil
}

func (s *SessionStore) FindByID(ctx context.Context, id string) (*model.Session, error) {
	var session model.Session
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, expires_at, created_at FROM sessions
		 WHERE id = $1 AND expires_at > NOW()`,
		id,
	).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("finding session: %w", err)
	}
	return &session, nil
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return fmt.Errorf("deleting expired sessions: %w", err)
	}
	return nil
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

**Step 4: Run tests, verify pass**

Run: `go test ./internal/store/ -v -run TestSessionStore`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: session store with create, find, delete, expired cleanup"
```

---

### Task 6: Auth Package (Password Hashing + Auth Middleware)

**Files:**
- Create: `internal/auth/password.go`
- Create: `internal/auth/password_test.go`
- Create: `internal/auth/middleware.go`

**Step 1: Write failing tests for password hashing**

```go
// internal/auth/password_test.go
package auth_test

import (
	"testing"

	"github.com/togglerino/togglerino/internal/auth"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := auth.HashPassword("mysecretpassword")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !auth.VerifyPassword(hash, "mysecretpassword") {
		t.Error("VerifyPassword returned false for correct password")
	}

	if auth.VerifyPassword(hash, "wrongpassword") {
		t.Error("VerifyPassword returned true for wrong password")
	}
}
```

**Step 2: Run test, verify fail**

Run: `go test ./internal/auth/ -v`
Expected: FAIL

**Step 3: Implement password hashing**

Run: `go get golang.org/x/crypto/bcrypt`

```go
// internal/auth/password.go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

**Step 4: Run tests, verify pass**

Run: `go test ./internal/auth/ -v`
Expected: PASS

**Step 5: Implement auth middleware**

```go
// internal/auth/middleware.go
package auth

import (
	"context"
	"net/http"

	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type contextKey string

const userContextKey contextKey = "user"

func UserFromContext(ctx context.Context) *model.User {
	u, _ := ctx.Value(userContextKey).(*model.User)
	return u
}

// SessionAuth middleware checks for a valid session cookie and loads the user.
func SessionAuth(sessions *store.SessionStore, users *store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessions.FindByID(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			user, err := users.FindByID(r.Context(), session.UserID)
			if err != nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole middleware checks that the authenticated user has the required role.
func RequireRole(role model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil || user.Role != role {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r.WithContext(r.Context()))
		})
	}
}
```

**Step 6: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: auth package with bcrypt password hashing and session middleware"
```

---

## Phase 3: Management API

### Task 7: Auth Handlers (Setup, Login, Logout)

**Files:**
- Create: `internal/handler/auth_handler.go`
- Create: `internal/handler/auth_handler_test.go`
- Create: `internal/handler/helpers.go`

**Step 1: Create JSON helper functions**

```go
// internal/handler/helpers.go
package handler

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
```

**Step 2: Implement auth handlers**

```go
// internal/handler/auth_handler.go
package handler

import (
	"net/http"
	"time"

	"github.com/togglerino/togglerino/internal/auth"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
)

type AuthHandler struct {
	users    *store.UserStore
	sessions *store.SessionStore
}

func NewAuthHandler(users *store.UserStore, sessions *store.SessionStore) *AuthHandler {
	return &AuthHandler{users: users, sessions: sessions}
}

// POST /api/v1/auth/setup — create the initial admin user (only works when no users exist)
func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	count, err := h.users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, "setup already completed")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.users.Create(r.Context(), req.Email, hash, model.RoleAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	session, err := h.sessions.Create(r.Context(), user.ID, 7*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	writeJSON(w, http.StatusCreated, user)
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.users.FindByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	session, err := h.sessions.Create(r.Context(), user.ID, 7*24*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60,
	})

	writeJSON(w, http.StatusOK, user)
}

// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		h.sessions.Delete(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// GET /api/v1/auth/me — returns the current user (requires session)
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// GET /api/v1/auth/status — returns whether setup is needed (no auth required)
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.Count(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": count == 0,
	})
}
```

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: auth handlers for setup, login, logout, me, status"
```

---

### Task 8: Project Store + Handlers

**Files:**
- Create: `internal/store/project_store.go`
- Create: `internal/store/project_store_test.go`
- Create: `internal/handler/project_handler.go`

Follow the same TDD pattern: write failing tests for the store (Create, List, FindByKey, Update, Delete), implement the store, then write the HTTP handler that delegates to it. The handler methods map 1:1 to the management API routes for projects.

**Store methods:** `Create(ctx, key, name, description)`, `List(ctx)`, `FindByKey(ctx, key)`, `Update(ctx, key, name, description)`, `Delete(ctx, key)`

**Handler routes:**
- `POST /api/v1/projects` → Create
- `GET /api/v1/projects` → List
- `GET /api/v1/projects/{key}` → Get
- `PUT /api/v1/projects/{key}` → Update
- `DELETE /api/v1/projects/{key}` → Delete

**Commit:** `"feat: project store and management API handlers"`

---

### Task 9: Environment Store + Handlers

**Files:**
- Create: `internal/store/environment_store.go`
- Create: `internal/store/environment_store_test.go`
- Create: `internal/handler/environment_handler.go`

**Store methods:** `Create(ctx, projectID, key, name)`, `ListByProject(ctx, projectID)`, `FindByKey(ctx, projectID, key)`, `Delete(ctx, id)`

When creating a project's first environment, auto-create `development`, `staging`, `production` as defaults (do this in the project handler's Create, not in the environment store).

**Handler routes:**
- `POST /api/v1/projects/{key}/environments` → Create
- `GET /api/v1/projects/{key}/environments` → List

**Commit:** `"feat: environment store and management API handlers"`

---

### Task 10: SDK Key Store + Handlers

**Files:**
- Create: `internal/store/sdk_key_store.go`
- Create: `internal/store/sdk_key_store_test.go`
- Create: `internal/handler/sdk_key_handler.go`

**Store methods:** `Create(ctx, environmentID, name)`, `ListByEnvironment(ctx, environmentID)`, `FindByKey(ctx, key)`, `Revoke(ctx, id)`

SDK key format: `sdk_` + 32 random hex characters.

**Commit:** `"feat: SDK key store and handlers"`

---

### Task 11: Flag Store + Handlers

**Files:**
- Create: `internal/store/flag_store.go`
- Create: `internal/store/flag_store_test.go`
- Create: `internal/handler/flag_handler.go`

**Store methods:**
- `Create(ctx, projectID, key, name, description, flagType, defaultValue, tags)` — also creates a `flag_environment_config` row for each environment in the project
- `ListByProject(ctx, projectID)` — with optional tag and search filters
- `FindByKey(ctx, projectID, key)` — includes all environment configs
- `Update(ctx, flagID, name, description, tags)`
- `Delete(ctx, flagID)`
- `UpdateEnvironmentConfig(ctx, flagID, environmentID, config)` — updates enabled, variants, rules, default_variant
- `GetEnvironmentConfig(ctx, flagID, environmentID)`

**Handler routes:** map to the flag management API routes from the design doc.

**Commit:** `"feat: flag store and management API handlers"`

---

### Task 12: Audit Log Store + Handler

**Files:**
- Create: `internal/store/audit_store.go`
- Create: `internal/handler/audit_handler.go`

**Store methods:** `Record(ctx, entry)`, `ListByProject(ctx, projectID, limit, offset)`

Integrate audit recording into the project, environment, and flag handlers — call `audit.Record()` after each mutation.

**Commit:** `"feat: audit log store, handler, and integration into mutation endpoints"`

---

## Phase 4: Evaluation Engine & Real-Time

### Task 13: Evaluation Engine

**Files:**
- Create: `internal/evaluation/engine.go`
- Create: `internal/evaluation/engine_test.go`
- Create: `internal/evaluation/operators.go`
- Create: `internal/evaluation/operators_test.go`
- Create: `internal/evaluation/hash.go`
- Create: `internal/evaluation/hash_test.go`

This is the core of togglerino. Build it with comprehensive tests.

**Step 1: Write operator tests**

Test every operator (equals, not_equals, contains, in, matches, etc.) with valid inputs, edge cases, and type mismatches.

**Step 2: Implement operators**

Each operator is a function `func(attributeValue any, conditionValue any) bool`.

**Step 3: Write hash tests**

Verify consistent hashing: same input always gives same bucket, different flag keys distribute differently, distribution is roughly uniform across a large sample.

**Step 4: Implement consistent hashing**

Use `crypto/sha256` on `flag_key + user_id`, take first 8 bytes as uint64, mod 100.

**Step 5: Write engine tests**

Test the full evaluation flow:
- Flag disabled → returns default
- Flag archived → returns default
- No rules match → returns default variant
- Single rule matches → returns rule variant
- Multiple rules, first match wins
- Percentage rollout with deterministic results
- Complex conditions with multiple attributes

**Step 6: Implement engine**

```go
func (e *Engine) Evaluate(flag *model.Flag, config *model.FlagEnvironmentConfig, ctx *model.EvaluationContext) *model.EvaluationResult
```

**Commit:** `"feat: evaluation engine with operators, consistent hashing, targeting rules"`

---

### Task 14: In-Memory Flag Cache

**Files:**
- Create: `internal/evaluation/cache.go`
- Create: `internal/evaluation/cache_test.go`

A thread-safe in-memory cache that holds all flag configs, keyed by `projectKey:environmentKey`. Loaded from Postgres on startup, refreshed per project/environment when a flag changes.

**Methods:** `LoadAll(ctx)`, `Refresh(ctx, projectID, environmentID)`, `GetFlags(projectKey, envKey)`, `GetFlag(projectKey, envKey, flagKey)`

Uses `sync.RWMutex` for concurrent reads.

**Commit:** `"feat: in-memory flag config cache with concurrent-safe access"`

---

### Task 15: Client Evaluation API

**Files:**
- Create: `internal/handler/evaluate_handler.go`
- Create: `internal/handler/evaluate_handler_test.go`
- Create: `internal/auth/sdk_auth.go`

**SDK auth middleware:** Reads `Authorization: Bearer sdk_xxx` header, looks up SDK key, resolves to project + environment, rejects if revoked.

**Handler routes:**
- `POST /api/v1/evaluate/{project}/{env}` → evaluate all flags
- `POST /api/v1/evaluate/{project}/{env}/{flag}` → evaluate single flag

Both use the in-memory cache + evaluation engine. Never hit the database per request.

**Commit:** `"feat: client evaluation API with SDK key auth"`

---

### Task 16: SSE Streaming

**Files:**
- Create: `internal/stream/hub.go`
- Create: `internal/stream/hub_test.go`
- Create: `internal/handler/stream_handler.go`

**Hub:** Manages SSE connections per project/environment. When a flag changes, the hub broadcasts the new evaluation to all connected clients for that scope.

**Methods:** `Subscribe(projectKey, envKey) <-chan Event`, `Unsubscribe(projectKey, envKey, ch)`, `Broadcast(projectKey, envKey, event)`

**Handler:** `GET /api/v1/stream/{project}/{env}` — sets SSE headers (`Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`), authenticates via SDK key, subscribes to hub, writes events until client disconnects.

Integrate broadcasting into flag update handlers — after updating a flag config, call `hub.Broadcast()`.

**Commit:** `"feat: SSE streaming hub and handler with real-time flag updates"`

---

## Phase 5: Wire It All Together

### Task 17: Router & Server Assembly

**Files:**
- Modify: `cmd/togglerino/main.go`

Wire all handlers, stores, middleware, cache, and hub into the HTTP router. Use Go 1.22+ `http.ServeMux` pattern matching (`GET /path`, `POST /path`). Group routes:

- Public: `/api/v1/auth/status`, `/api/v1/auth/setup`, `/api/v1/auth/login`
- Authed (session): `/api/v1/auth/me`, `/api/v1/auth/logout`, all management API routes
- SDK authed: `/api/v1/evaluate/*`, `/api/v1/stream/*`

Add CORS middleware for development (allow localhost origins).

**Step 1: Build and run the full server**

Run: `docker compose up -d && go run ./cmd/togglerino`

**Step 2: Manual smoke test**

```bash
# Check status
curl http://localhost:8080/api/v1/auth/status
# → {"setup_required":true}

# Setup admin
curl -X POST http://localhost:8080/api/v1/auth/setup \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@test.com","password":"secret123"}' -c cookies.txt

# Create project (with session cookie)
curl -X POST http://localhost:8080/api/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{"key":"web-app","name":"Web App"}' -b cookies.txt

# Create flag
curl -X POST http://localhost:8080/api/v1/projects/web-app/flags \
  -H 'Content-Type: application/json' \
  -d '{"key":"dark-mode","name":"Dark Mode","flag_type":"boolean","default_value":false}' -b cookies.txt

# Evaluate (with SDK key from dashboard or direct DB)
curl -X POST http://localhost:8080/api/v1/evaluate/web-app/development \
  -H 'Authorization: Bearer sdk_xxx' \
  -H 'Content-Type: application/json' \
  -d '{"context":{"user_id":"u1","attributes":{}}}'
```

**Commit:** `"feat: wire all handlers into router, full server assembly"`

---

## Phase 6: React Dashboard

### Task 18: React Project Setup

**Files:**
- Create: `web/` (Vite + React + TypeScript project)
- Modify: `cmd/togglerino/main.go` (add `go:embed` for web/dist)

**Step 1: Scaffold React project**

Run (from repo root):
```bash
cd web && npm create vite@latest . -- --template react-ts
npm install
npm install react-router-dom
npm install @tanstack/react-query
```

**Step 2: Configure Vite proxy for development**

In `web/vite.config.ts`, proxy `/api` to `http://localhost:8080` so the React dev server can talk to the Go backend.

**Step 3: Embed built dashboard into Go binary**

In `cmd/togglerino/main.go`, add `//go:embed all:../../web/dist` and serve it as a fallback for non-API routes (SPA routing).

**Commit:** `"feat: React dashboard scaffolding with Vite, TanStack Query, React Router"`

---

### Task 19: Dashboard Auth Screens

**Files:**
- Create: `web/src/pages/SetupPage.tsx`
- Create: `web/src/pages/LoginPage.tsx`
- Create: `web/src/hooks/useAuth.ts`
- Create: `web/src/api/client.ts`

Build the setup wizard (shown on first run) and login page. `useAuth` hook wraps `/api/v1/auth/me` to check auth state and redirect accordingly.

**Commit:** `"feat: dashboard login and setup wizard"`

---

### Task 20: Dashboard Core Pages

**Files:**
- Create: `web/src/pages/ProjectsPage.tsx` — list projects
- Create: `web/src/pages/ProjectDetailPage.tsx` — flag list with search/filter
- Create: `web/src/pages/FlagDetailPage.tsx` — environment tabs, toggle, rule builder, rollout slider
- Create: `web/src/components/RuleBuilder.tsx` — visual targeting rule editor
- Create: `web/src/components/VariantEditor.tsx`
- Create: `web/src/components/RolloutSlider.tsx`

This is the largest frontend task. Build incrementally:
1. Projects list (CRUD)
2. Flag list within a project
3. Flag detail with environment tabs and enable/disable toggle
4. Variant editor
5. Rule builder (conditions + operator + value)
6. Rollout slider

**Commit:** `"feat: dashboard core pages — projects, flags, rule builder, rollout"`

---

### Task 21: Dashboard Settings Pages

**Files:**
- Create: `web/src/pages/EnvironmentsPage.tsx`
- Create: `web/src/pages/TeamPage.tsx`
- Create: `web/src/pages/SDKKeysPage.tsx`
- Create: `web/src/pages/AuditLogPage.tsx`

Build the remaining dashboard pages: environment management, team/user management, SDK key management, and the audit log viewer.

**Commit:** `"feat: dashboard settings — environments, team, SDK keys, audit log"`

---

## Phase 7: JS/TS SDK

### Task 22: SDK Core

**Files:**
- Create: `sdks/javascript/` (npm package)
- Create: `sdks/javascript/src/client.ts`
- Create: `sdks/javascript/src/types.ts`
- Create: `sdks/javascript/src/sse.ts`
- Create: `sdks/javascript/src/polling.ts`
- Create: `sdks/javascript/src/__tests__/client.test.ts`

Build the `@togglerino/sdk` package:
1. `Togglerino` class with `initialize()`, typed getters, `on()`, `updateContext()`, `close()`
2. SSE connection manager with auto-reconnect
3. Polling fallback
4. Event emitter for `change` and `error` events

Use vitest for testing. Mock fetch and EventSource.

**Commit:** `"feat: @togglerino/sdk — JS/TS client with SSE and polling"`

---

### Task 23: React SDK

**Files:**
- Create: `sdks/react/src/provider.tsx`
- Create: `sdks/react/src/hooks.ts`
- Create: `sdks/react/src/__tests__/hooks.test.tsx`

Build `@togglerino/react`:
- `<TogglerioProvider>` — wraps app, initializes client
- `useFlag(key, defaultValue)` — returns current flag value, re-renders on change
- `useTogglerino()` — returns the client instance

**Commit:** `"feat: @togglerino/react — provider and useFlag hook"`

---

## Phase 8: Deployment

### Task 24: Docker Build

**Files:**
- Create: `Dockerfile` (multi-stage: build React, build Go, produce minimal image)
- Modify: `docker-compose.yml` (add togglerino service)

Multi-stage Dockerfile:
1. Stage 1: `node:20-alpine` — build React dashboard
2. Stage 2: `golang:1.23-alpine` — copy React dist, build Go binary with embedded dashboard
3. Stage 3: `alpine` — copy binary, expose port, set entrypoint

Update `docker-compose.yml` to include the togglerino service with `depends_on: postgres`.

**Final smoke test:**

```bash
docker compose up --build
curl http://localhost:8080/api/v1/auth/status
# → {"setup_required":true}
# Open http://localhost:8080 in browser → see setup wizard
```

**Commit:** `"feat: Dockerfile and docker-compose for self-hosted deployment"`

---

## Summary

| Phase | Tasks | What it delivers |
|-------|-------|-----------------|
| 1. Foundation | 1-3 | Go project, DB, migrations, models |
| 2. Auth & Store | 4-6 | Users, sessions, password hashing, middleware |
| 3. Management API | 7-12 | Full CRUD API for projects, environments, flags, audit |
| 4. Evaluation | 13-16 | Evaluation engine, caching, client API, SSE streaming |
| 5. Assembly | 17 | Fully wired server |
| 6. Dashboard | 18-21 | React frontend with all screens |
| 7. JS/TS SDK | 22-23 | Client SDK + React bindings |
| 8. Deployment | 24 | Docker image for self-hosting |
