# .NET SDK Design

## Overview

A .NET 8+ client SDK for Togglerino feature flags. Follows the same architecture as the JavaScript SDK — fetches flags on initialization, maintains an in-memory cache, and keeps flags updated via SSE streaming (default) or polling fallback.

## Package

- **NuGet package:** `Togglerino.Sdk`
- **Namespace:** `Togglerino.Sdk`
- **Directory:** `sdks/dotnet/`
- **Target:** .NET 8+

## Dependencies

- `System.Reactive` — `IObservable<T>` for flag change/deletion/error streams
- `Polly` — retry with exponential backoff and circuit-breaker for HTTP + SSE reconnection
- `Microsoft.Extensions.Logging.Abstractions` — `ILogger<T>` for structured logging

Zero other external dependencies. Serialization via `System.Text.Json` (BCL).

## Project Structure

```
sdks/dotnet/
├── Togglerino.Sdk.sln
├── src/
│   └── Togglerino.Sdk/
│       ├── Togglerino.Sdk.csproj
│       ├── TogglerioClient.cs
│       ├── TogglerioOptions.cs
│       ├── Models/
│       │   ├── EvaluationContext.cs
│       │   ├── EvaluationResult.cs
│       │   ├── FlagChangeEvent.cs
│       │   └── FlagDeletedEvent.cs
│       └── Internal/
│           ├── SseClient.cs
│           ├── FlagApiClient.cs
│           └── FlagStore.cs
└── tests/
    └── Togglerino.Sdk.Tests/
        ├── Togglerino.Sdk.Tests.csproj
        ├── TogglerioClientTests.cs
        ├── SseClientTests.cs
        └── FlagStoreTests.cs
```

## Public API

### Configuration

```csharp
public record TogglerioOptions
{
    public required string ServerUrl { get; init; }
    public required string SdkKey { get; init; }
    public EvaluationContext? Context { get; init; }
    public bool Streaming { get; init; } = true;
    public TimeSpan PollingInterval { get; init; } = TimeSpan.FromSeconds(30);
}
```

### Models

```csharp
public record EvaluationContext
{
    public string? UserId { get; init; }
    public Dictionary<string, object?>? Attributes { get; init; }
}

public record EvaluationResult
{
    public object? Value { get; init; }
    public string Variant { get; init; }
    public string Reason { get; init; }
}

public record FlagChangeEvent(string FlagKey, object? Value, string Variant);
public record FlagDeletedEvent(string FlagKey);
```

### TogglerioClient

```csharp
public class TogglerioClient : IAsyncDisposable, IDisposable
{
    public TogglerioClient(
        TogglerioOptions options,
        ILogger<TogglerioClient>? logger = null,
        HttpClient? httpClient = null);

    // Lifecycle
    public Task InitializeAsync(CancellationToken cancellationToken = default);

    // Typed getters (sync, read from in-memory cache)
    public bool GetBool(string key, bool defaultValue = false);
    public string GetString(string key, string defaultValue = "");
    public double GetNumber(string key, double defaultValue = 0);
    public T? GetJson<T>(string key, T? defaultValue = default);
    public EvaluationResult? GetDetail(string key);

    // Context
    public EvaluationContext GetContext();
    public Task UpdateContextAsync(EvaluationContext context, CancellationToken cancellationToken = default);

    // Reactive streams
    public IObservable<FlagChangeEvent> FlagChanges { get; }
    public IObservable<FlagDeletedEvent> FlagDeletions { get; }
    public IObservable<Exception> Errors { get; }

    // Dispose
    public ValueTask DisposeAsync();
    public void Dispose();
}
```

## Internal Components

### FlagApiClient

Thin wrapper over `HttpClient`. Sends `POST /api/v1/evaluate` with `Authorization: Bearer {sdkKey}` header and JSON body containing the evaluation context. Deserializes the response into `Dictionary<string, EvaluationResult>`.

Polly `ResiliencePipeline` wraps the call with retry (exponential backoff, 3 attempts) and timeout.

### FlagStore

`ConcurrentDictionary<string, EvaluationResult>` for lock-free reads from sync getters.

Exposes `IObservable<FlagChangeEvent>` and `IObservable<FlagDeletedEvent>` backed by Rx `Subject<T>`. On `ReplaceAll()`, diffs old vs new values — emits `FlagChangeEvent` only for flags whose values actually changed, and `FlagDeletedEvent` for flags that were removed. No events on initial population (matches JS SDK behavior).

### SseClient

Reads from `HttpClient.GetStreamAsync()` + `StreamReader.ReadLineAsync()`. Manual SSE line parser:

- Lines starting with `:` — ignored (keepalive)
- `event: flag_update` / `event: flag_deleted` — sets event type
- `data: {...}` — deserializes JSON, calls `FlagStore.ApplyUpdate` or `ApplyDeletion`
- Blank line — end of event, resets parser state

Reconnection via Polly `ResiliencePipeline` with exponential backoff (1s → 2s → 4s → ... → 30s cap), matching JS SDK behavior. During reconnection, falls back to polling temporarily. On successful reconnect, stops polling fallback.

### Polling Fallback

`PeriodicTimer` calls `FlagApiClient.FetchAllAsync()` → `FlagStore.ReplaceAll()` on each tick. Activated when `Streaming = false` or as temporary fallback during SSE reconnection.

## Data Flow

```
InitializeAsync()
  → FlagApiClient.FetchAllAsync()     POST /api/v1/evaluate
  → FlagStore.ReplaceAll(flags)       populates ConcurrentDictionary
  → if Streaming: SseClient.ConnectAsync()
    else: start PeriodicTimer

SSE event received:
  → SseClient parses event
  → FlagStore.ApplyUpdate() or ApplyDeletion()
  → Subject.OnNext() → IObservable subscribers notified

GetBool("my-flag"):
  → FlagStore.Get("my-flag")
  → ConcurrentDictionary lookup (lock-free)
  → cast + return, or defaultValue
```

## JSON Serialization

Global `JsonSerializerOptions` with `JsonNamingPolicy.SnakeCaseLower` to match the server API's snake_case convention. `DefaultIgnoreCondition.WhenWritingNull` to omit null fields.

`GetJson<T>` deserializes the stored `JsonElement` value into the caller's type.

## Error Handling

| Method | On error |
|--------|----------|
| `InitializeAsync` | Throws (can't operate without initial flags) |
| `UpdateContextAsync` | Throws (explicit user action) |
| `GetBool` / `GetString` / etc. | Returns `defaultValue`, pushes to `Errors` observable |
| SSE connection failure | Pushes to `Errors`, Polly retry + polling fallback |
| Polling fetch failure | Pushes to `Errors`, retries on next interval |
| `DisposeAsync` | Never throws |

## Disposal

`DisposeAsync` cancels the SSE connection (via `CancellationTokenSource`), stops the polling timer, completes all `Subject<T>` instances (completes subscribers' observable streams), and disposes the `HttpClient` only if it was internally created (not if injected).

## Testing

- **Framework:** xUnit + NSubstitute
- **HTTP mocking:** Custom `HttpMessageHandler` — no real network calls
- **TogglerioClientTests:** Public API behavior, initialization, getters, context updates, disposal idempotency
- **FlagStoreTests:** Cache diff logic, change/deletion event emission, thread safety
- **SseClientTests:** SSE line parsing against raw text (flag_update, flag_deleted, keepalives, malformed events)
- **FlagApiClientTests:** Request format, response deserialization, HTTP error handling
