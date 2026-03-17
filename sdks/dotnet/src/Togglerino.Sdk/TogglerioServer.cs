using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Logging.Abstractions;
using Polly;
using Polly.Retry;
using Togglerino.Sdk.Internal;

namespace Togglerino.Sdk;

/// <summary>
/// Server-side SDK client that caches flag definitions locally and evaluates
/// flags per-request without network calls. Definitions are kept up to date
/// via SSE streaming or polling.
/// </summary>
public sealed class TogglerioServer : IAsyncDisposable, IDisposable
{
    private readonly TogglerioServerOptions _options;
    private readonly HttpClient _httpClient;
    private readonly bool _ownsHttpClient;
    private readonly ILogger<TogglerioServer> _logger;
    private readonly DefinitionStore _store;
    private readonly string _baseUrl;

    private CancellationTokenSource? _streamCts;
    private Task? _streamTask;
    private PeriodicTimer? _pollTimer;
    private CancellationTokenSource? _pollCts;
    private Task? _pollTask;
    private bool _disposed;

    public TogglerioServer(
        TogglerioServerOptions options,
        ILogger<TogglerioServer>? logger = null,
        HttpClient? httpClient = null)
    {
        _options = options ?? throw new ArgumentNullException(nameof(options));
        _logger = logger ?? NullLogger<TogglerioServer>.Instance;

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

        _store = new DefinitionStore();
        _baseUrl = options.ServerUrl.TrimEnd('/');
    }

    /// <summary>
    /// Fetches initial definitions and starts SSE streaming or polling to keep them current.
    /// </summary>
    public async Task InitializeAsync(CancellationToken cancellationToken = default)
    {
        _logger.LogInformation("Initializing Togglerino server-side client");

        await FetchDefinitionsAsync(cancellationToken);

        if (_options.Streaming)
        {
            StartStreaming();
        }
        else
        {
            StartPolling();
        }
    }

    /// <summary>
    /// Evaluates all cached flag definitions for the given context and returns
    /// a <see cref="FlagEvaluator"/> for reading individual flag values.
    /// </summary>
    public FlagEvaluator Evaluate(EvaluationContext? context = null)
    {
        var flags = _store.GetFlags();
        var segments = _store.GetSegments();
        var ctx = context ?? new EvaluationContext();

        var results = new Dictionary<string, EvaluationResult>(flags.Count);
        foreach (var flag in flags)
        {
            results[flag.Key] = EvaluationEngine.Evaluate(flag, ctx, segments);
        }

        return new FlagEvaluator(results);
    }

    /// <summary>
    /// Fetches flag and segment definitions from the definitions API.
    /// </summary>
    private async Task FetchDefinitionsAsync(CancellationToken cancellationToken)
    {
        var url = $"{_baseUrl}/api/v1/definitions";

        using var request = new HttpRequestMessage(HttpMethod.Get, url);
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", _options.SdkKey);

        using var response = await _httpClient.SendAsync(request, cancellationToken);
        response.EnsureSuccessStatusCode();

        var definitions = await response.Content.ReadFromJsonAsync<DefinitionsResponse>(cancellationToken: cancellationToken);

        if (definitions is not null)
        {
            _store.Update(definitions.Flags, definitions.Segments);
            _logger.LogInformation("Loaded {FlagCount} flags and {SegmentCount} segments",
                definitions.Flags.Count, definitions.Segments.Count);
        }
    }

    private void StartStreaming()
    {
        _streamCts = new CancellationTokenSource();
        _streamTask = RunSseAsync(_streamCts.Token);
    }

    private async Task RunSseAsync(CancellationToken cancellationToken)
    {
        var pipeline = new ResiliencePipelineBuilder()
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

        await pipeline.ExecuteAsync(async ct =>
        {
            using var request = new HttpRequestMessage(HttpMethod.Get, $"{_baseUrl}/api/v1/stream");
            request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", _options.SdkKey);

            using var response = await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, ct);
            response.EnsureSuccessStatusCode();

            using var stream = await response.Content.ReadAsStreamAsync(ct);
            using var reader = new StreamReader(stream);

            await foreach (var evt in SseParser.ReadEventsAsync(reader, ct))
            {
                if (evt.EventType is "flag_update" or "flag_deleted")
                {
                    try
                    {
                        await FetchDefinitionsAsync(ct);
                    }
                    catch (Exception ex)
                    {
                        _logger.LogWarning(ex, "Failed to re-fetch definitions after SSE {EventType} event for flag {FlagKey}",
                            evt.EventType, evt.FlagKey);
                    }
                }
            }

            // Stream ended without error — reconnect
            throw new InvalidOperationException("SSE stream ended unexpectedly");
        }, cancellationToken);
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
                await FetchDefinitionsAsync(cancellationToken);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogError(ex, "Polling failed");
            }
        }
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;

        _streamCts?.Cancel();
        _streamCts?.Dispose();
        _pollCts?.Cancel();
        _pollTimer?.Dispose();
        _store.Dispose();
        if (_ownsHttpClient) _httpClient.Dispose();
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;

        _streamCts?.Cancel();
        _streamCts?.Dispose();
        _pollCts?.Cancel();
        _pollTimer?.Dispose();
        _store.Dispose();
        if (_ownsHttpClient) _httpClient.Dispose();

        if (_streamTask is not null)
        {
            try { await _streamTask; } catch { /* expected on cancellation */ }
        }

        if (_pollTask is not null)
        {
            try { await _pollTask; } catch { /* expected on cancellation */ }
        }
    }

    /// <summary>
    /// Provides typed accessors for flag evaluation results.
    /// Returned by <see cref="TogglerioServer.Evaluate"/> with pre-computed results for all flags.
    /// </summary>
    public sealed class FlagEvaluator
    {
        private readonly Dictionary<string, EvaluationResult> _results;

        internal FlagEvaluator(Dictionary<string, EvaluationResult> results)
        {
            _results = results;
        }

        public bool GetBool(string key, bool defaultValue = false)
        {
            if (!_results.TryGetValue(key, out var result)) return defaultValue;

            try
            {
                return result.Value.GetBoolean();
            }
            catch
            {
                return defaultValue;
            }
        }

        public string GetString(string key, string defaultValue = "")
        {
            if (!_results.TryGetValue(key, out var result)) return defaultValue;

            try
            {
                return result.Value.GetString() ?? defaultValue;
            }
            catch
            {
                return defaultValue;
            }
        }

        public double GetNumber(string key, double defaultValue = 0)
        {
            if (!_results.TryGetValue(key, out var result)) return defaultValue;

            try
            {
                return result.Value.GetDouble();
            }
            catch
            {
                return defaultValue;
            }
        }

        public T? GetJson<T>(string key, T? defaultValue = default)
        {
            if (!_results.TryGetValue(key, out var result)) return defaultValue;

            try
            {
                return JsonSerializer.Deserialize<T>(result.Value.GetRawText());
            }
            catch
            {
                return defaultValue;
            }
        }

        public EvaluationResult? GetDetail(string key)
        {
            return _results.GetValueOrDefault(key);
        }
    }
}
