---
sidebar_position: 4
title: Upgrading
---

# Upgrading

Togglerino is designed for straightforward upgrades. Database migrations run automatically on startup, so the upgrade process is: get the new version, restart.

## Before Upgrading

Check the [CHANGELOG](https://github.com/joCur/togglerino/blob/main/CHANGELOG.md) for breaking changes before upgrading. While Togglerino avoids breaking changes where possible, major version bumps may require manual action.

**Back up your database** before upgrading, especially for major version changes:

```bash
pg_dump -h localhost -U togglerino togglerino > backup_$(date +%Y%m%d_%H%M%S).sql
```

## Docker

```bash
# Pull the latest image
docker compose pull

# Restart with the new image
docker compose up -d
```

To upgrade to a specific version:

```yaml
# docker-compose.yml
services:
  togglerino:
    image: ghcr.io/joCur/togglerino:1.2.0  # pin to a specific version
```

Then:

```bash
docker compose up -d
```

## Pre-built Binary

1. Download the new release from the [GitHub Releases](https://github.com/joCur/togglerino/releases) page.
2. Stop the running process (e.g., `systemctl stop togglerino` or send `SIGTERM`).
3. Replace the binary.
4. Start the new version.

Togglerino performs a graceful shutdown on `SIGINT` or `SIGTERM`, completing in-flight requests (with a 10-second timeout) and closing SSE connections cleanly.

## Database Migrations

Migrations run automatically on startup. Key details:

- Each migration runs inside a **transaction**, so a failed migration is rolled back cleanly without leaving the database in a partial state.
- Applied migrations are tracked in the `schema_migrations` table. Only new migrations are applied on each startup.
- Migration files follow the naming convention `NNN_name.up.sql`. They are embedded in the binary and require no external files.

You do not need to run migrations manually for standard upgrades.

## Rollback

If an upgrade causes issues:

1. **Stop** the new version of Togglerino.
2. **Restore** your database from the backup taken before the upgrade:
   ```bash
   psql -h localhost -U togglerino togglerino < backup_20260309_120000.sql
   ```
3. **Start** the previous version of the binary (or pull the previous Docker image tag).

:::note
Downgrade migrations (`.down.sql` files) exist in the source tree but are **not** run automatically. Restoring from a database backup is the recommended rollback approach.
:::

## Zero-Downtime Upgrades

For deployments that require zero downtime:

1. Run the new version alongside the old version behind a load balancer.
2. The new instance runs migrations on startup. Since migrations run in transactions and are idempotent (tracked by version), this is safe even while the old instance is still running.
3. Once the new instance is healthy (check `/healthz`), drain connections from the old instance and shut it down.

Ensure all instances share the same `SESSION_SECRET` so user sessions remain valid across the transition.
