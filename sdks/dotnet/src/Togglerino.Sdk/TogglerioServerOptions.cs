namespace Togglerino.Sdk;

public class TogglerioServerOptions
{
    public required string ServerUrl { get; init; }
    public required string SdkKey { get; init; }
    public bool Streaming { get; init; } = true;
    public TimeSpan PollingInterval { get; init; } = TimeSpan.FromSeconds(30);
}
