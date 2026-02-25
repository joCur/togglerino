# Togglerino

Self-hosted feature flag management platform. A single Go binary serves the management API, SDK evaluation API, embedded React dashboard, and SSE streaming for real-time flag updates.

## Features

- **Flag types**: boolean, string, number, JSON
- **Targeting rules**: attribute-based conditions with AND logic
- **Percentage rollouts**: consistent hashing ensures users get stable assignments
- **Multi-environment**: manage flags per environment (dev, staging, production)
- **Real-time updates**: SSE streaming pushes flag changes to connected SDKs instantly
- **Team management**: invite members via shareable links, admin/member roles
- **Audit log**: tracks all flag and project changes with before/after values
- **Client SDKs**: JavaScript and React SDKs with automatic SSE reconnection

## Quick Start

### Docker Compose

```bash
docker compose up
```

This starts PostgreSQL and togglerino. Open **http://localhost:8090** to set up your admin account.

### From Source

Requires Go 1.25+, Node 20+, and a running PostgreSQL instance.

```bash
# Build frontend
cd web && npm install && npm run build && cd ..

# Build and run
go build -o togglerino ./cmd/togglerino
DATABASE_URL="postgres://user:pass@localhost:5432/togglerino?sslmode=disable" ./togglerino
```

Open **http://localhost:8080** and create your admin account.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable` | PostgreSQL connection string |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins |
| `LOG_FORMAT` | `json` | Log format: `json` or `text` |

## SDK Usage

### JavaScript

```bash
npm install @togglerino/sdk
```

```typescript
import { Togglerino } from '@togglerino/sdk'

const client = new Togglerino({
  serverUrl: 'https://flags.example.com',
  sdkKey: 'sdk_your_key_here',
  project: 'my-project',
  environment: 'production',
  context: { userId: 'user-123' },
})

await client.initialize()

if (client.getBool('new-checkout', false)) {
  // show new checkout flow
}

client.on('change', ({ flagKey, value }) => {
  console.log(`${flagKey} changed to ${value}`)
})
```

### React

```bash
npm install @togglerino/react
```

```tsx
import { TogglerioProvider, useFlag } from '@togglerino/react'

function App() {
  return (
    <TogglerioProvider config={{
      serverUrl: 'https://flags.example.com',
      sdkKey: 'sdk_your_key_here',
      project: 'my-project',
      environment: 'production',
    }}>
      <MyComponent />
    </TogglerioProvider>
  )
}

function MyComponent() {
  const showBanner = useFlag('show-banner', false)
  return showBanner ? <Banner /> : null
}
```

## Architecture

```
┌─────────────────────────────────────────────┐
│              Single Go Binary               │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ React    │  │ Mgmt API │  │ SDK API  │  │
│  │ Dashboard│  │ (session)│  │ (sdk-key)│  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
│       │              │              │        │
│       │     ┌────────┴────────┐     │        │
│       │     │  Flag Eval      │     │        │
│       │     │  Engine + Cache │     │        │
│       │     └────────┬────────┘     │        │
│       │              │         ┌────┴─────┐  │
│       │              │         │ SSE Hub  │  │
│       │              │         └──────────┘  │
│  ┌────┴──────────────┴──────────────────┐   │
│  │           PostgreSQL (pgx)           │   │
│  └──────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

- **Management API**: Session-authenticated endpoints for the dashboard (CRUD for projects, flags, environments, SDK keys, users)
- **SDK API**: SDK-key-authenticated endpoints for flag evaluation and SSE streaming
- **Flag evaluation**: archived check → disabled check → targeting rules (first match) → percentage rollout → default variant
- **SSE streaming**: real-time flag change notifications to connected SDK clients

## Deployment

Togglerino is a single binary designed to run behind a reverse proxy (Nginx, Caddy) for TLS termination.

```
Internet → Reverse Proxy (TLS) → Togglerino → PostgreSQL
```

Database migrations run automatically on startup.

## Development

```bash
# Run all Go tests (requires PostgreSQL)
docker compose up -d postgres
go test ./...

# Run JavaScript SDK tests
cd sdks/javascript && npm test

# Run React SDK tests
cd sdks/react && npm test

# Frontend dev server (hot reload)
cd web && npm run dev

# Lint frontend
cd web && npm run lint
```

## License

MIT
