---
sidebar_position: 1
title: SDK Overview
---

# SDK Overview

Togglerino provides official SDKs for JavaScript, React, Go, and .NET. All SDKs follow the same pattern:

1. **Initialize** with your server URL, SDK key, and optional evaluation context.
2. **Evaluate flags** using type-safe getters with default fallbacks.
3. **Receive real-time updates** via SSE streaming (with automatic polling fallback).

## SDK Comparison

| Feature | JavaScript | React | Go | .NET |
|---------|-----------|-------|-----|------|
| Package | `@togglerino/sdk` | `@togglerino/react` | `github.com/togglerino/togglerino/sdks/go` | `Togglerino.Sdk` |
| Registry | npm | npm | Go modules | NuGet |
| Streaming (SSE) | Yes | Yes (via JS SDK) | Yes | Yes |
| Polling fallback | Yes | Yes | Yes | Yes |
| Default poll interval | 30s | 30s | 30s | 30s |
| Output format | CJS + ESM | CJS + ESM | Go module | .NET 8+ |

## Common Concepts

### Evaluation Context

Every SDK accepts an evaluation context containing a `user_id` and an arbitrary `attributes` map. This context is sent with every evaluation request so the server can apply targeting rules.

```json
{
  "userId": "user-123",
  "attributes": {
    "plan": "pro",
    "country": "US"
  }
}
```

### Typed Getters

All SDKs provide type-safe flag evaluation methods with default fallbacks:

- **`getBool`** / **`BoolValue`** / **`GetBool`** — returns a boolean flag value
- **`getString`** / **`StringValue`** / **`GetString`** — returns a string flag value
- **`getNumber`** / **`NumberValue`** / **`GetNumber`** — returns a numeric flag value
- **`getJson`** / **`JSONValue`** / **`GetJson`** — returns a deserialized JSON flag value

If the flag is not found or the value does not match the expected type, the default value is returned.

### Streaming

SDKs connect to the Togglerino server via Server-Sent Events (SSE) for real-time flag updates. When a flag changes on the server, all connected clients receive the update immediately.

If the SSE connection fails, SDKs automatically fall back to periodic polling and attempt to reconnect with exponential backoff.

### Events

All SDKs let you subscribe to lifecycle and flag change events:

- **Ready** — initial flags have been fetched
- **Change** — a flag value has changed
- **Deleted** — a flag has been removed
- **Error** — a fetch or connection error occurred
- **Reconnecting** — SSE is attempting to reconnect
- **Reconnected** — SSE connection restored

### Context Updates

You can change the evaluation context at runtime (for example, when a user logs in). The SDK re-fetches all flags with the new context and emits change events for any flags whose values differ.

## Getting an SDK Key

SDK keys are scoped to a specific project and environment. To create one:

1. Open your project in the Togglerino dashboard.
2. Navigate to **Environments** in the sidebar.
3. Select the environment you want to connect to (e.g., `production`).
4. Click **SDK Keys**, then **Create Key**.
5. Copy the generated key value — it starts with `sdk_`.

Use this key when initializing any SDK. Each environment should have its own SDK key.
