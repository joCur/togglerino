using System.Text;
using System.Text.Json;
using Xunit;

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
        var sseText = ": connected\n\nevent: flag_update\ndata: {\"flagKey\":\"dark-mode\",\"value\":true,\"variant\":\"on\"}\n\nevent: flag_deleted\ndata: {\"flagKey\":\"old-flag\"}\n\n";

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
