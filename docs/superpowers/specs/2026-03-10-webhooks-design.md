# Webhooks for Flag Change Events

**Issue:** #33
**Date:** 2026-03-10
**Status:** Approved

## Summary

Add webhook support so flag changes and management events trigger HTTP callbacks to external systems. Project-scoped, with fine-grained event type filtering, HMAC-SHA256 signing, async delivery with retries, and a delivery log.

## Decisions

- **Event type filtering:** Fine-grained (e.g., `flag.created`, `flag_config.updated`) — no wildcards
- **Delivery model:** In-process goroutines with database-persisted delivery log; startup recovery for recent failed deliveries
- **Scope:** Full stack — backend API + management UI
- **Signing:** Auto-generated HMAC-SHA256 secret per webhook, displayed once on creation
- **Test button:** Sends synthetic `webhook.test` event with dummy payload
- **Webhook scope:** Project-scoped only (no org-level)
- **Architecture:** Direct integration (Approach B) — dispatcher injected into handlers alongside audit/SSE

## Data Model

### `webhooks` table

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | auto-generated |
| `project_id` | UUID FK → projects | ON DELETE CASCADE |
| `name` | TEXT NOT NULL | human-readable label |
| `url` | TEXT NOT NULL | delivery target URL |
| `secret` | TEXT NOT NULL | HMAC-SHA256 signing key, auto-generated |
| `event_types` | TEXT[] NOT NULL | e.g., `{flag.created, flag.updated}` |
| `enabled` | BOOLEAN DEFAULT true | toggle without deleting |
| `created_at` | TIMESTAMPTZ | |
| `updated_at` | TIMESTAMPTZ | |

### `webhook_deliveries` table

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID PK | |
| `webhook_id` | UUID FK → webhooks | ON DELETE CASCADE |
| `event_type` | TEXT NOT NULL | |
| `payload` | JSONB NOT NULL | full event payload |
| `status_code` | INT | HTTP response code (null if network error) |
| `response_body` | TEXT | truncated first 1KB |
| `error` | TEXT | error message if failed |
| `attempt` | INT NOT NULL DEFAULT 1 | 1, 2, or 3 |
| `success` | BOOLEAN NOT NULL | |
| `duration_ms` | INT | |
| `created_at` | TIMESTAMPTZ | |

Indexes: `webhook_id`, `created_at DESC`, composite `(webhook_id, created_at DESC)`.

### Delivery log retention

A background goroutine runs every 6 hours and deletes deliveries older than 30 days (`DELETE FROM webhook_deliveries WHERE created_at < NOW() - INTERVAL '30 days'`). This follows the pattern of the existing staleness checker (periodic background goroutine in `main.go`).

## Event Types

`flag.created`, `flag.updated`, `flag.deleted`, `flag.archived`, `flag_config.updated`, `segment.created`, `segment.updated`, `segment.deleted`, `environment.created`, `webhook.test`

## Webhook Dispatcher

New `internal/webhook/` package.

### `Event` struct

```go
type Event struct {
    Type      string          // e.g., "flag.created"
    Timestamp time.Time
    ProjectID string
    Actor     *Actor          // user ID + email
    Entity    json.RawMessage // entity snapshot
}
```

### `Dispatcher`

- `NewDispatcher(store *store.WebhookStore, deliveryStore *store.WebhookDeliveryStore)` — injected into handlers
- `Dispatch(ctx context.Context, projectID string, event Event)` — matches enabled webhooks by project + event type, spawns goroutine per match
- Best-effort: errors logged, never blocks the handler

### Delivery logic

1. Marshal payload JSON
2. HMAC-SHA256 signature of raw payload using webhook secret
3. POST to URL with headers: `Content-Type: application/json`, `X-Togglerino-Signature: sha256=<hex>`, `X-Togglerino-Event: <event_type>`, `X-Togglerino-Delivery: <delivery_id>`
4. Timeout: 10s
5. Success: 2xx status
6. Retry: up to 3 total attempts, exponential backoff (5s, 25s, 125s)
7. Each attempt persisted to `webhook_deliveries`

### Startup recovery

On boot, query deliveries where `success = false AND attempt < 3 AND created_at > NOW() - INTERVAL '1 hour'`. Re-enqueue for retry.

## API Endpoints

All session-authed, require `project:settings` permission.

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/projects/{key}/webhooks` | Create (returns secret once) |
| `GET` | `/api/v1/projects/{key}/webhooks` | List (paginated, secret masked) |
| `GET` | `/api/v1/projects/{key}/webhooks/{id}` | Get (secret masked) |
| `PUT` | `/api/v1/projects/{key}/webhooks/{id}` | Update (name, url, event_types, enabled) |
| `DELETE` | `/api/v1/projects/{key}/webhooks/{id}` | Delete |
| `POST` | `/api/v1/projects/{key}/webhooks/{id}/test` | Send `webhook.test` event |
| `GET` | `/api/v1/projects/{key}/webhooks/{id}/deliveries` | Delivery log (paginated) |

### URL validation

Backend validates webhook URLs on create/update:
- Must be a valid URL with `https` scheme (allow `http` only for `localhost`/`127.0.0.1` for development)
- Reject private/internal IP ranges (10.x, 172.16-31.x, 192.168.x, 169.254.x) for SSRF prevention
- Maximum URL length: 2048 characters

### Secret format

`whsec_` prefix + 32 bytes `crypto/rand` hex-encoded.

### Create response (only time secret is visible)

```json
{
  "id": "uuid",
  "name": "Slack notifications",
  "url": "https://example.com/webhook",
  "secret": "whsec_a1b2c3...",
  "event_types": ["flag.created", "flag.updated"],
  "enabled": true,
  "created_at": "...",
  "updated_at": "..."
}
```

## Handler Integration

`Dispatcher` injected into `FlagHandler`, `SegmentHandler`, `EnvironmentHandler`, `WebhookHandler`.

**For `FlagHandler` and `SegmentHandler`** (which already have audit/hub/cache), the dispatch call sits alongside existing side effects:

```go
h.audit.Record(ctx, auditEntry)          // existing
h.cache.Refresh(ctx, h.pool, pKey, eKey) // existing
h.hub.Broadcast(pKey, eKey, sseEvent)    // existing
h.webhooks.Dispatch(ctx, project.ID, webhookEvent) // new
```

**For `EnvironmentHandler`** — currently has only `EnvironmentStore` and `ProjectStore` dependencies. Adding webhook dispatch requires also adding the `Dispatcher` dependency to its constructor. No audit/hub/cache changes needed for environment creation; only the webhook dispatch is added.

**Handlers that dispatch:**
- `FlagHandler` — flag CRUD, archive, config update
- `SegmentHandler` — segment CRUD
- `EnvironmentHandler` — environment creation (dispatcher added as new dependency)
- `WebhookHandler` — test event

## Management UI

Webhooks live under the project settings page (`/projects/:key/settings`) as a **"Webhooks" tab**, requiring `project:settings` permission.

### Webhook list (within settings tab)
- Table: Name, URL (truncated), Event Types (badges), Enabled (switch), last delivery status
- "Create Webhook" button

### Create/Edit dialog
- Fields: Name, URL (validated), Event Types (multi-select checkboxes), Enabled (switch)
- On create: dialog showing secret with copy button + "won't be shown again" warning

### Webhook detail page
- Header: name, URL, enabled toggle, edit/delete buttons
- Event types as badges
- "Send Test" button with toast result
- Delivery log table: timestamp, event type, status code, success/fail badge, duration, expandable row for payload + response

## Migration

Next available migration number at time of writing: `025`. Verify at implementation time.

## Documentation Updates

Update CLAUDE.md and docs-site for: new API routes, new env vars (if any), handler/package table, and key patterns section.

## Future Considerations (out of scope)

- Secret rotation endpoint (`POST .../webhooks/{id}/rotate-secret`)
- Org-level webhooks
- Additional event types (`project.updated`, `flag.staleness_changed`)
- Dedicated `webhooks:manage` permission (currently reuses `project:settings`)
