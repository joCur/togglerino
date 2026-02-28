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
