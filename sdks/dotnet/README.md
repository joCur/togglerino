# Togglerino .NET SDK

Official .NET SDK for [Togglerino](https://github.com/joCur/togglerino) feature flag management.

## Installation

```bash
dotnet add package Togglerino.Sdk
```

## Quick Start

```csharp
using Togglerino.Sdk;

var options = new TogglerioOptions
{
    ServerUrl = "https://your-togglerino-instance.com",
    SdkKey = "your-sdk-key",
    Context = new EvaluationContext
    {
        UserId = "user-123",
        Properties = new Dictionary<string, string>
        {
            ["plan"] = "pro",
            ["country"] = "US"
        }
    }
};

await using var client = new TogglerioClient(options);
await client.InitializeAsync();

bool showFeature = client.GetBool("new-dashboard", defaultValue: false);
string theme = client.GetString("theme", defaultValue: "light");
double rate = client.GetNumber("rate-limit", defaultValue: 100);
```

## Features

- **Real-time updates** via SSE streaming (default) or polling fallback
- **Typed accessors** for boolean, string, number, and JSON flags
- **Reactive events** via `IObservable<T>` for flag changes and deletions
- **Evaluation context** with user targeting and custom properties
- **Resilient** with Polly-based retry and reconnection
- **Microsoft.Extensions.Logging** integration
- **.NET 8+** with full nullable annotation support

## Configuration

```csharp
var options = new TogglerioOptions
{
    ServerUrl = "https://your-instance.com",  // Required
    SdkKey = "your-sdk-key",                  // Required
    Streaming = true,                          // Default: true (SSE)
    PollingInterval = TimeSpan.FromSeconds(30) // Default: 30s (when Streaming = false)
};
```

## Subscribing to Changes

```csharp
client.FlagChanges.Subscribe(change =>
    Console.WriteLine($"Flag '{change.Key}' changed to {change.NewValue}"));

client.FlagDeletions.Subscribe(deletion =>
    Console.WriteLine($"Flag '{deletion.Key}' was deleted"));

client.Errors.Subscribe(error =>
    Console.WriteLine($"Error: {error.Message}"));
```

## Updating Context

```csharp
await client.UpdateContextAsync(new EvaluationContext
{
    UserId = "user-456",
    Properties = new Dictionary<string, string> { ["plan"] = "enterprise" }
});
```

## License

MIT
