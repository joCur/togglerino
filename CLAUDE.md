# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Togglerino

A self-hosted feature flag management platform. Single Go binary serves: management API, client/SDK evaluation API, embedded React dashboard, and SSE streaming for real-time flag updates.

## Build & Run Commands

### Backend (Go)

```bash
go build -o togglerino ./cmd/togglerino   # Build binary
go test ./...                              # Run all tests
go test ./internal/evaluation/...          # Run tests for a single package
```

### Frontend (React dashboard, embedded in Go binary)

```bash
cd web && npm install && npm run build     # Build dashboard (required before go build)
cd web && npm run dev                      # Vite dev server
cd web && npm run lint                     # ESLint
```

### SDKs

```bash
cd sdks/javascript && npm test             # JavaScript SDK tests (vitest)
cd sdks/react && npm test                  # React SDK tests (vitest)
```

### Docker

```bash
docker compose up                          # Start PostgreSQL + togglerino locally
```

### Environment Variables

- `PORT` — HTTP port (default: 8080)
- `DATABASE_URL` — PostgreSQL connection string (default: `postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable`)
- `CORS_ORIGINS` — Comma-separated allowed origins (default: `*`)
- `LOG_FORMAT` — Log format: `json` or `text` (default: `json`)

## Architecture

### Go Backend (`cmd/togglerino/`, `internal/`)

Single entry point in `cmd/togglerino/main.go` wires up all dependencies, runs migrations, loads flags into cache, and starts the HTTP server. Uses stdlib `net/http` with `http.NewServeMux` for routing.

Key internal packages:

| Package | Responsibility |
|---------|---------------|
| `auth` | Session middleware (`SessionAuth`), SDK key middleware (`SDKAuth`), bcrypt password hashing, context-based user extraction |
| `config` | Env-var config loading |
| `evaluation` | Flag evaluation engine (consistent hashing for rollouts, condition matching) + in-memory cache (`RWMutex`-protected map keyed by `projectKey:envKey`) |
| `handler` | HTTP handlers split into management API (session-authed) and client API (SDK-key-authed) |
| `model` | Domain types: Flag, FlagEnvironmentConfig, Variant, TargetingRule, Condition, EvaluationContext |
| `store` | PostgreSQL repositories using pgx/v5, database pool creation, migration runner |
| `stream` | SSE pub/sub hub — broadcasts flag changes to subscribed SDK clients |

### Frontend (`web/`)

React 19 + TypeScript + Vite. Uses React Router v7 for routing and TanStack Query for server state. Built output in `web/dist/` is embedded in the Go binary via `go:embed`.

### Client SDKs (`sdks/`)

- `sdks/javascript/` — `@togglerino/sdk`: TypeScript SDK with SSE streaming, built with tsup
- `sdks/react/` — `@togglerino/react`: React context provider + `useFlag` hook

## Key Patterns

- **Two auth paths**: Session-based (cookies) for management UI, SDK-key-based (header) for client SDKs
- **Flag evaluation flow**: Check archived → check disabled → evaluate targeting rules in order (first match wins) → apply percentage rollout via consistent hashing → fall back to default variant
- **Cache invalidation**: In-memory cache loaded at startup via `cache.LoadAll()`, refreshed on flag mutations through handlers
- **SSE streaming**: Hub notifies connected SDK clients on flag changes, keyed by `projectKey:envKey`
- **Dependency injection**: Stores and handlers created in `main.go` and passed via constructors
- **SQL migrations**: Embedded via `migrations/` package, run on startup before serving traffic
- **SPA fallback**: Go file server tries static file first, falls back to `index.html` for React Router

## Database

PostgreSQL 16. Core tables: `users`, `sessions`, `projects`, `environments`, `flags`, `flag_environment_configs`, `sdk_keys`, `audit_log`. Migrations live in `migrations/`.

## Testing

Go tests require a running PostgreSQL instance. Tests use `testPool()` helper that reads `DATABASE_URL` (falls back to default local connection). Run `docker compose up` to get a local database before running `go test ./...`.
