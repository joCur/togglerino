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
