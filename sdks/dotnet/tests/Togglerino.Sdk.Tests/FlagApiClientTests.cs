using System.Net;
using System.Text.Json;
using Togglerino.Sdk.Tests.Helpers;
using Xunit;

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
