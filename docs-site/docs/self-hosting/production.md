---
sidebar_position: 3
title: Production Deployment
---

# Production Deployment

This guide covers the key considerations for running Togglerino in a production environment.

## Architecture Overview

A typical production deployment looks like this:

```
Internet → Reverse Proxy (TLS) → Togglerino (:8080) → PostgreSQL
```

Togglerino serves everything from a single binary: the management dashboard, management API, SDK evaluation API, and SSE streaming for real-time flag updates. Place a reverse proxy in front to handle TLS termination and, optionally, load balancing.

## Reverse Proxy Configuration

### nginx

```nginx
server {
    listen 443 ssl http2;
    server_name flags.example.com;

    ssl_certificate     /etc/ssl/certs/flags.example.com.crt;
    ssl_certificate_key /etc/ssl/private/flags.example.com.key;

    location / {
        proxy_pass http://127.0.0.1:8080;

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Required for SSE streaming
        proxy_buffering off;
        proxy_set_header X-Accel-Buffering no;
        proxy_set_header Connection '';
        proxy_http_version 1.1;
        chunked_transfer_encoding off;

        # Keep SSE connections alive
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }
}
```

### Caddy

Caddy handles TLS automatically and does not buffer responses by default, making it the simplest option:

```
flags.example.com {
    reverse_proxy localhost:8080
}
```

:::warning[SSE Streaming]
Your reverse proxy **must not buffer responses**. Togglerino uses Server-Sent Events (SSE) to push real-time flag updates to connected SDK clients. If the proxy buffers these responses, clients will not receive updates until the buffer fills or the connection closes.

- **nginx**: Set `proxy_buffering off` and `X-Accel-Buffering: no` as shown above.
- **Caddy**: Handles this correctly out of the box.
- **AWS ALB/NLB**: Works with SSE by default.
- **Cloudflare**: Disable response buffering or use Cloudflare Tunnel.
:::

## TLS

Terminate TLS at the reverse proxy. Togglerino itself serves plain HTTP. Set the `BASE_URL` environment variable to your external HTTPS URL so that OIDC callback URLs are generated correctly:

```bash
BASE_URL="https://flags.example.com"
```

## PostgreSQL

- **Version**: PostgreSQL 16+ recommended.
- **Connection pooling**: For high-traffic deployments, consider running [PgBouncer](https://www.pgbouncer.org/) between Togglerino and PostgreSQL.
- **Backups**: Togglerino has no built-in backup mechanism. Set up regular backups using `pg_dump`, WAL archiving, or your cloud provider's managed backup solution. Backups are essential for safe [upgrades](./upgrading.md) and disaster recovery.
- **High availability**: Use PostgreSQL streaming replication or a managed PostgreSQL service (e.g., AWS RDS, Google Cloud SQL, Azure Database for PostgreSQL) for automatic failover.

## Health Checks

Togglerino exposes a health check endpoint:

```
GET /healthz
```

Response: `{"status":"ok"}` with HTTP 200.

Use this endpoint for:
- Load balancer health checks
- Container orchestrator liveness probes (Kubernetes, ECS)
- Uptime monitoring

Example Kubernetes liveness probe:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Session Secret

In production, you **must** set `SESSION_SECRET` explicitly:

```bash
SESSION_SECRET="$(openssl rand -hex 32)"
```

If running multiple instances behind a load balancer, all instances must share the same `SESSION_SECRET`. Without it, sessions created on one instance will not be valid on another, and OIDC authentication flows will break.

## Resource Sizing

Togglerino is a lightweight single binary with an in-memory flag cache. Typical resource requirements are modest:

- **CPU**: 1 vCPU is sufficient for most workloads.
- **Memory**: Base memory usage is low. The primary memory consumer is the in-memory flag cache and active SSE connections.
- **SSE connections**: Each connected SDK client maintains one persistent SSE connection with a buffered channel (size 16). Plan for the number of concurrent SDK clients in your deployment. Events are dropped for slow subscribers to prevent unbounded memory growth.
- **Disk**: Minimal. The binary is self-contained and logs to stdout.

For most teams, a single instance with 1 vCPU and 512 MB of RAM handles thousands of SDK clients comfortably. Scale PostgreSQL resources based on audit log volume and the number of flags/projects.

## Multi-Instance Deployments

Togglerino can run multiple instances behind a load balancer. Requirements:

- All instances must share the same `SESSION_SECRET`.
- All instances must connect to the same PostgreSQL database.
- SSE connections are per-instance. Each SDK client connects to one instance and receives updates from that instance's in-memory hub. Flag changes made on any instance are written to the database, and each instance refreshes its own cache on mutation.

## Next Steps

- [Configuration](./configuration.md) — full environment variable reference
- [Upgrading](./upgrading.md) — how to upgrade to new versions
