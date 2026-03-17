---
sidebar_position: 2
title: JavaScript / TypeScript
---

# JavaScript / TypeScript SDK

The `@togglerino/sdk` package provides a TypeScript-first client for evaluating feature flags with real-time SSE streaming.

## Installation

```bash
npm install @togglerino/sdk
```

The package ships both CommonJS and ESM builds with full TypeScript declarations.

## Quick Start

```typescript
import { Togglerino } from '@togglerino/sdk'

const client = new Togglerino({
  serverUrl: 'https://flags.example.com',
  sdkKey: 'sdk_your_key_here',
  context: {
    userId: 'user-123',
    attributes: { plan: 'pro', country: 'US' },
  },
})

await client.initialize()

const showNewCheckout = client.getBool('new-checkout-flow', false)
const theme = client.getString('app-theme', 'light')
const maxItems = client.getNumber('max-items', 10)
```

## Configuration

The `Togglerino` constructor accepts a `TogglerinoConfig` object:

| Property | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `serverUrl` | `string` | Yes | — | Base URL of the Togglerino server |
| `sdkKey` | `string` | Yes | — | SDK authentication key |
| `context` | `EvaluationContext` | No | `{}` | Initial evaluation context |
| `streaming` | `boolean` | No | `true` | Use SSE for real-time updates |
| `pollingInterval` | `number` | No | `30000` | Polling interval in milliseconds |

### Evaluation Context

```typescript
interface EvaluationContext {
  userId?: string
  attributes?: Record<string, unknown>
}
```

The `userId` maps to `user_id` on the server and is used for targeting rules and percentage rollouts. The `attributes` map can contain any key-value pairs for targeting.

## Evaluating Flags

All flag getters are synchronous and read from the local in-memory cache. You must call `initialize()` before reading flag values.

### `getBool(key, defaultValue?)`

```typescript
const enabled = client.getBool('dark-mode', false)
```

Returns the boolean value of the flag, or `defaultValue` (defaults to `false`) if the flag is missing or not a boolean.

### `getString(key, defaultValue?)`

```typescript
const color = client.getString('button-color', 'blue')
```

Returns the string value of the flag, or `defaultValue` (defaults to `''`) if the flag is missing or not a string.

### `getNumber(key, defaultValue?)`

```typescript
const limit = client.getNumber('rate-limit', 100)
```

Returns the numeric value of the flag, or `defaultValue` (defaults to `0`) if the flag is missing or not a number.

### `getJson<T>(key, defaultValue?)`

```typescript
interface LayoutConfig {
  columns: number
  showSidebar: boolean
}

const layout = client.getJson<LayoutConfig>('layout-config', { columns: 2, showSidebar: true })
```

Returns the flag value cast to `T`, or `defaultValue` if the flag is missing.

### `getDetail(key)`

```typescript
const detail = client.getDetail('my-flag')
// { value: true, variant: "on", reason: "targeting_match" }
```

Returns the full `EvaluationResult` containing `value`, `variant`, and `reason`, or `undefined` if the flag is not found.

## Events

Subscribe to SDK events using `client.on(event, listener)`. Each call returns an unsubscribe function.

### `ready`

Fired after `initialize()` completes and the initial flags have been fetched.

```typescript
const unsubscribe = client.on('ready', () => {
  console.log('Togglerino is ready')
})
```

### `change`

Fired when a flag value changes (via SSE or polling).

```typescript
client.on('change', (event: FlagChangeEvent) => {
  console.log(`${event.flagKey} changed to ${event.value} (variant: ${event.variant})`)
})
```

The `FlagChangeEvent` contains:
- `flagKey: string` — the flag key that changed
- `value: unknown` — the new flag value
- `variant: string` — the new variant name

### `deleted`

Fired when a flag is removed via SSE.

```typescript
client.on('deleted', (event: FlagDeletedEvent) => {
  console.log(`${event.flagKey} was deleted`)
})
```

### `error`

Fired on fetch or SSE connection errors.

```typescript
client.on('error', (error: Error) => {
  console.error('Togglerino error:', error.message)
})
```

### `reconnecting`

Fired when the SDK is scheduling an SSE reconnection attempt.

```typescript
client.on('reconnecting', (event) => {
  console.log(`Reconnecting: attempt ${event.attempt}, delay ${event.delay}ms`)
})
```

### `reconnected`

Fired when SSE successfully reconnects after a disconnection.

```typescript
client.on('reconnected', () => {
  console.log('SSE reconnected')
})
```

### `context_change`

Fired after `updateContext()` completes.

```typescript
client.on('context_change', (context: EvaluationContext) => {
  console.log('Context updated:', context)
})
```

### Unsubscribing

Every `on()` call returns an unsubscribe function:

```typescript
const unsubscribe = client.on('change', handler)

// Later, stop listening:
unsubscribe()
```

## Updating Context

Change the evaluation context at runtime with `updateContext()`. This re-fetches all flags with the new context and emits `change` events for any flags whose values differ.

```typescript
await client.updateContext({ userId: 'user-456' })

// Context is merged with the existing context.
// To update just attributes:
await client.updateContext({
  attributes: { plan: 'enterprise' },
})
```

You can also read the current context:

```typescript
const context = client.getContext()
```

## Cleanup

When you are done with the client, call `close()` to stop SSE streaming or polling and remove all event listeners.

```typescript
client.close()
```

## Server-Side Usage

For Node.js servers handling multiple users, use `TogglerioServer` from `@togglerino/sdk/server`. It fetches flag definitions once and evaluates them locally — no network call per request.

```typescript
import { TogglerioServer } from '@togglerino/sdk/server'

const server = new TogglerioServer({
  serverUrl: 'https://flags.example.com',
  sdkKey: 'sdk_your_key_here',
})
await server.initialize()

// Per-request — pure local evaluation, no network call
app.get('/api/features', async (req, res) => {
  const flags = server.evaluate({ userId: req.userId, attributes: { plan: req.userPlan } })
  res.json({ newCheckout: flags.getBool('new-checkout', false) })
})

// Shutdown
process.on('SIGTERM', () => { server.close() })
```

Key points:
- `evaluate()` is synchronous — it runs the full targeting and rollout logic in-memory with zero network overhead.
- Initialize **once** at application startup and reuse the same `TogglerioServer` instance across all requests. Re-initializing per request defeats the purpose and causes unnecessary network traffic.
- The server subscribes to SSE updates and keeps its flag definitions current automatically.

See [Client vs. Server SDKs](../core-concepts/client-vs-server-sdks) for guidance on when to use each approach.

## Full Example

```typescript
import { Togglerino } from '@togglerino/sdk'

async function main() {
  const client = new Togglerino({
    serverUrl: 'https://flags.example.com',
    sdkKey: 'sdk_your_key_here',
    context: {
      userId: 'user-123',
      attributes: { plan: 'pro' },
    },
  })

  client.on('change', (event) => {
    console.log(`Flag ${event.flagKey} changed to ${event.value}`)
  })

  client.on('error', (error) => {
    console.error('Error:', error.message)
  })

  await client.initialize()

  const showBanner = client.getBool('show-banner', false)
  const welcomeMessage = client.getString('welcome-message', 'Hello!')

  console.log({ showBanner, welcomeMessage })

  // Later, when the user changes:
  await client.updateContext({ userId: 'user-456' })

  // When done:
  client.close()
}

main()
```
