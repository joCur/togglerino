---
sidebar_position: 4
title: Go
---

# Go SDK

The Togglerino Go SDK provides a client for evaluating feature flags with real-time SSE streaming. It uses only the Go standard library (plus `log/slog` for structured logging).

## Installation

```bash
go get github.com/togglerino/togglerino/sdks/go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    togglerino "github.com/togglerino/togglerino/sdks/go"
)

func main() {
    ctx := context.Background()
    client, err := togglerino.New(ctx, togglerino.Config{
        ServerURL: "https://flags.example.com",
        SDKKey:    "sdk_your_key_here",
        Context:   &togglerino.EvaluationContext{UserID: "user-123"},
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    if client.BoolValue("new-checkout-flow", false) {
        fmt.Println("New checkout enabled!")
    }
}
```

`New` fetches the initial flag state and starts background synchronization (SSE or polling). The provided `context.Context` is used only for the initial fetch; a separate background context governs the sync goroutine's lifetime.

## Configuration

The `Config` struct controls client behavior:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `ServerURL` | `string` | Yes | — | Base URL of the Togglerino server |
| `SDKKey` | `string` | Yes | — | SDK authentication key |
| `Context` | `*EvaluationContext` | No | `nil` | Initial evaluation context |
| `Streaming` | `*bool` | No | `true` | Use SSE for real-time updates |
| `PollingInterval` | `time.Duration` | No | `30s` | Polling interval when not streaming |
| `HTTPClient` | `*http.Client` | No | `http.DefaultClient` | Custom HTTP client |
| `Logger` | `*slog.Logger` | No | `slog.Default()` | Custom structured logger |

Since `Streaming` is a `*bool`, use a pointer to disable it:

```go
streaming := false
client, err := togglerino.New(ctx, togglerino.Config{
    ServerURL: "https://flags.example.com",
    SDKKey:    "sdk_your_key_here",
    Streaming: &streaming,
})
```

### Evaluation Context

```go
type EvaluationContext struct {
    UserID     string         `json:"user_id"`
    Attributes map[string]any `json:"attributes,omitempty"`
}
```

The `UserID` is used for targeting rules and percentage rollouts. The `Attributes` map can contain any key-value pairs for targeting.

## Evaluating Flags

All flag getters are thread-safe and read from the local in-memory cache.

### `BoolValue(key, defaultValue)`

```go
enabled := client.BoolValue("dark-mode", false)
```

Returns the boolean value of the flag, or `defaultValue` if the flag is missing or not a boolean.

### `StringValue(key, defaultValue)`

```go
color := client.StringValue("button-color", "blue")
```

Returns the string value of the flag, or `defaultValue` if the flag is missing or not a string.

### `NumberValue(key, defaultValue)`

```go
limit := client.NumberValue("rate-limit", 100.0)
```

Returns the `float64` value of the flag, or `defaultValue` if the flag is missing or not a number.

### `JSONValue(key, target, defaultValue)`

```go
type LayoutConfig struct {
    Columns     int  `json:"columns"`
    ShowSidebar bool `json:"showSidebar"`
}

var layout LayoutConfig
err := client.JSONValue("layout-config", &layout, LayoutConfig{Columns: 2, ShowSidebar: true})
```

Unmarshals the flag value into `target`. If the flag is missing, `defaultValue` is used instead. Returns an error if marshaling or unmarshaling fails.

### `Detail(key)`

```go
result, exists := client.Detail("my-flag")
if exists {
    fmt.Printf("Value: %v, Variant: %s, Reason: %s\n", result.Value, result.Variant, result.Reason)
}
```

Returns the full `EvaluationResult` and a boolean indicating whether the flag exists in the cache.

The `EvaluationResult` struct:

```go
type EvaluationResult struct {
    Value   any    `json:"value"`
    Variant string `json:"variant"`
    Reason  string `json:"reason"`
}
```

## Event Callbacks

Register callbacks for SDK lifecycle events. Each method returns an unsubscribe `func()`.

### `OnReady`

Called when the client has finished initialization.

```go
unsubscribe := client.OnReady(func() {
    fmt.Println("Togglerino is ready")
})
defer unsubscribe()
```

### `OnChange`

Called when a flag value changes.

```go
unsubscribe := client.OnChange(func(event togglerino.FlagChangeEvent) {
    fmt.Printf("Flag %s changed to %v (variant: %s)\n", event.FlagKey, event.Value, event.Variant)
})
```

The `FlagChangeEvent` struct:

```go
type FlagChangeEvent struct {
    FlagKey  string `json:"flagKey"`
    Value    any    `json:"value"`
    Variant  string `json:"variant"`
    OldValue any    // previous value (not serialized)
}
```

### `OnDeleted`

Called when a flag is removed.

```go
unsubscribe := client.OnDeleted(func(event togglerino.FlagDeletedEvent) {
    fmt.Printf("Flag %s was deleted\n", event.FlagKey)
})
```

### `OnError`

Called when a fetch or connection error occurs.

```go
unsubscribe := client.OnError(func(err error) {
    fmt.Printf("Togglerino error: %v\n", err)
})
```

### `OnReconnecting`

Called when the client is attempting to reconnect the SSE stream.

```go
unsubscribe := client.OnReconnecting(func(attempt int, delay time.Duration) {
    fmt.Printf("Reconnecting: attempt %d, delay %s\n", attempt, delay)
})
```

### `OnReconnected`

Called when the SSE connection is restored.

```go
unsubscribe := client.OnReconnected(func() {
    fmt.Println("SSE reconnected")
})
```

### `OnContextChange`

Called after `UpdateContext` completes successfully.

```go
unsubscribe := client.OnContextChange(func(ctx togglerino.EvaluationContext) {
    fmt.Printf("Context updated: user=%s\n", ctx.UserID)
})
```

## Updating Context

Change the evaluation context at runtime with `UpdateContext`. This merges the new context into the existing one (non-empty `UserID` replaces, attributes are merged), re-fetches all flags, and emits a context change event.

```go
err := client.UpdateContext(ctx, &togglerino.EvaluationContext{
    UserID: "user-456",
})
```

To update attributes without changing the user:

```go
err := client.UpdateContext(ctx, &togglerino.EvaluationContext{
    Attributes: map[string]any{"plan": "enterprise"},
})
```

You can also read the current context (returns a copy):

```go
context := client.GetContext()
```

## Cleanup

Call `Close()` to shut down background goroutines, wait for them to finish, and clear all event listeners. It is safe to call multiple times. Use `defer` immediately after creating the client.

```go
client, err := togglerino.New(ctx, cfg)
if err != nil {
    return err
}
defer client.Close()
```

## Full Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "time"

    togglerino "github.com/togglerino/togglerino/sdks/go"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    client, err := togglerino.New(ctx, togglerino.Config{
        ServerURL: "https://flags.example.com",
        SDKKey:    "sdk_your_key_here",
        Context: &togglerino.EvaluationContext{
            UserID:     "user-123",
            Attributes: map[string]any{"plan": "pro"},
        },
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
        os.Exit(1)
    }
    defer client.Close()

    client.OnChange(func(event togglerino.FlagChangeEvent) {
        fmt.Printf("Flag %s changed: %v -> %v\n", event.FlagKey, event.OldValue, event.Value)
    })

    client.OnError(func(err error) {
        fmt.Printf("Error: %v\n", err)
    })

    // Evaluate flags
    if client.BoolValue("new-checkout-flow", false) {
        fmt.Println("New checkout enabled!")
    }

    welcomeMsg := client.StringValue("welcome-message", "Hello!")
    fmt.Println(welcomeMsg)

    // Update context for a different user
    _ = client.UpdateContext(ctx, &togglerino.EvaluationContext{UserID: "user-456"})

    // Wait for interrupt
    <-ctx.Done()
    fmt.Println("Shutting down")
}
```
