using System.Runtime.CompilerServices;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using Polly;
using Polly.Retry;

namespace Togglerino.Sdk.Internal;

internal readonly record struct SseParsedEvent(string EventType, string FlagKey, JsonElement Value, string Variant);

internal static class SseParser
{
    public static SseParsedEvent? ParseEvent(string[] lines)
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
            var variant = root.TryGetProperty("variant", out var var_) ? var_.GetString() ?? "" : "";

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
