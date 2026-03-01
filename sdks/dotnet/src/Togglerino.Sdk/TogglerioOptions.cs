namespace Togglerino.Sdk;

public record TogglerioOptions
{
    public required string ServerUrl { get; init; }
    public required string SdkKey { get; init; }
    public EvaluationContext? Context { get; init; }
    public bool Streaming { get; init; } = true;
    public TimeSpan PollingInterval { get; init; } = TimeSpan.FromSeconds(30);
}
