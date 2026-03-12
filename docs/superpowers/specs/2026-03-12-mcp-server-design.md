# MCP Server for Flag Management

**Issue:** #124
**Date:** 2026-03-12
**Status:** Draft

## Overview

Build an MCP (Model Context Protocol) server that exposes Togglerino flag management operations as tools, enabling AI assistants (Claude Code, Cursor, etc.) to manage feature flags programmatically during development sessions. Accompanied by a Personal Access Token (PAT) system for authentication.

## Motivation

MCP is becoming a standard protocol for AI tool integration. Developers should be able to manage feature flags directly from their AI-powered development environments without switching to the dashboard. The primary use case is an AI assistant creating and configuring feature flags during a coding session where those flags are being used in code.

## Architecture

Two components:

1. **Personal Access Tokens (backend)** — New auth mechanism for programmatic API access
2. **MCP Server (TypeScript)** — Local stdio process that proxies to the Togglerino management API

```
Developer's machine                          Remote server
┌─────────────────────┐                     ┌──────────────────┐
│ AI Assistant         │                     │ Togglerino       │
│   ↕ stdio            │                     │                  │
│ @togglerino/mcp     │ ── HTTP/Bearer ──→  │ Management API   │
│   (npx)             │    (PAT auth)        │   (existing)     │
└─────────────────────┘                     └──────────────────┘
```

The MCP server is a thin HTTP client. It does not import Togglerino internals — it communicates exclusively via the public management API.

## Part 1: Personal Access Tokens

### Database

New migration: `personal_access_tokens` table.

| Column | Type | Notes |
|--------|------|-------|
| `id` | UUID | PK, default `gen_random_uuid()` |
| `user_id` | UUID | FK → `users(id)` ON DELETE CASCADE |
| `name` | TEXT NOT NULL | User-provided label, e.g. "Claude Code" |
| `token_hash` | TEXT NOT NULL | SHA-256 hex digest of the full token |
| `token_prefix` | VARCHAR(12) NOT NULL | First 8 chars of token for display (e.g. `pat_abcd`) |
| `expires_at` | TIMESTAMPTZ | Nullable — null means no expiry |
| `last_used_at` | TIMESTAMPTZ | Nullable — updated on each authenticated request |
| `created_at` | TIMESTAMPTZ NOT NULL | Default `now()` |

Index: `UNIQUE(token_hash)` for fast lookup.

### Token Format

`pat_<40 hex chars>` — 20 bytes of `crypto/rand`, hex-encoded, prefixed with `pat_`.

Token is displayed once at creation and never stored or retrievable in plaintext. Stored as SHA-256 hash (not bcrypt — tokens are high-entropy random values, not user-chosen passwords; SHA-256 allows O(1) lookup per request).

### API Endpoints

Session-authenticated. Available to all users (each user manages their own tokens).

#### `POST /api/v1/auth/tokens`

Create a new PAT.

**Request:**
```json
{
  "name": "Claude Code",
  "expires_at": "2026-06-12T00:00:00Z"
}
```

`expires_at` is optional. Omitting it creates a token that does not expire.

**Response (201):**
```json
{
  "id": "uuid",
  "name": "Claude Code",
  "token": "pat_a1b2c3d4e5f6...",
  "token_prefix": "pat_a1b2",
  "expires_at": "2026-06-12T00:00:00Z",
  "created_at": "2026-03-12T10:00:00Z"
}
```

The `token` field is only present in the creation response.

#### `GET /api/v1/auth/tokens`

List current user's tokens.

**Response (200):**
```json
[
  {
    "id": "uuid",
    "name": "Claude Code",
    "token_prefix": "pat_a1b2",
    "expires_at": "2026-06-12T00:00:00Z",
    "last_used_at": "2026-03-12T14:30:00Z",
    "created_at": "2026-03-12T10:00:00Z"
  }
]
```

#### `DELETE /api/v1/auth/tokens/{id}`

Revoke (delete) a token. Only the token's owner can delete it (admins cannot delete other users' tokens).

**Response:** 204 No Content.

### Authentication Middleware

New `PATAuth` middleware, usable alongside `SessionAuth`.

Flow:
1. Check `Authorization: Bearer pat_...` header
2. SHA-256 hash the token
3. Look up `token_hash` in `personal_access_tokens`
4. Check `expires_at` (if set, must be in the future)
5. Update `last_used_at`
6. Load user from `user_id`
7. Store user in request context (same as `SessionAuth` does)

From this point, the existing RBAC pipeline applies identically: `RequireOrgPermission`, `RequireProjectPermission`, role resolver, environment locks — all work because they read the user from context.

Management API routes accept **either** session auth (cookie) **or** PAT auth (Bearer header). Implementation: a combined middleware that tries PAT first (if `Authorization` header present), falls back to session cookie.

### Dashboard UI

New "API Tokens" section on the existing account page (`/account`):

- **Token list**: table showing name, prefix, last used, expires, created, with a revoke button per row
- **Create token**: form with name (required) and expiry date (optional). On submit, shows the full token once in a copyable display with a warning that it won't be shown again

No new routes — this is a section within the existing account page.

## Part 2: MCP Server

### Location & Package

- Directory: `mcp/` in the repo root
- Package name: `@togglerino/mcp`
- Published to npm, runnable via `npx @togglerino/mcp`

### Tech Stack

- TypeScript
- `@modelcontextprotocol/sdk` — MCP stdio transport and tool registration
- `tsup` — bundling (CJS + ESM, consistent with existing SDKs)
- `vitest` — testing (consistent with existing SDKs)

### Configuration

Environment variables:

| Variable | Required | Description |
|----------|----------|-------------|
| `TOGGLERINO_URL` | Yes | Base URL of the Togglerino instance |
| `TOGGLERINO_API_KEY` | Yes | Personal Access Token |
| `TOGGLERINO_PROJECT` | No | Default project key |

### MCP Client Configuration Example

```json
{
  "mcpServers": {
    "togglerino": {
      "command": "npx",
      "args": ["@togglerino/mcp"],
      "env": {
        "TOGGLERINO_URL": "https://flags.mycompany.com",
        "TOGGLERINO_API_KEY": "pat_a1b2c3d4e5f6...",
        "TOGGLERINO_PROJECT": "my-project"
      }
    }
  }
}
```

### Project Structure

```
mcp/
├── package.json
├── tsconfig.json
├── tsup.config.ts
├── vitest.config.ts
├── src/
│   ├── index.ts          # Entry point — creates MCP server, registers tools
│   ├── client.ts         # HTTP client wrapping fetch with Bearer auth
│   └── tools/
│       ├── projects.ts   # list_projects
│       ├── flags.ts      # list_flags, get_flag, create_flag, update_flag, toggle_flag, update_flag_config
│       ├── environments.ts # list_environments
│       └── segments.ts   # list_segments, get_segment
└── tests/
    ├── client.test.ts
    └── tools/
        ├── projects.test.ts
        ├── flags.test.ts
        ├── environments.test.ts
        └── segments.test.ts
```

### Tools

All tools that accept `projectKey` use `TOGGLERINO_PROJECT` as the default when the parameter is omitted. If neither is available, the tool returns an error directing the AI to specify a project or call `list_projects` first.

#### `list_projects`

**Parameters:** none
**API:** `GET /api/v1/projects`
**Returns:** Array of projects with key, name, description.

#### `list_flags`

**Parameters:** `projectKey?`, `search?`, `tag?`
**API:** `GET /api/v1/projects/{key}/flags?search=&tag=`
**Returns:** Array of flags with key, name, type, valueType, tags, enabled status per environment.

#### `get_flag`

**Parameters:** `projectKey?`, `flagKey`
**API:** `GET /api/v1/projects/{key}/flags/{flag}`
**Returns:** Full flag details including variants, per-environment configs, targeting rules.

#### `create_flag`

**Parameters:** `projectKey?`, `name`, `key`, `type` (release/experiment/operational/kill-switch/permission), `valueType` (boolean/string/number/json), `variants`
**API:** `POST /api/v1/projects/{key}/flags`
**Returns:** Created flag.

#### `update_flag`

**Parameters:** `projectKey?`, `flagKey`, `name?`, `description?`, `tags?`
**API:** `PUT /api/v1/projects/{key}/flags/{flag}`
**Returns:** Updated flag.

#### `toggle_flag`

**Parameters:** `projectKey?`, `flagKey`, `environmentKey`, `enabled`
**API:** `PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}`
**Returns:** Updated environment config.

#### `update_flag_config`

**Parameters:** `projectKey?`, `flagKey`, `environmentKey`, `defaultVariant?`, `rules?`, `rolloutPercentage?`
**API:** `PUT /api/v1/projects/{key}/flags/{flag}/environments/{env}`
**Returns:** Updated environment config.

#### `list_environments`

**Parameters:** `projectKey?`
**API:** `GET /api/v1/projects/{key}/environments`
**Returns:** Array of environments with key and name.

#### `list_segments`

**Parameters:** `projectKey?`
**API:** `GET /api/v1/projects/{key}/segments`
**Returns:** Array of segments with key, name, description.

#### `get_segment`

**Parameters:** `projectKey?`, `segmentKey`
**API:** `GET /api/v1/projects/{key}/segments/{segmentKey}`
**Returns:** Full segment details including conditions.

### Error Handling

API errors are passed through as structured MCP tool errors. The AI sees the Togglerino error message (e.g., "flag key already exists", "permission denied", "environment is locked") and can react accordingly. HTTP status codes are mapped:

- 400 → tool error with validation message
- 401 → tool error: "Authentication failed — check your API key"
- 403 → tool error: "Permission denied" with the server's message
- 404 → tool error: "Not found" with context
- 409 → tool error: conflict message from server
- 429 → tool error: "Rate limited — retry after {n} seconds"
- 5xx → tool error: "Server error — try again later"

## Part 3: Testing

### Backend (Go)

- **Store tests**: `internal/store/` — CRUD operations for `personal_access_tokens` (create, list by user, lookup by hash, delete, expiry filtering)
- **Middleware tests**: `internal/auth/` — PAT auth middleware (valid token, expired token, deleted token, malformed token, missing header falls through to session auth)
- **Handler tests**: `internal/handler/` — token CRUD endpoints (create returns token once, list omits token, delete by owner only)

### MCP Server (TypeScript)

- **Unit tests**: Mock the HTTP client, verify each tool calls the correct endpoint with correct parameters and transforms responses
- **Client tests**: Verify auth header, URL construction, error mapping
- No integration tests against a live instance in v1

### CI

- New `test-mcp` job in `ci.yml` — `cd mcp && npm install && npm test` (same pattern as `test-sdks`)
- Existing `test-go` job covers PAT backend code
- `build` job gates on the new `test-mcp` job

## Part 4: Release

Separate `release-please` configuration for the `mcp/` directory:

- Independent version lifecycle from the main Togglerino binary
- Own `package.json` version and changelog
- Release triggers npm publish of `@togglerino/mcp`
- Added as an additional job in `release.yml` or a separate `release-mcp.yml`

## Out of Scope (v1)

- SDK evaluation operations (for client SDKs, not management)
- Webhook management
- User/team management
- OIDC configuration
- Archive flag
- SDK key listing
- Segment create/update
- Audit log access
- Scoped/restricted PATs (tokens inherit full user permissions)
- Default environment config (`TOGGLERINO_DEFAULT_ENVIRONMENT`)
- Integration tests against live Togglerino instance
