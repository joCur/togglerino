# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Togglerino

A self-hosted feature flag management platform. Single Go binary serves: management API, client/SDK evaluation API, embedded React dashboard, and SSE streaming for real-time flag updates.

Go module: `github.com/togglerino/togglerino` (Go 1.25.0, stdlib `net/http` + `log/slog`, no external HTTP or logging frameworks). Key deps: `pgx/v5`, `golang.org/x/crypto`, `coreos/go-oidc/v3`, `golang.org/x/oauth2`.

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
cd sdks/go && go test ./...                # Go SDK tests
```

JS/React SDKs use `tsup` for bundling, outputting CJS + ESM with TypeScript declarations. `@togglerino/react` references `@togglerino/sdk` via local file path for development.

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
- `SESSION_SECRET` — HMAC key for OIDC state/link cookies (auto-generated if unset; set for persistence across restarts)
- `BASE_URL` — External base URL for OIDC callback (auto-derived from requests if unset)
- `OIDC_ISSUER_URL` — OIDC issuer URL (env var override for DB config)
- `OIDC_CLIENT_ID` — OIDC client ID (env var override for DB config)
- `OIDC_CLIENT_SECRET` — OIDC client secret (env var override for DB config)
- `OIDC_DEFAULT_ROLE` — Default role for OIDC-provisioned users: `admin` or `member` (default: `member`)

## Development Workflow

Always use `dev.sh` to start development. It runs PostgreSQL + Go backend in Docker and isolates ports per worktree:

```bash
./dev.sh                                   # Start PostgreSQL + Go backend in Docker
cd web && npm install && npm run dev       # Vite dev server with HMR (in another terminal)
./dev.sh --down                            # Stop and remove containers
```

`dev.sh` prints the assigned ports on startup. Vite proxies `/api` requests to the backend automatically. The frontend has full hot reload — edit React components, styles, or API calls and see changes instantly.

To pick up backend API/schema changes: `./dev.sh` again (rebuilds the Go binary + runs migrations).

`dev.sh` derives a deterministic port offset (0-99) from the directory name via `cksum`, so each worktree gets unique ports. It sets `COMPOSE_PROJECT_NAME` to isolate Docker volumes/containers and configures `DATABASE_URL`, `BACKEND_PORT`, and `VITE_API_URL` automatically. Uses `docker-compose.dev.yml` (the main `docker-compose.yml` is unchanged for self-hosting).

For fullstack development (running Go outside Docker), use `./dev.sh --go` to start only PostgreSQL, then run Go manually with `LOG_FORMAT=text go run ./cmd/togglerino`.

### Debugging

**Frontend → backend request tracing**: The Go backend logs every request with method, path, status, and duration_ms (`internal/logging`). Run with `LOG_FORMAT=text` for human-readable output. Match a failing frontend request by its path in the backend logs.

**Common frontend debugging scenarios**:
- **401 on API calls**: Session cookie expired or missing. Check browser DevTools → Application → Cookies for `session_id`. Re-login via the UI
- **CORS errors**: Ensure `CORS_ORIGINS=*` is set (default in Docker Compose)
- **Proxy not working**: `dev.sh` configures `VITE_API_URL` automatically. If running without `dev.sh`, ensure the Vite proxy target matches the backend port
- **Stale UI after backend changes**: TanStack Query caches aggressively. Hard-refresh or clear the query cache in React DevTools

**Backend debugging tips**:
- Use `LOG_FORMAT=text` for readable logs during development
- All API errors return JSON `{"error": "message"}` — check the response body, not just the status code
- Database issues: connect directly with `psql postgres://togglerino:togglerino@localhost:5432/togglerino` to inspect state

## Architecture

### Go Backend (`cmd/togglerino/`, `internal/`)

Single entry point in `cmd/togglerino/main.go` wires up all dependencies, runs migrations, loads flags into cache, and starts the HTTP server. Uses stdlib `net/http` with `http.NewServeMux` for routing. Graceful shutdown on SIGINT/SIGTERM (10s timeout), closes SSE hub and DB pool.

Key internal packages:

| Package | Responsibility |
|---------|---------------|
| `auth` | Session middleware (`SessionAuth`), SDK key middleware (`SDKAuth`), permission middleware (`RequireOrgPermission`, `RequireProjectPermission`), role resolver (`BuildRoleResolver`), bcrypt password hashing, context-based user extraction |
| `config` | Env-var config loading |
| `evaluation` | Flag evaluation engine (consistent hashing via SHA-256 for rollouts, 16 condition operators including `segment_match`) + in-memory cache (`RWMutex`-protected map keyed by `projectKey:envKey` for flags, `projectKey` for segments) |
| `handler` | HTTP handlers split into management API (session-authed) and client API (SDK-key-authed) |
| `logging` | Configures `log/slog` (JSON/text), provides HTTP request logging middleware (method, path, status, duration_ms) |
| `model` | Domain types: Flag (value types: `boolean`, `string`, `number`, `json`; flag types: `release`, `experiment`, `operational`, `kill-switch`, `permission`; lifecycle: `active`, `potentially_stale`, `stale`, `archived`), FlagEnvironmentConfig, Variant, TargetingRule, Condition, EvaluationContext, Segment, User (roles: `admin`, `member`, optional `display_name`), Permission, ProjectRole (`admin`, `editor`, `viewer`), ProjectMember, ProjectSettings, UnknownFlag, ContextAttribute |
| `ratelimit` | Fixed-window per-IP rate limiter, applied to auth endpoints (10 req/60s) |
| `staleness` | Automated flag staleness checker — periodic background goroutine (1hr interval) that transitions flags through lifecycle states based on per-project flag lifetime settings |
| `oidc` | OIDC provider wrapper (`coreos/go-oidc/v3`), HMAC-signed state/link cookies, secure random generation |
| `store` | PostgreSQL repositories using pgx/v5, database pool creation, migration runner |
| `stream` | SSE pub/sub hub — broadcasts flag changes to subscribed SDK clients |

### Frontend (`web/`)

React 19 + TypeScript + Vite. Uses React Router v7 for routing and TanStack Query for server state. Built output in `web/dist/` is embedded in the Go binary via `go:embed`. Vite dev server proxies `/api` requests to `http://localhost:8090`.

**Styling**: Tailwind CSS v4 (via `@tailwindcss/vite` plugin) + shadcn/ui (New York style, neutral base color). Dark-only theme with CSS custom properties defined in `web/src/index.css`. Uses `cn()` utility from `web/src/lib/utils.ts` (`clsx` + `tailwind-merge`). Path alias `@/` maps to `web/src/`. Accent color: amber `#d4956a`. Fonts: `Sora` sans-serif, `Fira Code` monospace.

**UI components** (`web/src/components/ui/`): shadcn/ui components — alert, badge, button, card, collapsible, command, dialog, dropdown-menu, input, label, popover, select, sheet, switch, table, tabs, textarea. Built on Radix UI primitives + `class-variance-authority`. Add new components via `npx shadcn@latest add <component>`.

**API client**: `web/src/api/client.ts` — thin `fetch` wrapper at `/api/v1`, sends `credentials: include` for session cookies.

**Routes**:
- `/projects` — project list
- `/projects/:key` — project detail
- `/projects/:key/lifecycle` — flag lifecycle board
- `/projects/:key/flags/:flag` — flag detail
- `/projects/:key/environments` — environment list
- `/projects/:key/environments/:env/sdk-keys` — SDK keys
- `/projects/:key/audit-log` — audit log
- `/projects/:key/segments` — segment management
- `/projects/:key/settings` — project settings
- `/account` — user account page (display name, password change, SSO identities)
- `/preferences` — user preferences (theme selector)
- `/settings` — admin-only settings (OIDC config)
- `/settings/team` — team management
- `/invite/:token` — accept invite (public)
- `/reset-password/:token` — password reset (public)
- `/link-account` — OIDC account linking (password confirmation, public)

### Client SDKs (`sdks/`)

- `sdks/javascript/` — `@togglerino/sdk`: TypeScript SDK with SSE streaming, built with tsup
- `sdks/react/` — `@togglerino/react`: React context provider + `useFlag` hook
- `sdks/dotnet/` — `Togglerino.Sdk`: .NET 8+ SDK with IObservable events, Polly resilience, built with dotnet
- `sdks/go/` — Go SDK with SSE streaming and polling, pure stdlib

## API Routes

### Public (no auth, some rate-limited)

- `GET /healthz` — health check (`{"status":"ok"}`)
- `GET /api/v1/auth/status` — returns `{"setup_required": true}` when no users exist
- `POST /api/v1/auth/setup` — create first admin user (rate-limited, 409 if users exist)
- `POST /api/v1/auth/login` — session login (rate-limited)
- `POST /api/v1/auth/logout` — delete session cookie
- `POST /api/v1/auth/accept-invite` — create account from invite token (rate-limited)
- `POST /api/v1/auth/reset-password` — reset password with token (rate-limited)
- `GET /api/v1/auth/oidc/authorize` — redirects to OIDC identity provider
- `GET /api/v1/auth/oidc/callback` — OIDC callback (exchanges code, creates/links session)
- `POST /api/v1/auth/oidc/link` — link OIDC identity to existing account (rate-limited, protected by signed `oidc_pending` cookie)

### Session-authed (management UI)

- `GET /api/v1/auth/me` — current user
- `PUT /api/v1/auth/me` — update profile (display name)
- `POST /api/v1/auth/change-password` — change password (rate-limited)
- `GET /api/v1/auth/oidc/identities` — list current user's linked OIDC identities
- **OIDC config (admin-only)**: `GET /PUT /api/v1/auth/oidc/config`, `DELETE /api/v1/auth/oidc/config`
- **Users (admin-only)**: `GET /api/v1/management/users`, `POST .../invite`, `GET .../invites`, `DELETE .../{id}`, `POST .../{id}/reset-password`, `GET /PUT .../{id}/projects` (project assignments)
- **Org settings (admin-only)**: `GET /PUT /api/v1/settings/base-project-role`
- **Projects**: CRUD on `/api/v1/projects[/{key}]` (create/delete require org permissions, list filtered by access)
- **Project members**: `GET /POST /api/v1/projects/{key}/members`, `PUT /DELETE .../members/{userId}` (require `project:settings`)
- **Environments**: `POST`, `GET` on `/api/v1/projects/{key}/environments`
- **SDK Keys**: `POST`, `GET`, `DELETE` on `/api/v1/projects/{key}/environments/{env}/sdk-keys[/{id}]`
- **Flags**: CRUD on `/api/v1/projects/{key}/flags[/{flag}]`, `PUT .../flags/{flag}/environments/{env}` for per-env config, `PUT .../flags/{flag}/archive` for archiving, `PUT .../flags/{flag}/staleness` for staleness override
- **Flags query params**: `?tag=` and `?search=` for filtering
- **Unknown flags**: `GET /api/v1/projects/{key}/unknown-flags`, `DELETE .../unknown-flags/{id}` (dismiss)
- **Project settings**: `GET /PUT /api/v1/projects/{key}/settings/flags` — per-project flag lifetime configuration
- **Segments**: CRUD on `/api/v1/projects/{key}/segments[/{segmentKey}]`, `GET .../segments/{segmentKey}/usage` for referencing flags
- **Context attributes**: `GET /api/v1/projects/{key}/context-attributes` — autocomplete for rule builder
- **Audit log**: `GET /api/v1/projects/{key}/audit-log?limit=50&offset=0`

### SDK-authed (client SDKs)

- `POST /api/v1/evaluate` — evaluate all flags
- `POST /api/v1/evaluate/{flag}` — evaluate single flag
- `GET /api/v1/stream` — SSE stream of flag updates

## Key Patterns

- **Two auth paths**: Session-based (cookies, `session_id`, HttpOnly, SameSite=Lax, 7-day MaxAge) for management UI; SDK-key-based (header) for client SDKs
- **RBAC**: Two global roles (`admin`, `member`) for org-level permissions + three project roles (`admin`, `editor`, `viewer`) for project-level permissions. Org-wide "base project role" setting (default: `editor`) determines default project access for members; `none` requires explicit project membership. Per-project role overrides via `project_members` table. `RequireOrgPermission` middleware for org-level checks, `RequireProjectPermission` middleware for project-level checks (extracts project key from URL, resolves effective role via `BuildRoleResolver`). Global admins bypass all project permission checks. Org permissions: `org:users:manage`, `org:oidc:manage`, `org:projects:create`, `org:projects:delete`. Project permissions: `flags:read/write`, `environments:read/write`, `sdk_keys:manage`, `segments:write`, `templates:manage`, `project:settings`
- **Invite & password reset**: Both use the `invites` table. Invite tokens expire in 7 days, reset tokens in 24 hours. Tokens are atomically claimed via conditional UPDATE (TOCTOU-safe)
- **Initial setup**: First-run flow creates the initial admin user. Frontend `AuthRouter` detects `setup_required` and shows `SetupPage`
- **Flag value types**: `boolean`, `string`, `number`, `json`
- **Flag types** (purpose/category): `release`, `experiment`, `operational`, `kill-switch`, `permission` — used to determine expected lifetime and staleness thresholds
- **Flag lifecycle**: `active` → `potentially_stale` → `stale` → `archived`. Staleness checker runs hourly, comparing flag age to per-project lifetime settings (configurable per flag type, defaults: release/experiment 40 days, operational 7 days, kill-switch/permission permanent)
- **Flag evaluation flow**: Check archived → check disabled → evaluate targeting rules in order (first match wins) → apply percentage rollout via consistent hashing (SHA-256 of `flagKey+userID` → mod 100) → fall back to default variant
- **Condition operators**: `equals`, `not_equals`, `contains`, `not_contains`, `starts_with`, `ends_with`, `greater_than`, `less_than`, `gte`, `lte`, `in`, `not_in`, `exists`, `not_exists`, `matches` (regex), `segment_match` (references a reusable segment by key)
- **Reusable segments**: Project-scoped, named groups of targeting conditions shared across flags. A targeting rule condition with `operator: "segment_match"` and `value: "<segment-key>"` evaluates the segment's conditions. Segments cannot reference other segments (enforced at write time). Delete blocked if segment is referenced by flags (409)
- **Default environments**: Project creation auto-creates `development`, `staging`, `production`
- **Cache invalidation**: In-memory cache loaded at startup via `cache.LoadAll()`, refreshed on flag mutations through handlers
- **SSE streaming**: Hub notifies connected SDK clients on flag changes, keyed by `projectKey:envKey`. Initial `: connected` keepalive, events use `event: flag_update`. Buffered channels (size 16), events dropped for slow subscribers
- **Unknown flag tracking**: SDK evaluations for non-existent flag keys are recorded with request counts and first/last seen timestamps, surfaced in the management UI for cleanup
- **Context attribute autocomplete**: Attributes seen in SDK evaluation contexts are tracked per project and surfaced in the rule builder for autocomplete
- **Audit log**: Best-effort recording (errors logged, don't fail requests). Stores full JSON snapshots of old/new entity state. Events: flag/project create/update/delete, flag config update, lifecycle status changes
- **Rate limiting**: Fixed-window per-IP on auth endpoints (10 req/60s, returns 429 + `Retry-After`)
- **CORS**: When `CORS_ORIGINS=*`, all origins allowed. Specific list → exact-match only, 403 for unlisted origins on OPTIONS. Sends `Allow-Credentials: true`
- **Dependency injection**: Stores and handlers created in `main.go` and passed via constructors
- **SQL migrations**: Embedded via `migrations/` package using `embed.FS`, run on startup. Tracks versions in `schema_migrations` table, each migration runs in a transaction. Files: `NNN_name.up.sql` / `NNN_name.down.sql` (only `.up.sql` applied automatically)
- **OIDC SSO**: Single OIDC provider (configurable via admin UI or env vars). Authorization Code Flow with HMAC-signed state/nonce cookies. Three callback outcomes: existing identity → session, email match → password-confirmed account linking, new user → auto-provisioned with configurable default role. Provider config stored in `oidc_providers` table, identity links in `oidc_identities`. `sync.RWMutex`-protected hot-reloadable provider in `OIDCHandler`
- **SPA fallback**: Go file server tries static file first, falls back to `index.html` for React Router

## Database

PostgreSQL 16. Core tables: `users`, `sessions`, `projects`, `environments`, `flags`, `flag_environment_configs`, `sdk_keys`, `audit_log`, `invites`, `context_attributes`, `unknown_flags`, `project_settings`, `segments`, `oidc_providers`, `oidc_identities`, `org_settings`, `project_members`. Migrations in `migrations/` (currently: `001_initial_schema` through `016_rbac`).

## Testing

Go tests require a running PostgreSQL instance. Tests use `testPool()` helper that reads `DATABASE_URL` (falls back to default local connection). Run `docker compose up` to get a local database before running `go test ./...`.

## CI/CD

- **`.github/workflows/ci.yml`**: Six jobs — `test-go` (postgres service container, builds frontend for `go:embed`, runs `go test`), `test-sdks` (JS + React SDK tests), `test-dotnet-sdk` (.NET SDK tests), `test-go-sdk` (Go SDK tests), `lint-frontend` (`npm run lint`), `build` (gates on all five, full binary build). Runs on push/PR to `main`.
- **`.github/workflows/release.yml`**: Uses `release-please-action@v4` (`release-type: simple`). On release, builds and pushes Docker image to **ghcr.io** with semver + `latest` tags. Changelog auto-generated from Conventional Commits.

## Documentation Site

User-facing docs live in `docs-site/` (Docusaurus 3). Built and deployed to GitHub Pages on release.

```bash
cd docs-site && npm install && npm run build   # Build docs site
cd docs-site && npm start                      # Local dev server with hot reload
```

**Docs maintenance rule**: If you change API endpoints, env vars, UI flows, SDK interfaces, or flag evaluation behavior, update the relevant docs page in `docs-site/docs/`. Key mappings:
- Env vars (`internal/config/`) → `docs-site/docs/self-hosting/configuration.md`
- API routes (`internal/handler/`) → `docs-site/docs/api-reference/`
- SDK changes (`sdks/`) → `docs-site/docs/sdks/`
- Flag evaluation (`internal/evaluation/`) → `docs-site/docs/core-concepts/`
- Dashboard UI (`web/src/`) → `docs-site/docs/dashboard/`

## Other

- `docs/plans/` — design documents and implementation plans (planning artifacts, not API docs)
