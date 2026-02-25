# Production Hardening Design

**Date**: 2026-02-25
**Goal**: Make togglerino ready for small production use with a real team, deployed via Docker Compose behind a reverse proxy.

## Context

Togglerino's core platform works: flag CRUD, evaluation engine, SSE streaming, JS/React SDKs, and dashboard. What's missing is operational hardening for production reliability and multi-user team use.

## 1. Graceful Shutdown

Add SIGTERM/SIGINT handling to `cmd/togglerino/main.go`:
- Call `http.Server.Shutdown()` with 10s timeout to drain in-flight requests
- Close the SSE hub (disconnect streaming clients cleanly)
- Close the database pool
- Log shutdown progress

## 2. User Invite & Management

Admin-only user management with invite links (no SMTP dependency).

**Backend endpoints:**
- `POST /api/management/users/invite` — admin creates invite, returns a one-time token/link
- `POST /api/auth/accept-invite` — new user accepts invite with token, sets password
- `GET /api/management/users` — list all users
- `DELETE /api/management/users/{id}` — remove a user

**Invite flow:**
1. Admin enters email + role in dashboard
2. System generates invite token (stored in DB with expiry)
3. Admin copies invite link, shares via Slack/etc.
4. New user visits link, sets their password
5. Token is consumed, user is active

**Role enforcement:**
- Wire up existing `RequireRole` middleware on admin-only routes (user management, project delete)

**Frontend:**
- Replace Team page placeholder with member list + invite form
- Add invite link copy UI
- Add accept-invite page at `/invite/{token}`

**Database:**
- New `invites` table: id, email, role, token, expires_at, accepted_at, invited_by

## 3. CORS Configuration

- Add `CORS_ORIGINS` env var (comma-separated allowed origins)
- Default: `*` (dev compatibility)
- Production: set to actual domain(s)
- Applied in existing CORS middleware in main.go

## 4. Structured Logging

Use Go stdlib `log/slog` (no new dependencies).

- **Request logging middleware**: method, path, status code, duration
- **Error logging**: replace silent failures in handlers with slog calls
- **Startup logging**: config values (redacted secrets), migration status, server ready
- **Format**: JSON by default (production-friendly), configurable text mode via `LOG_FORMAT` env var

## 5. Password Reset (Admin-Initiated)

Reuses invite token infrastructure:
- `POST /api/management/users/{id}/reset-password` — admin-only, generates reset token/link
- User visits link, sets new password
- Same accept-invite page with "reset" mode
- Token has expiry (e.g. 24h)

## 6. SDK SSE Reconnection

In `@togglerino/sdk` JavaScript SDK:
- On SSE disconnect, retry with exponential backoff: 1s, 2s, 4s, 8s... capped at 30s
- Keep polling as fallback while retrying SSE
- Emit `reconnecting` and `reconnected` events
- Unlimited retry attempts by default (configurable)

## 7. Rate Limiting on Auth Endpoints

In-memory rate limiter (fine for single-instance Docker Compose):
- Apply to: login, accept-invite, password reset endpoints
- Algorithm: sliding window per IP
- Default: 10 attempts per minute per IP
- Returns 429 Too Many Requests with Retry-After header

## Non-Goals

- Email delivery (SMTP)
- Multi-instance SSE (Redis pub/sub)
- Kubernetes-specific probes
- OpenAPI spec
- Frontend tests
- Metrics/Prometheus endpoints
