---
sidebar_position: 2
title: Configuration
---

# Configuration

Togglerino is configured entirely through environment variables. There are no configuration files to manage.

## Environment Variable Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable` | PostgreSQL connection string |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins. Use `*` for development only. |
| `LOG_FORMAT` | `json` | Log output format: `json` or `text` |
| `SESSION_SECRET` | (auto-generated) | HMAC key for session and OIDC state cookies. Auto-generated if unset. **Set explicitly for persistence across restarts.** |
| `BASE_URL` | (auto-derived from requests) | External base URL for OIDC callbacks (e.g., `https://flags.example.com`). Set when behind a reverse proxy. |
| `OIDC_ISSUER_URL` | — | OIDC provider issuer URL. Overrides database config if set. |
| `OIDC_CLIENT_ID` | — | OIDC client ID. Overrides database config if set. |
| `OIDC_CLIENT_SECRET` | — | OIDC client secret. Overrides database config if set. |
| `OIDC_DEFAULT_ROLE` | `member` | Default role for OIDC-provisioned users: `admin` or `member` |

## Example

```bash
export PORT=8080
export DATABASE_URL="postgres://togglerino:s3cret@db.example.com:5432/togglerino?sslmode=require"
export CORS_ORIGINS="https://flags.example.com"
export LOG_FORMAT=json
export SESSION_SECRET="your-random-secret-at-least-32-characters"
export BASE_URL="https://flags.example.com"

./togglerino
```

Or with Docker:

```yaml
services:
  togglerino:
    image: ghcr.io/joCur/togglerino:latest
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://togglerino:s3cret@db.example.com:5432/togglerino?sslmode=require
      CORS_ORIGINS: "https://flags.example.com"
      SESSION_SECRET: "your-random-secret-at-least-32-characters"
      BASE_URL: "https://flags.example.com"
```

## Notes

### CORS Origins

The default value `*` allows requests from any origin. This is convenient for development but should **not** be used in production. In production, list only the specific origins that need access:

```bash
CORS_ORIGINS="https://flags.example.com,https://admin.example.com"
```

### Session Secret

If `SESSION_SECRET` is not set, Togglerino generates a random secret on each startup. This means:

- All active user sessions are invalidated on every restart.
- OIDC state verification will fail if the server restarts mid-authentication flow.

For production, always set `SESSION_SECRET` to a stable, random string (at least 32 characters).

```bash
# Generate a random secret
openssl rand -hex 32
```

### OIDC Environment Variables

The OIDC variables (`OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`) override any OIDC configuration stored in the database. This is useful for:

- **Infrastructure-as-code deployments** where configuration is managed through environment variables or secrets managers rather than the admin UI.
- **Keeping secrets out of the database** by injecting them at runtime.

If these variables are not set, Togglerino falls back to OIDC configuration stored in the database (manageable through the admin UI under Settings).

### Log Format

Use `json` (the default) for production environments where logs are ingested by a log aggregation system. Use `text` during local development for human-readable output:

```bash
LOG_FORMAT=text ./togglerino
```

### Base URL

`BASE_URL` is required when Togglerino runs behind a reverse proxy. It tells the server its externally-visible URL, which is used to construct OIDC callback URLs. If unset, the base URL is derived from incoming request headers, which may not reflect the correct external address when behind a proxy.

```bash
BASE_URL="https://flags.example.com"
```
