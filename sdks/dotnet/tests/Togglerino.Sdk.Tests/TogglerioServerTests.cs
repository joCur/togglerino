using System.Net;
using System.Text.Json;
using Togglerino.Sdk.Tests.Helpers;
using Xunit;

namespace Togglerino.Sdk.Tests;

public class TogglerioServerTests
{
    private const string ServerUrl = "http://localhost:8080";
    private const string SdkKey = "test-sdk-key";

    private static readonly string DefinitionsResponse = """
    {
        "flags": [
            {
                "key": "dark-mode",
                "valueType": "boolean",
                "status": "active",
                "defaultValue": true,
                "variants": [
                    { "name": "on", "value": true },
                    { "name": "off", "value": false }
                ],
                "config": {
                    "enabled": true,
                    "fallthroughVariant": "on",
                    "offVariant": "off",
                    "targetingRules": []
                }
            },
            {
                "key": "greeting",
                "valueType": "string",
                "status": "active",
                "defaultValue": null,
                "variants": [
                    { "name": "en", "value": "hello" },
                    { "name": "es", "value": "hola" }
                ],
                "config": {
                    "enabled": true,
                    "fallthroughVariant": "en",
                    "offVariant": null,
                    "targetingRules": []
                }
            },
            {
                "key": "max-items",
                "valueType": "number",
                "status": "active",
                "defaultValue": null,
                "variants": [
                    { "name": "high", "value": 42 },
                    { "name": "low", "value": 10 }
                ],
                "config": {
                    "enabled": true,
                    "fallthroughVariant": "high",
                    "offVariant": null,
                    "targetingRules": []
                }
            },
            {
                "key": "config",
                "valueType": "json",
                "status": "active",
                "defaultValue": null,
                "variants": [
                    { "name": "v1", "value": { "theme": "dark" } }
                ],
                "config": {
                    "enabled": true,
                    "fallthroughVariant": "v1",
                    "offVariant": null,
                    "targetingRules": []
                }
            }
        ],
        "segments": []
    }
    """;

    private static TogglerioServer CreateServer(MockHttpHandler handler, bool streaming = false)
    {
        var httpClient = new HttpClient(handler);
        var options = new TogglerioServerOptions
        {
            ServerUrl = ServerUrl,
            SdkKey = SdkKey,
            Streaming = streaming,
            PollingInterval = TimeSpan.FromHours(1), // prevent polling in tests
        };
        return new TogglerioServer(options, httpClient: httpClient);
    }

    private static MockHttpHandler CreateDefinitionsHandler(string json = "")
    {
        return new MockHttpHandler((request, _) =>
        {
            var response = new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(
                    string.IsNullOrEmpty(json) ? DefinitionsResponse : json,
                    System.Text.Encoding.UTF8,
                    "application/json"),
            };
            return Task.FromResult(response);
        });
    }

    [Fact]
    public async Task InitializeAsync_FetchesDefinitions()
    {
        string? capturedUrl = null;
        string? capturedAuth = null;
        var handler = new MockHttpHandler((request, _) =>
        {
            capturedUrl = request.RequestUri?.ToString();
            capturedAuth = request.Headers.Authorization?.ToString();
            return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(DefinitionsResponse, System.Text.Encoding.UTF8, "application/json"),
            });
        });

        await using var server = CreateServer(handler);
        await server.InitializeAsync();

        Assert.Equal($"{ServerUrl}/api/v1/definitions", capturedUrl);
        Assert.Equal($"Bearer {SdkKey}", capturedAuth);
    }

    [Fact]
    public async Task Evaluate_ReturnsEvaluatorWithCorrectBoolValue()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate(new EvaluationContext { UserId = "user-1" });

        Assert.True(evaluator.GetBool("dark-mode"));
    }

    [Fact]
    public async Task Evaluate_ReturnsEvaluatorWithCorrectStringValue()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate(new EvaluationContext { UserId = "user-1" });

        Assert.Equal("hello", evaluator.GetString("greeting"));
    }

    [Fact]
    public async Task Evaluate_ReturnsEvaluatorWithCorrectNumberValue()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate(new EvaluationContext { UserId = "user-1" });

        Assert.Equal(42.0, evaluator.GetNumber("max-items"));
    }

    [Fact]
    public async Task Evaluate_GetJsonDeserializesToType()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate(new EvaluationContext { UserId = "user-1" });

        var config = evaluator.GetJson<Dictionary<string, string>>("config");
        Assert.NotNull(config);
        Assert.Equal("dark", config!["theme"]);
    }

    [Fact]
    public async Task Evaluate_GetDetailReturnsFullResult()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate(new EvaluationContext { UserId = "user-1" });

        var detail = evaluator.GetDetail("greeting");
        Assert.NotNull(detail);
        Assert.Equal("en", detail!.Variant);
        Assert.Equal("default", detail.Reason);
    }

    [Fact]
    public async Task Evaluate_GetDetailReturnsNullWhenNotFound()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate();

        Assert.Null(evaluator.GetDetail("nonexistent"));
    }

    [Fact]
    public async Task Evaluate_ReturnsDefaultWhenFlagNotFound()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate();

        Assert.False(evaluator.GetBool("nonexistent"));
        Assert.True(evaluator.GetBool("nonexistent", true));
        Assert.Equal("", evaluator.GetString("nonexistent"));
        Assert.Equal("fallback", evaluator.GetString("nonexistent", "fallback"));
        Assert.Equal(0, evaluator.GetNumber("nonexistent"));
        Assert.Equal(99, evaluator.GetNumber("nonexistent", 99));
    }

    [Fact]
    public async Task Evaluate_DifferentContextsProduceDifferentResults()
    {
        var json = """
        {
            "flags": [
                {
                    "key": "beta-feature",
                    "valueType": "string",
                    "status": "active",
                    "defaultValue": null,
                    "variants": [
                        { "name": "on", "value": "enabled" },
                        { "name": "off", "value": "disabled" }
                    ],
                    "config": {
                        "enabled": true,
                        "fallthroughVariant": "off",
                        "offVariant": null,
                        "targetingRules": [
                            {
                                "variant": "on",
                                "percentage": null,
                                "conditions": [
                                    {
                                        "attribute": "user_id",
                                        "operator": "equals",
                                        "value": "vip-user"
                                    }
                                ]
                            }
                        ]
                    }
                }
            ],
            "segments": []
        }
        """;

        await using var server = CreateServer(CreateDefinitionsHandler(json));
        await server.InitializeAsync();

        var vipEvaluator = server.Evaluate(new EvaluationContext { UserId = "vip-user" });
        Assert.Equal("enabled", vipEvaluator.GetString("beta-feature"));

        var regularEvaluator = server.Evaluate(new EvaluationContext { UserId = "regular-user" });
        Assert.Equal("disabled", regularEvaluator.GetString("beta-feature"));
    }

    [Fact]
    public async Task Evaluate_WithSegmentMatch()
    {
        var json = """
        {
            "flags": [
                {
                    "key": "premium-feature",
                    "valueType": "boolean",
                    "status": "active",
                    "defaultValue": true,
                    "variants": [
                        { "name": "on", "value": true },
                        { "name": "off", "value": false }
                    ],
                    "config": {
                        "enabled": true,
                        "fallthroughVariant": "on",
                        "offVariant": "off",
                        "targetingRules": [
                            {
                                "variant": "off",
                                "percentage": null,
                                "conditions": [
                                    {
                                        "attribute": "",
                                        "operator": "segment_match",
                                        "value": "free-users"
                                    }
                                ]
                            }
                        ]
                    }
                }
            ],
            "segments": [
                {
                    "key": "free-users",
                    "conditions": [
                        {
                            "attribute": "plan",
                            "operator": "equals",
                            "value": "free"
                        }
                    ]
                }
            ]
        }
        """;

        await using var server = CreateServer(CreateDefinitionsHandler(json));
        await server.InitializeAsync();

        var freeUser = server.Evaluate(new EvaluationContext
        {
            UserId = "user-1",
            Attributes = new Dictionary<string, object?> { ["plan"] = "free" },
        });
        Assert.False(freeUser.GetBool("premium-feature"));

        var premiumUser = server.Evaluate(new EvaluationContext
        {
            UserId = "user-2",
            Attributes = new Dictionary<string, object?> { ["plan"] = "premium" },
        });
        Assert.True(premiumUser.GetBool("premium-feature"));
    }

    [Fact]
    public async Task Evaluate_DisabledFlagReturnsFalseForBoolean()
    {
        var json = """
        {
            "flags": [
                {
                    "key": "disabled-flag",
                    "valueType": "boolean",
                    "status": "active",
                    "defaultValue": true,
                    "variants": [
                        { "name": "on", "value": true },
                        { "name": "off", "value": false }
                    ],
                    "config": {
                        "enabled": false,
                        "fallthroughVariant": "on",
                        "offVariant": "off",
                        "targetingRules": []
                    }
                }
            ],
            "segments": []
        }
        """;

        await using var server = CreateServer(CreateDefinitionsHandler(json));
        await server.InitializeAsync();

        var evaluator = server.Evaluate();
        Assert.False(evaluator.GetBool("disabled-flag"));

        var detail = evaluator.GetDetail("disabled-flag");
        Assert.NotNull(detail);
        Assert.Equal("disabled", detail!.Reason);
    }

    [Fact]
    public async Task InitializeAsync_ThrowsOnHttpFailure()
    {
        var handler = new MockHttpHandler((_, _) =>
            Task.FromResult(new HttpResponseMessage(HttpStatusCode.InternalServerError)));

        await using var server = CreateServer(handler);

        await Assert.ThrowsAsync<HttpRequestException>(() => server.InitializeAsync());
    }

    [Fact]
    public async Task DisposeAsync_IsIdempotent()
    {
        var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        await server.DisposeAsync();
        await server.DisposeAsync(); // should not throw
    }

    [Fact]
    public async Task Dispose_IsIdempotent()
    {
        var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        server.Dispose();
        server.Dispose(); // should not throw
    }

    [Fact]
    public async Task Evaluate_WithNullContextUsesEmptyContext()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate(null);

        Assert.True(evaluator.GetBool("dark-mode"));
        Assert.Equal("hello", evaluator.GetString("greeting"));
    }

    [Fact]
    public async Task GetBool_ReturnsDefaultWhenTypeMismatch()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate();

        // "greeting" is a string, not a boolean
        Assert.False(evaluator.GetBool("greeting"));
        Assert.True(evaluator.GetBool("greeting", true));
    }

    [Fact]
    public async Task GetNumber_ReturnsDefaultWhenTypeMismatch()
    {
        await using var server = CreateServer(CreateDefinitionsHandler());
        await server.InitializeAsync();

        var evaluator = server.Evaluate();

        // "greeting" is a string, not a number
        Assert.Equal(0, evaluator.GetNumber("greeting"));
        Assert.Equal(99.5, evaluator.GetNumber("greeting", 99.5));
    }
}
