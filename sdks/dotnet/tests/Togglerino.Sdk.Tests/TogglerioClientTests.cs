using System.Net;
using System.Reactive.Linq;
using System.Text.Json;
using Togglerino.Sdk.Tests.Helpers;
using Xunit;

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
        var options = new TogglerioOptions
        {
            ServerUrl = ServerUrl,
            SdkKey = SdkKey,
            Streaming = false,
            Context = new EvaluationContext { UserId = "user-1" },
        };
        using var client = new TogglerioClient(options, httpClient: new HttpClient(CreateSuccessHandler()));
        await client.InitializeAsync();

        var ctx = client.GetContext();
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
