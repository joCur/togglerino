# Togglerino Design Document

**Date:** 2026-02-25
**Status:** Approved

## Vision

Togglerino is a self-hosted feature flag management platform. It centralizes feature flags for all parts of an application — web, desktop, mobile, backend — with a single service, a management dashboard, and client SDKs.

## Target Audience

Start simple for solo developers and small teams, architect for growth toward mid-size and enterprise teams over time.

## Tech Stack

- **Backend:** Go (single monolithic binary)
- **Dashboard:** React (Vite + TypeScript), embedded in the Go binary via `go:embed`
- **Database:** PostgreSQL
- **Deployment:** Self-hosted via Docker image or standalone binary

## Architecture

### Monolithic Go Service

A single Go binary serves all concerns:

- Management API (CRUD for projects, flags, environments, users)
- Client/SDK evaluation API (flag evaluation, SSE streaming)
- React dashboard (embedded static files)
- Session management and authentication

Internally, the code is structured with clear package boundaries so that components can be extracted into separate services later if scaling demands it.

### Project Structure

```
togglerino/
├── cmd/
│   └── togglerino/          # main.go entry point
├── internal/
│   ├── auth/                 # Session management, password hashing, middleware
│   ├── config/               # App configuration (env vars, config file)
│   ├── evaluation/           # Flag evaluation engine
│   ├── model/                # Domain types
│   ├── handler/              # HTTP handlers (management + client API)
│   ├── store/                # PostgreSQL repository layer + migrations
│   ├── stream/               # SSE hub — manages connections, broadcasts changes
│   └── middleware/           # Logging, CORS, rate limiting, auth guards
├── migrations/               # SQL migration files
├── web/                      # React dashboard (Vite project)
│   ├── src/
│   └── dist/                 # Built output, embedded via go:embed
├── Dockerfile
├── docker-compose.yml        # togglerino + PostgreSQL
└── go.mod
```

## Data Model

### Hierarchy

```
Organization (implicit, single-tenant for self-hosted)
  └── Project ("web-app", "mobile-app", "api")
        └── Environment ("development", "staging", "production")
              └── Flag State (enabled, rules, rollout %, variants)
```

### Flag Structure

- **key**: Unique identifier within a project (e.g., "dark-mode")
- **name**: Human-readable display name
- **description**: What this flag controls
- **type**: boolean | string | number | json
- **default_value**: Returned when the flag is disabled or no rules match
- **tags**: Organizational labels for filtering

### Flag Environment Config (per environment)

- **enabled**: Master kill switch — if false, always returns default_value
- **default_variant**: Returned when no targeting rules match
- **targeting_rules**: Ordered list of rules, first match wins
- **variants**: Named value options (e.g., "on"→true, "off"→false)

### Targeting Rules

Each rule has:
- **conditions**: Attribute-based conditions (all must match)
- **variant**: The variant to return if conditions match
- **percentage_rollout** (optional): Apply to only N% of matching users

## API Design

### Management API

```
POST   /api/v1/projects                    # Create project
GET    /api/v1/projects                    # List projects
GET    /api/v1/projects/:key               # Get project
PUT    /api/v1/projects/:key               # Update project
DELETE /api/v1/projects/:key               # Delete project

POST   /api/v1/projects/:key/environments  # Create environment
GET    /api/v1/projects/:key/environments  # List environments

POST   /api/v1/projects/:key/flags         # Create flag
GET    /api/v1/projects/:key/flags         # List flags (with filtering/search)
GET    /api/v1/projects/:key/flags/:flag   # Get flag (all env configs)
PUT    /api/v1/projects/:key/flags/:flag   # Update flag metadata
DELETE /api/v1/projects/:key/flags/:flag   # Delete flag

PUT    /api/v1/projects/:key/flags/:flag/environments/:env
                                            # Update flag config for environment

GET    /api/v1/projects/:key/audit-log     # Audit log of changes
```

### Client/SDK API

```
POST   /api/v1/evaluate/:project/:env      # Evaluate all flags for a context
POST   /api/v1/evaluate/:project/:env/:flag # Evaluate single flag
GET    /api/v1/stream/:project/:env         # SSE stream of flag changes
```

### Authentication

**Management API / Dashboard:**
- User accounts with email + bcrypt-hashed passwords
- Server-side sessions stored in PostgreSQL, HTTP-only secure cookies
- First user created becomes initial admin (setup wizard on first run)
- Two roles: admin (full access) and member (manage flags, no user/project management)

**Client/SDK API:**
- SDK keys scoped to a specific project + environment (read-only)
- Generated from the dashboard, revocable at any time

### Evaluation Request/Response

**Request body:**
```json
{
  "context": {
    "user_id": "user-123",
    "attributes": {
      "plan": "pro",
      "country": "DE",
      "created_at": "2024-01-15"
    }
  }
}
```

**Response:**
```json
{
  "flags": {
    "dark-mode": { "value": true, "variant": "on", "reason": "rule_match" },
    "new-checkout": { "value": false, "variant": "off", "reason": "disabled" }
  }
}
```

## Evaluation Engine

### Algorithm

```
evaluate(flag, environment_config, context):
  1. If flag is archived → return default_value
  2. If environment_config.enabled == false → return default_value
  3. For each rule in targeting_rules (ordered):
     a. Evaluate rule conditions against context.attributes
     b. If all conditions match:
        - If rule has percentage_rollout:
            hash = consistent_hash(flag.key + context.user_id)
            if hash <= rollout_percentage → return rule.variant
            else → continue to next rule
        - Else → return rule.variant
  4. No rules matched → return default_variant
```

### Supported Operators

- `equals`, `not_equals` — exact match
- `contains`, `not_contains` — substring/list membership
- `starts_with`, `ends_with` — string prefix/suffix
- `greater_than`, `less_than`, `gte`, `lte` — numeric/date comparison
- `in`, `not_in` — value in a list
- `exists`, `not_exists` — attribute presence check
- `matches` — regex match

### Consistent Hashing for Rollouts

- `hash(flag_key + user_id) % 100` → deterministic 0-99 bucket
- Same user always gets the same result for the same flag
- Different flags distribute users differently

### Caching

- All flag configs loaded into memory at startup
- On flag change: affected project/environment config reloaded from Postgres into cache
- Evaluation reads only from memory — never hits the DB per request

## Real-Time Updates (SSE)

- Clients connect to `/api/v1/stream/:project/:env` with their SDK key
- Server maintains an in-memory hub of active connections per project/environment
- When a flag changes, the hub broadcasts the updated flag value to all connected clients
- Clients that don't support SSE fall back to periodic polling of the evaluate endpoint

## JS/TS SDK

### Usage

```typescript
import { Togglerino } from '@togglerino/sdk';

const client = new Togglerino({
  serverUrl: 'https://flags.mycompany.com',
  sdkKey: 'sdk-env-abc123',
  context: {
    userId: 'user-123',
    attributes: { plan: 'pro', country: 'DE' }
  },
  streaming: true,
  pollingInterval: 30_000,
});

await client.initialize();

client.getBool('dark-mode');          // true
client.getString('checkout-variant'); // "variant-a"
client.getNumber('max-uploads');      // 10
client.getJson('banner-config');      // { text: "...", color: "..." }

client.on('change', (key, newValue, oldValue) => { ... });

client.updateContext({ attributes: { plan: 'enterprise' } });

client.close();
```

### React Integration (`@togglerino/react`)

```typescript
const darkMode = useFlag('dark-mode', false);
```

### SDK Responsibilities

- Fetch all flag values on init (single POST to evaluate endpoint)
- Maintain SSE connection for real-time updates with auto-reconnect
- Fall back to polling if SSE fails or `streaming: false`
- Local cache of evaluated values — typed getters are synchronous reads
- Works in browser and Node.js (isomorphic)

## Dashboard

### Screens

1. **Login / Setup** — First-run wizard creates initial admin account
2. **Projects list** — Overview of all projects with flag counts
3. **Project detail** — Flags list with search, filter by tag, filter by environment state
4. **Flag detail** — Per-environment config with:
   - Enable/disable toggle
   - Visual targeting rule builder
   - Percentage rollout slider
   - Variant definitions
   - Activity log
5. **Environments management** — Create/rename/delete per project
6. **Team management** — Invite users, assign roles
7. **SDK keys** — Generate and revoke per environment
8. **Audit log** — Global chronological log of all changes

### UX Principles

- Changes take effect immediately (no publish step) — kill switch is the safety net
- Confirmation dialog for production environment changes
- Flag changes broadcast to connected SDKs within seconds

## Audit Logging

Every mutation is recorded:
- Who made the change
- What changed (entity type, entity ID)
- When (timestamp)
- Old value and new value

Stored in PostgreSQL, queryable from dashboard and API.

## Future Considerations (not in MVP)

- OIDC / OAuth2 provider support for SSO
- User segments (reusable groups for targeting)
- Scheduled flag changes (enable at a specific time)
- Webhooks for flag change notifications
- Additional SDKs (Go, Swift, Kotlin, Python)
- Flag lifecycle management (stale flag detection, archival)
- Cloud-hosted offering
