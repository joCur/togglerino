---
sidebar_position: 5
title: .NET
---

# .NET SDK

The `Togglerino.Sdk` package provides a .NET 8+ client for evaluating feature flags with real-time SSE streaming and reactive event handling via `IObservable<T>`.

## Installation

```bash
dotnet add package Togglerino.Sdk
```

## Quick Start

```csharp
using Togglerino.Sdk;

await using var client = new TogglerioClient(new TogglerioOptions
{
    ServerUrl = "https://flags.example.com",
    SdkKey = "sdk_your_key_here",
    Context = new EvaluationContext { UserId = "user-123" },
});

await client.InitializeAsync();

if (client.GetBool("new-checkout-flow"))
{
    Console.WriteLine("New checkout enabled!");
}
```

## Configuration

### `TogglerioOptions`

The `TogglerioOptions` record controls client behavior:

| Property | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `ServerUrl` | `string` | Yes | — | Base URL of the Togglerino server |
| `SdkKey` | `string` | Yes | — | SDK authentication key |
| `Context` | `EvaluationContext?` | No | `null` | Initial evaluation context |
| `Streaming` | `bool` | No | `true` | Use SSE for real-time updates |
| `PollingInterval` | `TimeSpan` | No | `30 seconds` | Polling interval when not streaming |

### Constructor Parameters

The `TogglerioClient` constructor also accepts optional dependencies:

```csharp
var client = new TogglerioClient(
    options,
    logger: loggerFactory.CreateLogger<TogglerioClient>(),  // optional ILogger<TogglerioClient>
    httpClient: myHttpClient                                 // optional HttpClient
);
```

If no `HttpClient` is provided, the client creates and owns one internally. If you supply your own, the client will not dispose it.

### Evaluation Context

```csharp
public record EvaluationContext
{
    public string? UserId { get; init; }
    public Dictionary<string, object?>? Attributes { get; init; }
}
```

The `UserId` is used for targeting rules and percentage rollouts. The `Attributes` dictionary can contain any key-value pairs for targeting.

## Evaluating Flags

All flag getters are synchronous and read from the local in-memory cache. Call `InitializeAsync()` before reading flag values.

### `GetBool(key, defaultValue)`

```csharp
bool enabled = client.GetBool("dark-mode");           // default: false
bool enabled = client.GetBool("dark-mode", true);     // explicit default
```

Returns the boolean value of the flag, or `defaultValue` (defaults to `false`) if the flag is missing or not a boolean.

### `GetString(key, defaultValue)`

```csharp
string color = client.GetString("button-color", "blue");
```

Returns the string value of the flag, or `defaultValue` (defaults to `""`) if the flag is missing or not a string.

### `GetNumber(key, defaultValue)`

```csharp
double limit = client.GetNumber("rate-limit", 100);
```

Returns the `double` value of the flag, or `defaultValue` (defaults to `0`) if the flag is missing or not a number.

### `GetJson<T>(key, defaultValue)`

```csharp
var layout = client.GetJson<LayoutConfig>("layout-config");
```

Deserializes the flag value to `T` using `System.Text.Json`. Returns `defaultValue` (defaults to `default(T)`) if the flag is missing or deserialization fails.

### `GetDetail(key)`

```csharp
EvaluationResult? detail = client.GetDetail("my-flag");
if (detail is not null)
{
    Console.WriteLine($"Value: {detail.Value}, Variant: {detail.Variant}, Reason: {detail.Reason}");
}
```

Returns the full `EvaluationResult` containing:

- `Value` (`JsonElement`) — the raw flag value
- `Variant` (`string`) — the matched variant name
- `Reason` (`string`) — why this value was returned

Returns `null` if the flag is not found.

## Reactive Events

The .NET SDK uses `IObservable<T>` (from `System.Reactive`) for event handling.

### Flag Changes

Subscribe to flag value changes:

```csharp
client.FlagChanges.Subscribe(e =>
{
    Console.WriteLine($"Flag {e.FlagKey} changed to {e.Value} (variant: {e.Variant})");
});
```

The `FlagChangeEvent` record:

```csharp
public record FlagChangeEvent(string FlagKey, JsonElement Value, string Variant);
```

### Flag Deletions

Subscribe to flag removals:

```csharp
client.FlagDeletions.Subscribe(e =>
{
    Console.WriteLine($"Flag {e.FlagKey} was deleted");
});
```

The `FlagDeletedEvent` record:

```csharp
public record FlagDeletedEvent(string FlagKey);
```

### Errors

Subscribe to fetch and connection errors:

```csharp
client.Errors.Subscribe(ex =>
{
    Console.Error.WriteLine($"Togglerino error: {ex.Message}");
});
```

### Unsubscribing

`Subscribe` returns an `IDisposable` that you can dispose to stop receiving events:

```csharp
var subscription = client.FlagChanges.Subscribe(e =>
{
    Console.WriteLine($"{e.FlagKey} = {e.Value}");
});

// Later, stop listening:
subscription.Dispose();
```

## Updating Context

Change the evaluation context at runtime. This re-fetches all flags with the new context and emits change events for any flags whose values differ.

```csharp
await client.UpdateContextAsync(new EvaluationContext
{
    UserId = "user-456",
    Attributes = new Dictionary<string, object?> { ["plan"] = "enterprise" },
});
```

You can also read the current context (returns a copy):

```csharp
EvaluationContext context = client.GetContext();
```

## Disposal

`TogglerioClient` implements both `IAsyncDisposable` and `IDisposable`.

Prefer `await using` for async disposal:

```csharp
await using var client = new TogglerioClient(options);
await client.InitializeAsync();
// ... use client ...
// Automatically disposed at end of scope
```

Synchronous disposal also works:

```csharp
using var client = new TogglerioClient(options);
```

Disposal stops SSE streaming or polling, completes observable sequences, and cleans up resources. If you provided your own `HttpClient`, it will not be disposed.

## Server-Side Usage

For ASP.NET Core applications handling multiple users, use `TogglerioServer` instead of `TogglerioClient`. It fetches flag definitions once and evaluates them locally — no network call per request.

```csharp
var server = new TogglerioServer(new TogglerioServerOptions {
    ServerUrl = "https://flags.example.com",
    SdkKey = "sdk_your_key_here",
});
await server.InitializeAsync();

// Per-request — pure local evaluation, no network call
app.MapGet("/api/features", (HttpContext ctx) => {
    var flags = server.Evaluate(new EvaluationContext { UserId = ctx.User.Identity?.Name });
    return Results.Ok(new { NewCheckout = flags.GetBool("new-checkout", false) });
});
```

Register `TogglerioServer` as a singleton in your DI container so it is initialized once and shared across all requests:

```csharp
builder.Services.AddSingleton<TogglerioServer>(sp =>
{
    var server = new TogglerioServer(new TogglerioServerOptions
    {
        ServerUrl = "https://flags.example.com",
        SdkKey = "sdk_your_key_here",
    });
    server.InitializeAsync().GetAwaiter().GetResult();
    return server;
});
```

Key points:
- `Evaluate()` is synchronous — it runs targeting and rollout logic entirely in-memory with zero network overhead.
- Register as a **singleton** so it is initialized once and reused across all requests. Instantiating per request defeats the purpose and causes unnecessary network traffic.
- The server subscribes to SSE updates and keeps its flag definitions current automatically.

See [Client vs. Server SDKs](../core-concepts/client-vs-server-sdks) for guidance on when to use each approach.

## Full Example

```csharp
using Togglerino.Sdk;

await using var client = new TogglerioClient(new TogglerioOptions
{
    ServerUrl = "https://flags.example.com",
    SdkKey = "sdk_your_key_here",
    Context = new EvaluationContext
    {
        UserId = "user-123",
        Attributes = new Dictionary<string, object?>
        {
            ["plan"] = "pro",
            ["country"] = "US",
        },
    },
});

// Subscribe to events before initializing
client.FlagChanges.Subscribe(e =>
    Console.WriteLine($"Flag {e.FlagKey} changed to {e.Value}"));

client.Errors.Subscribe(ex =>
    Console.Error.WriteLine($"Error: {ex.Message}"));

await client.InitializeAsync();

// Evaluate flags
if (client.GetBool("new-checkout-flow"))
{
    Console.WriteLine("New checkout enabled!");
}

var welcomeMessage = client.GetString("welcome-message", "Hello!");
Console.WriteLine(welcomeMessage);

var maxItems = client.GetNumber("max-items", 10);
Console.WriteLine($"Max items: {maxItems}");

// Update context for a different user
await client.UpdateContextAsync(new EvaluationContext { UserId = "user-456" });

// Keep running to receive SSE updates
Console.WriteLine("Press Enter to exit...");
Console.ReadLine();
```
