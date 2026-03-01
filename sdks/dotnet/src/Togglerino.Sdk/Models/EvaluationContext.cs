using System.Text.Json.Serialization;

namespace Togglerino.Sdk;

public record EvaluationContext
{
    [JsonPropertyName("user_id")]
    public string? UserId { get; init; }

    [JsonPropertyName("attributes")]
    public Dictionary<string, object?>? Attributes { get; init; }
}
