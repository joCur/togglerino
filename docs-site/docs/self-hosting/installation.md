---
sidebar_position: 1
title: Installation
---

# Installation

Togglerino is distributed as a single binary that serves the management UI, API, and SDK evaluation endpoints. Choose the installation method that best fits your environment.

:::info
Database migrations run automatically on startup for all installation methods. No manual migration step is required.
:::

## Docker (Recommended)

The easiest way to run Togglerino is with Docker Compose. This starts both PostgreSQL and Togglerino with a single command.

**1. Create a `docker-compose.yml` file:**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: togglerino
      POSTGRES_PASSWORD: togglerino
      POSTGRES_DB: togglerino
    volumes:
      - pgdata:/var/lib/postgresql/data

  togglerino:
    image: ghcr.io/togglerino/togglerino:latest
    ports:
      - "8090:8080"
    environment:
      DATABASE_URL: postgres://togglerino:togglerino@postgres:5432/togglerino?sslmode=disable
      PORT: "8080"
      CORS_ORIGINS: "*"
      LOG_FORMAT: json
    depends_on:
      - postgres

volumes:
  pgdata:
```

**2. Start the services:**

```bash
docker compose up -d
```

**3. Open your browser:**

Navigate to [http://localhost:8090](http://localhost:8090). You will be prompted to create the initial admin user on first launch.

:::note
The Compose file maps host port **8090** to container port **8080**. You can change `8090` to any available port on your host.
:::

### Pulling the image directly

If you prefer to run the container without Compose, pull the image and provide your own PostgreSQL:

```bash
docker pull ghcr.io/togglerino/togglerino:latest

docker run -d \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@your-postgres-host:5432/togglerino?sslmode=disable" \
  ghcr.io/togglerino/togglerino:latest
```

## Pre-built Binary

Download a pre-built binary from the [GitHub Releases](https://github.com/togglerino/togglerino/releases) page.

**Requirements:**
- An external PostgreSQL instance (16+ recommended)

**Steps:**

```bash
# Download the latest release for your platform
# (check the Releases page for available archives)

# Make the binary executable (Linux/macOS)
chmod +x togglerino

# Run with your PostgreSQL connection string
DATABASE_URL="postgres://user:pass@localhost:5432/togglerino?sslmode=disable" ./togglerino
```

Togglerino listens on port **8080** by default. Set the `PORT` environment variable to change this.

## Build from Source

Build Togglerino from the source code in the GitHub repository.

**Requirements:**
- Go 1.25+
- Node.js 20+
- PostgreSQL 16+ (running and accessible)

**Steps:**

```bash
# Clone the repository
git clone https://github.com/togglerino/togglerino.git
cd togglerino

# 1. Build the frontend (must be done first — the Go binary embeds web/dist/)
cd web && npm install && npm run build && cd ..

# 2. Build the Go binary
go build -o togglerino ./cmd/togglerino

# 3. Run
DATABASE_URL="postgres://user:pass@localhost:5432/togglerino?sslmode=disable" ./togglerino
```

:::warning
The frontend **must** be built before the Go binary. The Go build embeds the `web/dist/` directory via `go:embed`. Building without it will fail.
:::

## Next Steps

- [Configuration](./configuration.md) — environment variables and tuning options
- [Production Deployment](./production.md) — reverse proxy, TLS, and scaling guidance
