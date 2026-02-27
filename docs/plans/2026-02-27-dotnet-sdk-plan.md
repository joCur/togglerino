# .NET SDK Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a .NET 8+ client SDK for Togglerino feature flags with IObservable-based change streams, Polly resilience, and ILogger support.

**Architecture:** Single `TogglerioClient` class backed by three internal components: `FlagApiClient` (HTTP), `SseClient` (SSE streaming), `FlagStore` (thread-safe cache with Rx subjects). SSE is default with polling fallback. Polly handles retry/reconnection with exponential backoff.

**Tech Stack:** .NET 8, System.Reactive, Polly (v8+), Microsoft.Extensions.Logging.Abstractions, xUnit, NSubstitute, System.Text.Json

---

### Task 1: Project Scaffolding

**Files:**
- Create: `sdks/dotnet/Togglerino.Sdk.sln`
- Create: `sdks/dotnet/src/Togglerino.Sdk/Togglerino.Sdk.csproj`
- Create: `sdks/dotnet/tests/Togglerino.Sdk.Tests/Togglerino.Sdk.Tests.csproj`

**Step 1: Create the solution and project directories**

Run:
```bash
cd sdks && mkdir -p dotnet/src/Togglerino.Sdk/Models dotnet/src/Togglerino.Sdk/Internal dotnet/tests/Togglerino.Sdk.Tests
```

**Step 2: Create the SDK project file**

Create `sdks/dotnet/src/Togglerino.Sdk/Togglerino.Sdk.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <RootNamespace>Togglerino.Sdk</RootNamespace>
    <PackageId>Togglerino.Sdk</PackageId>
    <Version>0.1.0</Version>
    <Description>Togglerino feature flag SDK for .NET</Description>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.Extensions.Logging.Abstractions" Version="8.0.3" />
    <PackageReference Include="Polly.Core" Version="8.5.2" />
    <PackageReference Include="System.Reactive" Version="6.0.1" />
  </ItemGroup>

</Project>
```

**Step 3: Create the test project file**

Create `sdks/dotnet/tests/Togglerino.Sdk.Tests/Togglerino.Sdk.Tests.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <IsPackable>false</IsPackable>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.12.0" />
    <PackageReference Include="NSubstitute" Version="5.3.0" />
    <PackageReference Include="xunit" Version="2.9.3" />
    <PackageReference Include="xunit.runner.visualstudio" Version="2.8.2" />
  </ItemGroup>

  <ItemGroup>
    <ProjectReference Include="../../src/Togglerino.Sdk/Togglerino.Sdk.csproj" />
  </ItemGroup>

</Project>
```

**Step 4: Create the solution file**

Run:
```bash
cd sdks/dotnet
dotnet new sln --name Togglerino.Sdk
dotnet sln add src/Togglerino.Sdk/Togglerino.Sdk.csproj
dotnet sln add tests/Togglerino.Sdk.Tests/Togglerino.Sdk.Tests.csproj
```

**Step 5: Verify the solution builds**

Run: `cd sdks/dotnet && dotnet build`
Expected: Build succeeded with 0 errors.

**Step 6: Verify tests run (empty)**

Run: `cd sdks/dotnet && dotnet test`
Expected: 0 tests discovered, passed.

**Step 7: Commit**

```bash
git add sdks/dotnet/
git commit -m "feat(dotnet-sdk): scaffold solution with src and test projects"
```

---

### Task 2: Model Types

**Files:**
- Create: `sdks/dotnet/src/Togglerino.Sdk/TogglerioOptions.cs`
- Create: `sdks/dotnet/src/Togglerino.Sdk/Models/EvaluationContext.cs`
- Create: `sdks/dotnet/src/Togglerino.Sdk/Models/EvaluationResult.cs`
- Create: `sdks/dotnet/src/Togglerino.Sdk/Models/FlagChangeEvent.cs`
- Create: `sdks/dotnet/src/Togglerino.Sdk/Models/FlagDeletedEvent.cs`

**Step 1: Create TogglerioOptions**

Create `sdks/dotnet/src/Togglerino.Sdk/TogglerioOptions.cs`:

```csharp
namespace Togglerino.Sdk;

public record TogglerioOptions
{
    public required string ServerUrl { get; init; }
    public required string SdkKey { get; init; }
    public EvaluationContext? Context { get; init; }
    public bool Streaming { get; init; } = true;
    public TimeSpan PollingInterval { get; init; } = TimeSpan.FromSeconds(30);
}
```

**Step 2: Create EvaluationContext**

Create `sdks/dotnet/src/Togglerino.Sdk/Models/EvaluationContext.cs`:

```csharp
using System.Text.Json.Serialization;

namespace Togglerino.Sdk;

public record EvaluationContext
{
    [JsonPropertyName("user_id")]
    public string? UserId { get; init; }

    [JsonPropertyName("attributes")]
    public Dictionary<string, object?>? Attributes { get; init; }
}
```

**Step 3: Create EvaluationResult**

Create `sdks/dotnet/src/Togglerino.Sdk/Models/EvaluationResult.cs`:

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Togglerino.Sdk;

public record EvaluationResult
{
    [JsonPropertyName("value")]
    public JsonElement Value { get; init; }

    [JsonPropertyName("variant")]
    public string Variant { get; init; } = "";

    [JsonPropertyName("reason")]
    public string Reason { get; init; } = "";
}
```

Note: `Value` is `JsonElement` rather than `object?` — this avoids boxing issues and allows `GetJson<T>` to deserialize from the raw JSON representation reliably.

**Step 4: Create FlagChangeEvent and FlagDeletedEvent**

Create `sdks/dotnet/src/Togglerino.Sdk/Models/FlagChangeEvent.cs`:

```csharp
using System.Text.Json;

namespace Togglerino.Sdk;

public record FlagChangeEvent(string FlagKey, JsonElement Value, string Variant);
```

Create `sdks/dotnet/src/Togglerino.Sdk/Models/FlagDeletedEvent.cs`:

```csharp
namespace Togglerino.Sdk;

public record FlagDeletedEvent(string FlagKey);
```

**Step 5: Verify it builds**

Run: `cd sdks/dotnet && dotnet build`
Expected: Build succeeded.

**Step 6: Commit**

```bash
git add sdks/dotnet/src/Togglerino.Sdk/
git commit -m "feat(dotnet-sdk): add model types and configuration record"
```

---

### Task 3: FlagStore (TDD)

**Files:**
- Create: `sdks/dotnet/src/Togglerino.Sdk/Internal/FlagStore.cs`
- Create: `sdks/dotnet/tests/Togglerino.Sdk.Tests/FlagStoreTests.cs`

**Step 1: Write the failing tests**

Create `sdks/dotnet/tests/Togglerino.Sdk.Tests/FlagStoreTests.cs`:

```csharp
using System.Reactive.Linq;
using System.Text.Json;

namespace Togglerino.Sdk.Tests;

public class FlagStoreTests
{
    private static EvaluationResult MakeResult(object value, string variant = "v1", string reason = "default")
    {
        var json = JsonSerializer.SerializeToElement(value);
        return new EvaluationResult { Value = json, Variant = variant, Reason = reason };
    }

    [Fact]
    public void Get_ReturnsNull_WhenFlagNotFound()
    {
        var store = new Internal.FlagStore();
        Assert.Null(store.Get("nonexistent"));
    }

    [Fact]
    public void ReplaceAll_PopulatesCache()
    {
        var store = new Internal.FlagStore();
        var flags = new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
            ["flag-b"] = MakeResult("hello"),
        };

        store.ReplaceAll(flags, emitEvents: false);

        Assert.NotNull(store.Get("flag-a"));
        Assert.NotNull(store.Get("flag-b"));
    }

    [Fact]
    public async Task ReplaceAll_EmitsChangeEvents_ForChangedFlags()
    {
        var store = new Internal.FlagStore();
        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true, "on"),
        }, emitEvents: false);

        var changes = new List<FlagChangeEvent>();
        using var sub = store.Changes.Subscribe(changes.Add);

        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(false, "off"),
        }, emitEvents: true);

        Assert.Single(changes);
        Assert.Equal("flag-a", changes[0].FlagKey);
        Assert.Equal("off", changes[0].Variant);
    }

    [Fact]
    public async Task ReplaceAll_EmitsDeletedEvents_ForRemovedFlags()
    {
        var store = new Internal.FlagStore();
        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
            ["flag-b"] = MakeResult("hello"),
        }, emitEvents: false);

        var deletions = new List<FlagDeletedEvent>();
        using var sub = store.Deletions.Subscribe(deletions.Add);

        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
        }, emitEvents: true);

        Assert.Single(deletions);
        Assert.Equal("flag-b", deletions[0].FlagKey);
    }

    [Fact]
    public void ReplaceAll_NoEvents_WhenValuesUnchanged()
    {
        var store = new Internal.FlagStore();
        var flags = new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true, "on"),
        };
        store.ReplaceAll(flags, emitEvents: false);

        var changes = new List<FlagChangeEvent>();
        var deletions = new List<FlagDeletedEvent>();
        using var sub1 = store.Changes.Subscribe(changes.Add);
        using var sub2 = store.Deletions.Subscribe(deletions.Add);

        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true, "on"),
        }, emitEvents: true);

        Assert.Empty(changes);
        Assert.Empty(deletions);
    }

    [Fact]
    public void ApplyUpdate_UpdatesCacheAndEmitsChange()
    {
        var store = new Internal.FlagStore();
        var changes = new List<FlagChangeEvent>();
        using var sub = store.Changes.Subscribe(changes.Add);

        store.ApplyUpdate("flag-a", MakeResult(42, "answer"));

        Assert.NotNull(store.Get("flag-a"));
        Assert.Single(changes);
        Assert.Equal("flag-a", changes[0].FlagKey);
    }

    [Fact]
    public void ApplyDeletion_RemovesFromCacheAndEmitsDeleted()
    {
        var store = new Internal.FlagStore();
        store.ReplaceAll(new Dictionary<string, EvaluationResult>
        {
            ["flag-a"] = MakeResult(true),
        }, emitEvents: false);

        var deletions = new List<FlagDeletedEvent>();
        using var sub = store.Deletions.Subscribe(deletions.Add);

        store.ApplyDeletion("flag-a");

        Assert.Null(store.Get("flag-a"));
        Assert.Single(deletions);
    }
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/dotnet && dotnet test`
Expected: FAIL — `Internal.FlagStore` does not exist.

**Step 3: Implement FlagStore**

Create `sdks/dotnet/src/Togglerino.Sdk/Internal/FlagStore.cs`:

```csharp
using System.Collections.Concurrent;
using System.Reactive.Subjects;
using System.Text.Json;

namespace Togglerino.Sdk.Internal;

internal sealed class FlagStore : IDisposable
{
    private readonly ConcurrentDictionary<string, EvaluationResult> _flags = new();
    private readonly Subject<FlagChangeEvent> _changes = new();
    private readonly Subject<FlagDeletedEvent> _deletions = new();

    public IObservable<FlagChangeEvent> Changes => _changes;
    public IObservable<FlagDeletedEvent> Deletions => _deletions;

    public EvaluationResult? Get(string key)
    {
        return _flags.GetValueOrDefault(key);
    }

    public void ReplaceAll(Dictionary<string, EvaluationResult> newFlags, bool emitEvents)
    {
        if (emitEvents)
        {
            // Detect deleted flags
            foreach (var key in _flags.Keys)
            {
                if (!newFlags.ContainsKey(key))
                {
                    _flags.TryRemove(key, out _);
                    _deletions.OnNext(new FlagDeletedEvent(key));
                }
            }

            // Detect new or changed flags
            foreach (var (key, newResult) in newFlags)
            {
                if (_flags.TryGetValue(key, out var oldResult))
                {
                    if (!ValuesEqual(oldResult, newResult))
                    {
                        _flags[key] = newResult;
                        _changes.OnNext(new FlagChangeEvent(key, newResult.Value, newResult.Variant));
                    }
                }
                else
                {
                    _flags[key] = newResult;
                    _changes.OnNext(new FlagChangeEvent(key, newResult.Value, newResult.Variant));
                }
            }
        }
        else
        {
            _flags.Clear();
            foreach (var (key, result) in newFlags)
            {
                _flags[key] = result;
            }
        }
    }

    public void ApplyUpdate(string flagKey, EvaluationResult result)
    {
        _flags[flagKey] = result;
        _changes.OnNext(new FlagChangeEvent(flagKey, result.Value, result.Variant));
    }

    public void ApplyDeletion(string flagKey)
    {
        if (_flags.TryRemove(flagKey, out _))
        {
            _deletions.OnNext(new FlagDeletedEvent(flagKey));
        }
    }

    public void Dispose()
    {
        _changes.OnCompleted();
        _changes.Dispose();
        _deletions.OnCompleted();
        _deletions.Dispose();
    }

    private static bool ValuesEqual(EvaluationResult a, EvaluationResult b)
    {
        return a.Variant == b.Variant
            && a.Reason == b.Reason
            && a.Value.GetRawText() == b.Value.GetRawText();
    }
}
```

**Step 4: Run tests to verify they pass**

Run: `cd sdks/dotnet && dotnet test --filter FlagStoreTests`
Expected: All 7 tests PASS.

**Step 5: Commit**

```bash
git add sdks/dotnet/
git commit -m "feat(dotnet-sdk): add FlagStore with thread-safe cache and Rx events"
```

---

### Task 4: FlagApiClient (TDD)

**Files:**
- Create: `sdks/dotnet/src/Togglerino.Sdk/Internal/FlagApiClient.cs`
- Create: `sdks/dotnet/tests/Togglerino.Sdk.Tests/FlagApiClientTests.cs`
- Create: `sdks/dotnet/tests/Togglerino.Sdk.Tests/Helpers/MockHttpHandler.cs`

**Step 1: Create the mock HTTP handler helper**

Create `sdks/dotnet/tests/Togglerino.Sdk.Tests/Helpers/MockHttpHandler.cs`:

```csharp
namespace Togglerino.Sdk.Tests.Helpers;

internal class MockHttpHandler : HttpMessageHandler
{
    private readonly Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> _handler;

    public MockHttpHandler(Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> handler)
    {
        _handler = handler;
    }

    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        return _handler(request, cancellationToken);
    }
}
```

**Step 2: Write the failing tests**

Create `sdks/dotnet/tests/Togglerino.Sdk.Tests/FlagApiClientTests.cs`:

```csharp
using System.Net;
using System.Text.Json;
using Togglerino.Sdk.Tests.Helpers;

namespace Togglerino.Sdk.Tests;

public class FlagApiClientTests
{
    private const string ServerUrl = "http://localhost:8080";
    private const string SdkKey = "test-sdk-key";

    [Fact]
    public async Task FetchAllAsync_SendsCorrectRequest()
    {
        HttpRequestMessage? capturedRequest = null;

        var handler = new MockHttpHandler((req, _) =>
        {
            capturedRequest = req;
            var response = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent("""{"flags":{}}""", System.Text.Encoding.UTF8, "application/json"),
            };
            return Task.FromResult(response);
        });

        var httpClient = new HttpClient(handler);
        var client = new Internal.FlagApiClient(httpClient, ServerUrl, SdkKey);

        var context = new EvaluationContext
        {
            UserId = "user-123",
            Attributes = new Dictionary<string, object?> { ["plan"] = "pro" },
        };

        await client.FetchAllAsync(context, CancellationToken.None);

        Assert.NotNull(capturedRequest);
        Assert.Equal(HttpMethod.Post, capturedRequest!.Method);
        Assert.Equal($"{ServerUrl}/api/v1/evaluate", capturedRequest.RequestUri!.ToString());
        Assert.Equal($"Bearer {SdkKey}", capturedRequest.Headers.Authorization!.ToString());
    }

    [Fact]
    public async Task FetchAllAsync_DeserializesResponse()
    {
        var responseJson = """
        {
            "flags": {
                "dark-mode": { "value": true, "variant": "on", "reason": "default" },
                "max-uploads": { "value": 10, "variant": "ten", "reason": "rule_match" }
            }
        }
        """;

        var handler = new MockHttpHandler((_, _) =>
        {
            var response = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(responseJson, System.Text.Encoding.UTF8, "application/json"),
            };
            return Task.FromResult(response);
        });

        var httpClient = new HttpClient(handler);
        var client = new Internal.FlagApiClient(httpClient, ServerUrl, SdkKey);
        var flags = await client.FetchAllAsync(null, CancellationToken.None);

        Assert.Equal(2, flags.Count);
        Assert.True(flags.ContainsKey("dark-mode"));
        Assert.True(flags.ContainsKey("max-uploads"));
        Assert.Equal("on", flags["dark-mode"].Variant);
        Assert.Equal("rule_match", flags["max-uploads"].Reason);
    }

    [Fact]
    public async Task FetchAllAsync_ThrowsOnHttpError()
    {
        var handler = new MockHttpHandler((_, _) =>
        {
            var response = new HttpResponseMessage(HttpStatusCode.Unauthorized);
            return Task.FromResult(response);
        });

        var httpClient = new HttpClient(handler);
        var client = new Internal.FlagApiClient(httpClient, ServerUrl, SdkKey);

        await Assert.ThrowsAsync<HttpRequestException>(
            () => client.FetchAllAsync(null, CancellationToken.None));
    }

    [Fact]
    public async Task FetchAllAsync_SendsNullContextAsEmptyObject()
    {
        string? capturedBody = null;

        var handler = new MockHttpHandler(async (req, _) =>
        {
            capturedBody = await req.Content!.ReadAsStringAsync();
            return new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent("""{"flags":{}}""", System.Text.Encoding.UTF8, "application/json"),
            };
        });

        var httpClient = new HttpClient(handler);
        var client = new Internal.FlagApiClient(httpClient, ServerUrl, SdkKey);

        await client.FetchAllAsync(null, CancellationToken.None);

        Assert.NotNull(capturedBody);
        var doc = JsonDocument.Parse(capturedBody!);
        Assert.True(doc.RootElement.TryGetProperty("context", out _));
    }
}
```

**Step 3: Run tests to verify they fail**

Run: `cd sdks/dotnet && dotnet test --filter FlagApiClientTests`
Expected: FAIL — `Internal.FlagApiClient` does not exist.

**Step 4: Implement FlagApiClient**

Create `sdks/dotnet/src/Togglerino.Sdk/Internal/FlagApiClient.cs`:

```csharp
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Togglerino.Sdk.Internal;

internal sealed class FlagApiClient
{
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
    };

    private readonly HttpClient _httpClient;
    private readonly string _baseUrl;
    private readonly string _sdkKey;

    public FlagApiClient(HttpClient httpClient, string serverUrl, string sdkKey)
    {
        _httpClient = httpClient;
        _baseUrl = serverUrl.TrimEnd('/');
        _sdkKey = sdkKey;
    }

    public async Task<Dictionary<string, EvaluationResult>> FetchAllAsync(
        EvaluationContext? context,
        CancellationToken cancellationToken)
    {
        var url = $"{_baseUrl}/api/v1/evaluate";
        var body = new EvaluateRequest { Context = context ?? new EvaluationContext() };

        using var request = new HttpRequestMessage(HttpMethod.Post, url);
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", _sdkKey);
        request.Content = JsonContent.Create(body, options: JsonOptions);

        using var response = await _httpClient.SendAsync(request, cancellationToken);
        response.EnsureSuccessStatusCode();

        var result = await response.Content.ReadFromJsonAsync<EvaluateResponse>(JsonOptions, cancellationToken);
        return result?.Flags ?? new Dictionary<string, EvaluationResult>();
    }

    private sealed record EvaluateRequest
    {
        [JsonPropertyName("context")]
        public EvaluationContext Context { get; init; } = new();
    }

    private sealed record EvaluateResponse
    {
        [JsonPropertyName("flags")]
        public Dictionary<string, EvaluationResult> Flags { get; init; } = new();
    }
}
```

**Step 5: Run tests to verify they pass**

Run: `cd sdks/dotnet && dotnet test --filter FlagApiClientTests`
Expected: All 4 tests PASS.

**Step 6: Commit**

```bash
git add sdks/dotnet/
git commit -m "feat(dotnet-sdk): add FlagApiClient with HTTP evaluation endpoint"
```

---

### Task 5: SseClient (TDD)

**Files:**
- Create: `sdks/dotnet/src/Togglerino.Sdk/Internal/SseClient.cs`
- Create: `sdks/dotnet/tests/Togglerino.Sdk.Tests/SseClientTests.cs`

**Step 1: Write the failing tests**

Create `sdks/dotnet/tests/Togglerino.Sdk.Tests/SseClientTests.cs`:

```csharp
using System.Text;
using System.Text.Json;

namespace Togglerino.Sdk.Tests;

public class SseClientTests
{
    [Fact]
    public void ParseEvent_FlagUpdate()
    {
        var lines = new[]
        {
            "event: flag_update",
            """data: {"flagKey":"dark-mode","value":true,"variant":"on"}""",
            "",
        };

        var result = Internal.SseParser.ParseEvent(lines);

        Assert.NotNull(result);
        Assert.Equal("flag_update", result!.Value.EventType);
        Assert.Equal("dark-mode", result.Value.FlagKey);
        Assert.Equal("on", result.Value.Variant);
    }

    [Fact]
    public void ParseEvent_FlagDeleted()
    {
        var lines = new[]
        {
            "event: flag_deleted",
            """data: {"flagKey":"dark-mode"}""",
            "",
        };

        var result = Internal.SseParser.ParseEvent(lines);

        Assert.NotNull(result);
        Assert.Equal("flag_deleted", result!.Value.EventType);
        Assert.Equal("dark-mode", result.Value.FlagKey);
    }

    [Fact]
    public void ParseEvent_IgnoresKeepalive()
    {
        var lines = new[] { ": connected", "" };
        var result = Internal.SseParser.ParseEvent(lines);
        Assert.Null(result);
    }

    [Fact]
    public void ParseEvent_IgnoresUnknownEventType()
    {
        var lines = new[]
        {
            "event: unknown_type",
            """data: {"flagKey":"test"}""",
            "",
        };

        var result = Internal.SseParser.ParseEvent(lines);
        Assert.Null(result);
    }

    [Fact]
    public void ParseEvent_HandlesEmptyData()
    {
        var lines = new[]
        {
            "event: flag_update",
            "data: ",
            "",
        };

        var result = Internal.SseParser.ParseEvent(lines);
        Assert.Null(result);
    }

    [Fact]
    public async Task ReadEvents_ParsesStreamOfEvents()
    {
        var sseText = """
                      : connected

                      event: flag_update
                      data: {"flagKey":"dark-mode","value":true,"variant":"on"}

                      event: flag_deleted
                      data: {"flagKey":"old-flag"}


                      """;

        var stream = new MemoryStream(Encoding.UTF8.GetBytes(sseText));
        var reader = new StreamReader(stream);
        var events = new List<Internal.SseParsedEvent>();

        await foreach (var evt in Internal.SseParser.ReadEventsAsync(reader, CancellationToken.None))
        {
            events.Add(evt);
        }

        Assert.Equal(2, events.Count);
        Assert.Equal("flag_update", events[0].EventType);
        Assert.Equal("dark-mode", events[0].FlagKey);
        Assert.Equal("flag_deleted", events[1].EventType);
        Assert.Equal("old-flag", events[1].FlagKey);
    }
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/dotnet && dotnet test --filter SseClientTests`
Expected: FAIL — `Internal.SseParser` does not exist.

**Step 3: Implement SseParser and SseClient**

Create `sdks/dotnet/src/Togglerino.Sdk/Internal/SseClient.cs`:

```csharp
using System.Runtime.CompilerServices;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using Polly;
using Polly.Retry;

namespace Togglerino.Sdk.Internal;

internal readonly record struct SseParsedEvent(string EventType, string FlagKey, JsonElement Value, string Variant);

internal static class SseParser
{
    public static SseParsedEvent? ParseEvent(ReadOnlySpan<string> lines)
    {
        string? eventType = null;
        string? data = null;

        foreach (var line in lines)
        {
            if (string.IsNullOrEmpty(line))
                continue;

            if (line.StartsWith(':'))
                continue;

            if (line.StartsWith("event: "))
                eventType = line["event: ".Length..];
            else if (line.StartsWith("data: "))
                data = line["data: ".Length..];
        }

        if (eventType is not ("flag_update" or "flag_deleted"))
            return null;

        if (string.IsNullOrWhiteSpace(data))
            return null;

        try
        {
            var doc = JsonDocument.Parse(data);
            var root = doc.RootElement;

            var flagKey = root.GetProperty("flagKey").GetString()!;
            var value = root.TryGetProperty("value", out var v) ? v.Clone() : default;
            var variant = root.TryGetProperty("variant", out var var) ? var.GetString() ?? "" : "";

            return new SseParsedEvent(eventType, flagKey, value, variant);
        }
        catch
        {
            return null;
        }
    }

    public static async IAsyncEnumerable<SseParsedEvent> ReadEventsAsync(
        StreamReader reader,
        [EnumeratorCancellation] CancellationToken cancellationToken)
    {
        var buffer = new List<string>();

        while (!cancellationToken.IsCancellationRequested)
        {
            var line = await reader.ReadLineAsync(cancellationToken);
            if (line is null)
                break; // stream ended

            if (line == "")
            {
                // End of event
                if (buffer.Count > 0)
                {
                    var parsed = ParseEvent(buffer.ToArray());
                    buffer.Clear();
                    if (parsed.HasValue)
                        yield return parsed.Value;
                }
            }
            else
            {
                buffer.Add(line);
            }
        }
    }
}

internal sealed class SseClient : IDisposable
{
    private readonly HttpClient _httpClient;
    private readonly string _baseUrl;
    private readonly string _sdkKey;
    private readonly FlagStore _store;
    private readonly ILogger _logger;
    private readonly ResiliencePipeline _resiliencePipeline;
    private CancellationTokenSource? _cts;
    private Task? _readTask;

    public SseClient(
        HttpClient httpClient,
        string serverUrl,
        string sdkKey,
        FlagStore store,
        ILogger logger)
    {
        _httpClient = httpClient;
        _baseUrl = serverUrl.TrimEnd('/');
        _sdkKey = sdkKey;
        _store = store;
        _logger = logger;

        _resiliencePipeline = new ResiliencePipelineBuilder()
            .AddRetry(new RetryStrategyOptions
            {
                MaxRetryAttempts = int.MaxValue,
                DelayGenerator = args =>
                {
                    var delay = TimeSpan.FromSeconds(Math.Min(Math.Pow(2, args.AttemptNumber), 30));
                    return ValueTask.FromResult<TimeSpan?>(delay);
                },
                ShouldHandle = new PredicateBuilder().Handle<Exception>(),
                OnRetry = args =>
                {
                    _logger.LogWarning("SSE reconnecting (attempt {Attempt}, delay {Delay}s)",
                        args.AttemptNumber + 1, args.RetryDelay.TotalSeconds);
                    return ValueTask.CompletedTask;
                },
            })
            .Build();
    }

    public event Action? OnReconnecting;
    public event Action? OnReconnected;

    public void Start()
    {
        _cts = new CancellationTokenSource();
        _readTask = RunAsync(_cts.Token);
    }

    private async Task RunAsync(CancellationToken cancellationToken)
    {
        bool isReconnect = false;

        await _resiliencePipeline.ExecuteAsync(async ct =>
        {
            if (isReconnect)
                OnReconnecting?.Invoke();

            using var request = new HttpRequestMessage(HttpMethod.Get, $"{_baseUrl}/api/v1/stream");
            request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", _sdkKey);

            using var response = await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, ct);
            response.EnsureSuccessStatusCode();

            using var stream = await response.Content.ReadAsStreamAsync(ct);
            using var reader = new StreamReader(stream);

            if (isReconnect)
                OnReconnected?.Invoke();

            isReconnect = true;

            await foreach (var evt in SseParser.ReadEventsAsync(reader, ct))
            {
                if (evt.EventType == "flag_update")
                {
                    var result = new EvaluationResult
                    {
                        Value = evt.Value,
                        Variant = evt.Variant,
                        Reason = "stream",
                    };
                    _store.ApplyUpdate(evt.FlagKey, result);
                }
                else if (evt.EventType == "flag_deleted")
                {
                    _store.ApplyDeletion(evt.FlagKey);
                }
            }

            // Stream ended without error — reconnect
            throw new InvalidOperationException("SSE stream ended unexpectedly");
        }, cancellationToken);
    }

    public void Dispose()
    {
        _cts?.Cancel();
        _cts?.Dispose();
        try { _readTask?.GetAwaiter().GetResult(); } catch { /* expected on cancellation */ }
    }
}
```

**Step 4: Run tests to verify they pass**

Run: `cd sdks/dotnet && dotnet test --filter SseClientTests`
Expected: All 6 tests PASS.

**Step 5: Commit**

```bash
git add sdks/dotnet/
git commit -m "feat(dotnet-sdk): add SSE parser and client with Polly retry"
```

---

### Task 6: TogglerioClient (TDD)

**Files:**
- Create: `sdks/dotnet/src/Togglerino.Sdk/TogglerioClient.cs`
- Create: `sdks/dotnet/tests/Togglerino.Sdk.Tests/TogglerioClientTests.cs`

**Step 1: Write the failing tests**

Create `sdks/dotnet/tests/Togglerino.Sdk.Tests/TogglerioClientTests.cs`:

```csharp
using System.Net;
using System.Reactive.Linq;
using System.Text.Json;
using Togglerino.Sdk.Tests.Helpers;

namespace Togglerino.Sdk.Tests;

public class TogglerioClientTests
{
    private const string ServerUrl = "http://localhost:8080";
    private const string SdkKey = "test-sdk-key";

    private static readonly string EvaluateResponse = """
    {
        "flags": {
            "dark-mode": { "value": true, "variant": "on", "reason": "default" },
            "greeting": { "value": "hello", "variant": "en", "reason": "rule_match" },
            "max-items": { "value": 42, "variant": "high", "reason": "default" },
            "config": { "value": { "theme": "dark" }, "variant": "v1", "reason": "default" }
        }
    }
    """;

    private static TogglerioClient CreateClient(MockHttpHandler handler, bool streaming = false)
    {
        var httpClient = new HttpClient(handler);
        var options = new TogglerioOptions
        {
            ServerUrl = ServerUrl,
            SdkKey = SdkKey,
            Streaming = streaming,
        };
        return new TogglerioClient(options, httpClient: httpClient);
    }

    private static MockHttpHandler CreateSuccessHandler(string json = "")
    {
        return new MockHttpHandler((_, _) =>
        {
            var response = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(
                    string.IsNullOrEmpty(json) ? EvaluateResponse : json,
                    System.Text.Encoding.UTF8,
                    "application/json"),
            };
            return Task.FromResult(response);
        });
    }

    [Fact]
    public async Task InitializeAsync_FetchesFlagsAndMakesThemAvailable()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        Assert.True(client.GetBool("dark-mode"));
        Assert.Equal("hello", client.GetString("greeting"));
        Assert.Equal(42.0, client.GetNumber("max-items"));
    }

    [Fact]
    public async Task GetBool_ReturnsDefaultWhenFlagNotFound()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        Assert.False(client.GetBool("nonexistent"));
        Assert.True(client.GetBool("nonexistent", true));
    }

    [Fact]
    public async Task GetString_ReturnsDefaultWhenFlagNotFound()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        Assert.Equal("", client.GetString("nonexistent"));
        Assert.Equal("fallback", client.GetString("nonexistent", "fallback"));
    }

    [Fact]
    public async Task GetNumber_ReturnsDefaultWhenFlagNotFound()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        Assert.Equal(0, client.GetNumber("nonexistent"));
        Assert.Equal(99, client.GetNumber("nonexistent", 99));
    }

    [Fact]
    public async Task GetJson_DeserializesToType()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        var config = client.GetJson<Dictionary<string, string>>("config");
        Assert.NotNull(config);
        Assert.Equal("dark", config!["theme"]);
    }

    [Fact]
    public async Task GetDetail_ReturnsFullResult()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        var detail = client.GetDetail("dark-mode");
        Assert.NotNull(detail);
        Assert.Equal("on", detail!.Variant);
        Assert.Equal("default", detail.Reason);
    }

    [Fact]
    public async Task GetDetail_ReturnsNullWhenNotFound()
    {
        using var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        Assert.Null(client.GetDetail("nonexistent"));
    }

    [Fact]
    public async Task UpdateContextAsync_RefetchesFlags()
    {
        int callCount = 0;
        var handler = new MockHttpHandler((_, _) =>
        {
            callCount++;
            var json = callCount == 1
                ? """{"flags":{"flag-a":{"value":true,"variant":"on","reason":"default"}}}"""
                : """{"flags":{"flag-a":{"value":false,"variant":"off","reason":"rule_match"}}}""";

            return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(json, System.Text.Encoding.UTF8, "application/json"),
            });
        });

        using var client = CreateClient(handler);
        await client.InitializeAsync();

        Assert.True(client.GetBool("flag-a"));

        await client.UpdateContextAsync(new EvaluationContext { UserId = "user-2" });

        Assert.False(client.GetBool("flag-a"));
    }

    [Fact]
    public async Task GetContext_ReturnsDefensiveCopy()
    {
        using var client = CreateClient(CreateSuccessHandler());
        var options = new TogglerioOptions
        {
            ServerUrl = ServerUrl,
            SdkKey = SdkKey,
            Streaming = false,
            Context = new EvaluationContext { UserId = "user-1" },
        };
        using var clientWithCtx = new TogglerioClient(options, httpClient: new HttpClient(CreateSuccessHandler()));
        await clientWithCtx.InitializeAsync();

        var ctx = clientWithCtx.GetContext();
        Assert.Equal("user-1", ctx.UserId);
    }

    [Fact]
    public async Task Dispose_IsIdempotent()
    {
        var client = CreateClient(CreateSuccessHandler());
        await client.InitializeAsync();

        client.Dispose();
        client.Dispose(); // should not throw
    }

    [Fact]
    public async Task InitializeAsync_ThrowsOnHttpFailure()
    {
        var handler = new MockHttpHandler((_, _) =>
            Task.FromResult(new HttpResponseMessage(HttpStatusCode.InternalServerError)));

        using var client = CreateClient(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => client.InitializeAsync());
    }
}
```

**Step 2: Run tests to verify they fail**

Run: `cd sdks/dotnet && dotnet test --filter TogglerioClientTests`
Expected: FAIL — `TogglerioClient` does not exist.

**Step 3: Implement TogglerioClient**

Create `sdks/dotnet/src/Togglerino.Sdk/TogglerioClient.cs`:

```csharp
using System.Reactive.Subjects;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Togglerino.Sdk.Internal;

namespace Togglerino.Sdk;

public sealed class TogglerioClient : IAsyncDisposable, IDisposable
{
    private readonly TogglerioOptions _options;
    private readonly HttpClient _httpClient;
    private readonly bool _ownsHttpClient;
    private readonly ILogger<TogglerioClient> _logger;
    private readonly FlagStore _store;
    private readonly FlagApiClient _apiClient;
    private readonly Subject<Exception> _errors = new();
    private EvaluationContext _context;

    private SseClient? _sseClient;
    private PeriodicTimer? _pollTimer;
    private CancellationTokenSource? _pollCts;
    private Task? _pollTask;
    private bool _disposed;

    public TogglerioClient(
        TogglerioOptions options,
        ILogger<TogglerioClient>? logger = null,
        HttpClient? httpClient = null)
    {
        _options = options ?? throw new ArgumentNullException(nameof(options));
        _logger = logger ?? NullLogger<TogglerioClient>.Instance;
        _context = options.Context ?? new EvaluationContext();

        if (httpClient is not null)
        {
            _httpClient = httpClient;
            _ownsHttpClient = false;
        }
        else
        {
            _httpClient = new HttpClient();
            _ownsHttpClient = true;
        }

        _store = new FlagStore();
        _apiClient = new FlagApiClient(_httpClient, options.ServerUrl, options.SdkKey);
    }

    public IObservable<FlagChangeEvent> FlagChanges => _store.Changes;
    public IObservable<FlagDeletedEvent> FlagDeletions => _store.Deletions;
    public IObservable<Exception> Errors => _errors;

    public async Task InitializeAsync(CancellationToken cancellationToken = default)
    {
        _logger.LogInformation("Initializing Togglerino client");

        var flags = await _apiClient.FetchAllAsync(_context, cancellationToken);
        _store.ReplaceAll(flags, emitEvents: false);

        _logger.LogInformation("Loaded {Count} flags", flags.Count);

        if (_options.Streaming)
        {
            StartStreaming();
        }
        else
        {
            StartPolling();
        }
    }

    public bool GetBool(string key, bool defaultValue = false)
    {
        var result = _store.Get(key);
        if (result is null) return defaultValue;

        try
        {
            return result.Value.GetBoolean();
        }
        catch
        {
            _errors.OnNext(new InvalidCastException($"Flag '{key}' is not a boolean"));
            return defaultValue;
        }
    }

    public string GetString(string key, string defaultValue = "")
    {
        var result = _store.Get(key);
        if (result is null) return defaultValue;

        try
        {
            return result.Value.GetString() ?? defaultValue;
        }
        catch
        {
            _errors.OnNext(new InvalidCastException($"Flag '{key}' is not a string"));
            return defaultValue;
        }
    }

    public double GetNumber(string key, double defaultValue = 0)
    {
        var result = _store.Get(key);
        if (result is null) return defaultValue;

        try
        {
            return result.Value.GetDouble();
        }
        catch
        {
            _errors.OnNext(new InvalidCastException($"Flag '{key}' is not a number"));
            return defaultValue;
        }
    }

    public T? GetJson<T>(string key, T? defaultValue = default)
    {
        var result = _store.Get(key);
        if (result is null) return defaultValue;

        try
        {
            return JsonSerializer.Deserialize<T>(result.Value.GetRawText());
        }
        catch
        {
            _errors.OnNext(new InvalidCastException($"Flag '{key}' could not be deserialized to {typeof(T).Name}"));
            return defaultValue;
        }
    }

    public EvaluationResult? GetDetail(string key) => _store.Get(key);

    public EvaluationContext GetContext() => _context with { };

    public async Task UpdateContextAsync(EvaluationContext context, CancellationToken cancellationToken = default)
    {
        _context = context ?? throw new ArgumentNullException(nameof(context));

        var flags = await _apiClient.FetchAllAsync(_context, cancellationToken);
        _store.ReplaceAll(flags, emitEvents: true);

        _logger.LogDebug("Context updated, refreshed {Count} flags", flags.Count);
    }

    private void StartStreaming()
    {
        _sseClient = new SseClient(_httpClient, _options.ServerUrl, _options.SdkKey, _store, _logger);
        _sseClient.OnReconnecting += () => _logger.LogWarning("SSE reconnecting");
        _sseClient.OnReconnected += () => _logger.LogInformation("SSE reconnected");
        _sseClient.Start();
    }

    private void StartPolling()
    {
        _pollCts = new CancellationTokenSource();
        _pollTimer = new PeriodicTimer(_options.PollingInterval);
        _pollTask = PollAsync(_pollCts.Token);
    }

    private async Task PollAsync(CancellationToken cancellationToken)
    {
        while (await _pollTimer!.WaitForNextTickAsync(cancellationToken))
        {
            try
            {
                var flags = await _apiClient.FetchAllAsync(_context, cancellationToken);
                _store.ReplaceAll(flags, emitEvents: true);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogError(ex, "Polling failed");
                _errors.OnNext(ex);
            }
        }
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;

        _pollCts?.Cancel();
        _pollTimer?.Dispose();
        _sseClient?.Dispose();
        _store.Dispose();
        _errors.OnCompleted();
        _errors.Dispose();
        if (_ownsHttpClient) _httpClient.Dispose();
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;

        _pollCts?.Cancel();
        _pollTimer?.Dispose();
        _sseClient?.Dispose();
        _store.Dispose();
        _errors.OnCompleted();
        _errors.Dispose();
        if (_ownsHttpClient) _httpClient.Dispose();

        if (_pollTask is not null)
        {
            try { await _pollTask; } catch { /* expected on cancellation */ }
        }
    }
}
```

**Step 4: Run tests to verify they pass**

Run: `cd sdks/dotnet && dotnet test --filter TogglerioClientTests`
Expected: All 11 tests PASS.

**Step 5: Run all tests**

Run: `cd sdks/dotnet && dotnet test`
Expected: All 28 tests PASS (7 FlagStore + 4 FlagApiClient + 6 SseClient + 11 TogglerioClient).

**Step 6: Commit**

```bash
git add sdks/dotnet/
git commit -m "feat(dotnet-sdk): add TogglerioClient with typed getters, Rx observables, SSE and polling"
```

---

### Task 7: CI Integration

**Files:**
- Modify: `.github/workflows/ci.yml`

**Step 1: Add a dotnet-sdk test job to CI**

Add the following job to `.github/workflows/ci.yml` after the existing `test-sdks` job:

```yaml
  test-dotnet-sdk:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-dotnet@v4
        with:
          dotnet-version: '8.0.x'
      - name: Restore
        run: cd sdks/dotnet && dotnet restore
      - name: Build
        run: cd sdks/dotnet && dotnet build --no-restore
      - name: Test
        run: cd sdks/dotnet && dotnet test --no-build --verbosity normal
```

Also add `test-dotnet-sdk` to the `needs` array of the `build` job so it gates the final build.

**Step 2: Verify the workflow YAML is valid**

Run: `cd sdks/dotnet && dotnet test --verbosity normal`
Expected: All tests pass locally, confirming the commands in CI are correct.

**Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add .NET SDK test job to CI pipeline"
```

---

### Task 8: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add .NET SDK section to CLAUDE.md**

Add to the `### SDKs` section under `## Build & Run Commands`:

```markdown
cd sdks/dotnet && dotnet test              # .NET SDK tests (xUnit)
```

Add to the `### Client SDKs` section under `## Architecture`:

```markdown
- `sdks/dotnet/` — `Togglerino.Sdk`: .NET 8+ SDK with IObservable events, Polly resilience, built with dotnet
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add .NET SDK to CLAUDE.md"
```
