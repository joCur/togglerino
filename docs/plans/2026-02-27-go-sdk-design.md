# Go SDK Design

## Overview

Port the JavaScript SDK to Go, providing a Go client SDK for togglerino that can be used in backend services. Lives at `sdks/go/` as a separate Go module (`github.com/joCur/togglerino/sdks/go`).

## Decisions

- **Location**: `sdks/go/` as a separate Go module (consistent with JS SDK under `sdks/javascript/`)
- **Module path**: `github.com/joCur/togglerino/sdks/go`
- **Evaluation mode**: Remote client only (fetches flags from server via HTTP/SSE)
- **Architecture**: Single `Client` struct (mirrors JS SDK's `Togglerino` class)
- **Streaming**: SSE streaming with polling fallback (full parity with JS SDK)
- **API style**: Typed getter methods (`BoolValue`, `StringValue`, `NumberValue`, `JSONValue`)
- **Dependencies**: Zero external dependencies (stdlib only)

## Package Structure

```
sdks/go/
├── go.mod              # module github.com/joCur/togglerino/sdks/go
├── client.go           # Client struct, New(), Close()
├── config.go           # Config struct, defaults
├── flags.go            # BoolValue, StringValue, NumberValue, JSONValue, Detail
├── context.go          # EvaluationContext, UpdateContext()
├── events.go           # Event types, On* callback registration
├── stream.go           # SSE streaming + reconnection with exponential backoff
├── polling.go          # Polling fallback
├── types.go            # Shared types (EvaluationResult, FlagChangeEvent, etc.)
├── client_test.go      # Tests
└── example_test.go     # Testable examples for godoc
```

Single package named `togglerino`.

## Core Types

```go
type Config struct {
    ServerURL       string             // Required: base URL of togglerino server
    SDKKey          string             // Required: SDK authentication key
    Context         *EvaluationContext  // Optional: initial evaluation context
    Streaming       *bool              // Optional: enable SSE (default: true)
    PollingInterval time.Duration      // Optional: polling interval (default: 30s)
    HTTPClient      *http.Client       // Optional: custom HTTP client
    Logger          *slog.Logger       // Optional: structured logger
}

type EvaluationContext struct {
    UserID     string         `json:"user_id"`
    Attributes map[string]any `json:"attributes"`
}

type EvaluationResult struct {
    Value   any    `json:"value"`
    Variant string `json:"variant"`
    Reason  string `json:"reason"`
}

type FlagChangeEvent struct {
    FlagKey  string
    Value    any
    Variant  string
    OldValue any
}

type FlagDeletedEvent struct {
    FlagKey string
}
```

## Client API

```go
// Create and initialize (fetches flags, starts streaming)
client, err := togglerino.New(ctx, togglerino.Config{
    ServerURL: "http://localhost:8080",
    SDKKey:    "sdk_abc123",
    Context:   &togglerino.EvaluationContext{UserID: "user-42"},
})
defer client.Close()

// Typed flag getters (synchronous, from in-memory cache)
enabled := client.BoolValue("feature-x", false)
theme   := client.StringValue("theme", "light")
limit   := client.NumberValue("rate-limit", 100)

var cfg MyConfig
client.JSONValue("service-config", &cfg, defaultCfg)

// Full evaluation detail
detail, ok := client.Detail("feature-x")

// Update context (re-fetches all flags from server)
err := client.UpdateContext(ctx, &togglerino.EvaluationContext{
    UserID:     "user-123",
    Attributes: map[string]any{"plan": "pro"},
})

// Event callbacks (each returns an unsubscribe function)
unsub := client.OnChange(func(e togglerino.FlagChangeEvent) { ... })
unsub()

client.OnReady(func() { ... })
client.OnError(func(err error) { ... })
client.OnDeleted(func(e togglerino.FlagDeletedEvent) { ... })
client.OnReconnecting(func(attempt int, delay time.Duration) { ... })
client.OnReconnected(func() { ... })
client.OnContextChange(func(ctx togglerino.EvaluationContext) { ... })
```

### Key Go-idiomatic differences from JS SDK

- `New()` auto-initializes (fetches flags, starts streaming) — no separate `Initialize()` call
- `New()` and `UpdateContext()` accept `context.Context` for cancellation/timeout
- `JSONValue()` unmarshals into a target (like `json.Unmarshal`) rather than returning `any`
- Typed `On*` callback functions per event type (no generic string-based event names)
- `*slog.Logger` for structured logging
- `*http.Client` injection for custom transports/timeouts

## SSE Streaming & Reconnection

Mirrors JS SDK behavior exactly:

- SSE connection via `net/http` GET to `/api/v1/stream` with `Authorization: Bearer` header
- Custom SSE line parser (Go stdlib has no SSE client): reads body line-by-line, splits on `\n\n`, parses `event:` and `data:` fields
- Handles `flag_update` events (updates cache, emits `OnChange`) and `flag_deleted` events (removes from cache, emits `OnDeleted`)
- Ignores comment lines (`: connected` keepalive)
- Reconnection with exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (capped) — same formula as JS SDK
- Polling fallback starts automatically during reconnection (configurable interval, default 30s)
- Polling stops when SSE reconnects successfully
- SSE runs in a background goroutine; `Close()` cancels via context and waits for cleanup

## Flag Change Detection

- Compares flag values using `encoding/json` marshal for deep equality (matches JS SDK's `JSON.stringify` comparison)
- Only emits `OnChange` when value actually changed
- Does not emit change events during initial `New()` fetch

## Thread Safety

- Flag cache protected by `sync.RWMutex` (read-lock for getters, write-lock for fetch/SSE updates)
- Callbacks invoked synchronously from the goroutine that detects the change
- `Close()` safe to call multiple times (uses `sync.Once`)

## Error Handling

- `New()` returns `(*Client, error)` — fails if initial flag fetch fails
- `UpdateContext()` returns `error` — fails if re-fetch fails
- SSE/polling errors emitted via `OnError` callbacks (don't crash the client)
- Sentinel errors: `ErrNotInitialized`, `ErrClosed`

## Server API Contracts

### POST /api/v1/evaluate

Request:
```json
{"context": {"user_id": "string", "attributes": {}}}
```

Response:
```json
{"flags": {"flag-key": {"value": true, "variant": "on", "reason": "default"}}}
```

Headers: `Authorization: Bearer <sdk-key>`, `Content-Type: application/json`

### GET /api/v1/stream

SSE event format:
```
event: flag_update
data: {"type":"flag_update","flagKey":"my-flag","value":true,"variant":"on"}

event: flag_deleted
data: {"type":"flag_deleted","flagKey":"my-flag"}
```

Headers: `Authorization: Bearer <sdk-key>`, `Accept: text/event-stream`
