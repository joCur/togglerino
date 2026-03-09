---
sidebar_position: 3
title: Client / Evaluation API
---

# Client / Evaluation API

The client API is used by SDKs to evaluate feature flags and receive real-time updates. All endpoints require SDK key authentication via the `Authorization` header.

## Authentication

Every request must include an SDK key in the `Authorization` header:

```
Authorization: Bearer <sdk-key>
```

The SDK key determines which project and environment the request is scoped to. SDK keys are created in the management dashboard under **Project > Environments > SDK Keys**.

If the SDK key is missing or invalid, the server returns `401 Unauthorized`.

---

## Evaluate All Flags

```
POST /api/v1/evaluate
```

Evaluates all flags for the SDK key's project and environment, optionally using the provided evaluation context for targeting rules and percentage rollouts.

### Request

```json
{
  "context": {
    "user_id": "user-123",
    "attributes": {
      "plan": "pro",
      "country": "US",
      "beta_tester": true
    }
  }
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `context` | No | Evaluation context. If omitted, flags are evaluated without user targeting. |
| `context.user_id` | No | Unique user identifier. Used for consistent percentage rollouts (hashed with the flag key) and personal overrides. |
| `context.attributes` | No | Key-value pairs used by targeting rule conditions. Values can be strings, numbers, booleans, or arrays. |

### Response (200)

```json
{
  "flags": {
    "new-checkout-flow": {
      "value": true,
      "variant": "enabled",
      "reason": "rule_match"
    },
    "max-items": {
      "value": 50,
      "variant": "pro-limit",
      "reason": "rule_match"
    },
    "dark-mode": {
      "value": false,
      "variant": "disabled",
      "reason": "default"
    },
    "maintenance-banner": {
      "value": "We are performing scheduled maintenance",
      "variant": "active",
      "reason": "disabled"
    }
  }
}
```

Each flag in the response includes:

| Field | Type | Description |
|-------|------|-------------|
| `value` | `boolean`, `string`, `number`, or `object` | The evaluated flag value |
| `variant` | `string` | The name of the matched variant |
| `reason` | `string` | Why this value was returned (see [Evaluation Reasons](#evaluation-reasons)) |

### Personal Overrides

If a `user_id` is provided and matches a personal override set by a developer, the override value is returned with `"reason": "override"` and `"variant": "override"`. Overrides bypass disabled flags but respect archived flags.

---

## Evaluate Single Flag

```
POST /api/v1/evaluate/{flagKey}
```

Evaluates a single flag by key. Uses the same context format as the bulk evaluation endpoint.

### Request

```json
{
  "context": {
    "user_id": "user-123",
    "attributes": {
      "plan": "pro"
    }
  }
}
```

### Response (200)

```json
{
  "value": true,
  "variant": "enabled",
  "reason": "rule_match"
}
```

### Error: Flag Not Found (404)

If the flag key does not exist for the SDK key's project, the server returns 404 and records the key as an "unknown flag" for visibility in the management dashboard.

```json
{
  "error": "flag not found"
}
```

---

## SSE Stream

```
GET /api/v1/stream
```

Opens a Server-Sent Events (SSE) connection that receives real-time flag updates for the SDK key's project and environment.

### Headers

```
Authorization: Bearer <sdk-key>
```

### Response

The response uses the `text/event-stream` content type with the following headers:

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

### Event Format

**Initial keepalive** (sent immediately on connection):

```
: connected

```

**Flag update events** (sent when a flag config changes):

```
event: flag_update
data: {"type":"flag_update","flag_key":"new-checkout-flow","value":true,"variant":"enabled"}

```

### Implementation Notes

- The connection stays open indefinitely. The server sends periodic keepalive comments to prevent proxy timeouts.
- Events are buffered per subscriber (buffer size: 16). If a client falls behind, events may be dropped.
- Clients should implement automatic reconnection with exponential backoff.
- On reconnect, perform a full evaluation (`POST /api/v1/evaluate`) to ensure you have the latest state, then resume streaming for subsequent updates.
- The stream is scoped to the SDK key's project and environment. Each unique project+environment combination requires its own connection.

### Example: Connecting with curl

```bash
curl -N -H "Authorization: Bearer your-sdk-key" \
  http://localhost:8080/api/v1/stream
```

---

## Evaluation Reasons

Every flag evaluation includes a `reason` field explaining why a particular value was returned:

| Reason | Description |
|--------|-------------|
| `default` | No targeting rules matched; the default variant was returned |
| `rule_match` | A targeting rule's conditions matched the evaluation context |
| `disabled` | The flag is disabled in this environment |
| `archived` | The flag has been archived |
| `override` | A personal override is active for this user |

---

## Health Check

```
GET /healthz
```

**No authentication required.** Returns a simple health check response.

**Response** (200):

```json
{
  "status": "ok"
}
```

Use this endpoint for load balancer health checks and monitoring.
