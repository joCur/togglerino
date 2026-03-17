---
sidebar_position: 8
title: Client vs. Server SDKs
---

# Client vs. Server SDKs

Togglerino provides two distinct SDK modes. Choosing the right one depends on where your code runs and how many users it serves simultaneously.

## Client-Side SDKs

Client-side SDKs are designed for environments where a single user's context is known up front — browsers, mobile apps, and desktop clients.

**How they work:**

1. The SDK connects to the Togglerino server with the user's evaluation context (user ID, attributes).
2. The server evaluates all flags for that specific user and returns the resolved values.
3. The SDK caches those values locally and optionally subscribes to SSE updates to stay current.

**When to use:**

- Single-page apps (React, Vue, Angular, plain JavaScript)
- Mobile apps
- Desktop clients

**Packages:** `@togglerino/sdk` (`Togglerino` class), `@togglerino/react`, the Go SDK (`togglerino.New`), and the .NET SDK (`TogglerioClient`).

## Server-Side SDKs

Server-side SDKs are designed for backend services that handle requests from many different users. Rather than making a network call for each user, the SDK downloads the full flag definitions once and evaluates them locally per request.

**How they work:**

1. On startup, the SDK fetches all flag definitions (rules, variants, targeting conditions) from the Togglerino server.
2. It subscribes to SSE updates to keep definitions current as flags change.
3. For each incoming request, your code calls `evaluate(context)` with the current user's context. This runs the full targeting and rollout logic in-memory — no network call.

**When to use:**

- Node.js / Go / .NET API servers
- Edge functions
- Any backend that serves requests on behalf of multiple users

**Packages:** `@togglerino/sdk/server` (`TogglerioServer` class), the Go SDK (`togglerino.NewServer`), and the .NET SDK (`TogglerioServer`).

## Evaluation Consistency

Client-side and server-side SDKs produce identical results. Both use the same evaluation algorithm:

1. If the flag is archived, return the default value.
2. If the flag is disabled in the environment, return the default value.
3. Evaluate targeting rules in order — the first matching rule wins.
4. Apply percentage rollouts via consistent hashing (SHA-256 of `flagKey + userId`, mod 100) for stable, sticky assignments.
5. Fall back to the environment's default variant.

A user assigned to the `treatment` variant by the server-side SDK will see the same variant in the client-side SDK, and vice versa, as long as the same user ID and flag configuration are used.

## Performance Considerations

| | Client-side | Server-side |
|---|---|---|
| Network calls | One call per context / user change | One call at startup, then SSE updates |
| Evaluation latency | Synchronous (local cache) | Synchronous (in-memory, zero network overhead) |
| Suitable for | Single user per instance | Many users per instance |
| Flag updates | Via SSE or polling | Via SSE or polling |

The server-side SDK is the right choice whenever your process handles more than one user. A single `evaluate()` call typically completes in microseconds.

## Common Mistakes

**Re-initializing per request (server-side)**

```typescript
// Wrong — creates a new connection on every request
app.get('/api/features', async (req, res) => {
  const server = new TogglerioServer({ ... })
  await server.initialize()           // network call every time!
  const flags = server.evaluate(...)
})
```

```typescript
// Correct — initialize once at startup
const server = new TogglerioServer({ ... })
await server.initialize()

app.get('/api/features', (req, res) => {
  const flags = server.evaluate(...)  // local, zero network overhead
})
```

**Using the client-side SDK on a multi-user server**

The client-side SDK ties its flag cache to a single evaluation context. On a server with concurrent requests, different users would overwrite each other's context, producing incorrect results. Use the server-side SDK instead — `evaluate(context)` is stateless and safe to call concurrently.
