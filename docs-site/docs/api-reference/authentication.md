---
sidebar_position: 1
title: Authentication
---

# Authentication

Togglerino uses two separate authentication methods depending on whether you are accessing the management API (dashboard) or the client/evaluation API (SDKs).

## Session Authentication (Management Dashboard)

The management API uses cookie-based session authentication. When you log in, the server sets an `HttpOnly` cookie named `session_id` with the following properties:

| Property   | Value          |
|------------|----------------|
| Name       | `session_id`   |
| HttpOnly   | `true`         |
| SameSite   | `Lax`          |
| MaxAge     | 7 days         |
| Path       | `/`            |

Include this cookie with all management API requests. Browsers handle this automatically when using `credentials: "include"` with `fetch`.

### Logging In

```
POST /api/v1/auth/login
```

**Request body:**

```json
{
  "email": "admin@example.com",
  "password": "your-password"
}
```

**Response** (200 OK):

The response includes the user object and sets the `session_id` cookie:

```json
{
  "id": "uuid",
  "email": "admin@example.com",
  "role": "admin",
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

**Errors:**

| Status | Description |
|--------|-------------|
| 400    | Invalid request body |
| 401    | Invalid credentials |
| 429    | Rate limit exceeded |

### Logging Out

```
POST /api/v1/auth/logout
```

Deletes the session on the server and clears the `session_id` cookie. No request body required.

**Response** (200 OK):

```json
{
  "status": "logged out"
}
```

### Initial Setup

When no users exist in the system, you must create the first admin account before any other API calls will work.

**Check setup status:**

```
GET /api/v1/auth/status
```

**Response** (200 OK):

```json
{
  "setup_required": true,
  "oidc_enabled": false
}
```

**Create the first admin:**

```
POST /api/v1/auth/setup
```

**Request body:**

```json
{
  "email": "admin@example.com",
  "password": "secure-password"
}
```

**Response** (201 Created): Returns the created user object and sets the `session_id` cookie.

**Errors:**

| Status | Description |
|--------|-------------|
| 400    | Missing email or password |
| 409    | Setup already completed (users exist) |
| 429    | Rate limit exceeded |

## Personal Access Tokens (Programmatic Access)

Personal access tokens (PATs) allow programmatic access to the management API without a browser session. They are used by the [MCP server](../integrations/mcp-server.md) and other automation tools.

### Creating a Token

Create tokens from the **Account** page in the dashboard, or via the API:

```
POST /api/v1/auth/tokens
```

**Request body:**

```json
{
  "name": "my-ci-token",
  "expires_at": "2026-12-31T23:59:59Z"
}
```

- `name` (required): A label for the token (max 100 characters)
- `expires_at` (optional): Expiration timestamp (must be in the future). Omit for a non-expiring token.

**Response** (201 Created):

```json
{
  "id": "uuid",
  "name": "my-ci-token",
  "token_prefix": "pat_a1b2c3d4",
  "expires_at": "2026-12-31T23:59:59Z",
  "created_at": "2026-03-12T00:00:00Z",
  "token": "pat_a1b2c3d4e5f6..."
}
```

The `token` field is shown **only once** at creation time. Store it securely.

### Using a Token

Pass the token in the `Authorization` header:

```
Authorization: Bearer pat_a1b2c3d4e5f6...
```

PATs have the same permissions as the user who created them. They work with all management API endpoints except profile updates (`PUT /api/v1/auth/me`), password changes, and PAT management itself.

### Listing Tokens

```
GET /api/v1/auth/tokens
```

Returns all tokens for the authenticated user (without the secret value).

### Revoking a Token

```
DELETE /api/v1/auth/tokens/{id}
```

Returns 204 No Content on success. Only the token owner can revoke their tokens.

## SDK Key Authentication (Client SDKs)

Client SDKs authenticate using SDK keys, which are scoped to a specific project and environment. Pass the key in the `Authorization` header:

```
Authorization: Bearer <sdk-key>
```

SDK keys are created through the management dashboard:

1. Navigate to your project
2. Go to **Environments**
3. Select an environment
4. Click **Create SDK Key**

The key is shown once at creation time. Store it securely.

All client API endpoints (`/api/v1/evaluate`, `/api/v1/evaluate/{flag}`, `/api/v1/stream`) require this header.

## Rate Limiting

Authentication endpoints are rate-limited to **10 requests per 60 seconds per IP address**. This applies to:

- `POST /api/v1/auth/setup`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/change-password`
- `POST /api/v1/auth/accept-invite`
- `POST /api/v1/auth/reset-password`
- `GET /api/v1/auth/oidc/callback`
- `POST /api/v1/auth/oidc/link`

When the rate limit is exceeded, the server returns:

- **Status:** 429 Too Many Requests
- **Header:** `Retry-After: <seconds>` indicating when you can retry

## Error Format

All API errors return a JSON body with an `error` field:

```json
{
  "error": "invalid credentials"
}
```

Always check the response body for details, not just the HTTP status code.
