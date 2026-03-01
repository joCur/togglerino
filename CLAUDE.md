# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Togglerino

A self-hosted feature flag management platform. Single Go binary serves: management API, client/SDK evaluation API, embedded React dashboard, and SSE streaming for real-time flag updates.

Go module: `github.com/togglerino/togglerino` (Go 1.25, stdlib `net/http` + `log/slog`, no external HTTP or logging frameworks). Key deps: `pgx/v5`, `golang.org/x/crypto`.

## Build & Run Commands

### Backend (Go)

```bash
go build -o togglerino ./cmd/togglerino   # Build binary (requires web/dist/ to exist)
go test ./...                              # Run all tests
go test ./internal/evaluation/...          # Run tests for a single package
```

**Important**: The frontend must be built before `go build` because `web/dist/` is embedded via `go:embed`. CI handles this explicitly.

### Frontend (React dashboard, embedded in Go binary)

```bash
cd web && npm install && npm run build     # Build dashboard (runs tsc -b && vite build)
cd web && npm run dev                      # Vite dev server
cd web && npm run lint                     # ESLint
```

### SDKs

```bash
cd sdks/javascript && npm test             # JavaScript SDK tests (vitest)
cd sdks/react && npm test                  # React SDK tests (vitest)
cd sdks/dotnet && dotnet test              # .NET SDK tests (xUnit)
```

Both SDKs use `tsup` for bundling, outputting CJS + ESM with TypeScript declarations. `@togglerino/react` references `@togglerino/sdk` via local file path for development.

### Docker

```bash
docker compose up                          # Start PostgreSQL + togglerino locally
```

Note: Docker Compose maps host port **8090** → container port 8080.

Multi-stage Dockerfile: `node:20-alpine` (frontend build) → `golang:1.25-alpine` (Go build, `CGO_ENABLED=0`) → `alpine:3.19` (runtime).

### Environment Variables

- `PORT` — HTTP port (default: 8080)
- `DATABASE_URL` — PostgreSQL connection string (default: `postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable`)
- `CORS_ORIGINS` — Comma-separated allowed origins (default: `*`)
- `LOG_FORMAT` — Log format: `json` or `text` (default: `json`)

## Architecture

### Go Backend (`cmd/togglerino/`, `internal/`)

Single entry point in `cmd/togglerino/main.go` wires up all dependencies, runs migrations, loads flags into cache, and starts the HTTP server. Uses stdlib `net/http` with `http.NewServeMux` for routing. Graceful shutdown on SIGINT/SIGTERM (10s timeout), closes SSE hub and DB pool.

Key internal packages:

| Package | Responsibility |
|---------|---------------|
| `auth` | Session middleware (`SessionAuth`), SDK key middleware (`SDKAuth`), role middleware (`RequireRole`), bcrypt password hashing, context-based user extraction |
| `config` | Env-var config loading |
| `evaluation` | Flag evaluation engine (consistent hashing via SHA-256 for rollouts, 15 condition operators) + in-memory cache (`RWMutex`-protected map keyed by `projectKey:envKey`) |
| `handler` | HTTP handlers split into management API (session-authed) and client API (SDK-key-authed) |
| `logging` | Configures `log/slog` (JSON/text), provides HTTP request logging middleware (method, path, status, duration_ms) |
| `model` | Domain types: Flag (types: `boolean`, `string`, `number`, `json`), FlagEnvironmentConfig, Variant, TargetingRule, Condition, EvaluationContext, User (roles: `admin`, `member`) |
| `ratelimit` | Fixed-window per-IP rate limiter, applied to auth endpoints (10 req/60s) |
| `store` | PostgreSQL repositories using pgx/v5, database pool creation, migration runner |
| `stream` | SSE pub/sub hub — broadcasts flag changes to subscribed SDK clients |

### Frontend (`web/`)

React 19 + TypeScript + Vite. Uses React Router v7 for routing and TanStack Query for server state. Built output in `web/dist/` is embedded in the Go binary via `go:embed`. Vite dev server proxies `/api` requests to `http://localhost:8090`.

**Styling**: Tailwind CSS v4 (via `@tailwindcss/vite` plugin) + shadcn/ui (New York style, neutral base color). Dark-only theme with CSS custom properties defined in `web/src/index.css`. Uses `cn()` utility from `web/src/lib/utils.ts` (`clsx` + `tailwind-merge`). Path alias `@/` maps to `web/src/`. Accent color: amber `#d4956a`. Fonts: `Sora` sans-serif, `Fira Code` monospace.

**UI components** (`web/src/components/ui/`): shadcn/ui components — alert, badge, button, card, dialog, input, label, select, switch, table, tabs, textarea. Built on Radix UI primitives + `class-variance-authority`. Add new components via `npx shadcn@latest add <component>`.

**API client**: `web/src/api/client.ts` — thin `fetch` wrapper at `/api/v1`, sends `credentials: include` for session cookies.

**Routes**:
- `/projects` — project list
- `/projects/:key` — project detail
- `/projects/:key/flags/:flag` — flag detail
- `/projects/:key/environments` — environment list
- `/projects/:key/environments/:env/sdk-keys` — SDK keys
- `/projects/:key/audit-log` — audit log
- `/projects/:key/settings` — project settings
- `/settings/team` — team management
- `/invite/:token` — accept invite (public)
- `/reset-password/:token` — password reset (public)

### Client SDKs (`sdks/`)

- `sdks/javascript/` — `@togglerino/sdk`: TypeScript SDK with SSE streaming, built with tsup
- `sdks/react/` — `@togglerino/react`: React context provider + `useFlag` hook
- `sdks/dotnet/` — `Togglerino.Sdk`: .NET 8+ SDK with IObservable events, Polly resilience, built with dotnet

## API Routes

### Public (no auth, some rate-limited)

- `GET /healthz` — health check (`{"status":"ok"}`)
- `GET /api/v1/auth/status` — returns `{"setup_required": true}` when no users exist
- `POST /api/v1/auth/setup` — create first admin user (rate-limited, 409 if users exist)
- `POST /api/v1/auth/login` — session login (rate-limited)
- `POST /api/v1/auth/logout` — delete session cookie
- `POST /api/v1/auth/accept-invite` — create account from invite token (rate-limited)
- `POST /api/v1/auth/reset-password` — reset password with token (rate-limited)

### Session-authed (management UI)

- `GET /api/v1/auth/me` — current user
- **Users (admin-only)**: `GET /api/v1/management/users`, `POST .../invite`, `GET .../invites`, `DELETE .../{id}`, `POST .../{id}/reset-password`
- **Projects**: CRUD on `/api/v1/projects[/{key}]` (delete is admin-only)
- **Environments**: `POST`, `GET` on `/api/v1/projects/{key}/environments`
- **SDK Keys**: `POST`, `GET`, `DELETE` on `/api/v1/projects/{key}/environments/{env}/sdk-keys[/{id}]`
- **Flags**: CRUD on `/api/v1/projects/{key}/flags[/{flag}]`, `PUT .../flags/{flag}/environments/{env}` for per-env config
- **Flags query params**: `?tag=` and `?search=` for filtering
- **Audit log**: `GET /api/v1/projects/{key}/audit-log?limit=50&offset=0`

### SDK-authed (client SDKs)

- `POST /api/v1/evaluate` — evaluate all flags
- `POST /api/v1/evaluate/{flag}` — evaluate single flag
- `GET /api/v1/stream` — SSE stream of flag updates

## Key Patterns

- **Two auth paths**: Session-based (cookies, `session_id`, HttpOnly, SameSite=Lax, 7-day MaxAge) for management UI; SDK-key-based (header) for client SDKs
- **RBAC**: Two roles (`admin`, `member`). `RequireRole` middleware enforces admin-only access on user management and project deletion
- **Invite & password reset**: Both use the `invites` table. Invite tokens expire in 7 days, reset tokens in 24 hours. Tokens are atomically claimed via conditional UPDATE (TOCTOU-safe)
- **Initial setup**: First-run flow creates the initial admin user. Frontend `AuthRouter` detects `setup_required` and shows `SetupPage`
- **Flag types**: `boolean`, `string`, `number`, `json`
- **Flag evaluation flow**: Check archived → check disabled → evaluate targeting rules in order (first match wins) → apply percentage rollout via consistent hashing (SHA-256 of `flagKey+userID` → mod 100) → fall back to default variant
- **Condition operators**: `equals`, `not_equals`, `contains`, `not_contains`, `starts_with`, `ends_with`, `greater_than`, `less_than`, `gte`, `lte`, `in`, `not_in`, `exists`, `not_exists`, `matches` (regex)
- **Default environments**: Project creation auto-creates `development`, `staging`, `production`
- **Cache invalidation**: In-memory cache loaded at startup via `cache.LoadAll()`, refreshed on flag mutations through handlers
- **SSE streaming**: Hub notifies connected SDK clients on flag changes, keyed by `projectKey:envKey`. Initial `: connected` keepalive, events use `event: flag_update`. Buffered channels (size 16), events dropped for slow subscribers
- **Audit log**: Best-effort recording (errors logged, don't fail requests). Stores full JSON snapshots of old/new entity state. Events: flag/project create/update/delete, flag config update
- **Rate limiting**: Fixed-window per-IP on auth endpoints (10 req/60s, returns 429 + `Retry-After`)
- **CORS**: When `CORS_ORIGINS=*`, all origins allowed. Specific list → exact-match only, 403 for unlisted origins on OPTIONS. Sends `Allow-Credentials: true`
- **Dependency injection**: Stores and handlers created in `main.go` and passed via constructors
- **SQL migrations**: Embedded via `migrations/` package using `embed.FS`, run on startup. Tracks versions in `schema_migrations` table, each migration runs in a transaction. Files: `NNN_name.up.sql` / `NNN_name.down.sql` (only `.up.sql` applied automatically)
- **SPA fallback**: Go file server tries static file first, falls back to `index.html` for React Router

## Database

PostgreSQL 16. Core tables: `users`, `sessions`, `projects`, `environments`, `flags`, `flag_environment_configs`, `sdk_keys`, `audit_log`, `invites`. Migrations in `migrations/` (currently: `001_initial_schema`, `002_invites`).

## Testing

Go tests require a running PostgreSQL instance. Tests use `testPool()` helper that reads `DATABASE_URL` (falls back to default local connection). Run `docker compose up` to get a local database before running `go test ./...`.

## CI/CD

- **`.github/workflows/ci.yml`**: Four jobs — `test-go` (postgres service container, builds frontend for `go:embed`, runs `go test`), `test-sdks` (JS + React SDK tests), `lint-frontend` (`npm run lint`), `build` (gates on all three, full binary build). Runs on push/PR to `main`.
- **`.github/workflows/release.yml`**: Uses `release-please-action@v4` (`release-type: simple`). On release, builds and pushes Docker image to **ghcr.io** with semver + `latest` tags. Changelog auto-generated from Conventional Commits.

## Other

- `docs/plans/` — design documents and implementation plans (planning artifacts, not API docs)
